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

func runCIGate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("ci-gate", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var repo, pr, baseURL string
	fs.StringVar(&repo, "github-repo", "", "GitHub repository in owner/repo form")
	fs.StringVar(&pr, "github-pr", "", "GitHub pull request number")
	fs.StringVar(&baseURL, "github-base-url", "", "GitHub API base url override")
	if err := fs.Parse(args); err != nil {
		return err
	}

	repo = strings.TrimSpace(repo)
	pr = strings.TrimSpace(pr)
	if repo == "" || pr == "" {
		return fmt.Errorf("--github-repo and --github-pr are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := newCIGateAdapter().Check(ctx, adapters.CIRef{
		Provider: "github",
		Repo:     repo,
		PR:       pr,
		BaseURL:  strings.TrimSpace(baseURL),
	})
	if err != nil {
		return err
	}
	for _, check := range result.Checks {
		fmt.Fprintf(stdout, "- %s: status=%s conclusion=%s\n", check.Name, check.Status, check.Conclusion)
	}
	switch result.Status {
	case adapters.CIStatusPassed:
		_, err := fmt.Fprintf(stdout, "ci gate passed for %s#%s\n", repo, pr)
		return err
	case adapters.CIStatusFailed, adapters.CIStatusPending, adapters.CIStatusUnknown:
		fmt.Fprintf(stdout, "ci gate blocked for %s#%s: %s\n", repo, pr, result.Status)
		return fmt.Errorf("ci gate blocked: %s", result.Status)
	default:
		return fmt.Errorf("ci gate blocked: unexpected status %s", result.Status)
	}
}
