package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
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

func TestRunQualityCommandUsesQualityRoot(t *testing.T) {
	artifactRoot := t.TempDir()
	repoRoot := t.TempDir()
	createDemandAtState(t, artifactRoot, workflow.Implementation)

	t.Setenv("DEVFLOW_CLI_HELPER", "pwd")
	executable := filepath.ToSlash(testCLIExecutable(t))
	commandText := fmt.Sprintf(`"%s" -test.run=^TestCLICommandHelper$`, executable)

	original := newDemandRunner
	defer func() { newDemandRunner = original }()
	newDemandRunner = func(string, permissions.PermissionMode) demandflow.Runner {
		return &demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageImplementation: {Text: "## 实现摘要\n\nstubbed implementation body\n"},
		}}
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"run",
		"--root", artifactRoot,
		"--quality-root", repoRoot,
		"--demand", "add-coupon-check",
		"--stage", "implementation",
		"--quality-command", commandText,
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run implementation: %v", err)
	}
	if !strings.Contains(stdout.String(), "quality gate passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	progress, err := os.ReadFile(filepath.Join(artifactRoot, ".devflow", "demands", "add-coupon-check", artifacts.ProgressFile))
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if !strings.Contains(string(progress), filepath.Clean(repoRoot)) {
		t.Fatalf("progress.md missing quality root %q: %q", repoRoot, string(progress))
	}
}

type cliRecordingRunner struct {
	root string
}

func (r *cliRecordingRunner) Run(_ context.Context, req demandflow.RunnerRequest) (demandflow.RunnerResponse, error) {
	r.root = req.Root
	return demandflow.RunnerResponse{Text: "# Requirements\n\ncli runner root recorded\n"}, nil
}

func TestRunUsesRunnerRootForDemandRunner(t *testing.T) {
	artifactRoot := t.TempDir()
	codeRoot := t.TempDir()
	createDemandAtState(t, artifactRoot, workflow.Created)

	recorder := &cliRecordingRunner{}
	original := newDemandRunner
	defer func() { newDemandRunner = original }()
	newDemandRunner = func(string, permissions.PermissionMode) demandflow.Runner {
		return recorder
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"run",
		"--root", artifactRoot,
		"--runner-root", codeRoot,
		"--demand", "add-coupon-check",
		"--stage", "requirements",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if recorder.root != codeRoot {
		t.Fatalf("runner root = %q, want %q", recorder.root, codeRoot)
	}
}

func TestConfigureMergeRequestSetsFlags(t *testing.T) {
	var opts demandflow.Options
	err := configureMergeRequest(demandflow.StageImplementation, true, "group/project", "feature/x", "main", "My MR", "desc", "", "", &opts)
	if err != nil {
		t.Fatalf("configureMergeRequest: %v", err)
	}
	if opts.MergeRequest.Adapter == nil {
		t.Fatal("MergeRequest adapter not set")
	}
	if opts.MergeRequest.Spec.Project != "group/project" {
		t.Fatalf("project = %q, want group/project", opts.MergeRequest.Spec.Project)
	}
	if opts.MergeRequest.Spec.SourceBranch != "feature/x" {
		t.Fatalf("source = %q, want feature/x", opts.MergeRequest.Spec.SourceBranch)
	}
	if opts.MergeRequest.Spec.TargetBranch != "main" {
		t.Fatalf("target = %q, want main", opts.MergeRequest.Spec.TargetBranch)
	}
	if opts.MergeRequest.Spec.Title != "My MR" {
		t.Fatalf("title = %q, want My MR", opts.MergeRequest.Spec.Title)
	}
}

func TestConfigureMergeRequestSkipsNonImplementation(t *testing.T) {
	var opts demandflow.Options
	err := configureMergeRequest(demandflow.StageRequirements, true, "group/project", "feature/x", "main", "title", "", "", "", &opts)
	if err != nil {
		t.Fatalf("configureMergeRequest: %v", err)
	}
	if opts.MergeRequest.Adapter != nil {
		t.Fatal("expected nil adapter for non-implementation stage")
	}
}

func TestConfigureMergeRequestSkipsMissingFlags(t *testing.T) {
	var opts demandflow.Options
	err := configureMergeRequest(demandflow.StageImplementation, false, "", "", "", "", "", "", "", &opts)
	if err != nil {
		t.Fatalf("configureMergeRequest: %v", err)
	}
	if opts.MergeRequest.Adapter != nil {
		t.Fatal("expected nil adapter when MR sync is not requested")
	}
}

func TestConfigureMergeRequestRejectsMissingRequiredFlagsWhenEnabled(t *testing.T) {
	tests := []struct {
		name                           string
		project, source, target, title string
	}{
		{"empty project", "", "feature/x", "main", "title"},
		{"empty source", "group/project", "", "main", "title"},
		{"empty target", "group/project", "feature/x", "", "title"},
		{"empty title", "group/project", "feature/x", "main", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var opts demandflow.Options
			err := configureMergeRequest(demandflow.StageImplementation, true, tc.project, tc.source, tc.target, tc.title, "", "", "", &opts)
			if err == nil {
				t.Fatal("expected missing flag error")
			}
		})
	}
}

