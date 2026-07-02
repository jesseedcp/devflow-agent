package codemap

import (
	"strings"
	"testing"
)

func TestRenderSummaryIncludesSourceAndTestFacts(t *testing.T) {
	idx := Index{Facts: []CodeFact{
		{Kind: "method", File: "internal/coupon/service.go", Name: "CheckEligibility", Line: 7},
		{Kind: "test", File: "internal/coupon/service_test.go", Name: "TestCheckEligibilityInactiveUser", Line: 5},
	}}
	got := RenderSummary(idx)
	for _, want := range []string{"# Codemap Summary", "internal/coupon/service.go", "CheckEligibility", "TestCheckEligibilityInactiveUser"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q:\n%s", want, got)
		}
	}
}
