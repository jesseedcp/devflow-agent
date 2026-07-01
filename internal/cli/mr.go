package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

// runChangeRequest is the provider-neutral entry point for change requests.
// In Wave 27 it wraps the existing GitLab merge-request implementation while
// the mr command remains supported for backward compatibility.
func runChangeRequest(args []string, stdout io.Writer, stderr io.Writer) error {
	return runMR(args, stdout, stderr)
}

func runMR(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("mr requires a subcommand: ensure")
	}
	switch args[0] {
	case "ensure":
		return runMREnsure(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown mr subcommand %q", args[0])
	}
}

func runMREnsure(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("mr ensure", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var provider, gitlabProject, githubRepo, sourceBranch, targetBranch, title, description, descriptionFile, gitlabBaseURL, githubBaseURL string
	fs.StringVar(&provider, "provider", "", "change request provider (gitlab or github)")
	fs.StringVar(&gitlabProject, "gitlab-project", "", "gitlab project path")
	fs.StringVar(&githubRepo, "github-repo", "", "github repository in owner/repo form")
	fs.StringVar(&sourceBranch, "source-branch", "", "source branch")
	fs.StringVar(&targetBranch, "target-branch", "", "target branch")
	fs.StringVar(&title, "title", "", "change request title")
	fs.StringVar(&description, "description", "", "change request description (alternative to --description-file)")
	fs.StringVar(&descriptionFile, "description-file", "", "path to description file")
	fs.StringVar(&gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")
	fs.StringVar(&githubBaseURL, "github-base-url", "", "github api base url override")

	if err := fs.Parse(args); err != nil {
		return err
	}

	desc, err := mergeRequestDescription(description, descriptionFile)
	if err != nil {
		return err
	}

	provider = strings.ToLower(strings.TrimSpace(provider))
	gitlabProject = strings.TrimSpace(gitlabProject)
	githubRepo = strings.TrimSpace(githubRepo)
	if provider == "" {
		switch {
		case githubRepo != "" && gitlabProject == "":
			provider = "github"
		default:
			provider = "gitlab"
		}
	}

	var adapter adapters.MergeRequestAdapter
	var spec adapters.MergeRequestSpec
	switch provider {
	case "github":
		if githubRepo == "" {
			return fmt.Errorf("--github-repo is required for provider github")
		}
		adapter = newGitHubMergeRequestAdapter()
		spec = adapters.MergeRequestSpec{
			Provider:     "github",
			Repo:         githubRepo,
			SourceBranch: strings.TrimSpace(sourceBranch),
			TargetBranch: strings.TrimSpace(targetBranch),
			Title:        strings.TrimSpace(title),
			Description:  desc,
			BaseURL:      strings.TrimSpace(githubBaseURL),
		}
	case "gitlab":
		if gitlabProject == "" {
			return fmt.Errorf("--gitlab-project is required for provider gitlab")
		}
		adapter = newMergeRequestAdapter()
		spec = adapters.MergeRequestSpec{
			Provider:     "gitlab",
			Project:      gitlabProject,
			SourceBranch: strings.TrimSpace(sourceBranch),
			TargetBranch: strings.TrimSpace(targetBranch),
			Title:        strings.TrimSpace(title),
			Description:  desc,
			BaseURL:      strings.TrimSpace(gitlabBaseURL),
		}
	default:
		return fmt.Errorf("unsupported --provider %q (want gitlab or github)", provider)
	}

	result, err := adapter.EnsureMergeRequest(context.Background(), spec)
	if err != nil {
		return fmt.Errorf("ensure change request: %w", err)
	}

	verb := "Created"
	if !result.WasCreated {
		verb = "Reused"
	}
	noun := "merge request"
	if provider == "github" {
		noun = "pull request"
	}
	fmt.Fprintf(stdout, "%s %s !%d\n", verb, noun, result.IID)
	fmt.Fprintf(stdout, "Title: %s\n", result.Title)
	fmt.Fprintf(stdout, "State: %s\n", result.State)
	fmt.Fprintf(stdout, "URL: %s\n", result.WebURL)
	return nil
}

// mergeRequestDescription returns the description from --description or
// --description-file. It rejects both being set at the same time.
func mergeRequestDescription(description, descriptionFile string) (string, error) {
	if description != "" && descriptionFile != "" {
		return "", fmt.Errorf("--description and --description-file cannot both be set")
	}
	if descriptionFile != "" {
		data, err := os.ReadFile(descriptionFile)
		if err != nil {
			return "", fmt.Errorf("read description file: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return description, nil
}
