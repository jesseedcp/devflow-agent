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
