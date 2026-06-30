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
	case StagePlan:
		return evaluatePlan(root, demandID), nil
	case StageVerification:
		return evaluateVerification(root, demandID)
	case StageCloseout:
		return evaluateCloseout(root, demandID), nil
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
	return buildStageEvaluation(StageRequirements, checks)
}

func evaluatePlan(root, demandID string) StageEvaluation {
	text := readEvaluationArtifact(root, demandID, artifacts.PlanFile)
	checks := []EvaluationCheck{
		requiredContentCheck("plan.exists", "plan.md has content", text, "blocker"),
		requiredSectionCheck("plan.steps", "implementation steps section has content", text, []string{"实施步骤", "implementation steps", "steps"}, "blocker"),
		requiredSectionCheck("plan.tests", "test strategy section has content", text, []string{"测试", "test strategy", "verification"}, "warning"),
		requiredSectionCheck("plan.risks", "risks section has content", text, []string{"风险", "risks"}, "warning"),
	}
	return buildStageEvaluation(StagePlan, checks)
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
	checks := []EvaluationCheck{
		statusCheck("verification.recorded", "verification evidence is recorded", latestStatus != "", "blocker", latestStatus),
		statusCheck("verification.pass", "latest verification status is pass", latestStatus == "pass", "blocker", latestStatus),
		statusCheck("verification.command", "verification command is recorded", latestCommand != "", "warning", latestCommand),
	}
	return buildStageEvaluation(StageVerification, checks), nil
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

func evaluateCloseout(root, demandID string) StageEvaluation {
	closeout := readEvaluationArtifact(root, demandID, artifacts.CloseoutFile)
	memory := readEvaluationArtifact(root, demandID, artifacts.MemoryCandidatesFile)
	checks := []EvaluationCheck{
		requiredContentCheck("closeout.exists", "closeout.md has content", closeout, "blocker"),
		requiredSectionCheck("closeout.result", "result section has content", closeout, []string{"需求结果", "result"}, "blocker"),
		statusCheck("closeout.memory", "memory candidates include reusable bullets", hasNonTemplateBullet(memory), "warning", evidenceSnippet(memory)),
	}
	return buildStageEvaluation(StageCloseout, checks)
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
