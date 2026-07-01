package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func runReviewGate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("review-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var project, mr, baseURL string
	var githubRepo, githubPR, githubBaseURL string
	fs.StringVar(&project, "gitlab-project", "", "GitLab project path or id")
	fs.StringVar(&mr, "gitlab-mr", "", "GitLab merge request iid")
	fs.StringVar(&baseURL, "gitlab-base-url", "", "GitLab base url override")
	fs.StringVar(&githubRepo, "github-repo", "", "GitHub repository in owner/repo form")
	fs.StringVar(&githubPR, "github-pr", "", "GitHub pull request number")
	fs.StringVar(&githubBaseURL, "github-base-url", "", "GitHub GraphQL base url override")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project = strings.TrimSpace(project)
	mr = strings.TrimSpace(mr)
	githubRepo = strings.TrimSpace(githubRepo)
	githubPR = strings.TrimSpace(githubPR)

	gitlabRequested := project != "" || mr != ""
	githubRequested := githubRepo != "" || githubPR != ""
	if gitlabRequested && githubRequested {
		return fmt.Errorf("provide either GitLab (--gitlab-project/--gitlab-mr) or GitHub (--github-repo/--github-pr) review targets, not both")
	}

	var adapter adapters.ReviewAdapter
	var ref adapters.ReviewRef
	var label string
	switch {
	case githubRequested:
		if githubRepo == "" || githubPR == "" {
			return fmt.Errorf("--github-repo and --github-pr are required")
		}
		adapter = newGitHubReviewAdapter()
		ref = adapters.ReviewRef{
			Provider:    "github",
			Repo:        githubRepo,
			PullRequest: githubPR,
			BaseURL:     strings.TrimSpace(githubBaseURL),
		}
		label = fmt.Sprintf("%s#%s", githubRepo, githubPR)
	default:
		if project == "" || mr == "" {
			return fmt.Errorf("--gitlab-project and --gitlab-mr are required")
		}
		adapter = newReviewAdapter()
		ref = adapters.ReviewRef{
			Provider:     "gitlab",
			Project:      project,
			MergeRequest: mr,
			BaseURL:      strings.TrimSpace(baseURL),
		}
		label = fmt.Sprintf("%s!%s", project, mr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	comments, err := adapter.ListUnresolved(ctx, ref)
	if err != nil {
		return err
	}
	if len(comments) == 0 {
		_, err := fmt.Fprintf(stdout, "review gate passed for %s: no unresolved blocking comments\n", label)
		return err
	}

	fmt.Fprintf(stdout, "review gate blocked for %s: %d unresolved blocking comment(s)\n", label, len(comments))
	for _, comment := range comments {
		location := comment.FilePath
		if comment.Line > 0 {
			location = fmt.Sprintf("%s:%d", comment.FilePath, comment.Line)
		}
		if strings.TrimSpace(location) == "" {
			location = "(no file location)"
		}
		category := comment.Category
		if category == "" {
			category = adapters.ClassifyReviewComment(comment.Body, comment.FilePath)
		}
		fmt.Fprintf(stdout, "- [%s] %s by %s: %s\n", category, location, comment.Author, strings.TrimSpace(comment.Body))
	}
	if githubRequested {
		return fmt.Errorf("review gate blocked by unresolved GitHub comments")
	}
	return fmt.Errorf("review gate blocked by unresolved GitLab comments")
}
