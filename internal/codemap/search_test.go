package codemap

import "testing"

func TestSearchRanksMatchingFacts(t *testing.T) {
	idx := Index{Facts: []CodeFact{
		{Kind: "func", File: "internal/coupon/service.go", Name: "CheckEligibility", Text: "coupon eligibility inactive user"},
		{Kind: "func", File: "internal/user/profile.go", Name: "RenderProfile", Text: "profile avatar"},
		{Kind: "test", File: "internal/coupon/service_test.go", Name: "TestCheckEligibilityInactiveUser", Text: "inactive coupon test"},
	}}
	got := Search(idx, "coupon eligibility inactive", 10)
	if len(got) != 2 {
		t.Fatalf("Search returned %d results, want 2", len(got))
	}
	if got[0].Fact.File != "internal/coupon/service.go" {
		t.Fatalf("top result file = %q", got[0].Fact.File)
	}
	if got[1].Fact.Kind != "test" {
		t.Fatalf("second result kind = %q, want test", got[1].Fact.Kind)
	}
}
