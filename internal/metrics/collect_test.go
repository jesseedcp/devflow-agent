package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestCollectDemandMetricsCountsEvents(t *testing.T) {
	created := time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC)
	demand := artifacts.Demand{
		ID:        "coupon-metrics",
		Title:     "Coupon metrics",
		State:     "completed",
		CreatedAt: created,
		UpdatedAt: created.Add(2 * time.Hour),
	}
	events := []artifacts.Event{
		{Time: created, Type: "demand.created", Message: "created"},
		{Time: created.Add(10 * time.Minute), Type: "stage.confirmed", Data: map[string]string{"stage": "requirements"}},
		{Time: created.Add(20 * time.Minute), Type: "stage.confirmed", Data: map[string]string{"stage": "plan"}},
		{Time: created.Add(30 * time.Minute), Type: "mr_review.action_required", Message: "returned to plan"},
		{Time: created.Add(40 * time.Minute), Type: "demand.returned_to_plan", Message: "returned to plan"},
		{Time: created.Add(50 * time.Minute), Type: "verification.recorded", Data: map[string]string{"status": "pass"}},
		{Time: created.Add(55 * time.Minute), Type: "verification.recorded", Data: map[string]string{"status": "fail"}},
		{Time: created.Add(60 * time.Minute), Type: "verification.evidence_recorded", Data: map[string]string{"status": "pass", "type": "api"}},
		{Time: created.Add(70 * time.Minute), Type: "verification.evidence_recorded", Data: map[string]string{"status": "blocked", "type": "link"}},
		{Time: created.Add(80 * time.Minute), Type: "wiki.candidates_distilled", Data: map[string]string{"count": "3"}},
		{Time: created.Add(90 * time.Minute), Type: "wiki.candidate_promoted"},
		{Time: created.Add(100 * time.Minute), Type: "wiki.candidate_rejected"},
		{Time: created.Add(110 * time.Minute), Type: "ci_gate.blocked"},
		{Time: created.Add(120 * time.Minute), Type: "ci_gate.passed"},
	}

	got := CollectDemand(demand, events)

	if got.DemandID != "coupon-metrics" || got.Title != "Coupon metrics" {
		t.Fatalf("identity = %s/%s", got.DemandID, got.Title)
	}
	if got.HumanConfirmations != 2 {
		t.Fatalf("HumanConfirmations = %d, want 2", got.HumanConfirmations)
	}
	if got.ReviewReturns != 2 || got.PlanReturns != 1 {
		t.Fatalf("returns = review %d plan %d, want review 2 plan 1", got.ReviewReturns, got.PlanReturns)
	}
	if got.VerificationRuns != 2 || got.VerificationPasses != 1 || got.VerificationFailures != 1 {
		t.Fatalf("verification = runs %d pass %d fail %d", got.VerificationRuns, got.VerificationPasses, got.VerificationFailures)
	}
	if got.AcceptancePasses != 1 || got.AcceptanceBlocked != 1 {
		t.Fatalf("acceptance = pass %d blocked %d", got.AcceptancePasses, got.AcceptanceBlocked)
	}
	if got.WikiCandidatesDistilled != 3 || got.WikiPromoted != 1 || got.WikiRejected != 1 {
		t.Fatalf("wiki = distilled %d promoted %d rejected %d", got.WikiCandidatesDistilled, got.WikiPromoted, got.WikiRejected)
	}
	if got.CIBlocked != 1 || got.CIPassed != 1 {
		t.Fatalf("ci = blocked %d passed %d", got.CIBlocked, got.CIPassed)
	}
	if got.TotalDuration != 2*time.Hour {
		t.Fatalf("TotalDuration = %s, want 2h", got.TotalDuration)
	}
}

func TestCollectDemandMetricsMarksPartialHistoricalData(t *testing.T) {
	created := time.Date(2026, 6, 20, 1, 0, 0, 0, time.UTC)
	demand := artifacts.Demand{
		ID:        "old-demand",
		Title:     "Old demand",
		State:     "completed",
		CreatedAt: created,
		UpdatedAt: created.Add(time.Hour),
	}
	events := []artifacts.Event{
		{Time: created, Type: "demand.created", Message: "created"},
		{Time: created.Add(10 * time.Minute), Type: "stage.confirmed", Data: map[string]string{"stage": "requirements"}},
	}
	got := CollectDemand(demand, events)
	if !got.PartialData {
		t.Fatal("PartialData = false, want true")
	}
	joined := strings.Join(got.Caveats, " | ")
	for _, want := range []string{"no verification events", "no acceptance evidence events", "no wiki decision events"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("caveats %q missing %q", joined, want)
		}
	}
}
