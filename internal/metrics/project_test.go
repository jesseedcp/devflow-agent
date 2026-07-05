package metrics

import (
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestCollectProjectMetricsScansDemands(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	first := artifacts.Demand{ID: "first-demand", Title: "First", Description: "first", State: "completed", CreatedAt: time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 5, 2, 0, 0, 0, time.UTC)}
	second := artifacts.Demand{ID: "second-demand", Title: "Second", Description: "second", State: "verification", CreatedAt: time.Date(2026, 7, 5, 3, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 5, 4, 0, 0, 0, time.UTC)}
	for _, demand := range []artifacts.Demand{first, second} {
		if err := store.CreateDemand(demand); err != nil {
			t.Fatalf("CreateDemand(%s) returned error: %v", demand.ID, err)
		}
	}
	if err := store.AppendEvent(first.ID, artifacts.Event{Type: "stage.confirmed", Data: map[string]string{"stage": "requirements"}}); err != nil {
		t.Fatalf("AppendEvent first confirmation returned error: %v", err)
	}
	if err := store.AppendEvent(first.ID, artifacts.Event{Type: "verification.recorded", Data: map[string]string{"status": "pass"}}); err != nil {
		t.Fatalf("AppendEvent first verification returned error: %v", err)
	}
	if err := store.AppendEvent(second.ID, artifacts.Event{Type: "verification.recorded", Data: map[string]string{"status": "fail"}}); err != nil {
		t.Fatalf("AppendEvent second verification returned error: %v", err)
	}

	got, err := CollectProject(root)
	if err != nil {
		t.Fatalf("CollectProject returned error: %v", err)
	}
	if got.DemandCount != 2 || got.CompletedCount != 1 {
		t.Fatalf("counts = demands %d completed %d", got.DemandCount, got.CompletedCount)
	}
	if got.TotalHumanConfirmations != 1 {
		t.Fatalf("TotalHumanConfirmations = %d, want 1", got.TotalHumanConfirmations)
	}
	if got.TotalVerificationRuns != 2 || got.TotalVerificationPasses != 1 || got.TotalVerificationFailures != 1 {
		t.Fatalf("verification totals = runs %d pass %d fail %d", got.TotalVerificationRuns, got.TotalVerificationPasses, got.TotalVerificationFailures)
	}
	if len(got.Demands) != 2 || got.Demands[0].DemandID != "first-demand" || got.Demands[1].DemandID != "second-demand" {
		t.Fatalf("demands = %#v", got.Demands)
	}
}
