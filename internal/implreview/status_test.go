package implreview

import "testing"

func TestRecommendNeedsImplementationDiffWhenNoChangedFiles(t *testing.T) {
	review := Review{VerificationStatus: "pass", AcceptancePass: 1, MRStatus: "cleared"}
	if got := Recommend(review); got != "needs_implementation_diff" {
		t.Fatalf("Recommend = %q, want needs_implementation_diff", got)
	}
}

func TestRecommendReadyWithChangedFiles(t *testing.T) {
	review := Review{
		ChangedFiles:       []string{"internal/coupon/service.go"},
		VerificationStatus: "pass",
		AcceptancePass:     1,
		MRStatus:           "cleared",
	}
	if got := Recommend(review); got != "ready_for_closeout" {
		t.Fatalf("Recommend = %q, want ready_for_closeout", got)
	}
}