package mysql

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

func testDSN() string {
	if v := os.Getenv("DEVFLOW_DATABASE_DSN"); v != "" {
		return v
	}
	return "devflow:devflow@tcp(127.0.0.1:3316)/devflow?parseTime=true"
}

func openStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), testDSN())
	if err != nil {
		t.Skipf("mysql unavailable, skipping store test: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cleanTables(t, st)
	return st
}

func cleanTables(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{"audit_events", "workspace_members", "demands", "workspaces", "users"} {
		if _, err := st.db.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
}

func TestMigrateCreatesTables(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	var count int
	err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name IN ('users','workspaces','workspace_members','audit_events','demands')`).Scan(&count)
	if err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if count != 5 {
		t.Fatalf("expected 5 core tables, got %d", count)
	}
}

func TestWorkspaceRoundTrip(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	w := store.Workspace{
		ID:           "ws-roundtrip",
		Name:         "Demo",
		ArtifactRoot: "/tmp/demo",
		CreatedAt:    time.Now().UTC().Truncate(time.Millisecond),
	}
	if err := st.UpsertWorkspace(ctx, w); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := st.GetWorkspace(ctx, "ws-roundtrip")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Demo" || got.ArtifactRoot != "/tmp/demo" {
		t.Fatalf("unexpected workspace: %+v", got)
	}
	list, err := st.ListWorkspaces(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(list))
	}
	w.Name = "Demo2"
	if err := st.UpsertWorkspace(ctx, w); err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	got2, err := st.GetWorkspace(ctx, "ws-roundtrip")
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got2.Name != "Demo2" {
		t.Fatalf("expected updated name Demo2, got %q", got2.Name)
	}
	if _, err := st.GetWorkspace(ctx, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing workspace, got %v", err)
	}
}

func TestAuditRoundTrip(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()
	if err := st.UpsertWorkspace(ctx, store.Workspace{ID: "ws-audit", Name: "Audit", ArtifactRoot: "/tmp/audit", CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}
	ev := store.NewAuditEvent("ws-audit", "user-1", store.AuditPromoteWiki, "wiki_candidate", "c-2", map[string]any{"name": "entry-a"})
	if err := st.AppendAudit(ctx, ev); err != nil {
		t.Fatalf("append audit: %v", err)
	}
	events, err := st.ListAudit(ctx, "ws-audit")
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	got := events[0]
	if got.Action != store.AuditPromoteWiki || got.SubjectID != "c-2" || got.ActorUserID != "user-1" {
		t.Fatalf("unexpected audit event: %+v", got)
	}
	if got.ID == "" || got.CreatedAt.IsZero() {
		t.Fatalf("audit event missing id/created_at: %+v", got)
	}
	if !strings.Contains(got.MetadataJSON, "entry-a") {
		t.Fatalf("metadata not persisted: %q", got.MetadataJSON)
	}
}