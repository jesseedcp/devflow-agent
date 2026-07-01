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

var newGitHubMergeRequestAdapter = func() adapters.MergeRequestAdapter {
	return adapters.GitHubPullRequestAdapter{}
}

var newReviewAdapter = func() adapters.ReviewAdapter {
	return adapters.GitLabReviewAdapter{}
}

var newGitHubReviewAdapter = func() adapters.ReviewAdapter {
	return adapters.GitHubReviewAdapter{}
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
	var createMR, createChangeRequest bool
	var changeRequestProvider string
	var createMRSourceBranch, createMRTargetBranch, createMRTitle, createMRDescription, createMRDescriptionFile string
	var githubRepo, githubPR, githubBaseURL string
	var reviewProvider string
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
	fs.StringVar(&githubRepo, "github-repo", "", "GitHub repository in owner/repo form for mr-review CI gate")
	fs.StringVar(&githubPR, "github-pr", "", "GitHub pull request number for mr-review CI gate")
	fs.StringVar(&githubBaseURL, "github-base-url", "", "GitHub API base url override")
	var githubCI bool
	fs.StringVar(&reviewProvider, "review-provider", "", "review provider for mr-review (gitlab or github)")
	fs.BoolVar(&githubCI, "github-ci", false, "enable GitHub CI gate during mr-review (implied for GitLab review when GitHub flags are set)")
	fs.BoolVar(&createMR, "create-mr", false, "create or reuse a GitLab merge request after implementation")
	fs.BoolVar(&createChangeRequest, "create-change-request", false, "create or reuse a change request after implementation (provider-neutral)")
	fs.StringVar(&changeRequestProvider, "change-request-provider", "", "change request provider for create-change-request (gitlab or github)")
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
		if err := configureReview(reviewProvider, gitlabProject, gitlabMR, gitlabBaseURL, githubRepo, githubPR, githubBaseURL, githubCI, &opts); err != nil {
			return err
		}
	}

	if err := configureChangeRequest(parsedStage, createMR, createChangeRequest, changeRequestProvider, gitlabProject, githubRepo, createMRSourceBranch, createMRTargetBranch, createMRTitle, createMRDescription, createMRDescriptionFile, gitlabBaseURL, githubBaseURL, &opts); err != nil {
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

func configureReview(provider, gitlabProject, gitlabMR, gitlabBaseURL, githubRepo, githubPR, githubBaseURL string, githubCI bool, opts *demandflow.Options) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	gitlabProject = strings.TrimSpace(gitlabProject)
	gitlabMR = strings.TrimSpace(gitlabMR)
	githubRepo = strings.TrimSpace(githubRepo)
	githubPR = strings.TrimSpace(githubPR)

	gitlabPresent := gitlabProject != "" || gitlabMR != ""
	githubPresent := githubRepo != "" || githubPR != ""

	if provider == "" {
		switch {
		case gitlabPresent:
			provider = "gitlab"
		case githubPresent:
			provider = "github"
		default:
			return fmt.Errorf("--gitlab-project and --gitlab-mr are required for mr-review")
		}
	}

	switch provider {
	case "gitlab":
		if gitlabProject == "" || gitlabMR == "" {
			return fmt.Errorf("--gitlab-project and --gitlab-mr are required for mr-review")
		}
		opts.Review = demandflow.ReviewOptions{
			Adapter: newReviewAdapter(),
			Ref: adapters.ReviewRef{
				Provider:     "gitlab",
				Project:      gitlabProject,
				MergeRequest: gitlabMR,
				BaseURL:      strings.TrimSpace(gitlabBaseURL),
			},
		}
		if githubPresent {
			if githubRepo == "" || githubPR == "" {
				return fmt.Errorf("--github-repo and --github-pr must be provided together for mr-review CI gate")
			}
			opts.CIGate = demandflow.CIGateOptions{
				Adapter: newCIGateAdapter(),
				Ref: adapters.CIRef{
					Provider: "github",
					Repo:     githubRepo,
					PR:       githubPR,
					BaseURL:  strings.TrimSpace(githubBaseURL),
				},
			}
		}
	case "github":
		if githubRepo == "" || githubPR == "" {
			return fmt.Errorf("--github-repo and --github-pr are required for mr-review with --review-provider github")
		}
		opts.Review = demandflow.ReviewOptions{
			Adapter: newGitHubReviewAdapter(),
			Ref: adapters.ReviewRef{
				Provider:    "github",
				Repo:        githubRepo,
				PullRequest: githubPR,
				BaseURL:     strings.TrimSpace(githubBaseURL),
			},
		}
		if githubCI {
			opts.CIGate = demandflow.CIGateOptions{
				Adapter: newCIGateAdapter(),
				Ref: adapters.CIRef{
					Provider: "github",
					Repo:     githubRepo,
					PR:       githubPR,
					BaseURL:  strings.TrimSpace(githubBaseURL),
				},
			}
		}
	default:
		return fmt.Errorf("unsupported --review-provider %q (want gitlab or github)", provider)
	}
	return nil
}
func configureChangeRequest(stage demandflow.Stage, createMR, createChangeRequest bool, provider, project, githubRepo, sourceBranch, targetBranch, title, description, descriptionFile, gitlabBaseURL, githubBaseURL string, opts *demandflow.Options) error {
	if stage != demandflow.StageImplementation {
		return nil
	}
	project = strings.TrimSpace(project)
	githubRepo = strings.TrimSpace(githubRepo)
	provider = strings.ToLower(strings.TrimSpace(provider))

	requested := createMR ||
		createChangeRequest ||
		provider != "" ||
		strings.TrimSpace(sourceBranch) != "" ||
		strings.TrimSpace(targetBranch) != "" ||
		strings.TrimSpace(title) != "" ||
		strings.TrimSpace(description) != "" ||
		strings.TrimSpace(descriptionFile) != ""
	if !requested {
		return nil
	}

	if provider == "" {
		switch {
		case githubRepo != "" && project == "":
			provider = "github"
		default:
			provider = "gitlab"
		}
	}

	desc, err := mergeRequestDescription(description, descriptionFile)
	if err != nil {
		return err
	}

	var missing []string
	if strings.TrimSpace(sourceBranch) == "" {
		missing = append(missing, "--create-mr-source-branch")
	}
	if strings.TrimSpace(targetBranch) == "" {
		missing = append(missing, "--create-mr-target-branch")
	}
	if strings.TrimSpace(title) == "" {
		missing = append(missing, "--create-mr-title")
	}

	switch provider {
	case "github":
		if githubRepo == "" {
			missing = append([]string{"--github-repo"}, missing...)
		}
		if len(missing) > 0 {
			return fmt.Errorf("%s required for create-change-request (github)", strings.Join(missing, ", "))
		}
		opts.ChangeRequest = demandflow.ChangeRequestOptions{
			Adapter: newGitHubMergeRequestAdapter(),
			Spec: adapters.MergeRequestSpec{
				Provider:     "github",
				Repo:         githubRepo,
				SourceBranch: strings.TrimSpace(sourceBranch),
				TargetBranch: strings.TrimSpace(targetBranch),
				Title:        strings.TrimSpace(title),
				Description:  desc,
				BaseURL:      strings.TrimSpace(githubBaseURL),
			},
		}
	case "gitlab":
		if project == "" {
			missing = append([]string{"--gitlab-project"}, missing...)
		}
		if len(missing) > 0 {
			return fmt.Errorf("%s required for --create-mr", strings.Join(missing, ", "))
		}
		opts.ChangeRequest = demandflow.ChangeRequestOptions{
			Adapter: newMergeRequestAdapter(),
			Spec: adapters.MergeRequestSpec{
				Provider:     "gitlab",
				Project:      project,
				SourceBranch: strings.TrimSpace(sourceBranch),
				TargetBranch: strings.TrimSpace(targetBranch),
				Title:        strings.TrimSpace(title),
				Description:  desc,
				BaseURL:      strings.TrimSpace(gitlabBaseURL),
			},
		}
	default:
		return fmt.Errorf("unsupported --change-request-provider %q (want gitlab or github)", provider)
	}
	return nil
}
