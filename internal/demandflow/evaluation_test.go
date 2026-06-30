package demandflow

import (
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
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
