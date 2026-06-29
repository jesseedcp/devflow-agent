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
	fs.StringVar(&project, "gitlab-project", "", "GitLab project path or id")
	fs.StringVar(&mr, "gitlab-mr", "", "GitLab merge request iid")
	fs.StringVar(&baseURL, "gitlab-base-url", "", "GitLab base url override")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project = strings.TrimSpace(project)
	mr = strings.TrimSpace(mr)
	if project == "" || mr == "" {
		return fmt.Errorf("--gitlab-project and --gitlab-mr are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	comments, err := newReviewAdapter().ListUnresolved(ctx, adapters.ReviewRef{
		Project:      project,
		MergeRequest: mr,
		BaseURL:      baseURL,
	})
	if err != nil {
		return err
	}
	if len(comments) == 0 {
		_, err := fmt.Fprintf(stdout, "review gate passed for %s!%s: no unresolved blocking comments\n", project, mr)
		return err
	}

	fmt.Fprintf(stdout, "review gate blocked for %s!%s: %d unresolved blocking comment(s)\n", project, mr, len(comments))
	for _, comment := range comments {
		location := comment.FilePath
		if comment.Line > 0 {
			location = fmt.Sprintf("%s:%d", comment.FilePath, comment.Line)
		}
		if strings.TrimSpace(location) == "" {
			location = "(no file location)"
		}
		fmt.Fprintf(stdout, "- %s by %s: %s\n", location, comment.Author, strings.TrimSpace(comment.Body))
	}
	return fmt.Errorf("review gate blocked by unresolved GitLab comments")
}
