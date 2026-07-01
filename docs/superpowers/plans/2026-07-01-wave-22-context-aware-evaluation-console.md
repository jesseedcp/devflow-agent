# Wave 22 Context-Aware Evaluation And Console Signals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade requirements evaluation so it checks whether `requirements.md` actually reflects `intake.md` and safely handles `context.md`, then surface those checks in console and workbench operator views.

**Architecture:** Keep evaluation deterministic and local. Extend the existing `demandflow.EvaluateDemand` requirements checks with text-section comparison helpers, then reuse the same `StageEvaluation.Checks` in CLI console and workbench rendering. Do not add workflow states, external adapters, embeddings, or LLM review.

**Tech Stack:** Go 1.25, existing `internal/artifacts`, `internal/demandflow`, `internal/cli`, PowerShell release-readiness script.

---

## Product Scope

Wave 20 made local PRD intake visible:

```text
intake.md -> requirements.md -> requirements_review
```

Wave 21 made reusable memory visible:

```text
context.md -> requirements review context
```

Wave 22 checks whether the generated/reviewed requirements are coherent with those inputs:

```text
intake.md + context.md + requirements.md -> deterministic requirements quality signals
```

New checks:

- `requirements.intake_coverage`
  - Warns when concrete bullets from intake goals/rules/acceptance do not appear in `requirements.md`.

- `requirements.context_presence`
  - Warns when `context.md` is still missing/template-like for a demand that has gone through intake.

- `requirements.stable_memory_reference`
  - Warns when approved stable memory exists in `context.md` but none of its snippets appear in requirements.

- `requirements.candidate_guard`
  - Warns when historical candidate memory exists but `requirements.md` has no useful `待确认问题` content.

Console/workbench visibility:

- `devflow evaluate --stage requirements` prints these check IDs.
- `devflow console --demand <id>` prints requirement check details under Quality.
- `devflow workbench --snapshot --demand <id>` prints requirement check details under Quality.

Out of scope:

- Semantic similarity.
- Ranked recall.
- Automatic requirements rewriting.
- New workflow stage or auto-confirmation.
- URL/WeChat/Aone/DingTalk adapters.

---

## File Structure

- Modify: `internal/demandflow/evaluation.go`
  - Add context-aware requirements checks.
  - Add deterministic helpers for section extraction, bullet extraction, and normalized matching.

- Modify: `internal/demandflow/evaluation_test.go`
  - Add focused checks for intake coverage, context presence, stable memory reference, and candidate guard.

- Modify: `internal/cli/evaluate_test.go`
  - Assert CLI output includes new check IDs.

- Modify: `internal/cli/console.go`
  - Print check-level requirement details in console quality output.

- Modify: `internal/cli/console_test.go`
  - Assert console detail includes context-aware requirement check IDs.

- Modify: `internal/cli/workbench_snapshot.go`
  - Print check-level requirement details in snapshot quality output.

- Modify: `internal/cli/workbench_test.go`
  - Assert snapshot includes context-aware requirement check IDs.

- Modify: `scripts/release-readiness.ps1`
  - Extend intake smoke to verify evaluate/console/workbench output includes context-aware checks.

- Modify: `docs/user-guide/backend-demand-loop.md`
  - Document context-aware requirements quality signals.

- Modify: `docs/release/v0.1.md`
  - Add Wave 22 release note.

---

## Task 1: Add Context-Aware Requirements Evaluation

**Files:**
- Modify: `internal/demandflow/evaluation.go`
- Modify: `internal/demandflow/evaluation_test.go`

- [ ] **Step 1: Add failing evaluation tests**

Append to `internal/demandflow/evaluation_test.go`:

```go
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
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, `# Context: Coupon

## Approved Stable Memory

- `+"`memory/coupon.md`"+`: Coupon active member checks must happen before claim writes.

## Historical Demand Candidates

- `+"`coupon-old`"+`: Candidate says expired coupons may use a generic error.
`); err != nil {
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
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, `# Context: Coupon

## Approved Stable Memory

No approved stable memory recalled.

## Historical Demand Candidates

- `+"`coupon-old`"+`: Candidate says expired coupons may use a generic error.
`); err != nil {
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
```

If `internal/demandflow/evaluation_test.go` does not already import `strings`, add it.

