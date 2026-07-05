package demandflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/releasecontrol"
	"github.com/jesseedcp/devflow-agent/internal/wiki"
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
		stages = []Stage{StageRequirements, StagePlan, StageImplementation, StageVerification, StageDeployment, StageObservation, StageRollback, StageCloseout}
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
	case StagePlan:
		return evaluatePlan(root, demandID), nil
	case StageImplementation:
		return evaluateImplementation(root, demandID), nil
	case StageVerification:
		return evaluateVerification(root, demandID)
	case StageDeployment:
		return evaluateDeployment(root, demandID), nil
	case StageObservation:
		return evaluateObservation(root, demandID), nil
	case StageRollback:
		return evaluateRollback(root, demandID), nil
	case StageCloseout:
		return evaluateCloseout(root, demandID), nil
	default:
		return StageEvaluation{Stage: stage, Status: EvaluationNotApplicable}, nil
	}
}

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
		contextPresenceCheck(intakeText, contextText),
		stableMemoryReferenceCheck(contextText, text),
		candidateMemoryGuardCheck(contextText, text),
	}
	return buildStageEvaluation(StageRequirements, checks)
}

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

func contextPresenceCheck(intakeText, contextText string) EvaluationCheck {
	if len(requirementRelevantBullets(intakeText, []string{"目标", "业务规则", "验收", "acceptance", "rule", "goal"})) == 0 {
		return EvaluationCheck{
			ID:       "requirements.context_presence",
			Label:    "context.md exists with recall sections",
			Status:   EvaluationNotApplicable,
			Severity: "warning",
			Evidence: "no intake context expected",
		}
	}
	trimmed := strings.TrimSpace(contextText)
	if trimmed == "" {
		return statusCheck("requirements.context_presence", "context.md exists with recall sections", false, "warning", "context.md empty or missing")
	}
	lower := strings.ToLower(trimmed)
	ok := strings.Contains(lower, "approved stable memory") && strings.Contains(lower, "historical demand candidates")
	return statusCheck("requirements.context_presence", "context.md exists with recall sections", ok, "warning", evidenceSnippet(contextText))
}

func stableMemoryReferenceCheck(contextText, requirementsText string) EvaluationCheck {
	stable := removeNoMemoryBullets(contextSectionBullets(contextText, "approved stable memory"))
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
	candidates := removeNoMemoryBullets(contextSectionBullets(contextText, "historical demand candidates"))
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

func evaluatePlan(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.PlanFile)
	codemapText := readEvaluationArtifact(root, demandID, artifacts.CodemapFile)
	planContextText := readEvaluationArtifact(root, demandID, artifacts.PlanContextFile)
	changeScopeText := readEvaluationArtifact(root, demandID, artifacts.ChangeScopeFile)
	checks := []EvaluationCheck{
		requiredContentCheck("plan.exists", "plan.md has content", text, "blocker"),
		requiredSectionCheck("plan.steps", "implementation steps section has content", text, []string{"实施步骤", "implementation steps", "steps"}, "blocker"),
		requiredSectionCheck("plan.tests", "test strategy section has content", text, []string{"测试", "test strategy", "verification"}, "warning"),
		requiredSectionCheck("plan.risks", "risks section has content", text, []string{"风险", "risks"}, "warning"),
		codemapReferenceCheck(codemapText, text),
		planContextGroundingCheck(planContextText),
		planChangeScopeCheck(changeScopeText, text),
	}
	return buildStageEvaluation(StagePlan, checks)
}

func evaluateImplementation(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.ImplementationReviewFile)
	checks := []EvaluationCheck{
		implementationReviewCheck(text),
	}
	return buildStageEvaluation(StageImplementation, checks)
}

