package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Server struct {
	ID                string     `json:"id"`
	Name              string     `json:"name"`
	Host              string     `json:"host"`
	SSHPort           int        `json:"ssh_port"`
	SSHUser           string     `json:"ssh_user"`
	SSHAuthType       string     `json:"ssh_auth_type"`
	EncryptedBlob     []byte     `json:"-"`
	EncryptionNonce   []byte     `json:"-"`
	ProxyType         string     `json:"proxy_type"`
	FakeTLS           bool       `json:"fake_tls"`
	SNIDomain         string     `json:"sni_domain"`
	MTProtoPort       int        `json:"mtproto_port"`
	Secret            string     `json:"secret"`
	ProxyLink         string     `json:"proxy_link"`
	ContainerName     string     `json:"container_name"`
	Tags              []string   `json:"tags"`
	Status            string     `json:"status"`
	PingStatus        string     `json:"ping_status"`
	TCPStatus         string     `json:"tcp_status"`
	PingRTTMs         *float64   `json:"ping_rtt_ms,omitempty"`
	DeployStatus      string     `json:"deploy_status"`
	Operation         string     `json:"operation"`
	OperationStep     string     `json:"operation_step"`
	OperationMessage  string     `json:"operation_message"`
	OperationProgress int        `json:"operation_progress"`
	LastCheckAt       *time.Time `json:"last_check_at,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type User struct {
	ID                 string
	Username           string
	PasswordHash       string
	MustChangePassword bool
	CreatedAt          time.Time
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS servers (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	host TEXT NOT NULL,
	ssh_port INTEGER NOT NULL DEFAULT 22,
	ssh_user TEXT NOT NULL,
	ssh_auth_type TEXT NOT NULL,
	encrypted_blob BLOB NOT NULL,
	encryption_nonce BLOB NOT NULL,
	sni_domain TEXT NOT NULL DEFAULT '',
	mtproto_port INTEGER NOT NULL DEFAULT 443,
	secret TEXT NOT NULL DEFAULT '',
	proxy_link TEXT NOT NULL DEFAULT '',
	container_name TEXT NOT NULL DEFAULT 'mtproto-mtg',
	status TEXT NOT NULL DEFAULT 'unknown',
	deploy_status TEXT NOT NULL DEFAULT 'pending',
	last_check_at TEXT,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := s.migrateHealthColumns(); err != nil {
		return err
	}
	if err := s.migrateUserColumns(); err != nil {
		return err
	}
	if err := s.migrateOperationColumns(); err != nil {
		return err
	}
	if err := s.migrateProxyColumns(); err != nil {
		return err
	}
	return s.migrateTagsColumn()
}

func (s *Store) migrateTagsColumn() error {
	stmt := `ALTER TABLE servers ADD COLUMN tags TEXT NOT NULL DEFAULT ''`
	if _, err := s.db.Exec(stmt); err != nil {
		if !isDuplicateColumn(err) {
			return fmt.Errorf("migrate tags column: %w", err)
		}
	}
	return nil
}

func (s *Store) migrateProxyColumns() error {
	columns := []string{
		`ALTER TABLE servers ADD COLUMN proxy_type TEXT NOT NULL DEFAULT 'mtg'`,
		`ALTER TABLE servers ADD COLUMN fake_tls INTEGER NOT NULL DEFAULT 0`,
	}
	for _, stmt := range columns {
		if _, err := s.db.Exec(stmt); err != nil {
			if !isDuplicateColumn(err) {
				return fmt.Errorf("migrate proxy columns: %w", err)
			}
		}
	}
	return nil
}

func (s *Store) migrateOperationColumns() error {
	columns := []string{
		`ALTER TABLE servers ADD COLUMN operation TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE servers ADD COLUMN operation_step TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE servers ADD COLUMN operation_message TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE servers ADD COLUMN operation_progress INTEGER NOT NULL DEFAULT 0`,
	}
	for _, stmt := range columns {
		if _, err := s.db.Exec(stmt); err != nil {
			if !isDuplicateColumn(err) {
				return fmt.Errorf("migrate operation columns: %w", err)
			}
		}
	}
	return nil
}

func (s *Store) migrateUserColumns() error {
	stmt := `ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 1`
	if _, err := s.db.Exec(stmt); err != nil {
		if !isDuplicateColumn(err) {
			return fmt.Errorf("migrate user columns: %w", err)
		}
	}
	return nil
}

func (s *Store) migrateHealthColumns() error {
	columns := []string{
		`ALTER TABLE servers ADD COLUMN ping_status TEXT NOT NULL DEFAULT 'unknown'`,
		`ALTER TABLE servers ADD COLUMN tcp_status TEXT NOT NULL DEFAULT 'unknown'`,
		`ALTER TABLE servers ADD COLUMN ping_rtt_ms REAL`,
	}
	for _, stmt := range columns {
		if _, err := s.db.Exec(stmt); err != nil {
			if !isDuplicateColumn(err) {
				return fmt.Errorf("migrate health columns: %w", err)
			}
		}
	}
	return nil
}

func isDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, must_change_password, created_at
		FROM users WHERE username = ?`, username)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, password_hash, must_change_password, created_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var createdAt string
	var mustChange int
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &mustChange, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	u.MustChangePassword = mustChange == 1
	t, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = t
	return &u, nil
}

