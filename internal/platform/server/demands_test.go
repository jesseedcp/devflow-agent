package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/platform/api"
	"github.com/jesseedcp/devflow-agent/internal/platform/store"
)

func demandsTestServer(t *testing.T, root string) (*httptest.Server, *fakeStore) {
	t.Helper()
	st := newFakeStore()
	st.workspaces["ws-1"] = store.Workspace{ID: "ws-1", Name: "Demo", ArtifactRoot: root}
	srv := New(Config{DevUserEmail: "admin@example.com", DevUserRole: store.RoleAdmin, DevUserID: "devlocal"}, st)
	ts := httptest.NewServer(srv.Handler())
	return ts, st
}

func seedServerDemand(t *testing.T, root, id, title, state string) {
	t.Helper()
	st := artifacts.NewStore(root)
	if err := st.CreateDemand(artifacts.Demand{ID: id, Title: title, Description: "demo", Source: "test", State: state}); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	if err := st.AppendEvent(id, artifacts.Event{Type: "note", Message: "started"}); err != nil {
		t.Fatalf("append event: %v", err)
	}
}

func TestListDemands(t *testing.T) {
	root := t.TempDir()
	seedServerDemand(t, root, "alpha-1", "Alpha", "plan")
	ts, _ := demandsTestServer(t, root)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/ws-1/demands")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []api.DemandSummary
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].DemandKey != "alpha-1" || list[0].Title != "Alpha" {
		t.Fatalf("unexpected demand list: %+v", list)
	}
}

func TestListDemandsEmptyRoot(t *testing.T) {
	ts, _ := demandsTestServer(t, t.TempDir())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/ws-1/demands")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(raw)) != "[]" {
		t.Fatalf("expected empty array, got %s", raw)
	}
}

func TestListDemandsWorkspaceNotFound(t *testing.T) {
	ts, _ := demandsTestServer(t, t.TempDir())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/missing/demands")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetDemand(t *testing.T) {
	root := t.TempDir()
	seedServerDemand(t, root, "beta-2", "Beta", "verification")
	ts, _ := demandsTestServer(t, root)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/ws-1/demands/beta-2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var detail api.DemandDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.DemandKey != "beta-2" || detail.Title != "Beta" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}

func TestGetDemandMissing(t *testing.T) {
	ts, _ := demandsTestServer(t, t.TempDir())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/ws-1/demands/missing")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetArtifact(t *testing.T) {
	root := t.TempDir()
	seedServerDemand(t, root, "gamma-3", "Gamma", "plan")
	ts, _ := demandsTestServer(t, root)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/ws-1/demands/gamma-3/artifacts/requirements.md")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Fatal("expected non-empty artifact body")
	}
}

func TestGetArtifactRejectsUnknownName(t *testing.T) {
	root := t.TempDir()
	seedServerDemand(t, root, "delta-4", "Delta", "plan")
	ts, _ := demandsTestServer(t, root)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/workspaces/ws-1/demands/delta-4/artifacts/bogus.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}