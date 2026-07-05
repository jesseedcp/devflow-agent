package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func newFakeGitHubActionsServer(t *testing.T, outcome string) *httptest.Server {
	t.Helper()
	var sawDispatch bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/dispatches"):
			sawDispatch = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/runs"):
			w.Header().Set("Content-Type", "application/json")
			status, conclusion := "completed", "success"
			switch outcome {
			case "failure":
				status, conclusion = "completed", "failure"
			case "pending":
				status, conclusion = "in_progress", ""
			}
			fmt.Fprintf(w, `{"workflow_runs":[{"id":123,"html_url":"https://example/runs/123","head_sha":"abc123","head_branch":"main","status":"%s","conclusion":"%s","created_at":"2026-07-05T10:00:00Z","updated_at":"2026-07-05T10:02:00Z"}]}`, status, conclusion)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(func() {
		if !sawDispatch && outcome == "dispatch-expected" {
			t.Fatal("expected dispatch endpoint to be called")
		}
	})
	return server
}

func TestDeployTriggerWritesDeploymentArtifact(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          "release-coupon",
		Title:       "Release coupon",
		Description: "release",
		Source:      "test",
		State:       string(workflow.Deployment),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	server := newFakeGitHubActionsServer(t, "success")
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"deploy", "trigger",
		"--root", root,
		"--demand", demand.ID,
		"--provider", "github",
		"--github-repo", "owner/repo",
		"--workflow", "release.yml",
		"--ref", "main",
		"--github-base-url", server.URL,
		"--github-token", "fake",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run deploy trigger returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.DeploymentFile))
	if err != nil {
		t.Fatalf("read deployment.md: %v", err)
	}
	for _, want := range []string{"Provider: `github_actions`", "Run ID: `123`", "Status: `passed`"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("deployment.md missing %q:\n%s", want, string(body))
		}
	}
	if !strings.Contains(stdout.String(), "deployment passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if updated.State != string(workflow.Observation) {
		t.Fatalf("state = %s, want observation", updated.State)
	}
}

func TestDeployTriggerFailedWritesRollback(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "release-fail",
		Title: "Release fail",
		State: string(workflow.Deployment),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	server := newFakeGitHubActionsServer(t, "failure")
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"deploy", "trigger",
		"--root", root,
		"--demand", demand.ID,
		"--github-repo", "owner/repo",
		"--workflow", "release.yml",
		"--ref", "main",
		"--github-base-url", server.URL,
		"--github-token", "fake",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run deploy trigger returned error: %v", err)
	}

	rollback, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.RollbackFile))
	if err != nil {
		t.Fatalf("read rollback.md: %v", err)
	}
	if !strings.Contains(string(rollback), "Decision: `pending`") {
		t.Fatalf("rollback.md missing pending decision:\n%s", string(rollback))
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if updated.State != string(workflow.BlockedNeedReleaseDecision) {
		t.Fatalf("state = %s, want blocked_need_release_decision", updated.State)
	}
}

func TestDeployStatusRefreshesWithoutDispatch(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "release-status",
		Title: "Release status",
		State: string(workflow.Deployment),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	server := newFakeGitHubActionsServer(t, "success")
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"deploy", "status",
		"--root", root,
		"--demand", demand.ID,
		"--github-repo", "owner/repo",
		"--workflow", "release.yml",
		"--ref", "main",
		"--github-base-url", server.URL,
		"--github-token", "fake",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run deploy status returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.DeploymentFile))
	if err != nil {
		t.Fatalf("read deployment.md: %v", err)
	}
	if !strings.Contains(string(body), "Status: `passed`") {
		t.Fatalf("deployment.md = %s", string(body))
	}
}

func TestDeployTriggerRejectsVerificationState(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "release-verify",
		Title: "Release verify",
		State: string(workflow.Verification),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{
		"deploy", "trigger",
		"--root", root,
		"--demand", demand.ID,
		"--github-repo", "owner/repo",
		"--workflow", "release.yml",
		"--ref", "main",
		"--github-token", "fake",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "confirm verification first") {
		t.Fatalf("err = %v, want verification-first error", err)
	}
}

func TestDeployTriggerMissingFlags(t *testing.T) {
	cases := [][]string{
		{"deploy", "trigger", "--demand", "x"},
		{"deploy", "trigger", "--demand", "x", "--github-repo", "owner/repo"},
		{"deploy", "trigger", "--demand", "x", "--github-repo", "owner/repo", "--workflow", "release.yml"},
	}
	for _, args := range cases {
		err := Run(args, &bytes.Buffer{}, &bytes.Buffer{})
		if err == nil {
			t.Fatalf("expected error for args %v", args)
		}
	}
}
