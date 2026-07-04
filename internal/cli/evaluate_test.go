package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestEvaluateCommandPrintsStageStatuses(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli", Title: "Eval CLI", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- rule\n\n## 验收标准\n\n- accept\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Evaluation: eval-cli", "requirements", "blockers", "warnings"} {
		if !strings.Contains(got, want) {
			t.Fatalf("evaluate output missing %q:\n%s", want, got)
		}
	}
}

func TestEvaluateCommandStrictReturnsErrorOnFailure(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli-strict", Title: "Eval CLI strict", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- rule\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "requirements", "--strict"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("err = %v, want evaluation failed", err)
	}
}

func TestEvaluateCommandStrictReturnsErrorOnWarning(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli-strict-warning", Title: "Eval CLI strict warning", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanFile, "# Plan\n\n## 实施步骤\n\n- build it\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "plan", "--strict"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "evaluation failed") {
		t.Fatalf("err = %v, want evaluation failed", err)
	}
}

func TestEvaluateCommandStageFilter(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli-stage", Title: "Eval CLI stage", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "plan"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "plan") {
		t.Fatalf("evaluate output missing plan:\n%s", got)
	}
	if strings.Contains(got, "requirements") {
		t.Fatalf("evaluate output included unrequested requirements:\n%s", got)
	}
}

func TestEvaluateCommandPrintsContextAwareRequirementChecks(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli-context", Title: "Eval CLI context", Description: "Evaluate context", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, "# Intake\n\n## 验收标准\n- Inactive users are blocked.\n"); err != nil {
		t.Fatalf("WriteArtifact intake returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, "# Context\n\n## Approved Stable Memory\n\nNo approved stable memory recalled.\n\n## Historical Demand Candidates\n\n- `coupon-old`: Candidate needs confirmation.\n"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- User status must be active.\n\n## 验收标准\n\n- Active users can claim coupons.\n\n## 待确认问题\n\n- 待人工补充。\n"); err != nil {
		t.Fatalf("WriteArtifact requirements returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "requirements"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"requirements.intake_coverage",
		"requirements.context_presence",
		"requirements.candidate_guard",
		"Inactive users are blocked",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("evaluate output missing %q:\n%s", want, got)
		}
	}
}

func TestEvaluateCommandPrintsManualEvidenceChecks(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-cli-evidence", Title: "Eval CLI evidence", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "verification"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"verification.acceptance_evidence", "verification.acceptance_evidence_pass"} {
		if !strings.Contains(got, want) {
			t.Fatalf("evaluate output missing %q:\n%s", want, got)
		}
	}
}

func TestEvaluateCommandPrintsWikiDecisionWarningEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "wiki-cli", Title: "Wiki CLI", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.CloseoutFile, "# Closeout\n\n## 需求结果\n\n- shipped\n"); err != nil {
		t.Fatal(err)
	}
	wikiText := "# Wiki Candidates: Wiki CLI\n\n## Stable Business Knowledge\n\n- Active membership gates coupons. (source: memory-candidates.md)\n\n## Process Improvement Candidates\n\nNo process improvement candidates distilled yet.\n\n## Archive Only\n\nNo archive-only material distilled yet.\n"
	if err := store.WriteArtifact(demand.ID, artifacts.WikiCandidatesFile, wikiText); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := Run([]string{"evaluate", "--root", root, "--demand", demand.ID, "--stage", "closeout"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evaluate returned error: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "closeout.wiki_candidates") {
		t.Fatalf("evaluate output missing closeout.wiki_candidates:\n%s", got)
	}
	if !strings.Contains(got, "closeout.wiki_decisions") {
		t.Fatalf("evaluate output missing closeout.wiki_decisions:\n%s", got)
	}
	if !strings.Contains(got, "pending wiki candidates need promote/reject review") {
		t.Fatalf("evaluate output missing wiki decision evidence:\n%s", got)
	}
}