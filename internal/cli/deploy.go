package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/releasecontrol"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func runDeploy(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: devflow deploy <trigger|status> ...")
	}
	switch args[0] {
	case "trigger":
		return runDeployCommand(args[1:], stdout, stderr, true)
	case "status":
		return runDeployCommand(args[1:], stdout, stderr, false)
	default:
		return fmt.Errorf("unknown deploy subcommand %q", args[0])
	}
}

type deployOptions struct {
	root        string
	demandID    string
	provider    string
	repo        string
	workflowID  string
	ref         string
	environment string
	baseURL     string
	token       string
}

func parseDeployOptions(args []string, stderr io.Writer) (deployOptions, error) {
	var opts deployOptions
	fs := flag.NewFlagSet("deploy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.StringVar(&opts.provider, "provider", "github", "deployment provider")
	fs.StringVar(&opts.repo, "github-repo", "", "GitHub repository in owner/repo form")
	fs.StringVar(&opts.workflowID, "workflow", "", "GitHub Actions workflow id or filename")
	fs.StringVar(&opts.ref, "ref", "", "branch or commit ref to deploy")
	fs.StringVar(&opts.environment, "environment", "", "deployment environment")
	fs.StringVar(&opts.baseURL, "github-base-url", "", "GitHub API base url override")
	fs.StringVar(&opts.token, "github-token", "", "GitHub token override")
	if err := fs.Parse(args); err != nil {
		return deployOptions{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	opts.provider = strings.TrimSpace(opts.provider)
	opts.repo = strings.TrimSpace(opts.repo)
	opts.workflowID = strings.TrimSpace(opts.workflowID)
	opts.ref = strings.TrimSpace(opts.ref)
	if opts.demandID == "" {
		return deployOptions{}, fmt.Errorf("--demand is required")
	}
	if opts.provider != "github" {
		return deployOptions{}, fmt.Errorf("--provider accepts only github for v0.9")
	}
	if opts.repo == "" {
		return deployOptions{}, fmt.Errorf("--github-repo is required")
	}
	if opts.workflowID == "" {
		return deployOptions{}, fmt.Errorf("--workflow is required")
	}
	if opts.ref == "" {
		return deployOptions{}, fmt.Errorf("--ref is required")
	}
	return opts, nil
}

func (opts deployOptions) deploymentRef() adapters.DeploymentRef {
	return adapters.DeploymentRef{
		Repo:        opts.repo,
		WorkflowID:  opts.workflowID,
		Ref:         opts.ref,
		Environment: opts.environment,
		BaseURL:     opts.baseURL,
		Token:       opts.token,
	}
}

func runDeployCommand(args []string, stdout io.Writer, stderr io.Writer, dispatch bool) error {
	opts, err := parseDeployOptions(args, stderr)
	if err != nil {
		return err
	}

	store := artifacts.NewStore(opts.root)
	demand, err := store.LoadDemand(opts.demandID)
	if err != nil {
		return err
	}
	current := workflow.State(demand.State)
	if dispatch && current != workflow.Deployment {
		if current == workflow.Verification {
			return fmt.Errorf("deploy trigger requires state deployment; confirm verification first")
		}
		return fmt.Errorf("deploy trigger requires state deployment, got %s", current)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adapter := adapters.GitHubActionsAdapter{Client: http.DefaultClient}
	var result adapters.DeploymentResult
	if dispatch {
		result, err = adapter.TriggerDeployment(ctx, opts.deploymentRef())
	} else {
		result, err = adapter.GetDeployment(ctx, opts.deploymentRef())
	}
	if err != nil {
		return err
	}

	return recordDeploymentResult(store, opts.demandID, demand.Title, result, stdout)
}

func recordDeploymentResult(store artifacts.Store, demandID, title string, result adapters.DeploymentResult, stdout io.Writer) error {
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	current := workflow.State(demand.State)
	record := deploymentRecordFromResult(result)
	if err := store.WriteArtifact(demandID, artifacts.DeploymentFile, releasecontrol.RenderDeployment(title, record)); err != nil {
		return err
	}
	if err := appendDeploymentEvent(store, demandID, result, record); err != nil {
		return err
	}

	switch result.Status {
	case adapters.DeploymentStatusPassed:
		if current == workflow.Deployment {
			if err := advanceDemandState(store, demandID, workflow.Deployment, workflow.Observation); err != nil {
				return err
			}
		}
		fmt.Fprintln(stdout, "deployment passed")
	case adapters.DeploymentStatusFailed:
		if err := writeRollbackRecommendation(store, demandID, title, "deployment failed", "release did not complete", "rerun deployment after fix"); err != nil {
			return err
		}
		if err := appendRollbackRecommended(store, demandID); err != nil {
			return err
		}
		if current == workflow.Deployment {
			if err := advanceDemandState(store, demandID, workflow.Deployment, workflow.BlockedNeedReleaseDecision); err != nil {
				return err
			}
		}
		fmt.Fprintln(stdout, "deployment failed")
	default:
		fmt.Fprintln(stdout, "deployment pending")
	}
	return nil
}

func deploymentRecordFromResult(result adapters.DeploymentResult) releasecontrol.DeploymentRecord {
	status := releasecontrol.StatusUnknown
	switch result.Status {
	case adapters.DeploymentStatusPassed:
		status = releasecontrol.StatusPassed
	case adapters.DeploymentStatusFailed:
		status = releasecontrol.StatusFailed
	case adapters.DeploymentStatusPending:
		status = releasecontrol.StatusPending
	}
	return releasecontrol.DeploymentRecord{
		Provider:    result.Provider,
		Repo:        result.Repo,
		WorkflowID:  result.WorkflowID,
		Ref:         result.Ref,
		Environment: result.Environment,
		RunID:       result.RunID,
		RunURL:      result.RunURL,
		HeadSHA:     result.HeadSHA,
		Status:      status,
		Conclusion:  result.Conclusion,
		Summary:     result.Message,
		TriggeredBy: "devflow",
		CreatedAt:   result.CreatedAt,
		UpdatedAt:   result.UpdatedAt,
	}
}

func appendDeploymentEvent(store artifacts.Store, demandID string, result adapters.DeploymentResult, record releasecontrol.DeploymentRecord) error {
	eventType := "deployment.pending"
	switch result.Status {
	case adapters.DeploymentStatusPassed:
		eventType = "deployment.passed"
	case adapters.DeploymentStatusFailed:
		eventType = "deployment.failed"
	}
	return store.AppendEvent(demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    eventType,
		Message: result.Message,
		Data: map[string]string{
			"provider":   "github_actions",
			"repo":       result.Repo,
			"workflow":   result.WorkflowID,
			"ref":        result.Ref,
			"run_id":     result.RunID,
			"run_url":    result.RunURL,
			"status":     string(record.Status),
			"conclusion": result.Conclusion,
		},
	})
}

func writeRollbackRecommendation(store artifacts.Store, demandID, title, trigger, impact, recommended string) error {
	record := releasecontrol.RollbackRecord{
		Trigger:     trigger,
		Impact:      impact,
		Recommended: recommended,
	}
	return store.WriteArtifact(demandID, artifacts.RollbackFile, releasecontrol.RenderRollback(title, record))
}

func appendRollbackRecommended(store artifacts.Store, demandID string) error {
	return store.AppendEvent(demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    "rollback.recommended",
		Message: "rollback decision recommended",
	})
}

func advanceDemandState(store artifacts.Store, demandID string, from, to workflow.State) error {
	return store.WithDemandLock(demandID, func() error {
		demand, err := store.LoadDemand(demandID)
		if err != nil {
			return err
		}
		if workflow.State(demand.State) != from {
			return fmt.Errorf("expected state %s, got %s", from, demand.State)
		}
		advanced, err := workflow.Advance(from, to)
		if err != nil {
			return err
		}
		demand.State = string(advanced)
		return store.SaveDemand(demand)
	})
}
