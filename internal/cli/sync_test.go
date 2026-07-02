package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestSyncGitHubIssuePostsDemandProgress(t *testing.T) {
	root := t.TempDir()
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Description: "Coupon work", Source: "test", State: string(workflow.Verification)}
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.VerificationFile, "# Verification\n\nPASS\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	var posted string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/issues/123/comments":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues/123/comments":
			body, _ := io.ReadAll(r.Body)
			posted = string(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":11}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"sync",
		"--root", root,
		"--demand", demand.ID,
		"--github-issue", "owner/repo#123",
		"--github-base-url", server.URL,
		"--github-token", "token",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("sync returned error: %v\n%s", err, stdout.String())
	}
	if !strings.Contains(posted, "Devflow Update: verification") {
		t.Fatalf("posted body missing verification update: %s", posted)
	}
	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if !cliTestHasEvent(events, "platform.sync_posted") {
		t.Fatalf("events missing platform.sync_posted: %#v", events)
	}
}

func TestSyncFeishuBitableUpdatesRecord(t *testing.T) {
	root := t.TempDir()
	demand := artifacts.Demand{ID: "coupon", Title: "Coupon", Description: "Coupon work", Source: "test", State: string(workflow.Verification)}
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.VerificationFile, "# Verification\n\nPASS\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	var patched string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = w.Write([]byte(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`))
		case "/open-apis/bitable/v1/apps/app_token/tables/tbl/records/rec1":
			body, _ := io.ReadAll(r.Body)
			patched = string(body)
			_, _ = w.Write([]byte(`{"code":0,"data":{"record":{"record_id":"rec1"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"sync",
		"--root", root,
		"--demand", demand.ID,
		"--feishu-bitable", "app_token",
		"--table", "tbl",
		"--record", "rec1",
		"--feishu-base-url", server.URL,
		"--feishu-app-id", "cli_test",
		"--feishu-app-secret", "secret",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("sync returned error: %v\n%s", err, stdout.String())
	}
	for _, want := range []string{"Devflow Demand ID", "coupon", "Devflow 状态", "verification", "验收摘要", "PASS"} {
		if !strings.Contains(patched, want) {
			t.Fatalf("patched body missing %q: %s", want, patched)
		}
	}
	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if !cliTestHasEvent(events, "platform.sync_posted") {
		t.Fatalf("events missing platform.sync_posted: %#v", events)
	}
}
