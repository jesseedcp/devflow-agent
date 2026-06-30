package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestStatusPrintsDemandStateArtifactsAndNextActions(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Created)

	var stdout bytes.Buffer
	if err := Run([]string{"status", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("status: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Demand: add-coupon-check",
		"State: created",
		"requirements.md",
		"Next:",
		"Draft requirements",
		"devflow run --demand add-coupon-check --stage requirements",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("status output missing %q:\n%s", want, output)
		}
	}
}

func TestNextPrintsFirstRecommendedCommandOnly(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Verification)

	var stdout bytes.Buffer
	if err := Run([]string{"next", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("next: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	want := `devflow run --demand add-coupon-check --stage verification --quality-command "go test ./..."`
	if got != want {
		t.Fatalf("next = %q want %q", got, want)
	}
	if strings.Contains(got, "Confirm verification") {
		t.Fatalf("next output included extra action: %q", got)
	}
}

func TestStatusRequiresDemand(t *testing.T) {
	var stdout bytes.Buffer
	err := Run([]string{"status"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v want --demand is required", err)
	}
}

func TestNextForCompletedDemandPrintsNoCommand(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Completed)

	var stdout bytes.Buffer
	if err := Run([]string{"next", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("next: %v", err)
	}
	if !strings.Contains(stdout.String(), "No next command for add-coupon-check in state completed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestHelpIncludesStatusAndNext(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"devflow status --demand <id>",
		"devflow next --demand <id>",
		"status    Show demand state, artifacts, and next actions",
		"next      Print the next recommended command for a demand",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}

func TestRunStatusPrintsWorkspaceSummary(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "cli-workspace", Title: "CLI workspace", Description: "Show summary", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 3, 0, 0, 0, time.UTC), Type: "stage.confirmed", Message: "requirements confirmed", Data: map[string]string{"stage": "requirements"}}); err != nil {
		t.Fatalf("AppendEvent requirements returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 3, 1, 0, 0, time.UTC), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent verification returned error: %v", err)
	}

	var out strings.Builder
	if err := runStatus([]string{"--root", root, "--demand", demand.ID}, &out); err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	got := out.String()
	wantParts := []string{
		"Demand: cli-workspace",
		"Stage summary:",
		"requirements   confirmed",
		"verification   passed",
		"Artifacts:",
		"requirements.md",
		"Verification:",
		"latest: PASS go test ./...",
		"Memory:",
		"Next:",
		"devflow confirm --demand cli-workspace --stage verification",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Next actions:") {
		t.Fatalf("status output printed duplicate legacy Next actions section:\n%s", got)
	}
}

func TestRunStatusPrintsVerificationFailureKind(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "cli-fail-kind", Title: "CLI fail kind", Description: "Show failure kind", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 3, 2, 0, 0, time.UTC), Type: "verification.recorded", Message: "verification fail", Data: map[string]string{"status": "fail", "command": "go test ./...", "failure_kind": "exit_nonzero"}}); err != nil {
		t.Fatalf("AppendEvent verification returned error: %v", err)
	}

	var out strings.Builder
	if err := runStatus([]string{"--root", root, "--demand", demand.ID}, &out); err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "latest: FAIL go test ./...") {
		t.Fatalf("status output missing FAIL line:\n%s", got)
	}
	if !strings.Contains(got, "failure_kind: exit_nonzero") {
		t.Fatalf("status output missing failure kind:\n%s", got)
	}
}

func TestRunStatusAllPrintsSortedWorkspaceList(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	for _, demand := range []artifacts.Demand{
		{ID: "z-complete", Title: "Z complete", Description: "Done", Source: "test", State: string(workflow.Completed)},
		{ID: "a-failed", Title: "A failed", Description: "Failed", Source: "test", State: string(workflow.FailedQualityGate)},
		{ID: "b-verify", Title: "B verify", Description: "Verify", Source: "test", State: string(workflow.Verification)},
	} {
		if err := store.CreateDemand(demand); err != nil {
			t.Fatalf("CreateDemand(%s) returned error: %v", demand.ID, err)
		}
	}

	var out strings.Builder
	if err := runStatus([]string{"--root", root, "--all"}, &out); err != nil {
		t.Fatalf("runStatus --all returned error: %v", err)
	}
	got := out.String()
	failedIndex := strings.Index(got, "a-failed")
	verifyIndex := strings.Index(got, "b-verify")
	completedIndex := strings.Index(got, "z-complete")
	if failedIndex < 0 || verifyIndex < 0 || completedIndex < 0 {
		t.Fatalf("status --all output missing demand rows:\n%s", got)
	}
	if !(failedIndex < verifyIndex && verifyIndex < completedIndex) {
		t.Fatalf("status --all output not sorted by attention priority:\n%s", got)
	}
	if !strings.Contains(got, "quality gate failed") || !strings.Contains(got, "needs verification evidence") {
		t.Fatalf("status --all output missing attention:\n%s", got)
	}
}

func TestRunStatusAllEmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	var out strings.Builder
	if err := runStatus([]string{"--root", root, "--all"}, &out); err != nil {
		t.Fatalf("runStatus --all returned error: %v", err)
	}
	if !strings.Contains(out.String(), "No demands found") {
		t.Fatalf("empty status --all output = %q, want No demands found", out.String())
	}
}
