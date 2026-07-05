package demandflow

import (
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestEvaluateRequirementsPassesWithRequiredSections(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-req", Title: "Eval req", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	body := "# Requirements: Eval req\n\n## 业务规则\n\n- active member only\n\n## 验收标准\n\n- active member can claim\n\n## 风险与歧义\n\n- coupon limits unclear\n"
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, body); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	eval, err := EvaluateDemand(root, demand.ID, StageRequirements)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	stage := eval.Stages[0]
	if stage.Status != EvaluationPass {
		t.Fatalf("requirements status = %s, want pass; checks=%#v", stage.Status, stage.Checks)
	}
}

func TestEvaluateRequirementsFailsMissingAcceptanceCriteria(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-req-fail", Title: "Eval req fail", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- rule\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	eval, err := EvaluateDemand(root, demand.ID, StageRequirements)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationFail {
		t.Fatalf("requirements status = %s, want fail", eval.Stages[0].Status)
	}
	if eval.Stages[0].Blockers != 1 {
		t.Fatalf("Blockers = %d, want 1", eval.Stages[0].Blockers)
	}
}

func TestEvaluatePlanWarnsWithoutTestStrategy(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-plan", Title: "Eval plan", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanFile, "# Plan\n\n## 实施步骤\n\n- build it\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	eval, err := EvaluateDemand(root, demand.ID, StagePlan)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationWarning {
		t.Fatalf("plan status = %s, want warning", eval.Stages[0].Status)
	}
}

func TestEvaluatePlanWarnsWhenCodemapContextMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "codemap-plan", Title: "Codemap Plan", Description: "Evaluate codemap plan", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanFile, "# Plan\n\n## Implementation Steps\n\n- Update coupon logic.\n\n## Test Strategy\n\n- Run tests.\n\n## Risks\n\n- Missing impacted files.\n"); err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateDemand(root, demand.ID, StagePlan)
	if err != nil {
		t.Fatal(err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "plan.codemap_reference")
	if check.Status != EvaluationWarning {
		t.Fatalf("plan.codemap_reference = %s, want warning", check.Status)
	}
}

func TestEvaluatePlanPassesWhenPlanReferencesCodemapFiles(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "codemap-plan-pass", Title: "Codemap Plan Pass", Description: "Evaluate codemap plan", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CodemapFile, "# Codemap Context\n\n- `internal/coupon/service.go:7` method `CheckEligibility` score=3\n- `internal/coupon/service_test.go:5` test `TestCheckEligibilityInactiveUser` score=2\n"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanFile, "# Plan\n\n## Implementation Steps\n\n- Update `internal/coupon/service.go`.\n\n## Test Strategy\n\n- Add `internal/coupon/service_test.go` coverage.\n\n## Risks\n\n- Keep inactive users blocked.\n"); err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateDemand(root, demand.ID, StagePlan)
	if err != nil {
		t.Fatal(err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "plan.codemap_reference")
	if check.Status != EvaluationPass {
		t.Fatalf("plan.codemap_reference = %s, want pass evidence=%s", check.Status, check.Evidence)
	}
}

func TestEvaluateVerificationFailsWithoutPassEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-verification", Title: "Eval verification", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC), Type: "verification.recorded", Message: "fail", Data: map[string]string{"status": "fail", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	eval, err := EvaluateDemand(root, demand.ID, StageVerification)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationFail {
		t.Fatalf("verification status = %s, want fail", eval.Stages[0].Status)
	}
}

func TestEvaluateVerificationAcceptsUppercasePassEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-verification-pass", Title: "Eval verification pass", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC), Type: "verification.recorded", Message: "pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 9, 1, 0, 0, time.UTC), Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	eval, err := EvaluateDemand(root, demand.ID, StageVerification)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationPass {
		t.Fatalf("verification status = %s, want pass", eval.Stages[0].Status)
	}
}

