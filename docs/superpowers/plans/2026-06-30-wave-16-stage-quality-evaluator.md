# Wave 16 Stage Quality Evaluator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add deterministic stage quality evaluation and expose it through `devflow evaluate` and console detail output.

**Architecture:** Implement pure file/event checks in `internal/demandflow` so evaluation is testable without LLM calls. Add a CLI report command, then enrich console detail with concise quality status while keeping evaluation informational unless `--strict` is requested.

**Tech Stack:** Go standard library, existing artifact store, existing workspace summary, existing CLI patterns.

---

## File Structure

- Create `internal/demandflow/evaluation.go`.
- Create `internal/demandflow/evaluation_test.go`.
- Create `internal/cli/evaluate.go`.
- Create `internal/cli/evaluate_test.go`.
- Modify `internal/cli/console.go` and `internal/cli/console_test.go`.
- Modify `internal/cli/cli.go`.
- Modify docs and release notes.

## Task 1: Evaluation Model And Requirements Checks

**Files:**
- Create: `internal/demandflow/evaluation.go`
- Create: `internal/demandflow/evaluation_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/demandflow/evaluation_test.go`:

```go
package demandflow

import (
	"testing"

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
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/demandflow -run TestEvaluateRequirements -count=1
```

Expected: missing `EvaluateDemand`.

- [ ] **Step 3: Implement model and requirements checks**

Create `internal/demandflow/evaluation.go`:

```go
package demandflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

type EvaluationStatus string

const (
	EvaluationPass          EvaluationStatus = "pass"
	EvaluationWarning       EvaluationStatus = "warning"
	EvaluationFail          EvaluationStatus = "fail"
	EvaluationNotApplicable EvaluationStatus = "not_applicable"
)

type EvaluationCheck struct {
	ID       string
	Label    string
	Status   EvaluationStatus
	Severity string
	Evidence string
}

type StageEvaluation struct {
	Stage    Stage
	Status   EvaluationStatus
	Checks   []EvaluationCheck
	Blockers int
	Warnings int
}

type DemandEvaluation struct {
	DemandID string
	Stages   []StageEvaluation
	Overall  EvaluationStatus
}

func EvaluateDemand(root, demandID string, stages ...Stage) (DemandEvaluation, error) {
	store := artifacts.NewStore(root)
	if _, err := store.LoadDemand(demandID); err != nil {
		return DemandEvaluation{}, err
	}
	if len(stages) == 0 {
		stages = []Stage{StageRequirements, StagePlan, StageVerification, StageCloseout}
	}
	out := DemandEvaluation{DemandID: demandID, Overall: EvaluationPass}
	for _, stage := range stages {
		stageEval, err := evaluateStage(root, demandID, stage)
		if err != nil {
			return DemandEvaluation{}, err
		}
		out.Stages = append(out.Stages, stageEval)
		out.Overall = combineEvaluationStatus(out.Overall, stageEval.Status)
	}
	return out, nil
}

func evaluateStage(root, demandID string, stage Stage) (StageEvaluation, error) {
	switch stage {
	case StageRequirements:
		return evaluateRequirements(root, demandID), nil
	default:
		return StageEvaluation{Stage: stage, Status: EvaluationNotApplicable}, nil
	}
}

func evaluateRequirements(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.RequirementsFile)
	checks := []EvaluationCheck{
		requiredContentCheck("requirements.exists", "requirements.md has content", text, "blocker"),
		requiredSectionCheck("requirements.acceptance", "acceptance criteria section has content", text, []string{"验收标准", "acceptance criteria"}, "blocker"),
		requiredSectionCheck("requirements.rules", "business rules section has content", text, []string{"业务规则", "business rules"}, "warning"),
		requiredSectionCheck("requirements.risks", "risks section has content", text, []string{"风险与歧义", "risks"}, "warning"),
	}
	return aggregateStageEvaluation(StageRequirements, checks)
}

func readEvaluationArtifact(root, demandID, name string) string {
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demandID, name))
	if err != nil {
		return ""
	}
	return string(data)
}

func requiredContentCheck(id, label, text, severity string) EvaluationCheck {
	if strings.TrimSpace(text) == "" {
		return EvaluationCheck{ID: id, Label: label, Status: EvaluationFail, Severity: severity}
	}
	return EvaluationCheck{ID: id, Label: label, Status: EvaluationPass, Severity: severity, Evidence: "content present"}
}

func requiredSectionCheck(id, label, text string, headings []string, severity string) EvaluationCheck {
	for _, heading := range headings {
		if sectionHasContent(text, heading) {
			return EvaluationCheck{ID: id, Label: label, Status: EvaluationPass, Severity: severity, Evidence: heading}
		}
	}
	return EvaluationCheck{ID: id, Label: label, Status: EvaluationFail, Severity: severity}
}

func sectionHasContent(text, heading string) bool {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(trimmed, "## ") {
			if inSection {
				return false
			}
			inSection = strings.Contains(lower, strings.ToLower(heading))
			continue
		}
		if inSection && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return true
		}
	}
	return false
}

func aggregateStageEvaluation(stage Stage, checks []EvaluationCheck) StageEvaluation {
	out := StageEvaluation{Stage: stage, Status: EvaluationPass, Checks: checks}
	for _, check := range checks {
		if check.Status == EvaluationFail && check.Severity == "blocker" {
			out.Blockers++
		}
		if check.Status == EvaluationFail && check.Severity == "warning" {
			out.Warnings++
		}
	}
	if out.Blockers > 0 {
		out.Status = EvaluationFail
	} else if out.Warnings > 0 {
		out.Status = EvaluationWarning
	}
	return out
}

func combineEvaluationStatus(left, right EvaluationStatus) EvaluationStatus {
	if left == EvaluationFail || right == EvaluationFail {
		return EvaluationFail
	}
	if left == EvaluationWarning || right == EvaluationWarning {
		return EvaluationWarning
	}
	if left == EvaluationNotApplicable {
		return right
	}
	return left
}

func (s EvaluationStatus) String() string {
	return fmt.Sprint(string(s))
}
```

