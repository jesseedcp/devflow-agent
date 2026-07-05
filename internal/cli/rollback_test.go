package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestRollbackConfirmRiskAcceptedAdvancesToCloseout(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          "rollback-coupon",
		Title:       "Rollback coupon",
		Description: "rollback",
		Source:      "test",
		State:       string(workflow.BlockedNeedReleaseDecision),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{
		"rollback", "confirm",
		"--root", root,
		"--demand", demand.ID,
		"--decision", "risk_accepted",
		"--by", "release-manager",
		"--summary", "known low-risk failure accepted",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("rollback confirm returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.RollbackFile))
	if err != nil {
		t.Fatalf("read rollback.md: %v", err)
	}
	if !strings.Contains(string(body), "Decision: `risk_accepted`") {
		t.Fatalf("rollback.md = %s", string(body))
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if updated.State != string(workflow.Closeout) {
		t.Fatalf("state = %s, want closeout", updated.State)
	}
}

func TestRollbackConfirmRedeployRequiredAdvancesToDeployment(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "rollback-redeploy",
		Title: "Rollback redeploy",
		State: string(workflow.BlockedNeedReleaseDecision),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{
		"rollback", "confirm",
		"--root", root,
		"--demand", demand.ID,
		"--decision", "redeploy_required",
		"--by", "release-manager",
		"--summary", "fix and redeploy",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("rollback confirm returned error: %v", err)
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if updated.State != string(workflow.Deployment) {
		t.Fatalf("state = %s, want deployment", updated.State)
	}
}

func TestRollbackConfirmRollbackConfirmedKeepsState(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "rollback-confirmed",
		Title: "Rollback confirmed",
		State: string(workflow.BlockedNeedReleaseDecision),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{
		"rollback", "confirm",
		"--root", root,
		"--demand", demand.ID,
		"--decision", "rollback_confirmed",
		"--by", "release-manager",
		"--summary", "rollback needed",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("rollback confirm returned error: %v", err)
	}
	updated, err := store.LoadDemand(demand.ID)
	if err != nil {
		t.Fatalf("LoadDemand returned error: %v", err)
	}
	if updated.State != string(workflow.BlockedNeedReleaseDecision) {
		t.Fatalf("state = %s, want blocked_need_release_decision", updated.State)
	}
}

func TestRollbackPlanWritesPendingDecision(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "rollback-plan",
		Title: "Rollback plan",
		State: string(workflow.BlockedNeedReleaseDecision),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{
		"rollback", "plan",
		"--root", root,
		"--demand", demand.ID,
		"--trigger", "deployment failed",
		"--impact", "release blocked",
		"--recommendation", "redeploy after fix",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("rollback plan returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.RollbackFile))
	if err != nil {
		t.Fatalf("read rollback.md: %v", err)
	}
	if !strings.Contains(string(body), "Decision: `pending`") {
		t.Fatalf("rollback.md missing pending decision:\n%s", string(body))
	}
	if !strings.Contains(string(body), "deployment failed") {
		t.Fatalf("rollback.md missing trigger:\n%s", string(body))
	}
}

func TestRollbackConfirmRejectsInvalidDecision(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "rollback-invalid",
		Title: "Rollback invalid",
		State: string(workflow.BlockedNeedReleaseDecision),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{
		"rollback", "confirm",
		"--root", root,
		"--demand", demand.ID,
		"--decision", "bogus",
		"--by", "release-manager",
		"--summary", "bad",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "invalid rollback decision") {
		t.Fatalf("err = %v, want invalid decision error", err)
	}
}

func TestRollbackConfirmRejectsWrongState(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:    "rollback-wrongstate",
		Title: "Rollback wrongstate",
		State: string(workflow.Observation),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{
		"rollback", "confirm",
		"--root", root,
		"--demand", demand.ID,
		"--decision", "risk_accepted",
		"--by", "release-manager",
		"--summary", "ok",
	}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "blocked_need_release_decision") {
		t.Fatalf("err = %v, want wrong-state error", err)
	}
}