func implementationReviewCheck(text string) EvaluationCheck {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || !strings.Contains(trimmed, "Recommendation:") {
		return EvaluationCheck{
			ID:       "implementation.review",
			Label:    "implementation review is recorded",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: "implementation-review.md missing or template-only",
		}
	}
	if strings.Contains(trimmed, "ready_for_closeout") {
		return statusCheck("implementation.review", "implementation review is recorded", true, "warning", evidenceSnippet(trimmed))
	}
	return EvaluationCheck{
		ID:       "implementation.review",
		Label:    "implementation review is recorded",
		Status:   EvaluationWarning,
		Severity: "warning",
		Evidence: evidenceSnippet(trimmed),
	}
}

func planContextGroundingCheck(planContextText string) EvaluationCheck {
	trimmed := strings.TrimSpace(planContextText)
	lower := strings.ToLower(trimmed)
	ok := strings.Contains(lower, "codemap context") && strings.Contains(trimmed, ".go")
	return statusCheck("plan.context_grounding", "plan context includes codemap facts", ok, "warning", evidenceSnippet(trimmed))
}

func planChangeScopeCheck(changeScopeText, planText string) EvaluationCheck {
	files := codeFilesFromCodemap(changeScopeText)
	if len(files) == 0 {
		return EvaluationCheck{
			ID:       "plan.change_scope",
			Label:    "plan declares source and test change scope",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: "change-scope.md has no source/test files",
		}
	}
	var referenced []string
	for _, file := range files {
		if strings.Contains(planText, file) {
			referenced = append(referenced, file)
		}
	}
	if len(referenced) == 0 {
		return EvaluationCheck{ID: "plan.change_scope", Label: "plan declares source and test change scope", Status: EvaluationWarning, Severity: "warning", Evidence: strings.Join(limitStrings(files, 3), " | ")}
	}
	return statusCheck("plan.change_scope", "plan declares source and test change scope", true, "warning", strings.Join(limitStrings(referenced, 3), " | "))
}

func codemapReferenceCheck(codemapText, planText string) EvaluationCheck {
	files := codeFilesFromCodemap(codemapText)
	if len(files) == 0 {
		return EvaluationCheck{
			ID:       "plan.codemap_reference",
			Label:    "plan references likely impacted files from codemap",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: "codemap.md has no likely impacted files",
		}
	}
	var referenced []string
	for _, file := range files {
		if strings.Contains(planText, file) {
			referenced = append(referenced, file)
		}
	}
	if len(referenced) == 0 {
		return EvaluationCheck{
			ID:       "plan.codemap_reference",
			Label:    "plan references likely impacted files from codemap",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: strings.Join(limitStrings(files, 3), " | "),
		}
	}
	return statusCheck("plan.codemap_reference", "plan references likely impacted files from codemap", true, "warning", strings.Join(limitStrings(referenced, 3), " | "))
}

func codeFilesFromCodemap(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		for _, field := range strings.Fields(line) {
			field = strings.Trim(field, "`:,()[]")
			if strings.HasSuffix(field, ".go") || strings.Contains(field, ".go:") {
				file := strings.Split(field, ":")[0]
				if strings.HasSuffix(file, ".go") && !seen[file] {
					seen[file] = true
					out = append(out, file)
				}
			}
		}
	}
	return out
}

func evaluateVerification(root, demandID string) (StageEvaluation, error) {
	store := artifacts.NewStore(root)
	events, err := store.ReadEvents(demandID)
	if err != nil {
		return StageEvaluation{}, err
	}
	latestStatus := ""
	latestCommand := ""
	for _, event := range events {
		if event.Type != "verification.recorded" {
			continue
		}
		latestStatus = normalizeVerificationEvaluationStatus(event.Data["status"])
		latestCommand = strings.TrimSpace(event.Data["command"])
	}
	manual := summarizeEvidence(events)
	checks := []EvaluationCheck{
		statusCheck("verification.recorded", "verification evidence is recorded", latestStatus != "", "blocker", latestStatus),
		statusCheck("verification.pass", "latest verification status is pass", latestStatus == "pass", "blocker", latestStatus),
		statusCheck("verification.command", "verification command is recorded", latestCommand != "", "warning", latestCommand),
		acceptanceEvidencePresenceCheck(manual),
		acceptanceEvidencePassCheck(manual),
	}
	return buildStageEvaluation(StageVerification, checks), nil
}

