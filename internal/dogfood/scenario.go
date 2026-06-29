package dogfood

import "github.com/jesseedcp/devflow-agent/internal/demandflow"

type Scenario struct {
	Name        string
	DemandID    string
	Title       string
	Description string
	Responses   map[demandflow.Stage]demandflow.RunnerResponse
}

func CouponEligibilityScenario() Scenario {
	return Scenario{
		Name:        "coupon-eligibility",
		DemandID:    "dogfood-coupon-eligibility",
		Title:       "Dogfood coupon eligibility",
		Description: "Only active members can claim coupons once",
		Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageRequirements: {
				Text: "# Requirements: Dogfood coupon eligibility\n\n## Business Rules\n\n- Active members can claim an unexpired coupon once.\n- Inactive users are rejected with a clear reason.\n- Missing coupons are rejected with a clear reason.\n- Expired coupons are rejected with a clear reason.\n- Duplicate claims are rejected with a clear reason.\n",
			},
			demandflow.StagePlan: {
				Text: "# Technical Plan: Dogfood coupon eligibility\n\n## Implementation Shape\n\n- Add an eligibility service that checks user status, coupon existence, expiration, and duplicate claims.\n- Cover each rejection reason with focused unit tests.\n- Keep persistence behind repository interfaces so policy can be tested without a database.\n",
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
