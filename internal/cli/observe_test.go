package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