func acceptanceEvidencePresenceCheck(summary EvidenceSummary) EvaluationCheck {
	total := summary.Pass + summary.Fail + summary.Blocked
	if total == 0 {
		return EvaluationCheck{
			ID:       "verification.acceptance_evidence",
			Label:    "acceptance evidence is recorded",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: "no acceptance evidence recorded",
		}
	}
	return statusCheck("verification.acceptance_evidence", "acceptance evidence is recorded", true, "warning", fmt.Sprintf("pass=%d fail=%d blocked=%d", summary.Pass, summary.Fail, summary.Blocked))
}

func acceptanceEvidencePassCheck(summary EvidenceSummary) EvaluationCheck {
	if summary.Fail > 0 || summary.Blocked > 0 {
		return EvaluationCheck{
			ID:       "verification.acceptance_evidence_pass",
			Label:    "acceptance evidence has at least one pass and no failures or blockers",
			Status:   EvaluationFail,
			Severity: "blocker",
			Evidence: fmt.Sprintf("pass=%d fail=%d blocked=%d", summary.Pass, summary.Fail, summary.Blocked),
		}
	}
	if summary.Pass == 0 {
		return EvaluationCheck{
			ID:       "verification.acceptance_evidence_pass",
			Label:    "acceptance evidence has at least one pass and no failures or blockers",
			Status:   EvaluationNotApplicable,
			Severity: "blocker",
			Evidence: "no acceptance evidence recorded",
		}
	}
	return statusCheck("verification.acceptance_evidence_pass", "acceptance evidence has at least one pass and no failures or blockers", true, "blocker", fmt.Sprintf("pass=%d fail=%d blocked=%d", summary.Pass, summary.Fail, summary.Blocked))
}

func normalizeVerificationEvaluationStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass", "passed", "success", "ok":
		return "pass"
	case "fail", "failed", "failure", "error":
		return "fail"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func evaluateDeployment(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.DeploymentFile)
	record := releasecontrol.ParseDeployment(text)
	checks := []EvaluationCheck{
		requiredContentCheck("deployment.exists", "deployment.md has content", text, "blocker"),
		statusCheck("deployment.status", "deployment passed", record.Status == releasecontrol.StatusPassed, "blocker", fmt.Sprintf("status=%s conclusion=%s run=%s", record.Status, record.Conclusion, record.RunID)),
		statusCheck("deployment.run", "deployment run id recorded", strings.TrimSpace(record.RunID) != "", "warning", record.RunID),
	}
	return buildStageEvaluation(StageDeployment, checks)
}

func evaluateObservation(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.ObservationFile)
	record := releasecontrol.ParseObservation(text)
	checks := []EvaluationCheck{
		requiredContentCheck("observation.exists", "observation.md has content", text, "blocker"),
		statusCheck("observation.status", "observation passed", record.Status == releasecontrol.StatusPassed, "blocker", fmt.Sprintf("status=%s deployment=%s", record.Status, record.DeploymentStatus)),
	}
	return buildStageEvaluation(StageObservation, checks)
}

