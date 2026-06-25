package auth

import (
	"errors"
	"strings"
	"unicode"
)

var (
	ErrPasswordTooShort    = errors.New("password must be at least 12 characters")
	ErrPasswordUppercase   = errors.New("password must contain an uppercase letter")
	ErrPasswordLowercase   = errors.New("password must contain a lowercase letter")
	ErrPasswordDigit       = errors.New("password must contain a digit")
	ErrPasswordSpecial     = errors.New("password must contain a special character")
	ErrPasswordSameAsUser  = errors.New("password must not match username")
	ErrPasswordUnchanged   = errors.New("new password must differ from current password")
)

func ValidatePassword(password, username string) error {
	if len(password) < 12 {
		return ErrPasswordTooShort
	}
	if strings.EqualFold(password, username) {
		return ErrPasswordSameAsUser
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case isSpecialChar(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return ErrPasswordUppercase
	}
	if !hasLower {
		return ErrPasswordLowercase
	}
	if !hasDigit {
		return ErrPasswordDigit
	}
	if !hasSpecial {
		return ErrPasswordSpecial
	}
	return nil
}

func isSpecialChar(r rune) bool {
	return strings.ContainsRune(`!@#$%^&*()_+-=[]{}|;:,.<>?`, r)
}
