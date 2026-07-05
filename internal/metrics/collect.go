package metrics

import (
	"strconv"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func CollectDemand(demand artifacts.Demand, events []artifacts.Event) DemandMetrics {
	out := DemandMetrics{
		DemandID:  demand.ID,
		Title:     demand.Title,
		State:     demand.State,
		CreatedAt: demand.CreatedAt,
		UpdatedAt: demand.UpdatedAt,
	}
	for i, event := range events {
		if i == 0 || event.Time.Before(out.FirstEventAt) {
			out.FirstEventAt = event.Time
		}
		if i == 0 || event.Time.After(out.LastEventAt) {
			out.LastEventAt = event.Time
		}
		applyEvent(&out, event)
	}
	if !out.CreatedAt.IsZero() && !out.UpdatedAt.IsZero() && out.UpdatedAt.After(out.CreatedAt) {
		out.TotalDuration = out.UpdatedAt.Sub(out.CreatedAt)
	} else if !out.FirstEventAt.IsZero() && !out.LastEventAt.IsZero() && out.LastEventAt.After(out.FirstEventAt) {
		out.TotalDuration = out.LastEventAt.Sub(out.FirstEventAt)
	}
	return out
}

func applyEvent(out *DemandMetrics, event artifacts.Event) {
	switch event.Type {
	case "stage.confirmed":
		out.HumanConfirmations++
	case "verification.recorded":
		out.VerificationRuns++
		switch normalizeStatus(event.Data["status"]) {
		case "pass":
			out.VerificationPasses++
		case "fail":
			out.VerificationFailures++
		}
	case "verification.evidence_recorded":
		switch normalizeStatus(event.Data["status"]) {
		case "pass":
			out.AcceptancePasses++
		case "fail":
			out.AcceptanceFailures++
		case "blocked":
			out.AcceptanceBlocked++
		}
	case "wiki.candidates_distilled":
		out.WikiCandidatesDistilled += positiveInt(event.Data["count"], 1)
	case "wiki.candidate_promoted":
		out.WikiPromoted++
	case "wiki.candidate_rejected":
		out.WikiRejected++
	case "ci_gate.blocked":
		out.CIBlocked++
	case "ci_gate.passed":
		out.CIPassed++
	}
	if strings.Contains(event.Type, "blocked") {
		out.BlockedEvents++
	}
	if isReturnEvent(event) {
		out.ReviewReturns++
		classifyReturn(out, event)
	}
}

func isReturnEvent(event artifacts.Event) bool {
	value := strings.ToLower(event.Type + " " + event.Message)
	return strings.Contains(value, "returned_to_requirements") ||
		strings.Contains(value, "returned_to_plan") ||
		strings.Contains(value, "returned_to_implementation") ||
		strings.Contains(value, "action_required")
}

func classifyReturn(out *DemandMetrics, event artifacts.Event) {
	t := strings.ToLower(event.Type)
	switch {
	case strings.Contains(t, "requirements"):
		out.RequirementsReturns++
	case strings.Contains(t, "plan"):
		out.PlanReturns++
	case strings.Contains(t, "implementation"):
		out.ImplementationReturns++
	}
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pass", "passed", "success", "ok":
		return "pass"
	case "fail", "failed", "failure", "error":
		return "fail"
	case "blocked", "block":
		return "blocked"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func positiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
