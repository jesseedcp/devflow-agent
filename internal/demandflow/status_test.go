package demandflow

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestNextActionsMapStatesToCommands(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state   workflow.State
		want    string
		command string
	}{
		{workflow.Created, "Draft requirements", "devflow run --demand add-coupon-check --stage requirements"},
		{workflow.RequirementsReview, "Confirm requirements", "devflow confirm --demand add-coupon-check --stage requirements"},
		{workflow.PlanDrafting, "Draft plan", "devflow run --demand add-coupon-check --stage plan"},
		{workflow.FailedQualityGate, "Retry implementation", "devflow run --demand add-coupon-check --stage implementation"},
		{workflow.MRReview, "Check MR review", "devflow run --demand add-coupon-check --stage mr-review"},
		{workflow.Completed, "No action", ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.state), func(t *testing.T) {
			actions := NextActions(tc.state, "add-coupon-check")
			if len(actions) == 0 {
				t.Fatalf("%s: no actions", tc.state)
			}
			if actions[0].Label != tc.want {
				t.Fatalf("%s: label = %q, want %q", tc.state, actions[0].Label, tc.want)
			}
			if tc.command != "" && !strings.Contains(actions[0].Command, tc.command) {
				t.Fatalf("%s: command = %q, want contains %q", tc.state, actions[0].Command, tc.command)
			}
		})
	}
}

func TestInspectStatusLoadsDemandArtifactsAndActions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "add-coupon-check",
		Title:       "coupon flow",
		Description: "coupon flow",
		Source:      "manual",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	report, err := InspectStatus(root, "add-coupon-check")
	if err != nil {
		t.Fatalf("inspect status: %v", err)
	}
	if report.State != workflow.Created {
		t.Fatalf("state = %s want %s", report.State, workflow.Created)
	}
	if report.DemandDir != filepath.Join(root, ".devflow", "demands", "add-coupon-check") {
		t.Fatalf("demand dir = %q", report.DemandDir)
	}
	if len(report.Actions) == 0 || report.Actions[0].Label != "Draft requirements" {
		t.Fatalf("actions = %#v", report.Actions)
	}

	foundRequirements := false
	for _, artifact := range report.Artifacts {
		if artifact.Name == artifacts.RequirementsFile {
			foundRequirements = true
			if !artifact.Exists || artifact.Size == 0 {
				t.Fatalf("requirements artifact = %#v", artifact)
			}
		}
	}
	if !foundRequirements {
		t.Fatalf("requirements artifact missing from %#v", report.Artifacts)
	}
}

func TestNextActionsQuoteDemandID(t *testing.T) {
	t.Parallel()

	actions := NextActions(workflow.Created, `coupon "flow"`)
	if !strings.Contains(actions[0].Command, `"coupon \"flow\""`) {
		t.Fatalf("command = %q", actions[0].Command)
	}
}

func TestNextActionsForMRReviewReturns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state workflow.State
		label string
		want  string
	}{
		{workflow.ReturnedToRequirements, "Revise requirements", "devflow run --demand add-coupon-check --stage requirements"},
		{workflow.ReturnedToPlan, "Revise plan", "devflow run --demand add-coupon-check --stage plan"},
		{workflow.Implementation, "Run implementation", "devflow run --demand add-coupon-check --stage implementation"},
	}

	for _, tc := range tests {
		t.Run(string(tc.state), func(t *testing.T) {
			actions := NextActions(tc.state, "add-coupon-check")
			if len(actions) == 0 {
				t.Fatal("expected action")
			}
			if actions[0].Label != tc.label {
				t.Fatalf("label = %q, want %q", actions[0].Label, tc.label)
			}
			if !strings.Contains(actions[0].Command, tc.want) {
				t.Fatalf("command = %q, want contains %q", actions[0].Command, tc.want)
			}
		})
	}
}

func TestInspectStatusVerificationPassPrefersConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          "status-pass-next",
		Title:       "Status pass next",
		Description: "Pass evidence next action",
		Source:      "test",
		State:       string(workflow.Verification),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 2, 0, 0, 0, time.UTC), Type: "verification.recorded", Message: "pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	report, err := InspectStatus(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectStatus returned error: %v", err)
	}
	if len(report.Actions) == 0 {
		t.Fatal("InspectStatus returned no actions")
	}
	if report.Actions[0].Label != "Confirm verification" {
		t.Fatalf("first action = %#v, want Confirm verification", report.Actions[0])
	}
}

func TestInspectStatusCompletedWithPendingMemoryPrefersMemoryReview(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          "status-memory-next",
		Title:       "Status memory next",
		Description: "Memory next action",
		Source:      "test",
		State:       string(workflow.Completed),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.MemoryCandidatesFile, "# Memory Candidates: Status memory next\n\n## 稳定知识候选\n\n- Reuse this product rule\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	report, err := InspectStatus(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectStatus returned error: %v", err)
	}
	if len(report.Actions) == 0 {
		t.Fatal("InspectStatus returned no actions")
	}
	if report.Actions[0].Label != "Review memory candidates" {
		t.Fatalf("first action = %#v, want Review memory candidates", report.Actions[0])
	}
}
