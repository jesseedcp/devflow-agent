package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/releasecontrol"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestObserveRefreshPassesAfterSuccessfulDeployment(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          "observe-coupon",
		Title:       "Observe coupon",
		Description: "observe",
		Source:      "test",
		State:       string(workflow.Observation),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	deployment := releasecontrol.RenderDeployment(demand.Title, releasecontrol.DeploymentRecord{
		Provider:   "github_actions",
		Repo:       "owner/repo",
		RunID:      "123",
		RunURL:     "https://example/runs/123",
		Status:     releasecontrol.StatusPassed,
		Conclusion: "success",
	})
	if err := store.WriteArtifact(demand.ID, artifacts.DeploymentFile, deployment); err != nil {
		t.Fatalf("WriteArtifact deployment returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"observe", "refresh", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("observe refresh returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.ObservationFile))
	if err != nil {
		t.Fatalf("read observation.md: %v", err)
	}
	if !strings.Contains(string(body), "Status: `passed`") {
		t.Fatalf("observation.md = %s", string(body))
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if updated.State != string(workflow.Closeout) {
		t.Fatalf("state = %s, want closeout", updated.State)
	}
}

func TestObserveRefreshBlocksAfterFailedDeployment(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "observe-fail",
		Title: "Observe fail",
		State: string(workflow.Observation),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	deployment := releasecontrol.RenderDeployment(demand.Title, releasecontrol.DeploymentRecord{
		Provider:   "github_actions",
		Repo:       "owner/repo",
		RunID:      "456",
		Status:     releasecontrol.StatusFailed,
		Conclusion: "failure",
	})
	if err := store.WriteArtifact(demand.ID, artifacts.DeploymentFile, deployment); err != nil {
		t.Fatalf("WriteArtifact deployment returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"observe", "refresh", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("observe refresh returned error: %v", err)
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

func TestObserveRefreshRejectsMissingDeploymentEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "observe-missing",
		Title: "Observe missing",
		State: string(workflow.Observation),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{"observe", "refresh", "--root", root, "--demand", demand.ID}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected error for missing deployment evidence")
	}
}

func TestObserveRefreshPassesWithHTTPHealthCheck(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "observe-health", Title: "Observe health", State: string(workflow.Observation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	deployment := releasecontrol.RenderDeployment(demand.Title, releasecontrol.DeploymentRecord{
		Provider:   "github_actions",
		Repo:       "owner/repo",
		RunID:      "123",
		RunURL:     "https://example/runs/123",
		Status:     releasecontrol.StatusPassed,
		Conclusion: "success",
	})
	if err := store.WriteArtifact(demand.ID, artifacts.DeploymentFile, deployment); err != nil {
		t.Fatal(err)
	}
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","version":"v1.2"}`))
	}))
	defer health.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"observe", "refresh",
		"--root", root,
		"--demand", demand.ID,
		"--health-url", health.URL + "/health",
		"--expect-status", "200",
		"--expect-contains", `"status":"ok"`,
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("observe refresh returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.ObservationFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Name: `http_health`", "Status: `passed`", `"status":"ok"`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("observation.md missing %q:\n%s", want, string(body))
		}
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != string(workflow.Closeout) {
		t.Fatalf("state = %s, want closeout", updated.State)
	}
}

func TestObserveRefreshBlocksWhenHTTPHealthFails(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "observe-health-fail", Title: "Observe health fail", State: string(workflow.Observation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	deployment := releasecontrol.RenderDeployment(demand.Title, releasecontrol.DeploymentRecord{
		Provider:   "github_actions",
		Repo:       "owner/repo",
		RunID:      "456",
		Status:     releasecontrol.StatusPassed,
		Conclusion: "success",
	})
	if err := store.WriteArtifact(demand.ID, artifacts.DeploymentFile, deployment); err != nil {
		t.Fatal(err)
	}
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"bad"}`))
	}))
	defer health.Close()

	err := Run([]string{
		"observe", "refresh",
		"--root", root,
		"--demand", demand.ID,
		"--health-url", health.URL + "/health",
		"--expect-status", "200",
		"--expect-contains", `"status":"ok"`,
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("observe refresh returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.ObservationFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Status: `failed`") {
		t.Fatalf("observation.md should fail:\n%s", string(body))
	}
	rollback, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.RollbackFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rollback), "Decision: `pending`") {
		t.Fatalf("rollback.md missing pending decision:\n%s", string(rollback))
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != string(workflow.BlockedNeedReleaseDecision) {
		t.Fatalf("state = %s, want blocked_need_release_decision", updated.State)
	}
}

func TestObserveRefreshBlockedHTTPHealthShowsProxyHintAndRedactsURL(t *testing.T) {
	oldTimeout := observeHealthTimeout
	observeHealthTimeout = 10 * time.Millisecond
	t.Cleanup(func() { observeHealthTimeout = oldTimeout })

	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "observe-health-blocked", Title: "Observe health blocked", State: string(workflow.Observation)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	deployment := releasecontrol.RenderDeployment(demand.Title, releasecontrol.DeploymentRecord{
		Provider:   "github_actions",
		Repo:       "owner/repo",
		RunID:      "789",
		Status:     releasecontrol.StatusPassed,
		Conclusion: "success",
	})
	if err := store.WriteArtifact(demand.ID, artifacts.DeploymentFile, deployment); err != nil {
		t.Fatal(err)
	}
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer health.Close()

	var stdout bytes.Buffer
	err := Run([]string{
		"observe", "refresh",
		"--root", root,
		"--demand", demand.ID,
		"--health-url", health.URL + "/health?token=abc",
		"--expect-status", "200",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("observe refresh returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.ObservationFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "Status: `blocked`") {
		t.Fatalf("observation.md should be blocked:\n%s", string(body))
	}
	rollback, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.RollbackFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rollback), "Decision: `pending`") {
		t.Fatalf("rollback.md missing pending decision:\n%s", string(rollback))
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != string(workflow.BlockedNeedReleaseDecision) {
		t.Fatalf("state = %s, want blocked_need_release_decision", updated.State)
	}
	output := stdout.String()
	for _, want := range []string{"observation blocked", "HTTPS_PROXY", "HTTP_PROXY"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "token=abc") || strings.Contains(string(body), "token=abc") {
		t.Fatalf("health URL token leaked\nstdout:\n%s\nbody:\n%s", output, string(body))
	}
}
