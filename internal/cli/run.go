package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
)

var newDemandRunner = func(configPath string, mode permissions.PermissionMode) demandflow.Runner {
	return demandflow.RuntimeRunner{ConfigPath: configPath, PermissionMode: mode, MaxIterations: 20}
}

var newReviewAdapter = func() adapters.ReviewAdapter {
	return adapters.GitLabReviewAdapter{}
}

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func runDemandStage(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var root, demandID, stage, configPath, permissionMode, gitlabProject, gitlabMR, gitlabBaseURL string
	var qualityCommands stringSliceFlag

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&stage, "stage", "", "demand stage")
	fs.StringVar(&configPath, "config", "", "devflow config path")
	fs.StringVar(&permissionMode, "permission-mode", "", "permission mode for implementation (acceptEdits or bypassPermissions)")
	fs.StringVar(&gitlabProject, "gitlab-project", "", "gitlab project path for mr-review")
	fs.StringVar(&gitlabMR, "gitlab-mr", "", "gitlab merge request iid for mr-review")
	fs.StringVar(&gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")
	fs.Var(&qualityCommands, "quality-command", "quality command as a quoted program and args (repeatable)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	demandID = strings.TrimSpace(demandID)
	stage = strings.TrimSpace(stage)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	if stage == "" {
		return fmt.Errorf("--stage is required")
	}

	parsedStage, err := demandflow.ParseStage(stage)
	if err != nil {
		return err
	}

	var commands []quality.Command
	for _, raw := range qualityCommands {
		parts, err := parseCommandLine(raw)
		if err != nil {
			return fmt.Errorf("parse --quality-command %q: %w", raw, err)
		}
		if len(parts) == 0 {
			continue
		}
		commands = append(commands, quality.Command{Name: parts[0], Args: parts[1:]})
	}

	opts := demandflow.Options{
		Root:            root,
		DemandID:        demandID,
		Stage:           parsedStage,
		QualityCommands: commands,
		Runner:          newDemandRunner(configPath, permissions.PermissionMode(permissionMode)),
		Now:             time.Now,
	}

	if parsedStage == demandflow.StageMRReview {
		if strings.TrimSpace(gitlabProject) == "" || strings.TrimSpace(gitlabMR) == "" {
			return fmt.Errorf("--gitlab-project and --gitlab-mr are required for mr-review")
		}
		opts.Review = demandflow.ReviewOptions{
			Adapter: newReviewAdapter(),
			Ref: adapters.ReviewRef{
				Project:      gitlabProject,
				MergeRequest: gitlabMR,
				BaseURL:      gitlabBaseURL,
			},
		}
	}
	engine := demandflow.NewEngine(root)
	result, err := engine.RunDetailed(context.Background(), opts)
	if err != nil {
		if result.DemandID != "" {
			printRunResult(stdout, result)
		}
		return err
	}
	printRunResult(stdout, result)
	return nil
}

func printRunResult(stdout io.Writer, result demandflow.RunResult) {
	fmt.Fprintf(stdout, "stage %s completed for %s\n", result.Stage, result.DemandID)
	if result.PreviousState != "" || result.CurrentState != "" {
		fmt.Fprintf(stdout, "state: %s -> %s\n", result.PreviousState, result.CurrentState)
	}
	if result.Message != "" {
		fmt.Fprintf(stdout, "%s\n", result.Message)
	}
	if len(result.Artifacts) > 0 {
		fmt.Fprintln(stdout, "artifacts:")
		for _, artifact := range result.Artifacts {
			fmt.Fprintf(stdout, "  - %s\n", artifact)
		}
	}
	if len(result.NextActions) > 0 {
		fmt.Fprintln(stdout, "next:")
		action := result.NextActions[0]
		fmt.Fprintf(stdout, "  %s\n", action.Label)
		if strings.TrimSpace(action.Command) != "" {
			fmt.Fprintf(stdout, "  %s\n", action.Command)
		}
	}
}