func TestRunImplementationCreateMergeRequestFlagsSyncMR(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Implementation)

	originalRunner := newDemandRunner
	defer func() { newDemandRunner = originalRunner }()
	newDemandRunner = func(string, permissions.PermissionMode) demandflow.Runner {
		return &demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageImplementation: {Text: "## 实现摘要\n\nstubbed implementation body\n"},
		}}
	}

	fakeAdapter := &fakeMergeRequestAdapter{result: adapters.MergeRequestResult{
		IID: 77, WebURL: "https://gitlab.example.com/group/project/-/merge_requests/77", Title: "Implement coupon", State: "opened", WasCreated: true,
	}}
	originalMRAdapter := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = originalMRAdapter }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter {
		return fakeAdapter
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"run",
		"--root", root,
		"--demand", "add-coupon-check",
		"--stage", "implementation",
		"--create-mr",
		"--gitlab-project", "group/project",
		"--create-mr-source-branch", "feature/coupon",
		"--create-mr-target-branch", "main",
		"--create-mr-title", "Implement coupon",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run implementation: %v", err)
	}
	if fakeAdapter.spec.Project != "group/project" {
		t.Fatalf("project = %q, want group/project", fakeAdapter.spec.Project)
	}
	if fakeAdapter.spec.SourceBranch != "feature/coupon" {
		t.Fatalf("source = %q, want feature/coupon", fakeAdapter.spec.SourceBranch)
	}
	progress, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.ProgressFile))
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if !strings.Contains(string(progress), "!77") {
		t.Fatalf("progress.md missing MR evidence:\n%s", string(progress))
	}
}

func TestRunImplementationUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.Implementation)
	t.Setenv("DEVFLOW_CLI_HELPER", "pwd")

	executable := filepath.ToSlash(testCLIExecutable(t))
	commandText := fmt.Sprintf(`"%s" -test.run=^TestCLICommandHelper$`, executable)
	configPath := filepath.Join(root, ".devflow", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	configBody := "providers:\n  - name: test\n    protocol: openai-compat\n    base_url: https://example.com/v1\n    model: test-model\nbackend_demand:\n  runner_root: .\n  quality_root: .\n  quality_commands:\n    - '" + commandText + "'\n  permission_mode: acceptEdits\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldRunner := newDemandRunner
	defer func() { newDemandRunner = oldRunner }()
	var gotMode permissions.PermissionMode
	newDemandRunner = func(configPath string, mode permissions.PermissionMode) demandflow.Runner {
		gotMode = mode
		return &demandflow.StaticRunner{Responses: map[demandflow.Stage]demandflow.RunnerResponse{
			demandflow.StageImplementation: {Text: "implemented"},
		}}
	}

	var stdout bytes.Buffer
	if err := Run([]string{"run", "--root", root, "--config", configPath, "--demand", "add-coupon-check", "--stage", "implementation"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("run returned error: %v\n%s", err, stdout.String())
	}
	if gotMode != permissions.ModeAcceptEdits {
		t.Fatalf("permission mode = %s, want acceptEdits", gotMode)
	}
	progress, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "add-coupon-check", artifacts.ProgressFile))
	if err != nil {
		t.Fatalf("read progress: %v", err)
	}
	if !strings.Contains(string(progress), "TestCLICommandHelper") {
		t.Fatalf("progress missing default quality command:\n%s", string(progress))
	}
}

func TestRunMRReviewWithGitHubCIAdvancesToVerification(t *testing.T) {
	root := t.TempDir()
	createDemandAtState(t, root, workflow.MRReview)

	originalReview := newReviewAdapter
	defer func() { newReviewAdapter = originalReview }()
	newReviewAdapter = func() adapters.ReviewAdapter { return &fakeReviewGateAdapter{} }

	originalCI := newCIGateAdapter
	defer func() { newCIGateAdapter = originalCI }()
	newCIGateAdapter = func() adapters.CIGateAdapter {
		return &fakeCIGateAdapter{result: adapters.CIResult{
			Provider: "github",
			Repo:     "owner/repo",
			PR:       "42",
			HeadSHA:  "abc123",
			Status:   adapters.CIStatusPassed,
			Message:  "github ci passed",
			Checks:   []adapters.CICheck{{Name: "Go verification", Status: "completed", Conclusion: "success"}},
		}}
	}

	var stdout bytes.Buffer
	err := Run([]string{"run", "--root", root, "--demand", "add-coupon-check", "--stage", "mr-review", "--gitlab-project", "group/project", "--gitlab-mr", "1", "--github-repo", "owner/repo", "--github-pr", "42"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout.String(), "state: mr_review -> verification") {
		t.Fatalf("stdout = %q want mr_review -> verification", stdout.String())
	}
	store := artifacts.NewStore(root)
	progress, readErr := os.ReadFile(filepath.Join(store.DemandDir("add-coupon-check"), artifacts.ProgressFile))
	if readErr != nil {
		t.Fatalf("read progress: %v", readErr)
	}
	if !strings.Contains(string(progress), "## CI Gate") {
		t.Fatalf("progress.md missing CI gate evidence:\n%s", string(progress))
	}
	events, readErr := os.ReadFile(filepath.Join(store.DemandDir("add-coupon-check"), artifacts.EventsFile))
	if readErr != nil {
		t.Fatalf("read events: %v", readErr)
	}
	if !strings.Contains(string(events), "ci_gate.passed") {
		t.Fatalf("events.jsonl missing ci_gate.passed:\n%s", string(events))
	}
}
