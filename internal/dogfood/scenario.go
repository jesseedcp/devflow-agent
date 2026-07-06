package dogfood

import "github.com/jesseedcp/devflow-agent/internal/demandflow"

type Scenario struct {
	Name        string
	DemandID    string
	Title       string
	Description string
	Codemap     string
	ChangeScope string
	Responses   map[demandflow.Stage]demandflow.RunnerResponse
}

func CouponEligibilityScenario() Scenario {
	return Scenario{
		Name:        "coupon-eligibility",
		DemandID:    "dogfood-coupon-eligibility",
		Title:       "Dogfood coupon eligibility",
		Description: "Only active members can claim coupons once",
		Codemap:     "# Codemap Context: Dogfood coupon eligibility\n\n- `internal/coupon/service.go:7` method `Eligible` score=3\n- `internal/coupon/service_test.go:5` test `TestEligibleRejectsInactiveUser` score=2\n",
		ChangeScope: "# Change Scope: Dogfood coupon eligibility\n\n## Source Files\n\n- `internal/coupon/service.go`\n\n## Test Files\n\n- `internal/coupon/service_test.go`\n\n## Out Of Scope\n\n",
		Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageRequirements: {
				Text: "# Requirements: Dogfood coupon eligibility\n\n## 目标行为\n\nActive members can claim an unexpired coupon once.\n\n## 非目标范围\n\nNo coupon issuance or refund flows.\n\n## 业务规则\n\n- Active members can claim an unexpired coupon once.\n- Inactive users are rejected with a clear reason.\n- Missing coupons are rejected with a clear reason.\n- Expired coupons are rejected with a clear reason.\n- Duplicate claims are rejected with a clear reason.\n\n## 用户/调用方影响\n\nCoupon claim API callers receive stable rejection reason codes.\n\n## 验收标准\n\n- Active members can claim one valid coupon successfully.\n- Inactive, missing, expired, and duplicate claims return distinct rejection reasons.\n\n## 风险与歧义\n\n- Coupon expiration and duplicate-claim checks must use the same clock source.\n\n## 待确认问题\n\nNone.\n\n## 人工确认记录\n\nPending.\n",
			},
			demandflow.StagePlan: {
				Text: "# Technical Plan: Dogfood coupon eligibility\n\n## 当前实现与代码事实\n\n`internal/coupon/service.go` method `Eligible` exists.\n\n## 目标设计\n\nAdd an eligibility service that checks user status, coupon existence, expiration, and duplicate claims, returning stable rejection reason codes.\n\n## 实施步骤\n\n- Update `internal/coupon/service.go` with an eligibility service that checks user status, coupon existence, expiration, and duplicate claims.\n- Keep persistence behind repository interfaces so policy can be tested without a database.\n- Return stable rejection reason codes for each failed rule.\n\n## 改动范围\n\n- `internal/coupon/service.go`\n\n## 数据结构/API/配置变化\n\nNo schema change; rejection reason codes are added.\n\n## 测试策略\n\n- Cover the successful claim path in `internal/coupon/service_test.go`.\n- Cover inactive user, missing coupon, expired coupon, and duplicate claim rejections.\n\n## 验收方式\n\nQuality gate and acceptance evidence from the coupon claim API.\n\n## 风险与回滚\n\n- Repository clock drift can make expiration tests flaky unless the clock is injectable.\n- Revert the service change to roll back.\n\n## 不做事项\n\nNo coupon issuance or refund flows.\n\n## 人工确认记录\n\nPending.\n",
			},
			demandflow.StageImplementation: {
				Text:        "## 实现摘要\n\nDeterministic dogfood records the intended backend implementation path without editing source files.\n\n## 代码改动\n\n- `internal/coupon/service.go`\n\n## 测试与验证\n\n- Quality gate executed against repository root.\n\n## 遗留问题\n\nNone.\n",
				ToolSummary: []string{"quality gate executed against repository root"},
			},
			demandflow.StageVerification: {
				Text: "# Verification: Dogfood coupon eligibility\n\n## 验收标准映射\n\n- Active claim success mapped to the eligible-member path.\n- Inactive, missing, expired, and duplicate rejections mapped to distinct reason codes.\n\n## 自动化测试结果\n\nQuality gate executed against repository root.\n\n## 手动验证记录\n\nNone.\n\n## 接口/日志/监控证据\n\n- Requirements, plan, implementation notes, MR review gate, quality gate, and closeout were exercised by the deterministic dogfood runner.\n\n## 未覆盖风险\n\nNone.\n\n## 结论\n\nPass.\n",
			},
			demandflow.StageCloseout: {
				Text: "# Closeout: Dogfood coupon eligibility\n\n## 需求结果\n\nThe full backend-demand workflow completed deterministically for the coupon eligibility scenario.\n\n## 关键产物链接\n\n- requirements.md\n- plan.md\n- verification.md\n\n## MR 评论与处理摘要\n\nNone.\n\n## 验收证据摘要\n\nQuality gate and acceptance evidence passed.\n\n## 稳定知识候选\n\n- Deterministic dogfood should cover every workflow gate before a release is considered ready.\n\n## 流程改进候选\n\nNone.\n\n## 一次性材料归档\n\n- closeout.md\n\n## 人工确认记录\n\nPending.\n\n---DEVFLOW-MEMORY-CANDIDATES---\n\n# Memory Candidates: Dogfood coupon eligibility\n\n## 稳定知识候选\n\n- Deterministic dogfood should cover every workflow gate before a release is considered ready.\n\n## 流程改进候选\n\nNone.\n\n## 不进入长期知识的材料\n\nNone.\n",
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