func (s *Store) CreateUser(ctx context.Context, u User) error {
	mustChange := 0
	if u.MustChangePassword {
		mustChange = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, password_hash, must_change_password, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, mustChange, u.CreatedAt.UTC().Format(time.RFC3339))
	return err
}

func (s *Store) UpdateUserPassword(ctx context.Context, id, passwordHash string, mustChange bool) error {
	flag := 0
	if mustChange {
		flag = 1
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ?`,
		passwordHash, flag, id)
	return err
}

func (s *Store) ListServers(ctx context.Context) ([]Server, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, host, ssh_port, ssh_user, ssh_auth_type,
		       encrypted_blob, encryption_nonce, proxy_type, fake_tls, sni_domain, mtproto_port,
		       secret, proxy_link, container_name, tags, status, ping_status, tcp_status,
		       ping_rtt_ms, deploy_status, operation, operation_step, operation_message,
		       operation_progress, last_check_at, last_error, created_at
		FROM servers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []Server
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}
	return servers, rows.Err()
}

func (s *Store) GetServer(ctx context.Context, id string) (*Server, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, host, ssh_port, ssh_user, ssh_auth_type,
		       encrypted_blob, encryption_nonce, proxy_type, fake_tls, sni_domain, mtproto_port,
		       secret, proxy_link, container_name, tags, status, ping_status, tcp_status,
		       ping_rtt_ms, deploy_status, operation, operation_step, operation_message,
		       operation_progress, last_check_at, last_error, created_at
		FROM servers WHERE id = ?`, id)

	srv, err := scanServer(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &srv, nil
}

func (s *Store) CreateServer(ctx context.Context, srv Server) error {
	lastCheck := ""
	if srv.LastCheckAt != nil {
		lastCheck = srv.LastCheckAt.UTC().Format(time.RFC3339)
	}
	fakeTLS := 0
	if srv.FakeTLS {
		fakeTLS = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO servers (
			id, name, host, ssh_port, ssh_user, ssh_auth_type,
			encrypted_blob, encryption_nonce, proxy_type, fake_tls, sni_domain, mtproto_port,
			secret, proxy_link, container_name, tags, status, deploy_status,
			last_check_at, last_error, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.Host, srv.SSHPort, srv.SSHUser, srv.SSHAuthType,
		srv.EncryptedBlob, srv.EncryptionNonce, srv.ProxyType, fakeTLS, srv.SNIDomain, srv.MTProtoPort,
		srv.Secret, srv.ProxyLink, srv.ContainerName, EncodeTags(srv.Tags), srv.Status, srv.DeployStatus,
		nullIfEmpty(lastCheck), srv.LastError, srv.CreatedAt.UTC().Format(time.RFC3339))
	return err
}

// UpdateServerMeta updates editable connection/metadata fields. SSH credentials
// are updated separately via UpdateServerCredentials.
func (s *Store) UpdateServerMeta(
	ctx context.Context,
	id, name, host string,
	sshPort int,
	sshUser, sshAuthType string,
	mtprotoPort int,
	tags []string,
) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			name = ?, host = ?, ssh_port = ?, ssh_user = ?, ssh_auth_type = ?,
			mtproto_port = ?, tags = ?
		WHERE id = ?`,
		name, host, sshPort, sshUser, sshAuthType, mtprotoPort, EncodeTags(tags), id)
	return err
}

// UpdateServerCredentials replaces the encrypted SSH credentials blob.
func (s *Store) UpdateServerCredentials(ctx context.Context, id string, blob, nonce []byte) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET encrypted_blob = ?, encryption_nonce = ? WHERE id = ?`,
		blob, nonce, id)
	return err
}

