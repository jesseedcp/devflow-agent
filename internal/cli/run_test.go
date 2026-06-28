package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func createDemandAtState(t *testing.T, root string, state workflow.State) artifacts.Store {
	t.Helper()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "add-coupon-check",
		Title:       "coupon flow",
		Description: "coupon flow",
		Source:      "manual",
		State:       string(state),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	return store
}

func TestRunRequiresDemand(t *testing.T) {
	var stdout bytes.Buffer
	err := Run([]string{"run", "--stage", "requirements"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v want --demand is required", err)
	}
}

func TestRunRequiresStage(t *testing.T) {
	var stdout bytes.Buffer
	err := Run([]string{"run", "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--stage is required") {
		t.Fatalf("err = %v want --stage is required", err)
	}
}

func TestRunRejectsUnsupportedStage(t *testing.T) {
	var stdout bytes.Buffer
	err := Run([]string{"run", "--demand", "add-coupon-check", "--stage", "bogus"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported stage") {
		t.Fatalf("err = %v want unsupported stage", err)
	}
}

func TestRunMRReviewRequiresGitLabRef(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.MRReview)

	var stdout bytes.Buffer
	err := Run([]string{"run", "--root", root, "--demand", "add-coupon-check", "--stage", "mr-review"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--gitlab-project and --gitlab-mr are required") {
		t.Fatalf("err = %v want gitlab ref required", err)
	}
}

func TestRunHelpIncludesRun(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(stdout.String(), "devflow run --demand") {
		t.Fatalf("help missing run usage: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Run one backend-demand agent stage") {
		t.Fatalf("help missing run command description: %q", stdout.String())
	}
}

func TestRunRequirementsStageWritesArtifact(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Created)

	original := newDemandRunner
	defer func() { newDemandRunner = original }()
	newDemandRunner = func(string, permissions.PermissionMode) demandflow.Runner {
		return &demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageRequirements: {Text: "# Requirements: coupon flow\n\n## 目标行为\n\nstubbed requirements body\n"},
		}}
	}

	var stdout bytes.Buffer
	err := Run([]string{"run", "--root", root, "--demand", "add-coupon-check", "--stage", "requirements"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "stage requirements completed for add-coupon-check") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.RequirementsFile))
	if err != nil {
		t.Fatalf("read requirements: %v", err)
	}
	if !strings.Contains(string(body), "stubbed requirements body") {
		t.Fatalf("requirements.md = %q", string(body))
	}
	demand, _ := artifacts.NewStore(root).LoadDemand("add-coupon-check")
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("state = %q want requirements_review", demand.State)
	}
}

func TestRunQualityCommandParsesQuotedArguments(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Implementation)

	t.Setenv("DEVFLOW_CLI_HELPER", "args")
	executable := filepath.ToSlash(testCLIExecutable(t))
	commandText := fmt.Sprintf(`"%s" -test.run=^TestCLICommandHelper$ -- --quality "hello world"`, executable)

	original := newDemandRunner
	defer func() { newDemandRunner = original }()
	newDemandRunner = func(string, permissions.PermissionMode) demandflow.Runner {
		return &demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageImplementation: {Text: "## 实现摘要\n\nstubbed implementation body\n"},
		}}
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"run", "--root", root, "--demand", "add-coupon-check",
		"--stage", "implementation",
		"--quality-command", commandText,
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "stage implementation completed for add-coupon-check") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	progress, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.ProgressFile))
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if !strings.Contains(string(progress), "stubbed implementation body") {
		t.Fatalf("progress.md missing runner output: %q", string(progress))
	}
	if !strings.Contains(string(progress), "hello world") {
		t.Fatalf("progress.md missing parsed quality command output: %q", string(progress))
	}
	demand, _ := artifacts.NewStore(root).LoadDemand("add-coupon-check")
	if demand.State != string(workflow.MRReview) {
		t.Fatalf("state = %q want mr_review", demand.State)
	}
}
