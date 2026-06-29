package demandflow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ConfirmOptions struct {
	Root     string
	DemandID string
	Stage    string
	By       string
	Summary  string
	Now      func() time.Time
}

type ConfirmResult struct {
	DemandID      string
	Stage         string
	Label         string
	PreviousState workflow.State
	CurrentState  workflow.State
	Artifact      string
}

func Confirm(opts ConfirmOptions) (ConfirmResult, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	demandID := strings.TrimSpace(opts.DemandID)
	stage := strings.TrimSpace(opts.Stage)
	by := NormalizeConfirmationText(opts.By)
	summary := NormalizeConfirmationText(opts.Summary)
	if demandID == "" || stage == "" || by == "" || summary == "" {
		return ConfirmResult{}, fmt.Errorf("--demand, --stage, --by, and --summary are required")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	artifactName, requiredCurrent, nextState, label, err := ConfirmationTarget(stage)
	if err != nil {
		return ConfirmResult{}, err
	}

	store := artifacts.NewStore(root)
	var result ConfirmResult
	err = store.WithDemandLock(demandID, func() error {
		demand, err := store.LoadDemand(demandID)
		if err != nil {
			return err
		}

		current := workflow.State(demand.State)
		result = ConfirmResult{
			DemandID:      demandID,
			Stage:         stage,
			Label:         label,
			PreviousState: current,
			Artifact:      artifactName,
		}
		if current != requiredCurrent {
			return fmt.Errorf("confirmation stage %q requires current state %s, got %s", stage, requiredCurrent, current)
		}

		advanced, err := workflow.Advance(current, nextState)
		if err != nil {
			return err
		}

		confirmedAt := opts.Now().UTC()
		cycleToken := demand.UpdatedAt.UTC().Format(time.RFC3339Nano)
		confirmationID := ConfirmationID(demandID, stage, cycleToken, by, summary)
		record := fmt.Sprintf("- %s confirmed by %s at %s: %s\n", label, by, confirmedAt.Format(time.RFC3339), summary)
		if err := store.EnsureConfirmationEvidence(demandID, artifactName, confirmationID, record, artifacts.Event{
			Time:    confirmedAt,
			Type:    "stage.confirmed",
			Message: label + " confirmed",
			Data: map[string]string{
				"by":              by,
				"stage":           stage,
				"summary":         summary,
				"confirmation_id": confirmationID,
			},
		}); err != nil {
			return err
		}

		demand.State = string(advanced)
		if err := store.SaveDemand(demand); err != nil {
			return err
		}
		result.CurrentState = advanced
		return nil
	})
	return result, err
}

func ConfirmationTarget(stage string) (artifact string, requiredCurrent workflow.State, next workflow.State, label string, err error) {
	switch stage {
	case "requirements":
		return artifacts.RequirementsFile, workflow.RequirementsReview, workflow.PlanDrafting, "requirements", nil
	case "plan":
		return artifacts.PlanFile, workflow.PlanReview, workflow.Implementation, "plan", nil
	case "verification":
		return artifacts.VerificationFile, workflow.Verification, workflow.Closeout, "verification", nil
	case "closeout":
		return artifacts.CloseoutFile, workflow.Closeout, workflow.Completed, "closeout", nil
	default:
		return "", "", "", "", fmt.Errorf("unsupported confirmation stage %q", stage)
	}
}

func NormalizeConfirmationText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func ConfirmationID(demandID, stage, cycleToken, by, summary string) string {
	normalizedDemandID := strings.ToLower(strings.TrimSpace(demandID))
	normalizedStage := strings.TrimSpace(stage)
	normalizedCycleToken := strings.TrimSpace(cycleToken)
	normalizedBy := NormalizeConfirmationText(by)
	normalizedSummary := NormalizeConfirmationText(summary)

	hash := sha256.Sum256([]byte(normalizedDemandID + "\x00" + normalizedStage + "\x00" + normalizedCycleToken + "\x00" + normalizedBy + "\x00" + normalizedSummary))
	return hex.EncodeToString(hash[:8])
}