- [ ] **Step 2: Run tests and verify red**

Run:

```powershell
go test ./internal/demandflow -run "TestEvaluateRequirementsChecksIntakeCoverage|TestEvaluateRequirementsWarns|TestEvaluateRequirementsChecksContextMemorySafety" -count=1
```

Expected: FAIL because the new check IDs are missing.

- [ ] **Step 3: Extend `evaluateRequirements`**

Modify `internal/demandflow/evaluation.go`.

Replace `evaluateRequirements` with:

```go
func evaluateRequirements(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.RequirementsFile)
	intakeText := readEvaluationArtifact(root, demandID, artifacts.IntakeFile)
	contextText := readEvaluationArtifact(root, demandID, artifacts.ContextFile)
	checks := []EvaluationCheck{
		requiredContentCheck("requirements.exists", "requirements.md has content", text, "blocker"),
		requiredSectionCheck("requirements.acceptance", "acceptance criteria section has content", text, []string{"验收标准", "acceptance criteria"}, "blocker"),
		requiredSectionCheck("requirements.rules", "business rules section has content", text, []string{"业务规则", "business rules"}, "warning"),
		requiredSectionCheck("requirements.risks", "risks section has content", text, []string{"风险与歧义", "risks"}, "warning"),
		intakeCoverageCheck(intakeText, text),
		contextPresenceCheck(contextText),
		stableMemoryReferenceCheck(contextText, text),
		candidateMemoryGuardCheck(contextText, text),
	}
	return buildStageEvaluation(StageRequirements, checks)
}
```

- [ ] **Step 4: Add context-aware helper functions**

Append these helpers near the other evaluation helpers in `internal/demandflow/evaluation.go`:

```go
func intakeCoverageCheck(intakeText, requirementsText string) EvaluationCheck {
	bullets := requirementRelevantBullets(intakeText, []string{"目标", "业务规则", "验收", "acceptance", "rule", "goal"})
	if len(bullets) == 0 {
		return EvaluationCheck{
			ID:       "requirements.intake_coverage",
			Label:    "requirements cover concrete intake bullets",
			Status:   EvaluationNotApplicable,
			Severity: "warning",
			Evidence: "no concrete intake bullets found",
		}
	}
	var missing []string
	for _, bullet := range bullets {
		if !normalizedContains(requirementsText, bullet) {
			missing = append(missing, bullet)
		}
	}
	if len(missing) == 0 {
		return statusCheck("requirements.intake_coverage", "requirements cover concrete intake bullets", true, "warning", fmt.Sprintf("%d intake bullets covered", len(bullets)))
	}
	return EvaluationCheck{
		ID:       "requirements.intake_coverage",
		Label:    "requirements cover concrete intake bullets",
		Status:   EvaluationWarning,
		Severity: "warning",
		Evidence: strings.Join(limitStrings(missing, 3), " | "),
	}
}

func contextPresenceCheck(contextText string) EvaluationCheck {
	trimmed := strings.TrimSpace(contextText)
	if trimmed == "" {
		return statusCheck("requirements.context_presence", "context.md exists with recall sections", false, "warning", "context.md empty or missing")
	}
	lower := strings.ToLower(trimmed)
	ok := strings.Contains(lower, "approved stable memory") && strings.Contains(lower, "historical demand candidates")
	return statusCheck("requirements.context_presence", "context.md exists with recall sections", ok, "warning", evidenceSnippet(contextText))
}

func stableMemoryReferenceCheck(contextText, requirementsText string) EvaluationCheck {
	stable := contextSectionBullets(contextText, "approved stable memory")
	stable = removeNoMemoryBullets(stable)
	if len(stable) == 0 {
		return EvaluationCheck{
			ID:       "requirements.stable_memory_reference",
			Label:    "requirements reference approved stable memory when present",
			Status:   EvaluationNotApplicable,
			Severity: "warning",
			Evidence: "no approved stable memory recalled",
		}
	}
	for _, bullet := range stable {
		snippet := memorySnippetText(bullet)
		if snippet != "" && normalizedContains(requirementsText, snippet) {
			return statusCheck("requirements.stable_memory_reference", "requirements reference approved stable memory when present", true, "warning", snippet)
		}
	}
	return EvaluationCheck{
		ID:       "requirements.stable_memory_reference",
		Label:    "requirements reference approved stable memory when present",
		Status:   EvaluationWarning,
		Severity: "warning",
		Evidence: strings.Join(limitStrings(stable, 3), " | "),
	}
}

func candidateMemoryGuardCheck(contextText, requirementsText string) EvaluationCheck {
	candidates := contextSectionBullets(contextText, "historical demand candidates")
	candidates = removeNoMemoryBullets(candidates)
	if len(candidates) == 0 {
		return EvaluationCheck{
			ID:       "requirements.candidate_guard",
			Label:    "candidate memory is routed to confirmation questions",
			Status:   EvaluationNotApplicable,
			Severity: "warning",
			Evidence: "no historical candidate memory recalled",
		}
	}
	questions := sectionAfterHeading(requirementsText, "待确认问题")
	ok := hasUsefulQuestion(questions)
	return statusCheck("requirements.candidate_guard", "candidate memory is routed to confirmation questions", ok, "warning", evidenceSnippet(questions))
}

func requirementRelevantBullets(text string, headings []string) []string {
	var out []string
	for _, heading := range headings {
		section := sectionAfterHeading(text, heading)
		out = append(out, topLevelBullets(section)...)
	}
	return uniqueNonTemplateBullets(out)
}

func contextSectionBullets(text, heading string) []string {
	return uniqueNonTemplateBullets(topLevelBullets(sectionAfterHeading(text, heading)))
}

func topLevelBullets(text string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func uniqueNonTemplateBullets(values []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.Join(strings.Fields(value), " ")
		lower := strings.ToLower(trimmed)
		if trimmed == "" || strings.Contains(lower, "待人工补充") || strings.Contains(lower, "待补充") || strings.Contains(lower, "placeholder") || strings.Contains(lower, "no approved stable memory") || strings.Contains(lower, "no historical candidate memory") {
			continue
		}
		if seen[lower] {
			continue
		}
		seen[lower] = true
		out = append(out, trimmed)
	}
	return out
}

func removeNoMemoryBullets(values []string) []string {
	var out []string
	for _, value := range values {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "no approved stable memory") || strings.Contains(lower, "no historical candidate memory") {
			continue
		}
		out = append(out, value)
	}
	return out
}

func memorySnippetText(value string) string {
	if idx := strings.Index(value, ":"); idx >= 0 && idx+1 < len(value) {
		return strings.TrimSpace(value[idx+1:])
	}
	return strings.Trim(value, "` ")
}

func normalizedContains(text, needle string) bool {
	textNorm := normalizeComparableText(text)
	needleNorm := normalizeComparableText(needle)
	if needleNorm == "" {
		return true
	}
	return strings.Contains(textNorm, needleNorm)
}

