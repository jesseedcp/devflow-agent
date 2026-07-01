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

	var gitlabProject, sourceBranch, targetBranch, title, description, descriptionFile, gitlabBaseURL string
	fs.StringVar(&gitlabProject, "gitlab-project", "", "gitlab project path")
	fs.StringVar(&sourceBranch, "source-branch", "", "source branch")
	fs.StringVar(&targetBranch, "target-branch", "", "target branch")
	fs.StringVar(&title, "title", "", "merge request title")
	fs.StringVar(&description, "description", "", "merge request description (alternative to --description-file)")
	fs.StringVar(&descriptionFile, "description-file", "", "path to description file")
	fs.StringVar(&gitlabBaseURL, "gitlab-base-url", "", "gitlab base url override")

	if err := fs.Parse(args); err != nil {
		return err
	}

	desc, err := mergeRequestDescription(description, descriptionFile)
	if err != nil {
		return err
	}

	spec := adapters.MergeRequestSpec{
		Project:      gitlabProject,
		SourceBranch: sourceBranch,
		TargetBranch: targetBranch,
		Title:        title,
		Description:  desc,
		BaseURL:      gitlabBaseURL,
	}
	adapter := newMergeRequestAdapter()
	result, err := adapter.EnsureMergeRequest(context.Background(), spec)
	if err != nil {
		return fmt.Errorf("mr ensure: %w", err)
	}

	verb := "Created"
	if !result.WasCreated {
		verb = "Reused"
	}
	fmt.Fprintf(stdout, "%s merge request !%d\n", verb, result.IID)
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
