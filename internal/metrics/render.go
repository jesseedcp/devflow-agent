package metrics

import (
	"fmt"
	"strings"
)

func RenderProject(report ProjectMetrics) string {
	var b strings.Builder
	b.WriteString("# Devflow Metrics\n\n")
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "- Demand count: %d\n", report.DemandCount)
	fmt.Fprintf(&b, "- Completed: %d\n", report.CompletedCount)
	fmt.Fprintf(&b, "- Blocked demands: %d\n", report.BlockedCount)
	fmt.Fprintf(&b, "- Human confirmations: %d\n", report.TotalHumanConfirmations)
	fmt.Fprintf(&b, "- Review returns: %d\n", report.TotalReviewReturns)
	fmt.Fprintf(&b, "- Verification runs: %d\n", report.TotalVerificationRuns)
	fmt.Fprintf(&b, "- Verification pass rate: %.0f%%\n", report.VerificationPassRate()*100)
	fmt.Fprintf(&b, "- Acceptance evidence: pass=%d fail=%d blocked=%d\n", report.TotalAcceptancePasses, report.TotalAcceptanceFailures, report.TotalAcceptanceBlocked)
	fmt.Fprintf(&b, "- Wiki decisions: %d/%d\n", report.TotalWikiPromoted+report.TotalWikiRejected, report.TotalWikiCandidates)
	fmt.Fprintf(&b, "- Partial historical data: %d\n\n", report.PartialDemandCount)
	b.WriteString("## Demands\n\n")
	b.WriteString("| Demand | Title | State | Duration | Human confirms | Review returns | Verification | Acceptance evidence | Wiki decisions |\n")
	b.WriteString("| --- | --- | --- | --- | ---: | ---: | --- | --- | --- |\n")
	for _, demand := range report.Demands {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %d | %d | %d/%d | %d/%d/%d | %d/%d |\n",
			demand.DemandID,
			escapeTableCell(demand.Title),
			demand.State,
			demand.TotalDuration,
			demand.HumanConfirmations,
			demand.ReviewReturns,
			demand.VerificationPasses,
			demand.VerificationRuns,
			demand.AcceptancePasses,
			demand.AcceptanceFailures,
			demand.AcceptanceBlocked,
			demand.WikiPromoted+demand.WikiRejected,
			demand.WikiCandidatesDistilled,
		)
	}
	b.WriteString("\n")
	if report.PartialDemandCount > 0 {
		b.WriteString("## Caveats\n\n")
		for _, demand := range report.Demands {
			if !demand.PartialData {
				continue
			}
			fmt.Fprintf(&b, "- %s: %s\n", demand.DemandID, strings.Join(demand.Caveats, "; "))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func escapeTableCell(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}