func normalizeComparableText(value string) string {
	value = strings.ToLower(value)
	replacer := strings.NewReplacer("`", " ", ".", " ", ",", " ", ":", " ", ";", " ", "，", " ", "。", " ", "：", " ", "；", " ", "-", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

func hasUsefulQuestion(text string) bool {
	for _, bullet := range uniqueNonTemplateBullets(topLevelBullets(text)) {
		if strings.TrimSpace(bullet) != "" {
			return true
		}
	}
	return false
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}
```

- [ ] **Step 5: Run focused tests and verify green**

Run:

```powershell
gofmt -w internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go
go test ./internal/demandflow -run "TestEvaluateRequirementsChecksIntakeCoverage|TestEvaluateRequirementsWarns|TestEvaluateRequirementsChecksContextMemorySafety" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit evaluation checks**

```powershell
git add internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go
git commit -m "Evaluate requirements against intake and recalled context" -m "Requirements review needs deterministic signals that the draft absorbed source intake and handled recalled memory safely. The requirements evaluator now reports intake coverage, context presence, stable-memory reference, and candidate-memory guard checks." -m "Constraint: Evaluation remains local and deterministic; no model calls or semantic ranking.`nConfidence: high`nScope-risk: moderate`nDirective: Treat candidate memory as unapproved context; do not convert candidate hits into business rules without human confirmation.`nTested: go test ./internal/demandflow -run \"TestEvaluateRequirementsChecksIntakeCoverage|TestEvaluateRequirementsWarns|TestEvaluateRequirementsChecksContextMemorySafety\" -count=1`nNot-tested: CLI rendering"
```

---

## Task 2: Show Requirement Check Details In `devflow evaluate`

**Files:**
- Modify: `internal/cli/evaluate_test.go`
- Modify: `internal/cli/evaluate.go`

Current `printEvaluation` already prints every check ID, but this task locks the new IDs into CLI behavior and improves evidence output for warnings.

- [ ] **Step 1: Add CLI check ID/evidence test**

Append to `internal/cli/evaluate_test.go`:

```go
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
```

- [ ] **Step 2: Run CLI test and verify red or weak output**

Run:

```powershell
go test ./internal/cli -run TestEvaluateCommandPrintsContextAwareRequirementChecks -count=1
```

Expected before implementation:

- If Task 1 is complete, it may already pass for check IDs but fail for evidence because `printEvaluation` does not print evidence.

- [ ] **Step 3: Print evidence for failed/warning checks**

Modify `printEvaluation` in `internal/cli/evaluate.go`:

```go
func printEvaluation(stdout io.Writer, evaluation demandflow.DemandEvaluation) {
	fmt.Fprintf(stdout, "Evaluation: %s\n", evaluation.DemandID)
	fmt.Fprintf(stdout, "Overall: %s\n\n", evaluation.Overall)
	for _, stage := range evaluation.Stages {
		fmt.Fprintf(stdout, "%-14s %-8s blockers=%d warnings=%d\n", stage.Stage, stage.Status, stage.Blockers, stage.Warnings)
		for _, check := range stage.Checks {
			fmt.Fprintf(stdout, "  %-36s %-14s %s\n", check.ID, check.Status, check.Label)
			if check.Evidence != "" && check.Status != demandflow.EvaluationPass {
				fmt.Fprintf(stdout, "    evidence: %s\n", check.Evidence)
			}
		}
	}
}
```

- [ ] **Step 4: Run CLI evaluate tests and verify green**

Run:

```powershell
gofmt -w internal/cli/evaluate.go internal/cli/evaluate_test.go
go test ./internal/cli -run "TestEvaluateCommand" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit CLI evaluate output**

```powershell
git add internal/cli/evaluate.go internal/cli/evaluate_test.go
git commit -m "Print context-aware evaluation evidence" -m "Operators need to see why a requirements check warned, not just that it warned. The evaluate command now prints evidence for non-passing checks and locks context-aware requirement checks into CLI output." -m "Constraint: Passing checks stay compact to keep evaluate output scannable.`nConfidence: high`nScope-risk: narrow`nTested: go test ./internal/cli -run \"TestEvaluateCommand\" -count=1`nNot-tested: console/workbench rendering"
```

---

## Task 3: Surface Requirement Quality Checks In Console

**Files:**
- Modify: `internal/cli/console.go`
- Modify: `internal/cli/console_test.go`

- [ ] **Step 1: Add console quality-detail test**

Append to `internal/cli/console_test.go`:

```go
func TestConsoleDetailPrintsContextAwareRequirementChecks(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "console-quality-context", Title: "Console quality context", Description: "Evaluate context", Source: "test", State: string(workflow.RequirementsReview)}
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
	if err := Run([]string{"console", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("console returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"requirements.intake_coverage",
		"requirements.candidate_guard",
		"Inactive users are blocked",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("console output missing %q:\n%s", want, got)
		}
	}
}
```

Ensure `internal/cli/console_test.go` imports `workflow` if it does not already.

- [ ] **Step 2: Run console test and verify red**

Run:

```powershell
go test ./internal/cli -run TestConsoleDetailPrintsContextAwareRequirementChecks -count=1
```

Expected: FAIL because console only prints stage-level quality.

- [ ] **Step 3: Print requirement check details in console quality**

Modify `printConsoleQuality` in `internal/cli/console.go`:

```go
func printConsoleQuality(stdout io.Writer, root, demandID string) {
	evaluation, err := demandflow.EvaluateDemand(root, demandID)
	if err != nil {
		fmt.Fprintf(stdout, "  unavailable: %v\n", err)
		return
	}
	for _, stage := range evaluation.Stages {
		fmt.Fprintf(stdout, "  %-14s %s", stage.Stage, stage.Status)
		if stage.Blockers > 0 || stage.Warnings > 0 {
			fmt.Fprintf(stdout, " blockers=%d warnings=%d", stage.Blockers, stage.Warnings)
		}
		fmt.Fprintln(stdout)
		if stage.Stage == demandflow.StageRequirements {
			printRequirementQualityChecks(stdout, stage)
		}
	}
}

func printRequirementQualityChecks(stdout io.Writer, stage demandflow.StageEvaluation) {
	for _, check := range stage.Checks {
		if !strings.HasPrefix(check.ID, "requirements.") {
			continue
		}
		if check.Status == demandflow.EvaluationPass || check.Status == demandflow.EvaluationNotApplicable {
			continue
		}
		fmt.Fprintf(stdout, "    %-36s %s\n", check.ID, check.Status)
		if strings.TrimSpace(check.Evidence) != "" {
			fmt.Fprintf(stdout, "      %s\n", check.Evidence)
		}
	}
}
```

- [ ] **Step 4: Run console tests and verify green**

Run:

```powershell
gofmt -w internal/cli/console.go internal/cli/console_test.go
go test ./internal/cli -run "TestConsole" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit console quality signals**

```powershell
git add internal/cli/console.go internal/cli/console_test.go
git commit -m "Show requirement quality warnings in console" -m "The operator console should make intake/context alignment problems visible before requirements confirmation. Console quality output now expands non-passing requirements checks with evidence." -m "Constraint: Console stays read-only unless --run-next is explicitly used.`nConfidence: high`nScope-risk: narrow`nTested: go test ./internal/cli -run \"TestConsole\" -count=1`nNot-tested: workbench snapshot"
```

---

## Task 4: Surface Requirement Quality Checks In Workbench Snapshot

**Files:**
- Modify: `internal/cli/workbench_snapshot.go`
- Modify: `internal/cli/workbench_model.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Add workbench snapshot quality test**

Append to `internal/cli/workbench_test.go`:

```go
func TestWorkbenchSnapshotPrintsContextAwareRequirementChecks(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workbench-quality-context", Title: "Workbench quality context", Description: "Evaluate context", Source: "test", State: string(workflow.RequirementsReview)}
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
	if err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"requirements.intake_coverage",
		"requirements.candidate_guard",
		"Inactive users are blocked",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("workbench snapshot missing %q:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run workbench test and verify red**

Run:

```powershell
go test ./internal/cli -run TestWorkbenchSnapshotPrintsContextAwareRequirementChecks -count=1
```

Expected: FAIL because snapshot only prints stage-level quality.

- [ ] **Step 3: Add reusable quality rendering helper**

To avoid duplicating console logic, add this helper to `internal/cli/console.go` below `printRequirementQualityChecks`:

```go
func renderRequirementQualityChecks(builder *strings.Builder, stage demandflow.StageEvaluation, indent string) {
	for _, check := range stage.Checks {
		if !strings.HasPrefix(check.ID, "requirements.") {
			continue
		}
		if check.Status == demandflow.EvaluationPass || check.Status == demandflow.EvaluationNotApplicable {
			continue
		}
		fmt.Fprintf(builder, "%s%-36s %s\n", indent, check.ID, check.Status)
		if strings.TrimSpace(check.Evidence) != "" {
			fmt.Fprintf(builder, "%s  %s\n", indent, check.Evidence)
		}
	}
}
```

Then replace the body of `printRequirementQualityChecks` with:

```go
func printRequirementQualityChecks(stdout io.Writer, stage demandflow.StageEvaluation) {
	var builder strings.Builder
	renderRequirementQualityChecks(&builder, stage, "    ")
	if builder.Len() > 0 {
		fmt.Fprint(stdout, builder.String())
	}
}
```

- [ ] **Step 4: Use helper in workbench snapshot**

Modify `renderWorkbenchSnapshot` in `internal/cli/workbench_snapshot.go`.

Inside the quality loop, after printing each stage line, add:

```go
			if stage.Stage == demandflow.StageRequirements {
				renderRequirementQualityChecks(&builder, stage, "    ")
			}
```

- [ ] **Step 5: Add quality block to interactive workbench detail**

Modify `renderDetail` in `internal/cli/workbench_model.go`. After the `Attention:` line and before `Next:`, add:

```go
	builder.WriteString("Quality:\n")
	evaluation, err := demandflow.EvaluateDemand(m.opts.root, summary.Workspace.Demand.ID)
	if err != nil {
		fmt.Fprintf(builder, "  unavailable: %v\n", err)
	} else {
		for _, stage := range evaluation.Stages {
			fmt.Fprintf(builder, "  %-14s %s", stage.Stage, stage.Status)
			if stage.Blockers > 0 || stage.Warnings > 0 {
				fmt.Fprintf(builder, " blockers=%d warnings=%d", stage.Blockers, stage.Warnings)
			}
			fmt.Fprintln(builder)
			if stage.Stage == demandflow.StageRequirements {
				renderRequirementQualityChecks(builder, stage, "    ")
			}
		}
	}
```

- [ ] **Step 6: Run workbench tests and verify green**

Run:

```powershell
gofmt -w internal/cli/console.go internal/cli/workbench_snapshot.go internal/cli/workbench_model.go internal/cli/workbench_test.go
go test ./internal/cli -run "TestWorkbench" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit workbench quality signals**

```powershell
git add internal/cli/console.go internal/cli/workbench_snapshot.go internal/cli/workbench_model.go internal/cli/workbench_test.go
git commit -m "Show requirement quality warnings in workbench" -m "Workbench should expose the same context-aware requirements warnings as console, so operators can spot intake or memory alignment problems from snapshots and detail views." -m "Constraint: Workbench rendering remains read-only; action shortcuts are unchanged.`nConfidence: high`nScope-risk: moderate`nTested: go test ./internal/cli -run \"TestWorkbench\" -count=1`nNot-tested: full release readiness"
```

---

## Task 5: Release Readiness And Documentation

**Files:**
- Modify: `scripts/release-readiness.ps1`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Extend release-readiness intake smoke**

Open `scripts/release-readiness.ps1`. In the existing `"intake smoke"` block, capture evaluate/console/workbench output and assert the new check appears.

Replace the evaluate/console calls inside that block with:

```powershell
    $evaluateOutput = .\dist\devflow-windows-amd64.exe evaluate --root $intakeRoot --demand coupon-eligibility --stage requirements 2>&1
    $evaluateOutput | Tee-Object -FilePath (Join-Path $intakeRoot 'evaluate-output.txt') | Out-Host
    if ($evaluateOutput -notmatch 'requirements\.intake_coverage') {
        throw "requirements.intake_coverage missing from evaluate output"
    }

    $consoleOutput = .\dist\devflow-windows-amd64.exe console --root $intakeRoot --demand coupon-eligibility 2>&1
    $consoleOutput | Tee-Object -FilePath (Join-Path $intakeRoot 'console-output.txt') | Out-Host
    if ($consoleOutput -notmatch 'Quality:') {
        throw "Quality section missing from console output"
    }

    $snapshotOutput = .\dist\devflow-windows-amd64.exe workbench --root $intakeRoot --snapshot --demand coupon-eligibility 2>&1
    $snapshotOutput | Tee-Object -FilePath (Join-Path $intakeRoot 'workbench-output.txt') | Out-Host
    if ($snapshotOutput -notmatch 'requirements\.intake_coverage') {
        throw "requirements.intake_coverage missing from workbench snapshot"
    }
```

Do not use `--strict` in this release-readiness intake smoke. The point is to assert that context-aware checks are visible; strict mode may fail on intentionally minimal example inputs.

- [ ] **Step 2: Update user guide**

In `docs/user-guide/backend-demand-loop.md`, after the deterministic stage evaluation section, add:

```markdown
For requirements, evaluation also checks intake/context alignment:

- `requirements.intake_coverage` warns when concrete intake bullets are missing from `requirements.md`.
- `requirements.context_presence` warns when `context.md` is missing or not recalled.
- `requirements.stable_memory_reference` warns when approved memory exists but is not reflected in requirements.
- `requirements.candidate_guard` warns when historical candidate memory exists but requirements have no useful confirmation question.

These checks are deterministic signals for human review. They do not approve requirements automatically.
```

In the operator console or workbench sections, add:

```markdown
Console and workbench snapshot views surface non-passing requirements checks under `Quality`, so the operator can fix intake/context alignment before confirming requirements.
```

- [ ] **Step 3: Update release notes**

In `docs/release/v0.1.md`, add:

```markdown
- Adds context-aware requirements evaluation for intake/context alignment. `evaluate`, `console`, and `workbench --snapshot` now surface warnings when requirements miss intake bullets or mishandle recalled memory.
```

Under limitations, add:

```markdown
- Context-aware evaluation is deterministic text matching in Wave 22. It is a review signal, not semantic proof that requirements are complete.
```

- [ ] **Step 4: Run docs/release smoke**

Run:

```powershell
rg -n "requirements\\.intake_coverage|candidate_guard|context-aware|workbench --snapshot" docs\user-guide\backend-demand-loop.md docs\release\v0.1.md
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave22
```

Expected:

- `rg` finds the new docs.
- Release readiness exits 0.
- Intake smoke prints evaluate, console, and workbench outputs.

- [ ] **Step 5: Commit docs and release gate**

```powershell
git add scripts/release-readiness.ps1 docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document and gate context-aware requirement signals" -m "Wave 22 makes intake/context alignment visible in the operator workflow. Release readiness and docs now cover the deterministic checks and where operators see them." -m "Constraint: Checks are review signals, not semantic proof or auto-approval.`nConfidence: high`nScope-risk: narrow`nTested: rg docs; scripts/release-readiness.ps1 -Version 0.1.0-wave22`nNot-tested: external PRD or semantic similarity"
```

---

## Final Verification

Run from the Wave 22 worktree:

```powershell
go test ./internal/demandflow ./internal/cli -count=1
go vet ./...
go build ./cmd/devflow
git diff --check
go test ./... -count=1 -timeout 5m
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave22
```

Manual smoke:

```powershell
$tmp = Join-Path $env:TEMP "devflow-wave22-context-eval-smoke"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $tmp | Out-Null
$prd = Join-Path $tmp "coupon-eligibility.md"
@"
# Coupon eligibility

## 目标
- Active members can claim coupons.

## 业务规则
- User status must be active.

## 验收标准
- Inactive users are blocked.
"@ | Set-Content -Encoding UTF8 $prd
go run ./cmd/devflow intake --root $tmp --file $prd
go run ./cmd/devflow evaluate --root $tmp --demand coupon-eligibility --stage requirements
go run ./cmd/devflow console --root $tmp --demand coupon-eligibility
go run ./cmd/devflow workbench --root $tmp --snapshot --demand coupon-eligibility
```

Expected:

- `evaluate` output contains `requirements.intake_coverage`.
- `console` output has `Quality:` and includes non-passing requirement check details when warnings exist.
- `workbench --snapshot` output has `Quality:` and includes requirement check details when warnings exist.
- Demand remains in `requirements_review`.

Open PR:

```powershell
git push -u origin wave22-context-aware-evaluation-console
gh pr create --base main --head wave22-context-aware-evaluation-console --title "Wave 22 context-aware requirements evaluation" --body "Adds deterministic intake/context alignment checks for requirements and surfaces them in evaluate, console, and workbench. Verification: go test ./...; go vet ./...; go build ./cmd/devflow; git diff --check; release-readiness 0.1.0-wave22."
```

Wait for CI:

```powershell
gh pr view --json number,state,mergeable,statusCheckRollup,url
```

Expected:

- PR is mergeable.
- Ubuntu and Windows Go verification pass.

---

## Self-Review

- Scope coverage: This plan implements user-selected options 1 and 2: context-aware requirements evaluation plus console/workbench visibility.
- Safety: No workflow state changes, no auto-confirmation, no external adapters, no LLM calls.
- Determinism: Checks are text/section/bullet matching only.
- Data boundaries: `intake.md`, `context.md`, and `requirements.md` stay distinct.
- Test coverage: Demandflow unit tests, CLI evaluate tests, console tests, workbench snapshot tests, release-readiness, and manual smoke are specified.
- Placeholder scan: All functions, IDs, commands, and file paths are concrete. Angle-bracket syntax appears only in existing CLI-style command examples.
