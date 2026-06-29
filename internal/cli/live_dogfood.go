package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/dogfood"
)

var runLiveDogfoodFunc = dogfood.RunLiveSandbox

func runLiveDogfood(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("live-dogfood", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var root, configPath, gitlabProject, gitlabMR, gitlabBaseURL string
	var withGitLab bool
	var timeoutSeconds int
	var maxIterations int
	fs.StringVar(&root, "root", "", "live dogfood root; defaults to a new temp directory")
	fs.StringVar(&configPath, "config", "", "devflow config path")
	fs.BoolVar(&withGitLab, "with-gitlab", false, "use real GitLab review gate instead of offline adapter")
	fs.StringVar(&gitlabProject, "gitlab-project", "", "GitLab project path or id")
	fs.StringVar(&gitlabMR, "gitlab-mr", "", "GitLab merge request iid")
	fs.StringVar(&gitlabBaseURL, "gitlab-base-url", "", "GitLab base url override")
	fs.IntVar(&timeoutSeconds, "timeout-seconds", 600, "live dogfood timeout in seconds")
	fs.IntVar(&maxIterations, "max-iterations", 30, "maximum RuntimeRunner iterations per stage")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if withGitLab && (gitlabProject == "" || gitlabMR == "") {
		return fmt.Errorf("--gitlab-project and --gitlab-mr are required with --with-gitlab")
	}
	if timeoutSeconds <= 0 {
		return fmt.Errorf("--timeout-seconds must be positive")
	}
	if maxIterations <= 0 {
		return fmt.Errorf("--max-iterations must be positive")
	}

	result, err := runLiveDogfoodFunc(context.Background(), dogfood.LiveOptions{
		Root:          root,
		ConfigPath:    configPath,
		UseGitLab:     withGitLab,
		Review:        adapters.ReviewRef{Project: gitlabProject, MergeRequest: gitlabMR, BaseURL: gitlabBaseURL},
		Timeout:       time.Duration(timeoutSeconds) * time.Second,
		MaxIterations: maxIterations,
		Now:           time.Now,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "live dogfood completed for %s\n", result.DemandID)
	fmt.Fprintf(stdout, "state: %s\n", result.FinalState)
	fmt.Fprintf(stdout, "root: %s\n", result.Root)
	fmt.Fprintf(stdout, "repo-root: %s\n", result.RepoRoot)
	fmt.Fprintf(stdout, "demand-root: %s\n", result.DemandRoot)
	fmt.Fprintf(stdout, "report: %s\n", result.ReportPath)
	fmt.Fprintf(stdout, "steps: %s\n", strconv.Itoa(len(result.Steps)))
	return nil
}
