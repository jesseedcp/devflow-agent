package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/releasecontrol"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func runRollback(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: devflow rollback <plan|confirm> ...")
	}
	switch args[0] {
	case "plan":
		return runRollbackPlan(args[1:], stdout, stderr)
	case "confirm":
		return runRollbackConfirm(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown rollback subcommand %q", args[0])
	}
}

func runRollbackPlan(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("rollback plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, trigger, impact, recommendation string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&trigger, "trigger", "", "rollback trigger")
	fs.StringVar(&impact, "impact", "", "rollback impact")
	fs.StringVar(&recommendation, "recommendation", "", "recommended action")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	current := workflow.State(demand.State)
	if !rollbackPlanAllowedState(current) {
		return fmt.Errorf("rollback plan requires a release state (deployment, observation, or blocked_need_release_decision), got %s", current)
	}

	record := releasecontrol.RollbackRecord{
		Trigger:     trigger,
		Impact:      impact,
		Recommended: recommendation,
	}
	if err := store.WriteArtifact(demandID, artifacts.RollbackFile, releasecontrol.RenderRollback(demand.Title, record)); err != nil {
		return err
	}
	if err := appendRollbackRecommended(store, demandID); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "rollback plan recorded")
	return nil
}

func rollbackPlanAllowedState(state workflow.State) bool {
	switch state {
	case workflow.Deployment, workflow.Observation, workflow.BlockedNeedReleaseDecision:
		return true
	}
	return false
}

func runRollbackConfirm(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("rollback confirm", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, decision, by, summary string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&decision, "decision", "", "rollback decision")
	fs.StringVar(&by, "by", "", "decision recorder")
	fs.StringVar(&summary, "summary", "", "decision summary")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	demandID = strings.TrimSpace(demandID)
	decision = strings.TrimSpace(decision)
	by = strings.TrimSpace(by)
	summary = strings.TrimSpace(summary)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	if by == "" || summary == "" {
		return fmt.Errorf("--by and --summary are required")
	}
	decisionValue, err := parseRollbackDecision(decision)
	if err != nil {
		return err
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	if workflow.State(demand.State) != workflow.BlockedNeedReleaseDecision {
		return fmt.Errorf("rollback confirm requires state blocked_need_release_decision, got %s", demand.State)
	}

	record := releasecontrol.RollbackRecord{
		Decision:      decisionValue,
		DecisionBy:    by,
		DecisionNotes: summary,
		RecordedAt:    time.Now().UTC(),
	}
	if err := store.WriteArtifact(demandID, artifacts.RollbackFile, releasecontrol.RenderRollback(demand.Title, record)); err != nil {
		return err
	}
	if err := store.AppendEvent(demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    "rollback.decision_recorded",
		Message: "rollback decision recorded",
		Data: map[string]string{
			"decision": string(decisionValue),
			"by":       by,
			"summary":  summary,
		},
	}); err != nil {
		return err
	}

	switch decisionValue {
	case releasecontrol.RollbackRiskAccepted:
		if err := advanceDemandState(store, demandID, workflow.BlockedNeedReleaseDecision, workflow.Closeout); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "rollback decision recorded: risk accepted")
	case releasecontrol.RollbackRedeployRequired:
		if err := advanceDemandState(store, demandID, workflow.BlockedNeedReleaseDecision, workflow.Deployment); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "rollback decision recorded: redeploy required")
	case releasecontrol.RollbackConfirmed:
		fmt.Fprintln(stdout, "rollback decision recorded: rollback confirmed")
	}
	return nil
}

func parseRollbackDecision(value string) (releasecontrol.RollbackDecision, error) {
	switch releasecontrol.RollbackDecision(value) {
	case releasecontrol.RollbackConfirmed, releasecontrol.RollbackRiskAccepted, releasecontrol.RollbackRedeployRequired:
		return releasecontrol.RollbackDecision(value), nil
	}
	return "", fmt.Errorf("invalid rollback decision %q (want rollback_confirmed, risk_accepted, or redeploy_required)", value)
}
