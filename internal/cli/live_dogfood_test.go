package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/dogfood"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestLiveDogfoodCommandPrintsResult(t *testing.T) {
	original := runLiveDogfoodFunc
	defer func() { runLiveDogfoodFunc = original }()
	runLiveDogfoodFunc = func(_ context.Context, opts dogfood.LiveOptions) (dogfood.LiveResult, error) {
		if opts.Timeout <= 0 || opts.MaxIterations != 12 {
			t.Fatalf("opts = %#v", opts)
		}
		return dogfood.LiveResult{
			Root:       "root",
			RepoRoot:   "root/repo",
			DemandRoot: "root/artifacts",
			DemandID:   "live-dogfood-coupon-eligibility",
			FinalState: workflow.Completed,
			ReportPath: "root/artifacts/report.md",
			Steps:      []dogfood.Step{{Name: "requirements", State: workflow.RequirementsReview, Output: "ok"}},
		}, nil
	}

	var stdout bytes.Buffer
	err := Run([]string{"live-dogfood", "--timeout-seconds", "30", "--max-iterations", "12"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("live-dogfood: %v", err)
	}
	for _, want := range []string{"live dogfood completed", "state: completed", "repo-root:", "steps: 1"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestLiveDogfoodRequiresGitLabRefWhenEnabled(t *testing.T) {
	err := Run([]string{"live-dogfood", "--with-gitlab", "--gitlab-project", "group/project"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--gitlab-project and --gitlab-mr are required") {
		t.Fatalf("err = %v, want gitlab ref error", err)
	}
}

func TestLiveDogfoodHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"devflow live-dogfood", "live-dogfood Run opt-in live provider sandbox dogfood"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