func evaluateRollback(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.RollbackFile)
	record := releasecontrol.ParseRollback(text)
	observationText := readEvaluationArtifact(root, demandID, artifacts.ObservationFile)
	observation := releasecontrol.ParseObservation(observationText)
	decided := record.Decision == releasecontrol.RollbackConfirmed ||
		record.Decision == releasecontrol.RollbackRiskAccepted ||
		record.Decision == releasecontrol.RollbackRedeployRequired
	checks := []EvaluationCheck{
		requiredContentCheck("rollback.exists", "rollback.md has content", text, "warning"),
	}
	if observation.Status == releasecontrol.StatusPassed {
		checks = append(checks, EvaluationCheck{
			ID:       "rollback.decision",
			Label:    "rollback decision recorded when needed",
			Status:   EvaluationNotApplicable,
			Severity: "warning",
			Evidence: "observation passed; no rollback needed",
		})
	} else {
		checks = append(checks, statusCheck("rollback.decision", "rollback decision recorded when needed", decided, "warning", string(record.Decision)))
	}
	return buildStageEvaluation(StageRollback, checks)
}

func releaseEvidenceCloseoutCheck(root, demandID string) EvaluationCheck {
	observationText := readEvaluationArtifact(root, demandID, artifacts.ObservationFile)
	rollbackText := readEvaluationArtifact(root, demandID, artifacts.RollbackFile)
	observation := releasecontrol.ParseObservation(observationText)
	rollback := releasecontrol.ParseRollback(rollbackText)
	passed := observation.Status == releasecontrol.StatusPassed || rollback.Decision == releasecontrol.RollbackRiskAccepted
	return statusCheck("closeout.release_evidence", "release-control evidence allows closeout", passed, "blocker", fmt.Sprintf("observation=%s rollback=%s", observation.Status, rollback.Decision))
}
func evaluateCloseout(root, demandID string) StageEvaluation {
	closeout := readEvaluationArtifact(root, demandID, artifacts.CloseoutFile)
	memory := readEvaluationArtifact(root, demandID, artifacts.MemoryCandidatesFile)
	wikiCandidatesText := readEvaluationArtifact(root, demandID, artifacts.WikiCandidatesFile)
	metricsText := readEvaluationArtifact(root, demandID, artifacts.MetricsFile)
	checks := []EvaluationCheck{
		requiredContentCheck("closeout.exists", "closeout.md has content", closeout, "blocker"),
		requiredSectionCheck("closeout.result", "result section has content", closeout, []string{"需求结果", "result"}, "blocker"),
		releaseEvidenceCloseoutCheck(root, demandID),
		statusCheck("closeout.memory", "memory candidates include reusable bullets", hasNonTemplateBullet(memory), "warning", evidenceSnippet(memory)),
		wikiCandidatesCheck(wikiCandidatesText),
		wikiDecisionsCheck(wikiCandidatesText),
		metricsReportCheck(metricsText),
	}
	return buildStageEvaluation(StageCloseout, checks)
}

func metricsReportCheck(text string) EvaluationCheck {
	trimmed := strings.TrimSpace(text)
	if strings.Contains(trimmed, "# Devflow Metrics") && strings.Contains(trimmed, "## Summary") {
		return EvaluationCheck{
			ID:       "closeout.metrics_report",
			Label:    "metrics report generated",
			Status:   EvaluationPass,
			Severity: "warning",
			Evidence: evidenceSnippet(trimmed),
		}
	}
	return EvaluationCheck{
		ID:       "closeout.metrics_report",
		Label:    "metrics report generated",
		Status:   EvaluationWarning,
		Severity: "warning",
		Evidence: "metrics.md missing or template-only; run `devflow metrics report --demand <id>`",
	}
}

func wikiCandidatesCheck(text string) EvaluationCheck {
	candidates := wiki.ParseCandidates(text)
	if len(candidates) > 0 {
		return EvaluationCheck{
			ID:       "closeout.wiki_candidates",
			Label:    "wiki candidates distilled from closeout material",
			Status:   EvaluationPass,
			Severity: "warning",
			Evidence: fmt.Sprintf("%d wiki candidates present", len(candidates)),
		}
	}
	return EvaluationCheck{
		ID:       "closeout.wiki_candidates",
		Label:    "wiki candidates distilled from closeout material",
		Status:   EvaluationWarning,
		Severity: "warning",
		Evidence: "wiki-candidates.md missing or only template lines; run `devflow wiki distill --demand <id>`",
	}
}