func TestEvaluateCloseoutWarnsWithoutMemoryCandidates(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-closeout", Title: "Eval closeout", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CloseoutFile, "# Closeout\n\n## 需求结果\n\n- shipped\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.MemoryCandidatesFile, "# Memory\n\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ObservationFile, "# Observation\n\nStatus: `passed`\n"); err != nil {
		t.Fatalf("WriteArtifact observation returned error: %v", err)
	}

	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationWarning {
		t.Fatalf("closeout status = %s, want warning", eval.Stages[0].Status)
	}
}
func TestEvaluateRequirementsChecksIntakeCoverage(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-intake-coverage", Title: "Eval intake coverage", Description: "coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, `# Intake: Coupon

## 原始需求材料

## 目标
- Active members can claim coupons.

## 业务规则
- User status must be active.

## 验收标准
- Inactive users are blocked.
`); err != nil {
		t.Fatalf("WriteArtifact intake returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, `# Requirements: Coupon

## 目标行为

- Active members can claim coupons.

## 非目标范围

- 待人工补充。

## 业务规则

- User status must be active.

## 用户/调用方影响

- 待确认。

## 验收标准

- Inactive users are blocked.

## 风险与歧义

- 待确认。

## 待确认问题

- Confirm inactive error code.

## 人工确认记录
`); err != nil {
		t.Fatalf("WriteArtifact requirements returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageRequirements)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "requirements.intake_coverage")
	if check.Status != EvaluationPass {
		t.Fatalf("intake coverage status = %s, evidence=%q", check.Status, check.Evidence)
	}
}

func TestEvaluateRequirementsWarnsOnMissingIntakeCoverage(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-intake-missing", Title: "Eval intake missing", Description: "coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, `# Intake: Coupon

## 原始需求材料

## 验收标准
- Inactive users are blocked.
`); err != nil {
		t.Fatalf("WriteArtifact intake returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, `# Requirements: Coupon

## 业务规则

- User status must be active.

## 验收标准

- Active users can claim coupons.
`); err != nil {
		t.Fatalf("WriteArtifact requirements returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageRequirements)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "requirements.intake_coverage")
	if check.Status != EvaluationWarning {
		t.Fatalf("intake coverage status = %s, want warning", check.Status)
	}
	if !strings.Contains(check.Evidence, "Inactive users are blocked") {
		t.Fatalf("evidence = %q, want missing intake bullet", check.Evidence)
	}
}

func TestEvaluateRequirementsChecksContextMemorySafety(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-context-safety", Title: "Eval context safety", Description: "coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, "# Context: Coupon\n\n## Approved Stable Memory\n\n- `memory/coupon.md`: Coupon active member checks must happen before claim writes.\n\n## Historical Demand Candidates\n\n- `coupon-old`: Candidate says expired coupons may use a generic error.\n"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, `# Requirements: Coupon

## 目标行为

- Coupon active member checks must happen before claim writes.

## 业务规则

- User status must be active.

## 验收标准

- Inactive users are blocked.

## 待确认问题

- Confirm whether expired coupons use a generic error.
`); err != nil {
		t.Fatalf("WriteArtifact requirements returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageRequirements)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	stable := findEvaluationCheck(t, evaluation.Stages[0], "requirements.stable_memory_reference")
	if stable.Status != EvaluationPass {
		t.Fatalf("stable memory reference status = %s evidence=%q", stable.Status, stable.Evidence)
	}
	candidate := findEvaluationCheck(t, evaluation.Stages[0], "requirements.candidate_guard")
	if candidate.Status != EvaluationPass {
		t.Fatalf("candidate guard status = %s evidence=%q", candidate.Status, candidate.Evidence)
	}
}

func TestEvaluateRequirementsWarnsWhenCandidateMemoryHasNoQuestion(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-candidate-no-question", Title: "Eval candidate no question", Description: "coupon eligibility", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, "# Context: Coupon\n\n## Approved Stable Memory\n\nNo approved stable memory recalled.\n\n## Historical Demand Candidates\n\n- `coupon-old`: Candidate says expired coupons may use a generic error.\n"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, `# Requirements: Coupon

## 业务规则

- User status must be active.

## 验收标准

- Inactive users are blocked.

## 待确认问题

- 待人工补充。
`); err != nil {
		t.Fatalf("WriteArtifact requirements returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageRequirements)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "requirements.candidate_guard")
	if check.Status != EvaluationWarning {
		t.Fatalf("candidate guard status = %s, want warning", check.Status)
	}
}

func findEvaluationCheck(t *testing.T, stage StageEvaluation, id string) EvaluationCheck {
	t.Helper()
	for _, check := range stage.Checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("check %s missing from %#v", id, stage.Checks)
	return EvaluationCheck{}
}

func TestEvaluatePlanWarnsWhenPlanContextMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "plan-context-missing", Title: "Plan Context Missing", Description: "Evaluate plan context", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanFile, "# Plan\n\n## Implementation Steps\n\n- Update coupon logic.\n\n## Test Strategy\n\n- Run tests.\n\n## Risks\n\n- Unknown.\n"); err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateDemand(root, demand.ID, StagePlan)
	if err != nil {
		t.Fatal(err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "plan.context_grounding")
	if check.Status != EvaluationWarning {
		t.Fatalf("plan.context_grounding = %s, want warning", check.Status)
	}
}

func TestEvaluatePlanPassesWithPlanContextAndChangeScope(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "plan-context-pass", Title: "Plan Context Pass", Description: "Evaluate plan context", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanContextFile, "# Plan Context\n\n## Codemap Context\n\n- `internal/coupon/service.go:7` method `CheckEligibility`\n"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ChangeScopeFile, "# Change Scope\n\n## Source Files\n\n- `internal/coupon/service.go`\n\n## Test Files\n\n- `internal/coupon/service_test.go`\n"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanFile, "# Plan\n\n## Implementation Steps\n\n- Update `internal/coupon/service.go`.\n\n## Test Strategy\n\n- Add `internal/coupon/service_test.go`.\n\n## Risks\n\n- Keep behavior compatible.\n"); err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateDemand(root, demand.ID, StagePlan)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"plan.context_grounding", "plan.change_scope"} {
		check := findEvaluationCheck(t, evaluation.Stages[0], id)
		if check.Status != EvaluationPass {
			t.Fatalf("%s = %s, want pass evidence=%s", id, check.Status, check.Evidence)
		}
	}
}

func TestEvaluateImplementationWarnsWhenImplementationReviewMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "implementation-review-missing", Title: "Implementation review missing", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateDemand(root, demand.ID, StageImplementation)
	if err != nil {
		t.Fatal(err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "implementation.review")
	if check.Status != EvaluationWarning {
		t.Fatalf("implementation.review = %s, want warning", check.Status)
	}
}

func TestEvaluateImplementationPassesReadyReview(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "implementation-review-pass", Title: "Implementation review pass", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ImplementationReviewFile, "# Implementation Review: implementation-review-pass\n\nRecommendation: `ready_for_closeout`\n\nSummary: in_scope=2 out_of_scope=0 missing_tests=0 verification=pass acceptance_pass=1 acceptance_fail=0 acceptance_blocked=0 mr=cleared\n"); err != nil {
		t.Fatal(err)
	}
	evaluation, err := EvaluateDemand(root, demand.ID, StageImplementation)
	if err != nil {
		t.Fatal(err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "implementation.review")
	if check.Status != EvaluationPass {
		t.Fatalf("implementation.review = %s, want pass evidence=%s", check.Status, check.Evidence)
	}
}

func TestEvaluateVerificationWarnsWhenManualEvidenceMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-manual-missing", Title: "Eval manual missing", Description: "Eval", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageVerification)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "verification.acceptance_evidence")
	if check.Status != EvaluationWarning {
		t.Fatalf("manual evidence status = %s, want warning", check.Status)
	}
}

func TestEvaluateVerificationFailsOnFailedManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-manual-fail", Title: "Eval manual fail", Description: "Eval", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent verification returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Message: "manual evidence fail", Data: map[string]string{"status": "fail", "type": "api", "criterion": "Inactive users are blocked", "summary": "Unexpected success"}}); err != nil {
		t.Fatalf("AppendEvent manual evidence returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageVerification)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "verification.acceptance_evidence_pass")
	if check.Status != EvaluationFail {
		t.Fatalf("manual evidence pass status = %s, want fail", check.Status)
	}
	if evaluation.Stages[0].Status != EvaluationFail {
		t.Fatalf("stage status = %s, want fail", evaluation.Stages[0].Status)
	}
}

func TestEvaluateCloseoutWikiCandidatesWarnsWhenTemplateOnly(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "wiki-template", Title: "Wiki template", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CloseoutFile, "# Closeout\n\n## 需求结果\n\n- shipped\n"); err != nil {
		t.Fatal(err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatal(err)
	}
	candidates := findEvaluationCheck(t, eval.Stages[0], "closeout.wiki_candidates")
	if candidates.Status != EvaluationWarning {
		t.Fatalf("closeout.wiki_candidates = %s, want warning", candidates.Status)
	}
	decisions := findEvaluationCheck(t, eval.Stages[0], "closeout.wiki_decisions")
	if decisions.Status != EvaluationWarning {
		t.Fatalf("closeout.wiki_decisions = %s, want warning", decisions.Status)
	}
}

func TestEvaluateCloseoutWikiDecisionsWarnsOnPendingCandidate(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "wiki-pending", Title: "Wiki pending", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CloseoutFile, "# Closeout\n\n## 需求结果\n\n- shipped\n"); err != nil {
		t.Fatal(err)
	}
	wikiText := "# Wiki Candidates: Wiki pending\n\n## Stable Business Knowledge\n\n- Active membership gates coupons. (source: memory-candidates.md)\n\n## Process Improvement Candidates\n\nNo process improvement candidates distilled yet.\n\n## Archive Only\n\nNo archive-only material distilled yet.\n"
	if err := store.WriteArtifact(demand.ID, artifacts.WikiCandidatesFile, wikiText); err != nil {
		t.Fatal(err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatal(err)
	}
	candidates := findEvaluationCheck(t, eval.Stages[0], "closeout.wiki_candidates")
	if candidates.Status != EvaluationPass {
		t.Fatalf("closeout.wiki_candidates = %s, want pass", candidates.Status)
	}
	decisions := findEvaluationCheck(t, eval.Stages[0], "closeout.wiki_decisions")
	if decisions.Status != EvaluationWarning {
		t.Fatalf("closeout.wiki_decisions = %s, want warning", decisions.Status)
	}
	if !strings.Contains(decisions.Evidence, "pending") {
		t.Fatalf("wiki_decisions evidence missing pending: %s", decisions.Evidence)
	}
}

func TestEvaluateCloseoutWikiDecisionsPassesWhenPromoted(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "wiki-decided", Title: "Wiki decided", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CloseoutFile, "# Closeout\n\n## 需求结果\n\n- shipped\n"); err != nil {
		t.Fatal(err)
	}
	wikiText := "# Wiki Candidates: Wiki decided\n\n## Stable Business Knowledge\n\n- Active membership gates coupons. (source: memory-candidates.md) [promoted: .devflow/wiki/coupon-rule.md]\n\n## Process Improvement Candidates\n\nNo process improvement candidates distilled yet.\n\n## Archive Only\n\nNo archive-only material distilled yet.\n"
	if err := store.WriteArtifact(demand.ID, artifacts.WikiCandidatesFile, wikiText); err != nil {
		t.Fatal(err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatal(err)
	}
	decisions := findEvaluationCheck(t, eval.Stages[0], "closeout.wiki_decisions")
	if decisions.Status != EvaluationPass {
		t.Fatalf("closeout.wiki_decisions = %s, want pass evidence=%s", decisions.Status, decisions.Evidence)
	}
}

func TestEvaluateCloseoutWarnsWhenMetricsReportMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "metrics-closeout", Title: "Metrics closeout", Description: "demo", State: string(workflow.Closeout)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CloseoutFile, "# Closeout\n\n## Result\n\nDone.\n"); err != nil {
		t.Fatalf("WriteArtifact closeout returned error: %v", err)
	}

	got := evaluateCloseout(root, demand.ID)
	check := findEvaluationCheck(t, got, "closeout.metrics_report")
	if check.Status != EvaluationWarning {
		t.Fatalf("metrics check status = %s, want warning", check.Status)
	}
	if !strings.Contains(check.Evidence, "devflow metrics report") {
		t.Fatalf("metrics evidence = %q", check.Evidence)
	}
}

func TestEvaluateCloseoutPassesWhenMetricsReportExists(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "metrics-closeout", Title: "Metrics closeout", Description: "demo", State: string(workflow.Closeout)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CloseoutFile, "# Closeout\n\n## Result\n\nDone.\n"); err != nil {
		t.Fatalf("WriteArtifact closeout returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.MetricsFile, "# Devflow Metrics\n\n## Summary\n\n- Demand count: 1\n"); err != nil {
		t.Fatalf("WriteArtifact metrics returned error: %v", err)
	}

	got := evaluateCloseout(root, demand.ID)
	check := findEvaluationCheck(t, got, "closeout.metrics_report")
	if check.Status != EvaluationPass {
		t.Fatalf("metrics check status = %s, want pass", check.Status)
	}
}

func setupPassingCloseout(t *testing.T, store artifacts.Store, demandID string) {
	t.Helper()
	if err := store.WriteArtifact(demandID, artifacts.CloseoutFile, "# Closeout\n\n## 需求结果\n\n- shipped\n"); err != nil {
		t.Fatalf("WriteArtifact closeout returned error: %v", err)
	}
	if err := store.WriteArtifact(demandID, artifacts.MemoryCandidatesFile, "# Memory Candidates\n\n- reused tenant validation rule\n"); err != nil {
		t.Fatalf("WriteArtifact memory returned error: %v", err)
	}
	wikiText := "# Wiki Candidates: closeout\n\n## Stable Business Knowledge\n\n- Active membership gates coupons. (source: memory-candidates.md) [promoted: .devflow/wiki/coupon-rule.md]\n\n## Process Improvement Candidates\n\nNo process improvement candidates distilled yet.\n\n## Archive Only\n\nNo archive-only material distilled yet.\n"
	if err := store.WriteArtifact(demandID, artifacts.WikiCandidatesFile, wikiText); err != nil {
		t.Fatalf("WriteArtifact wiki returned error: %v", err)
	}
	if err := store.WriteArtifact(demandID, artifacts.MetricsFile, "# Devflow Metrics\n\n## Summary\n\n- Demand count: 1\n"); err != nil {
		t.Fatalf("WriteArtifact metrics returned error: %v", err)
	}
}

func TestEvaluateDeploymentPassesWithPassedArtifact(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-deploy-pass", Title: "Deploy pass", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.DeploymentFile, "# Deployment\n\nStatus: `passed`\n\nRun ID: `12345`\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageDeployment)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationPass {
		t.Fatalf("deployment status = %s, want pass; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}

func TestEvaluateDeploymentFailsWithFailedArtifact(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-deploy-fail", Title: "Deploy fail", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.DeploymentFile, "# Deployment\n\nStatus: `failed`\n\nRun ID: `12345`\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageDeployment)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationFail {
		t.Fatalf("deployment status = %s, want fail; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}

func TestEvaluateObservationPassesWithPassedArtifact(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-obs-pass", Title: "Obs pass", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ObservationFile, "# Observation\n\nStatus: `passed`\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageObservation)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationPass {
		t.Fatalf("observation status = %s, want pass; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}

func TestEvaluateRollbackWarnsWhenUndecided(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-rollback-pending", Title: "Rollback pending", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageRollback)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationWarning {
		t.Fatalf("rollback status = %s, want warning; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}

func TestEvaluateCloseoutBlocksWhenObservationMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-closeout-block", Title: "Block", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	setupPassingCloseout(t, store, demand.ID)
	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationFail {
		t.Fatalf("closeout status = %s, want fail; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}

func TestEvaluateCloseoutPassesWhenObservationPassed(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-closeout-obs", Title: "Obs closeout", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	setupPassingCloseout(t, store, demand.ID)
	if err := store.WriteArtifact(demand.ID, artifacts.ObservationFile, "# Observation\n\nStatus: `passed`\n"); err != nil {
		t.Fatalf("WriteArtifact observation returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationPass {
		t.Fatalf("closeout status = %s, want pass; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}

func TestEvaluateCloseoutPassesWhenRiskAccepted(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-closeout-risk", Title: "Risk closeout", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	setupPassingCloseout(t, store, demand.ID)
	if err := store.WriteArtifact(demand.ID, artifacts.RollbackFile, "# Rollback\n\nDecision: `risk_accepted`\n"); err != nil {
		t.Fatalf("WriteArtifact rollback returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationPass {
		t.Fatalf("closeout status = %s, want pass; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}

func TestEvaluateCloseoutFailsWhenRollbackConfirmed(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-closeout-confirmed", Title: "Confirmed closeout", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	setupPassingCloseout(t, store, demand.ID)
	if err := store.WriteArtifact(demand.ID, artifacts.RollbackFile, "# Rollback\n\nDecision: `rollback_confirmed`\n"); err != nil {
		t.Fatalf("WriteArtifact rollback returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageCloseout)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationFail {
		t.Fatalf("closeout status = %s, want fail; checks=%#v", eval.Stages[0].Status, eval.Stages[0].Checks)
	}
}
