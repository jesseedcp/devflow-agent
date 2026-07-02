package dogfood

import "github.com/jesseedcp/devflow-agent/internal/demandflow"

type Scenario struct {
	Name        string
	DemandID    string
	Title       string
	Description string
	Codemap     string
	Responses   map[demandflow.Stage]demandflow.RunnerResponse
}

func CouponEligibilityScenario() Scenario {
	return Scenario{
		Name:        "coupon-eligibility",
		DemandID:    "dogfood-coupon-eligibility",
		Title:       "Dogfood coupon eligibility",
		Description: "Only active members can claim coupons once",
		Codemap:     "# Codemap Context: Dogfood coupon eligibility\n\n- `internal/coupon/service.go:7` method `Eligible` score=3\n- `internal/coupon/service_test.go:5` test `TestEligibleRejectsInactiveUser` score=2\n",
		Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageRequirements: {
				Text: "# Requirements: Dogfood coupon eligibility\n\n## Business Rules\n\n- Active members can claim an unexpired coupon once.\n- Inactive users are rejected with a clear reason.\n- Missing coupons are rejected with a clear reason.\n- Expired coupons are rejected with a clear reason.\n- Duplicate claims are rejected with a clear reason.\n\n## Acceptance Criteria\n\n- Active members can claim one valid coupon successfully.\n- Inactive, missing, expired, and duplicate claims return distinct rejection reasons.\n\n## Risks\n\n- Coupon expiration and duplicate-claim checks must use the same clock source.\n",
			},
			demandflow.StagePlan: {
				Text: "# Technical Plan: Dogfood coupon eligibility\n\n## Implementation Steps\n\n- Update `internal/coupon/service.go` with an eligibility service that checks user status, coupon existence, expiration, and duplicate claims.\n- Keep persistence behind repository interfaces so policy can be tested without a database.\n- Return stable rejection reason codes for each failed rule.\n\n## Test Strategy\n\n- Cover the successful claim path in `internal/coupon/service_test.go`.\n- Cover inactive user, missing coupon, expired coupon, and duplicate claim rejections.\n\n## Risks\n\n- Repository clock drift can make expiration tests flaky unless the clock is injectable.\n",
			},
			demandflow.StageImplementation: {
				Text:        "## Implementation Summary\n\nDeterministic dogfood records the intended backend implementation path without editing source files.\n",
				ToolSummary: []string{"quality gate executed against repository root"},
			},
			demandflow.StageVerification: {
				Text: "# Verification: Dogfood coupon eligibility\n\n## Evidence\n\n- Requirements, plan, implementation notes, MR review gate, quality gate, and closeout were exercised by the deterministic dogfood runner.\n",
			},
			demandflow.StageCloseout: {
				Text: "# Closeout: Dogfood coupon eligibility\n\n## Demand Result\n\nThe full backend-demand workflow completed deterministically for the coupon eligibility scenario.\n\n---DEVFLOW-MEMORY-CANDIDATES---\n\n# Memory Candidates: Dogfood coupon eligibility\n\n## Stable Knowledge Candidates\n\n- Deterministic dogfood should cover every workflow gate before a release is considered ready.\n",
			},
		},
	}
}

func ScenarioByName(name string) (Scenario, bool) {
	switch name {
	case "", "coupon-eligibility":
		return CouponEligibilityScenario(), true
	default:
		return Scenario{}, false
	}
}