- [ ] **Step 4: Verify and commit**

Run:

```powershell
gofmt -w internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go
go test ./internal/demandflow -run TestEvaluateRequirements -count=1
git add internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go
git commit -m "Evaluate requirements artifact quality" -m "Wave 16 starts with deterministic stage quality checks, beginning with requirements content and required sections." -m "Constraint: Evaluation is file-based and does not call an LLM." -m "Confidence: medium" -m "Scope-risk: narrow" -m "Tested: go test ./internal/demandflow -run TestEvaluateRequirements -count=1"
```

## Task 2: Add Plan, Verification, Closeout Checks

**Files:**
- Modify: `internal/demandflow/evaluation.go`
- Modify: `internal/demandflow/evaluation_test.go`

- [ ] **Step 1: Add tests for other stages**

Add tests:

```go
func TestEvaluateVerificationFailsWithoutRecordedEvent(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-verification", Title: "Eval verification", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.VerificationFile, "# Verification\n\n## 验收标准映射\n\n- mapped\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	eval, err := EvaluateDemand(root, demand.ID, StageVerification)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	if eval.Stages[0].Status != EvaluationFail {
		t.Fatalf("verification status = %s, want fail", eval.Stages[0].Status)
	}
}

func TestEvaluatePlanWarnsWithoutTestStrategy(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-plan", Title: "Eval plan", Description: "Evaluate", Source: "test"}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanFile, "# Plan\n\n## 改动范围\n\n- internal/api\n"); err != nil {
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
```

- [ ] **Step 2: Extend evaluator**

