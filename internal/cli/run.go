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

var newMergeRequestAdapter = func() adapters.MergeRequestAdapter {
	return adapters.GitLabReviewAdapter{}
}

var newReviewAdapter = func() adapters.ReviewAdapter {
	return adapters.GitLabReviewAdapter{}
}

var newCIGateAdapter = func() adapters.CIGateAdapter {
	return adapters.GitHubCIAdapter{}
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

	var root, runnerRoot, qualityRoot, demandID, stage, configPath, permissionMode, gitlabProject, gitlabMR, gitlabBaseURL string
	var createMR bool
	var createMRSourceBranch, createMRTargetBranch, createMRTitle, createMRDescription, createMRDescriptionFile string
	var qualityCommands stringSliceFlag

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&runnerRoot, "runner-root", "", "working directory for agent tools; defaults to --root")
	fs.StringVar(&qualityRoot, "quality-root", "", "working directory for quality commands; defaults to --root")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&stage, "stage", "", "demand stage")
	fs.StringVar(&configPath, "config", "", "devflow config path")
	fs.StringVar(&permissionMode, "permission-mode", "", "permission mode for implementation (acceptEdits or bypassPermissions)")
	fs.StringVar(&gitlabProject, "gitlab-project", "", "gitlab project path for mr-review or create-mr")
	fs.StringVar(&gitlabMR, "gitlab-mr", "", "gitlab merge request iid for mr-review")
	fs.StringVar(&gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")
	fs.BoolVar(&createMR, "create-mr", false, "create or reuse a GitLab merge request after implementation")
	fs.StringVar(&createMRSourceBranch, "create-mr-source-branch", "", "source branch for create-mr")
	fs.StringVar(&createMRTargetBranch, "create-mr-target-branch", "", "target branch for create-mr")
	fs.StringVar(&createMRTitle, "create-mr-title", "", "title for create-mr")
	fs.StringVar(&createMRDescription, "create-mr-description", "", "description for create-mr")
	fs.StringVar(&createMRDescriptionFile, "create-mr-description-file", "", "description file for create-mr")
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

	defaults, err := resolveDemandDefaults(configPath)
	if err != nil {
		return err
	}
	runnerRoot = firstNonEmpty(strings.TrimSpace(runnerRoot), defaults.RunnerRoot)
	qualityRoot = firstNonEmpty(strings.TrimSpace(qualityRoot), defaults.QualityRoot)
	permissionMode = firstNonEmpty(strings.TrimSpace(permissionMode), defaults.PermissionMode)
	gitlabProject = firstNonEmpty(strings.TrimSpace(gitlabProject), defaults.GitLabProject)
	gitlabBaseURL = firstNonEmpty(strings.TrimSpace(gitlabBaseURL), defaults.GitLabBaseURL)
	createMRTargetBranch = firstNonEmpty(strings.TrimSpace(createMRTargetBranch), defaults.CreateMRTargetBranch)
	if len(qualityCommands) == 0 {
		for _, command := range defaults.QualityCommands {
			qualityCommands = append(qualityCommands, command)
		}
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
		RunnerRoot:      strings.TrimSpace(runnerRoot),
		QualityRoot:     strings.TrimSpace(qualityRoot),
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

	if err := configureMergeRequest(parsedStage, createMR, gitlabProject, createMRSourceBranch, createMRTargetBranch, createMRTitle, createMRDescription, createMRDescriptionFile, gitlabBaseURL, &opts); err != nil {
		return err
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

func configureMergeRequest(stage demandflow.Stage, enabled bool, project, sourceBranch, targetBranch, title, description, descriptionFile, baseURL string, opts *demandflow.Options) error {
	if stage != demandflow.StageImplementation {
		return nil
	}
	requested := enabled ||
		strings.TrimSpace(sourceBranch) != "" ||
		strings.TrimSpace(targetBranch) != "" ||
		strings.TrimSpace(title) != "" ||
		strings.TrimSpace(description) != "" ||
		strings.TrimSpace(descriptionFile) != ""
	if !requested {
		return nil
	}

	var missing []string
	if strings.TrimSpace(project) == "" {
		missing = append(missing, "--gitlab-project")
	}
	if strings.TrimSpace(sourceBranch) == "" {
		missing = append(missing, "--create-mr-source-branch")
	}
	if strings.TrimSpace(targetBranch) == "" {
		missing = append(missing, "--create-mr-target-branch")
	}
	if strings.TrimSpace(title) == "" {
		missing = append(missing, "--create-mr-title")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s required for --create-mr", strings.Join(missing, ", "))
	}

	desc, err := mergeRequestDescription(description, descriptionFile)
	if err != nil {
		return err
	}

	opts.MergeRequest = demandflow.MergeRequestOptions{
		Adapter: newMergeRequestAdapter(),
		Spec: adapters.MergeRequestSpec{
			Project:      strings.TrimSpace(project),
			SourceBranch: strings.TrimSpace(sourceBranch),
			TargetBranch: strings.TrimSpace(targetBranch),
			Title:        strings.TrimSpace(title),
			Description:  desc,
			BaseURL:      strings.TrimSpace(baseURL),
		},
	}
	return nil
}
