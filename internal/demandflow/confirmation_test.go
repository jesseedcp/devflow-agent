package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestConfirmAdvancesRequirementsAndWritesEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "confirm-requirements",
		Title:       "Confirm requirements",
		Description: "Confirm requirements",
		Source:      "test",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	setDemandState(t, store, "confirm-requirements", workflow.RequirementsReview)

	result, err := Confirm(ConfirmOptions{
		Root:     root,
		DemandID: "confirm-requirements",
		Stage:    "requirements",
		By:       "alice\nadmin",
		Summary:  "looks\nright",
		Now:      fixedNow,
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if result.PreviousState != workflow.RequirementsReview || result.CurrentState != workflow.PlanDrafting {
		t.Fatalf("states = %s -> %s, want requirements_review -> plan_drafting", result.PreviousState, result.CurrentState)
	}
	demand, err := store.LoadDemand("confirm-requirements")
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	if demand.State != string(workflow.PlanDrafting) {
		t.Fatalf("state = %q, want %q", demand.State, workflow.PlanDrafting)
	}
	body, err := os.ReadFile(filepath.Join(store.DemandDir("confirm-requirements"), artifacts.RequirementsFile))
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "requirements confirmed by alice admin") {
		t.Fatalf("requirements evidence missing normalized confirmer:\n%s", text)
	}
	if !strings.Contains(text, "looks right") {
		t.Fatalf("requirements evidence missing normalized summary:\n%s", text)
	}
}

func TestConfirmRejectsWrongStateWithoutMutating(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "wrong-state",
		Title:       "Wrong state",
		Description: "Wrong state",
		Source:      "test",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	_, err := Confirm(ConfirmOptions{Root: root, DemandID: "wrong-state", Stage: "plan", By: "alice", Summary: "ok", Now: fixedNow})
	if err == nil || !strings.Contains(err.Error(), "requires current state plan_review") {
		t.Fatalf("err = %v, want wrong-state error", err)
	}
	demand, loadErr := store.LoadDemand("wrong-state")
	if loadErr != nil {
		t.Fatalf("load demand: %v", loadErr)
	}
	if demand.State != string(workflow.Created) {
		t.Fatalf("state mutated to %q", demand.State)
	}
}

func TestConfirmationIDIsStableAndSensitiveToCycleToken(t *testing.T) {
	left := ConfirmationID("demand", "requirements", "cycle-one", "alice", "ok")
	right := ConfirmationID("demand", "requirements", "cycle-two", "alice", "ok")
	if left == right {
		t.Fatalf("confirmation ids should differ across cycles: %s", left)
	}
	if again := ConfirmationID("demand", "requirements", "cycle-one", "alice", "ok"); again != left {
		t.Fatalf("confirmation id not stable: %s then %s", left, again)
	}
}