func (s *Store) UpdateServerDeploy(ctx context.Context, id string, sni, secret, proxyLink string, port int, deployStatus, lastError string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			sni_domain = ?, mtproto_port = ?, secret = ?, proxy_link = ?,
			deploy_status = ?, last_error = ?
		WHERE id = ?`,
		sni, port, secret, proxyLink, deployStatus, lastError, id)
	return err
}

func (s *Store) UpdateServerOperation(
	ctx context.Context,
	id, deployStatus, op, step, message string,
	progress int,
) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			deploy_status = ?, operation = ?, operation_step = ?,
			operation_message = ?, operation_progress = ?
		WHERE id = ?`,
		deployStatus, op, step, message, progress, id)
	return err
}

func (s *Store) ClearServerOperation(ctx context.Context, id, deployStatus string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			deploy_status = ?, operation = '', operation_step = '',
			operation_message = '', operation_progress = 0
		WHERE id = ?`,
		deployStatus, id)
	return err
}

func (s *Store) SetServerOperationFailed(ctx context.Context, id, deployStatus, lastError string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			deploy_status = ?, last_error = ?,
			operation = '', operation_step = '', operation_message = '', operation_progress = 0
		WHERE id = ?`,
		deployStatus, lastError, id)
	return err
}

func (s *Store) SetServerDeployFailed(ctx context.Context, id, sni, lastError string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			sni_domain = CASE WHEN ? != '' THEN ? ELSE sni_domain END,
			deploy_status = 'failed', last_error = ?,
			operation = '', operation_step = '', operation_message = '', operation_progress = 0
		WHERE id = ?`,
		sni, sni, lastError, id)
	return err
}

func (s *Store) UpdateServerHealth(
	ctx context.Context,
	id, status, pingStatus, tcpStatus string,
	pingRTTMs *float64,
	checkedAt time.Time,
) error {
	var pingRTT any
	if pingRTTMs != nil {
		pingRTT = *pingRTTMs
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE servers SET
			status = ?, ping_status = ?, tcp_status = ?, ping_rtt_ms = ?,
			last_check_at = ?
		WHERE id = ?`,
		status, pingStatus, tcpStatus, pingRTT,
		checkedAt.UTC().Format(time.RFC3339), id)
	return err
}

func (s *Store) DeleteServer(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM servers WHERE id = ?`, id)
	return err
}

type scannable interface {
	Scan(dest ...any) error
}

func scanServer(row scannable) (Server, error) {
	var srv Server
	var lastCheck, createdAt sql.NullString
	var pingRTT sql.NullFloat64
	var fakeTLS int
	var tagsStr string
	if err := row.Scan(
		&srv.ID, &srv.Name, &srv.Host, &srv.SSHPort, &srv.SSHUser, &srv.SSHAuthType,
		&srv.EncryptedBlob, &srv.EncryptionNonce, &srv.ProxyType, &fakeTLS, &srv.SNIDomain, &srv.MTProtoPort,
		&srv.Secret, &srv.ProxyLink, &srv.ContainerName, &tagsStr, &srv.Status, &srv.PingStatus, &srv.TCPStatus,
		&pingRTT, &srv.DeployStatus, &srv.Operation, &srv.OperationStep,
		&srv.OperationMessage, &srv.OperationProgress, &lastCheck, &srv.LastError, &createdAt,
	); err != nil {
		return Server{}, err
	}
	srv.FakeTLS = fakeTLS == 1
	srv.Tags = DecodeTags(tagsStr)
	if pingRTT.Valid {
		v := pingRTT.Float64
		srv.PingRTTMs = &v
	}
	if lastCheck.Valid && lastCheck.String != "" {
		t, err := time.Parse(time.RFC3339, lastCheck.String)
		if err != nil {
			return Server{}, err
		}
		srv.LastCheckAt = &t
	}
	t, err := time.Parse(time.RFC3339, createdAt.String)
	if err != nil {
		return Server{}, err
	}
	srv.CreatedAt = t
	return srv, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// EncodeTags normalizes and joins tags into a comma-separated string for
// storage. Tags are trimmed, de-duplicated (case-insensitive), and any embedded
// commas are stripped.
func EncodeTags(tags []string) string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(strings.ReplaceAll(t, ",", " "))
		t = strings.Join(strings.Fields(t), " ")
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return strings.Join(out, ",")
}

// DecodeTags splits a stored comma-separated tags string into a slice.
func DecodeTags(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