Add `evaluatePlan`, `evaluateVerification`, and `evaluateCloseout` using the same `requiredSectionCheck` pattern. Verification recorded event can use `artifacts.NewStore(root).ReadEvents(demandID)` and look for latest `verification.recorded`.

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -w internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go
go test ./internal/demandflow -run TestEvaluate -count=1
git add internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go
git commit -m "Evaluate plan verification and closeout quality" -m "Stage quality now covers the primary confirmation artifacts with deterministic checks for test strategy, verification evidence, and closeout memory material." -m "Constraint: Checks are intentionally structural and do not claim semantic correctness." -m "Confidence: medium" -m "Scope-risk: moderate" -m "Tested: go test ./internal/demandflow -run TestEvaluate -count=1"
```

## Task 3: Add `devflow evaluate`

**Files:**
- Create: `internal/cli/evaluate.go`
- Create: `internal/cli/evaluate_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write CLI tests**

Create tests for:

```go
func TestEvaluateCommandPrintsStageStatuses(t *testing.T)
func TestEvaluateCommandStrictReturnsErrorOnFailure(t *testing.T)
func TestEvaluateCommandStageFilter(t *testing.T)
```

Assertions:

- output contains `Evaluation: <id>`;
- output contains `requirements`;
- strict mode returns an error containing `evaluation failed`.

- [ ] **Step 2: Implement command**

`internal/cli/evaluate.go` should parse:

```powershell
--root
--demand
--stage
--strict
```

Use `demandflow.ParseStage` when stage is provided. Render stage status and blocker/warning counts.

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/evaluate.go internal/cli/evaluate_test.go internal/cli/cli.go
go test ./internal/cli -run TestEvaluate -count=1
git add internal/cli/evaluate.go internal/cli/evaluate_test.go internal/cli/cli.go
git commit -m "Expose deterministic stage evaluation in CLI" -m "Users can now inspect quality signals before confirming stage outputs, with strict mode for scripts." -m "Constraint: Evaluate reports quality but does not mutate demand state." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run TestEvaluate -count=1"
```

## Task 4: Add Console Quality Summary

**Files:**
- Modify: `internal/cli/console.go`
- Modify: `internal/cli/console_test.go`

- [ ] **Step 1: Add console test**

Add assertion that `devflow console --demand <id>` includes:

```text
Quality:
  requirements
```

- [ ] **Step 2: Implement concise quality rendering**

In `printConsoleDetail`, call `demandflow.EvaluateDemand(".", id)` is not enough because root is not available in summary. Pass root into rendering by changing `printConsoleDetail(stdout, summary)` to `printConsoleDetail(stdout, opts.root, summary)`.

Render:

```text
Quality:
  requirements   pass
  plan           warning
```

If evaluation fails to load, print:

```text
Quality:
  unavailable: <error>
```

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/console.go internal/cli/console_test.go
go test ./internal/cli -run "TestConsole|TestEvaluate" -count=1
git add internal/cli/console.go internal/cli/console_test.go
git commit -m "Show quality summary in console" -m "Console detail now includes deterministic evaluation status so users can see quality signals before manual confirmation." -m "Constraint: Console remains informational and does not fail because evaluation reports warnings." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run \"TestConsole|TestEvaluate\" -count=1"
```

## Task 5: Docs And Full Verification

- [ ] Document `devflow evaluate` in user guide.
- [ ] Add Wave 16 release note.
- [ ] Run:

```powershell
go test ./internal/demandflow ./internal/cli -count=1
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git diff --check
```

- [ ] Commit docs:

```powershell
git add docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document deterministic stage evaluation" -m "Wave 16 adds local structural quality checks, so the docs now explain evaluate, strict mode, and console quality summaries." -m "Constraint: Docs must state that evaluation is deterministic and not semantic LLM review." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check"
```

## Final Checklist

- `devflow evaluate` exists.
- requirements, plan, verification, closeout checks exist.
- `--strict` exits non-zero on warning/fail.
- console shows quality summary.
- full verification passes.
