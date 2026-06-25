package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/timurabdullin/mtprotoui/internal/store"
)

const cookieName = "mtproto_session"

var ErrPasswordChangeRequired = errors.New("password change required")

type Service struct {
	store     *store.Store
	jwtSecret []byte
}

type Claims struct {
	UserID             string `json:"uid"`
	Username           string `json:"usr"`
	MustChangePassword bool   `json:"mcp,omitempty"`
	jwt.RegisteredClaims
}

type LoginResult struct {
	Username           string
	MustChangePassword bool
}

func New(s *store.Store, jwtSecret []byte) *Service {
	return &Service{store: s, jwtSecret: jwtSecret}
}

func (s *Service) EnsureAdmin(ctx context.Context, username, password string) error {
	existing, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.store.CreateUser(ctx, store.User{
		ID:                 uuid.NewString(),
		Username:           username,
		PasswordHash:       string(hash),
		MustChangePassword: true,
		CreatedAt:          time.Now().UTC(),
	})
}

// ResetPassword forcibly sets the password for an existing user (creating the
// user if it does not exist yet) and requires a password change on next login.
// Used for the ADMIN_PASSWORD_RESET escape hatch, since EnsureAdmin is a no-op
// once the admin already exists.
func (s *Service) ResetPassword(ctx context.Context, username, password string) error {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return err
	}
	if user == nil {
		return s.EnsureAdmin(ctx, username, password)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.UpdateUserPassword(ctx, user.ID, string(hash), true)
}

func (s *Service) Login(ctx context.Context, username, password string) (LoginResult, string, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return LoginResult{}, "", err
	}
	if user == nil {
		return LoginResult{}, "", errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, "", errors.New("invalid credentials")
	}

	token, err := s.signToken(user)
	if err != nil {
		return LoginResult{}, "", err
	}

	return LoginResult{
		Username:           user.Username,
		MustChangePassword: user.MustChangePassword,
	}, token, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) (string, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user == nil {
		return "", errors.New("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return "", errors.New("invalid current password")
	}
	if currentPassword == newPassword {
		return "", ErrPasswordUnchanged
	}
	if err := ValidatePassword(newPassword, user.Username); err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	if err := s.store.UpdateUserPassword(ctx, user.ID, string(hash), false); err != nil {
		return "", err
	}

	user.PasswordHash = string(hash)
	user.MustChangePassword = false
	return s.signToken(user)
}

func (s *Service) signToken(user *store.User) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:             user.ID,
		Username:           user.Username,
		MustChangePassword: user.MustChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	})
	return token.SignedString(s.jwtSecret)
}

func (s *Service) SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

func (s *Service) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (s *Service) ParseRequest(r *http.Request) (*Claims, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseWithClaims(cookie.Value, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/logout" {
			next.ServeHTTP(w, r)
			return
		}

		claims, err := s.ParseRequest(r)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		if claims.MustChangePassword && !isPasswordChangeAllowedPath(r.URL.Path, r.Method) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"password_change_required"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isPasswordChangeAllowedPath(path, method string) bool {
	switch {
	case path == "/api/auth/me" && method == http.MethodGet:
		return true
	case path == "/api/auth/change-password" && method == http.MethodPost:
		return true
	case path == "/api/auth/logout" && method == http.MethodPost:
		return true
	default:
		return false
	}
}
