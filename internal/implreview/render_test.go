package implreview

import (
	"strings"
	"testing"
)

func TestRenderReviewIncludesScopeVerificationAndRecommendation(t *testing.T) {
	review := Review{
		DemandID:            "coupon",
		DeclaredSource:      []string{"internal/coupon/service.go"},
		DeclaredTests:       []string{"internal/coupon/service_test.go"},
		ChangedFiles:        []string{"internal/coupon/service.go", "README.md"},
		InScope:             []string{"internal/coupon/service.go"},
		OutOfScope:          []string{"README.md"},
		MissingTests:        []string{"internal/coupon/service_test.go"},
		VerificationStatus:  "pass",
		VerificationCommand: "go test ./...",
		AcceptancePass:      1,
		MRStatus:            "cleared",
		Recommendation:      "needs_scope_review",
	}
	got := Render(review)
	for _, want := range []string{"# Implementation Review: coupon", "README.md", "missing_tests=1", "verification=pass", "needs_scope_review"} {
		if !strings.Contains(got, want) {
			t.Fatalf("review missing %q:\n%s", want, got)
		}
	}
}
