package demandflow

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ArtifactInfo struct {
	Name   string
	Path   string
	Exists bool
	Size   int64
}

type NextAction struct {
	Label   string
	Command string
	Reason  string
}

type StatusReport struct {
	Demand    artifacts.Demand
	State     workflow.State
	DemandDir string
	Artifacts []ArtifactInfo
	Actions   []NextAction
}

func InspectStatus(root, demandID string) (StatusReport, error) {
	summary, err := InspectWorkspace(root, demandID)
	if err != nil {
		return StatusReport{}, err
	}
	artifacts := make([]ArtifactInfo, 0, len(summary.Artifacts))
	for _, artifact := range summary.Artifacts {
		artifacts = append(artifacts, ArtifactInfo{
			Name:   artifact.Name,
			Path:   artifact.Path,
			Exists: artifact.Exists,
			Size:   artifact.Size,
		})
	}
	return StatusReport{
		Demand:    summary.Demand,
		State:     summary.State,
		DemandDir: summary.DemandDir,
		Artifacts: artifacts,
		Actions:   summary.Actions,
	}, nil
}

func NextActions(state workflow.State, demandID string) []NextAction {
	idArg := shellQuote(demandID)
	switch state {
	case workflow.Created, workflow.ContextLoaded:
		return []NextAction{{Label: "Draft requirements", Command: "devflow run --demand " + idArg + " --stage requirements", Reason: "The demand needs requirements before human review."}}
	case workflow.RequirementsDrafting:
		return []NextAction{{Label: "Continue requirements", Command: "devflow run --demand " + idArg + " --stage requirements", Reason: "Requirements drafting is in progress."}}
	case workflow.RequirementsReview:
		return []NextAction{{Label: "Confirm requirements", Command: "devflow confirm --demand " + idArg + " --stage requirements --by <name> --summary <summary>", Reason: "Requirements need human confirmation before planning."}}
	case workflow.PlanDrafting:
		return []NextAction{{Label: "Draft plan", Command: "devflow run --demand " + idArg + " --stage plan", Reason: "The confirmed requirements are ready for planning."}}
	case workflow.PlanReview:
		return []NextAction{{Label: "Confirm plan", Command: "devflow confirm --demand " + idArg + " --stage plan --by <name> --summary <summary>", Reason: "The technical plan needs human confirmation before implementation."}}
	case workflow.Implementation:
		return []NextAction{{Label: "Run implementation", Command: "devflow run --demand " + idArg + " --stage implementation --permission-mode acceptEdits --quality-command \"go test ./...\"", Reason: "Implementation can now edit code and run quality gates."}}
	case workflow.ReturnedToRequirements:
		return []NextAction{{Label: "Revise requirements", Command: "devflow run --demand " + idArg + " --stage requirements", Reason: "MR review found requirements-level feedback; revise requirements before planning again."}}
	case workflow.ReturnedToPlan:
		return []NextAction{{Label: "Revise plan", Command: "devflow run --demand " + idArg + " --stage plan", Reason: "MR review found plan-level feedback; revise the plan before implementation resumes."}}
	case workflow.FailedQualityGate:
		return []NextAction{{Label: "Retry implementation", Command: "devflow run --demand " + idArg + " --stage implementation --permission-mode acceptEdits --quality-command \"go test ./...\"", Reason: "The previous quality gate failed; rerun implementation after addressing failures."}}
	case workflow.MRReview:
		return []NextAction{{Label: "Check MR review", Command: "devflow run --demand " + idArg + " --stage mr-review --gitlab-project <group/project> --gitlab-mr <iid>", Reason: "MR review must be clear before verification."}}
	case workflow.Verification:
		return []NextAction{
			{Label: "Draft verification", Command: "devflow run --demand " + idArg + " --stage verification --quality-command \"go test ./...\"", Reason: "Verification evidence should be generated or refreshed."},
			{Label: "Confirm verification", Command: "devflow confirm --demand " + idArg + " --stage verification --by <name> --summary <summary>", Reason: "Human confirmation advances verification to closeout."},
		}
	case workflow.Closeout:
		return []NextAction{
			{Label: "Draft closeout", Command: "devflow run --demand " + idArg + " --stage closeout", Reason: "Closeout and memory candidates should be generated or refreshed."},
			{Label: "Confirm closeout", Command: "devflow confirm --demand " + idArg + " --stage closeout --by <name> --summary <summary>", Reason: "Human confirmation completes the demand."},
		}
	case workflow.Completed:
		return []NextAction{{Label: "No action", Command: "", Reason: "The demand is complete."}}
	default:
		return []NextAction{{Label: "Inspect manually", Command: "devflow status --demand " + idArg, Reason: fmt.Sprintf("State %s has no automated recommendation.", state)}}
	}
}

func shellQuote(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\r\n\"'") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}
