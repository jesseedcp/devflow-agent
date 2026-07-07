package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

func rbacServer(t *testing.T, role store.Role, st *fakeStore) *httptest.Server {
	t.Helper()
	srv := New(Config{DevUserEmail: "admin@example.com", DevUserRole: role, DevUserID: "devlocal"}, st)
	return httptest.NewServer(srv.Handler())
}

func TestCreateWorkspaceForbiddenForViewer(t *testing.T) {
	ts := rbacServer(t, store.RoleViewer, newFakeStore())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/workspaces", "application/json", bytes.NewReader([]byte(`{"name":"X"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for Viewer, got %d", resp.StatusCode)
	}
}

func TestCreateWorkspaceForbiddenForDeveloper(t *testing.T) {
	ts := rbacServer(t, store.RoleDeveloper, newFakeStore())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/workspaces", "application/json", bytes.NewReader([]byte(`{"name":"X"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for Developer, got %d", resp.StatusCode)
	}
}

func TestCreateWorkspaceAuditedForAdmin(t *testing.T) {
	st := newFakeStore()
	ts := rbacServer(t, store.RoleAdmin, st)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/workspaces", "application/json", bytes.NewReader([]byte(`{"name":"X","artifact_root":"/tmp/x"}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for Admin, got %d", resp.StatusCode)
	}
	var created api.Workspace
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("expected workspace id")
	}

	resp2, err := http.Get(ts.URL + "/api/workspaces/" + created.ID + "/audit")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 audit, got %d", resp2.StatusCode)
	}
	var events []api.AuditEvent
	if err := json.NewDecoder(resp2.Body).Decode(&events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	ev := events[0]
	if ev.Action != store.AuditConfigureWorkspace || ev.SubjectID != created.ID || ev.ActorUserID != "devlocal" {
		t.Fatalf("unexpected audit event: %+v", ev)
	}
}

func TestListAudit(t *testing.T) {
	st := newFakeStore()
	st.workspaces["ws-1"] = store.Workspace{ID: "ws-1", Name: "Demo", ArtifactRoot: "/tmp"}
	_ = st.AppendAudit(context.Background(), store.AuditEvent{
		WorkspaceID:  "ws-1",
		ActorUserID:  "devlocal",
		Action:       store.AuditPromoteWiki,
		SubjectType:  "wiki_candidate",
		SubjectID:    "c-1",
		MetadataJSON: `{"name":"entry-a"}`,
	})
	ts := rbacServer(t, store.RoleAdmin, st)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/ws-1/audit")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var events []api.AuditEvent
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Action != store.AuditPromoteWiki {
		t.Fatalf("unexpected audit events: %+v", events)
	}
	meta, ok := events[0].Metadata.(map[string]any)
	if !ok || meta["name"] != "entry-a" {
		t.Fatalf("unexpected metadata: %+v", events[0].Metadata)
	}
}

func TestListAuditWorkspaceNotFound(t *testing.T) {
	ts := rbacServer(t, store.RoleAdmin, newFakeStore())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/missing/audit")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}