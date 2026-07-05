package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestRenderProjectMetricsIncludesSummaryAndDemandRows(t *testing.T) {
	report := ProjectMetrics{
		DemandCount:               2,
		CompletedCount:            1,
		TotalHumanConfirmations:   3,
		TotalReviewReturns:        1,
		TotalVerificationRuns:     2,
		TotalVerificationPasses:   1,
		TotalVerificationFailures: 1,
		TotalAcceptancePasses:     1,
		TotalWikiCandidates:       3,
		TotalWikiPromoted:         2,
		TotalWikiRejected:         1,
		Demands: []DemandMetrics{
			{DemandID: "coupon", Title: "Coupon", State: "completed", TotalDuration: 2 * time.Hour, HumanConfirmations: 2, ReviewReturns: 1, VerificationRuns: 1, VerificationPasses: 1, AcceptancePasses: 1, WikiCandidatesDistilled: 2, WikiPromoted: 1, WikiRejected: 1},
			{DemandID: "refund", Title: "Refund", State: "verification", TotalDuration: time.Hour, HumanConfirmations: 1, VerificationRuns: 1, VerificationFailures: 1, WikiCandidatesDistilled: 1, WikiPromoted: 1},
		},
	}

	body := RenderProject(report)
	for _, want := range []string{
		"# Devflow Metrics",
		"Demand count: 2",
		"Completed: 1",
		"Human confirmations: 3",
		"Review returns: 1",
		"Verification pass rate: 50%",
		"Wiki decisions: 3/3",
		"| coupon | Coupon | completed | 2h0m0s | 2 | 1 | 1/1 | 1/0/0 | 2/2 |",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics report missing %q:\n%s", want, body)
		}
	}
}