func wikiDecisionsCheck(text string) EvaluationCheck {
	candidates := wiki.ParseCandidates(text)
	if len(candidates) == 0 {
		return EvaluationCheck{
			ID:       "closeout.wiki_decisions",
			Label:    "all wiki candidates promoted or rejected",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: "no wiki candidates to decide",
		}
	}
	pending := 0
	for _, candidate := range candidates {
		if candidate.Status == wiki.StatusPending {
			pending++
		}
	}
	if pending > 0 {
		return EvaluationCheck{
			ID:       "closeout.wiki_decisions",
			Label:    "all wiki candidates promoted or rejected",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: fmt.Sprintf("%d pending wiki candidates need promote/reject review", pending),
		}
	}
	return EvaluationCheck{
		ID:       "closeout.wiki_decisions",
		Label:    "all wiki candidates promoted or rejected",
		Status:   EvaluationPass,
		Severity: "warning",
		Evidence: fmt.Sprintf("%d wiki candidates decided", len(candidates)),
	}
}

func buildStageEvaluation(stage Stage, checks []EvaluationCheck) StageEvaluation {
	out := StageEvaluation{Stage: stage, Status: EvaluationPass, Checks: checks}
	for _, check := range checks {
		switch check.Status {
		case EvaluationFail:
			if check.Severity == "blocker" {
				out.Blockers++
			} else {
				out.Warnings++
			}
		case EvaluationWarning:
			out.Warnings++
		}
	}
	switch {
	case out.Blockers > 0:
		out.Status = EvaluationFail
	case out.Warnings > 0:
		out.Status = EvaluationWarning
	}
	return out
}

func requiredContentCheck(id, label, text, severity string) EvaluationCheck {
	return statusCheck(id, label, strings.TrimSpace(text) != "", severity, evidenceSnippet(text))
}

func requiredSectionCheck(id, label, text string, headings []string, severity string) EvaluationCheck {
	found := false
	evidence := ""
	for _, heading := range headings {
		section := sectionAfterHeading(text, heading)
		if strings.TrimSpace(section) != "" {
			found = true
			evidence = evidenceSnippet(section)
			break
		}
	}
	return statusCheck(id, label, found, severity, evidence)
}

func statusCheck(id, label string, ok bool, severity, evidence string) EvaluationCheck {
	status := EvaluationPass
	if !ok {
		status = EvaluationFail
		if severity != "blocker" {
			status = EvaluationWarning
		}
	}
	return EvaluationCheck{ID: id, Label: label, Status: status, Severity: severity, Evidence: strings.TrimSpace(evidence)}
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

func readEvaluationArtifact(root, demandID, name string) string {
	path := filepath.Join(root, ".devflow", "demands", demandID, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func sectionAfterHeading(text, heading string) string {
	needle := strings.ToLower(heading)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	inSection := false
	var section strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if inSection {
				break
			}
			if strings.Contains(strings.ToLower(trimmed), needle) {
				inSection = true
			}
			continue
		}
		if inSection {
			section.WriteString(line)
			section.WriteByte('\n')
		}
	}
	return section.String()
}

func evidenceSnippet(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	if len(trimmed) > 120 {
		return fmt.Sprintf("%s...", trimmed[:120])
	}
	return trimmed
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
		if trimmed == "" || strings.Contains(lower, "todo") || strings.Contains(lower, "待人工补充") || strings.Contains(lower, "待补充") || strings.Contains(lower, "placeholder") || strings.Contains(lower, "no approved stable memory") || strings.Contains(lower, "no historical candidate memory") {
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

func hasNonTemplateBullet(text string) bool {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "-") {
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "todo") || strings.Contains(lower, "待补充") || strings.Contains(lower, "placeholder") {
			continue
		}
		return true
	}
	return false
}
