package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestSmokeRequiresTitleAndDescription(t *testing.T) {
	err := Run([]string{"smoke", "--description", "desc"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--title is required") {
		t.Fatalf("err = %v want title required", err)
	}
	err = Run([]string{"smoke", "--title", "title"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--description is required") {
		t.Fatalf("err = %v want description required", err)
	}
}

func TestSmokeCreatesDemandAndRunsRequirements(t *testing.T) {
	root := t.TempDir()
	original := newSmokeRunner
	defer func() { newSmokeRunner = original }()
	newSmokeRunner = func(string) demandflow.Runner {
		return &demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageRequirements: {Text: "# Requirements: Smoke coupon check\n\nsmoke requirements body\n"},
		}}
	}

	var stdout bytes.Buffer
	if err := Run([]string{"smoke", "--root", root, "--title", "Smoke coupon check", "--description", "Only active members can claim coupons"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("smoke: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"smoke-coupon-check", "requirements.md"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
	path := filepath.Join(root, ".devflow", "demands", "smoke-coupon-check", artifacts.RequirementsFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	if !strings.Contains(string(body), "smoke requirements body") {
		t.Fatalf("requirements.md = %q", string(body))
	}
	demand, err := artifacts.NewStore(root).LoadDemand("smoke-coupon-check")
	if err != nil {
		t.Fatalf("load demand: %v", err)
	}
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("state = %s want %s", demand.State, workflow.RequirementsReview)
	}
}

func TestSmokeHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"devflow smoke --title <title> --description <text>", "smoke    Run an explicit local requirements-stage smoke test"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}
