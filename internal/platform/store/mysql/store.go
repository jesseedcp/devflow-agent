package mysql

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

// Store is the MySQL-backed platform persistence implementation.
type Store struct {
	db *sql.DB
}

// Open validates the DSN, opens the connection pool, and confirms the server
// is reachable with a ping. It does not run migrations.
func Open(ctx context.Context, dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("mysql: database dsn is empty")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql: open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql: ping: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.db.Close() }

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

func newID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}

func nowUTC() time.Time { return time.Now().UTC() }

// UpsertWorkspace inserts or updates a workspace by id.
func (s *Store) UpsertWorkspace(ctx context.Context, w store.Workspace) error {
	if w.ID == "" {
		return errors.New("mysql: workspace id is required")
	}
	if w.CreatedAt.IsZero() {
		w.CreatedAt = nowUTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspaces (id, name, artifact_root, created_at) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE name = VALUES(name), artifact_root = VALUES(artifact_root)`, w.ID, w.Name, w.ArtifactRoot, w.CreatedAt)
	if err != nil {
		return fmt.Errorf("mysql: upsert workspace: %w", err)
	}
	return nil
}

// ListWorkspaces returns all workspaces ordered by creation time.
func (s *Store) ListWorkspaces(ctx context.Context) ([]store.Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, artifact_root, created_at FROM workspaces ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("mysql: list workspaces: %w", err)
	}
	defer rows.Close()
	var out []store.Workspace
	for rows.Next() {
		var w store.Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.ArtifactRoot, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("mysql: scan workspace: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: list workspaces rows: %w", err)
	}
	return out, nil
}

// GetWorkspace returns a single workspace by id.
func (s *Store) GetWorkspace(ctx context.Context, id string) (store.Workspace, error) {
	var w store.Workspace
	err := s.db.QueryRowContext(ctx, `SELECT id, name, artifact_root, created_at FROM workspaces WHERE id = ?`, id).
		Scan(&w.ID, &w.Name, &w.ArtifactRoot, &w.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Workspace{}, store.ErrNotFound
	}
	if err != nil {
		return store.Workspace{}, fmt.Errorf("mysql: get workspace: %w", err)
	}
	return w, nil
}

// UpsertUser inserts or updates a user by id.
func (s *Store) UpsertUser(ctx context.Context, u store.User) error {
	if u.ID == "" {
		return errors.New("mysql: user id is required")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = nowUTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, email, display_name, created_at) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE email = VALUES(email), display_name = VALUES(display_name)`, u.ID, u.Email, u.DisplayName, u.CreatedAt)
	if err != nil {
		return fmt.Errorf("mysql: upsert user: %w", err)
	}
	return nil
}

// GetUserByEmail returns the user with the given email.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx, `SELECT id, email, display_name, created_at FROM users WHERE email = ?`, email).
		Scan(&u.ID, &u.Email, &u.DisplayName, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return store.User{}, store.ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("mysql: get user by email: %w", err)
	}
	return u, nil
}

// UpsertWorkspaceMember inserts or updates a workspace membership.
func (s *Store) UpsertWorkspaceMember(ctx context.Context, m store.WorkspaceMember) error {
	if m.WorkspaceID == "" || m.UserID == "" {
		return errors.New("mysql: workspace member requires workspace_id and user_id")
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = nowUTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role, created_at) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE role = VALUES(role)`, m.WorkspaceID, m.UserID, string(m.Role), m.CreatedAt)
	if err != nil {
		return fmt.Errorf("mysql: upsert workspace member: %w", err)
	}
	return nil
}

// ListWorkspaceMembers returns memberships for a workspace.
func (s *Store) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]store.WorkspaceMember, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT workspace_id, user_id, role, created_at FROM workspace_members WHERE workspace_id = ? ORDER BY created_at, user_id`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list workspace members: %w", err)
	}
	defer rows.Close()
	var out []store.WorkspaceMember
	for rows.Next() {
		var m store.WorkspaceMember
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("mysql: scan workspace member: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: list workspace members rows: %w", err)
	}
	return out, nil
}

// AppendAudit records an audit event, assigning id and created_at when empty.
func (s *Store) AppendAudit(ctx context.Context, ev store.AuditEvent) error {
	if ev.ID == "" {
		ev.ID = newID()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = nowUTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_events (id, workspace_id, actor_user_id, action, subject_type, subject_id, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, ev.WorkspaceID, ev.ActorUserID, ev.Action, ev.SubjectType, ev.SubjectID, ev.MetadataJSON, ev.CreatedAt)
	if err != nil {
		return fmt.Errorf("mysql: append audit: %w", err)
	}
	return nil
}

// ListAudit returns audit events for a workspace, newest last.
func (s *Store) ListAudit(ctx context.Context, workspaceID string) ([]store.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, workspace_id, actor_user_id, action, subject_type, subject_id, metadata_json, created_at FROM audit_events WHERE workspace_id = ? ORDER BY created_at, id`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("mysql: list audit: %w", err)
	}
	defer rows.Close()
	var out []store.AuditEvent
	for rows.Next() {
		var ev store.AuditEvent
		if err := rows.Scan(&ev.ID, &ev.WorkspaceID, &ev.ActorUserID, &ev.Action, &ev.SubjectType, &ev.SubjectID, &ev.MetadataJSON, &ev.CreatedAt); err != nil {
			return nil, fmt.Errorf("mysql: scan audit: %w", err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("mysql: list audit rows: %w", err)
	}
	return out, nil
}