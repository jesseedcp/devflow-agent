package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

type fakeStore struct {
	workspaces map[string]store.Workspace
	users      map[string]store.User
	members    []store.WorkspaceMember
	audit      []store.AuditEvent
	pingErr    error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		workspaces: map[string]store.Workspace{},
		users:      map[string]store.User{},
	}
}

func (f *fakeStore) Ping(ctx context.Context) error                  { return f.pingErr }
func (f *fakeStore) Migrate(ctx context.Context) error               { return nil }
func (f *fakeStore) Close() error                                    { return nil }
func (f *fakeStore) UpsertWorkspace(ctx context.Context, w store.Workspace) error {
	f.workspaces[w.ID] = w
	return nil
}
func (f *fakeStore) ListWorkspaces(ctx context.Context) ([]store.Workspace, error) {
	out := make([]store.Workspace, 0, len(f.workspaces))
	for _, w := range f.workspaces {
		out = append(out, w)
	}
	return out, nil
}
func (f *fakeStore) GetWorkspace(ctx context.Context, id string) (store.Workspace, error) {
	w, ok := f.workspaces[id]
	if !ok {
		return store.Workspace{}, store.ErrNotFound
	}
	return w, nil
}
func (f *fakeStore) UpsertUser(ctx context.Context, u store.User) error { f.users[u.ID] = u; return nil }
func (f *fakeStore) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}
func (f *fakeStore) UpsertWorkspaceMember(ctx context.Context, m store.WorkspaceMember) error {
	f.members = append(f.members, m)
	return nil
}
func (f *fakeStore) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]store.WorkspaceMember, error) {
	out := make([]store.WorkspaceMember, 0)
	for _, m := range f.members {
		if m.WorkspaceID == workspaceID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (f *fakeStore) AppendAudit(ctx context.Context, ev store.AuditEvent) error {
	f.audit = append(f.audit, ev)
	return nil
}
func (f *fakeStore) ListAudit(ctx context.Context, workspaceID string) ([]store.AuditEvent, error) {
	out := make([]store.AuditEvent, 0)
	for _, e := range f.audit {
		if e.WorkspaceID == workspaceID {
			out = append(out, e)
		}
	}
	return out, nil
}

func newTestServer(t *testing.T, st *fakeStore) *httptest.Server {
	t.Helper()
	srv := New(Config{DevUserEmail: "admin@example.com", DevUserRole: store.RoleAdmin, DevUserID: "devlocal"}, st)
	return httptest.NewServer(srv.Handler())
}

func TestHealthOK(t *testing.T) {
	ts := newTestServer(t, newFakeStore())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "ok" || got.Database != "ok" {
		t.Fatalf("expected ok/ok, got %+v", got)
	}
}

func TestHealthDatabaseDown(t *testing.T) {
	st := newFakeStore()
	st.pingErr = errors.New("connection refused")
	ts := newTestServer(t, st)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	var got api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "degraded" || got.Database != "down" {
		t.Fatalf("expected degraded/down, got %+v", got)
	}
}

func TestMeReturnsDevIdentity(t *testing.T) {
	ts := newTestServer(t, newFakeStore())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.CurrentUser
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Email != "admin@example.com" || got.Role != "Admin" {
		t.Fatalf("unexpected current user: %+v", got)
	}
}

func TestWorkspacesRoundTrip(t *testing.T) {
	ts := newTestServer(t, newFakeStore())
	defer ts.Close()

	body := []byte(`{"name":"Demo","artifact_root":"/tmp/demo"}`)
	resp, err := http.Post(ts.URL+"/api/workspaces", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created api.Workspace
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "Demo" || created.ID == "" || created.ArtifactRoot != "/tmp/demo" {
		t.Fatalf("unexpected created workspace: %+v", created)
	}

	resp2, err := http.Get(ts.URL + "/api/workspaces")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	raw, _ := io.ReadAll(resp2.Body)
	var list []api.Workspace
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("decode list: %v (%s)", err, raw)
	}
	if len(list) != 1 || list[0].Name != "Demo" {
		t.Fatalf("unexpected workspace list: %s", raw)
	}
}

func TestCreateWorkspaceRejectsMissingName(t *testing.T) {
	ts := newTestServer(t, newFakeStore())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/workspaces", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}