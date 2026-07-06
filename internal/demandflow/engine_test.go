package demandflow

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/quality"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestStaticRunnerCapturesRequests(t *testing.T) {
	t.Parallel()

	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageRequirements: {Text: "requirements body"},
	}}
	resp, err := runner.Run(context.Background(), RunnerRequest{
		Stage:    StageRequirements,
		Root:     t.TempDir(),
		DemandID: "add-coupon-check",
		Prompt:   "prompt",
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if resp.Text != "requirements body" {
		t.Fatalf("response = %q want requirements body", resp.Text)
	}
	if len(runner.Requests) != 1 || runner.Requests[0].DemandID != "add-coupon-check" {
		t.Fatalf("request not captured: %#v", runner.Requests)
	}
}

var fixedNow = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }

type fakeQualityRunner struct {
	exitCode int
	stdout   string
	stderr   string
}

func (f fakeQualityRunner) Run(_ context.Context, _ string, name string, args ...string) quality.Result {
	return quality.Result{
		Command:  name,
		Args:     args,
		ExitCode: f.exitCode,
		Stdout:   f.stdout,
		Stderr:   f.stderr,
	}
}

type recordingQualityRunner struct {
	root string
}

func (r *recordingQualityRunner) Run(_ context.Context, root string, name string, args ...string) quality.Result {
	r.root = root
	return quality.Result{Command: name, Args: args, Dir: root, ExitCode: 0, Stdout: "ok"}
}

func newTestEngine(t *testing.T, state workflow.State) (Engine, string) {
	t.Helper()
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          "add-coupon-check",
		Title:       "coupon flow",
		Description: "coupon flow",
		Source:      "manual",
		State:       string(state),
	}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("create demand: %v", err)
	}
	return NewEngine(root), root
}

func readArtifact(t *testing.T, engine Engine, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(engine.Store.DemandDir("add-coupon-check"), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func TestEngineRequiresRunner(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Created)
	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageRequirements,
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "runner is required") {
		t.Fatalf("err = %v want runner is required", err)
	}
}

func TestImplementationQualityGateUsesQualityRoot(t *testing.T) {
	artifactRoot := t.TempDir()
	repoRoot := t.TempDir()
	store := artifacts.NewStore(artifactRoot)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "quality-root-check",
		Title:       "Quality root check",
		Description: "Quality commands should run in repo root",
		Source:      "test",
		State:       string(workflow.Implementation),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	runner := &recordingQualityRunner{}
	engine := NewEngine(artifactRoot)
	engine.Gate = quality.Gate{Runner: runner}

	err := engine.Run(context.Background(), Options{
		Root:        artifactRoot,
		QualityRoot: repoRoot,
		DemandID:    "quality-root-check",
		Stage:       StageImplementation,
		Runner: &StaticRunner{Responses: map[Stage]RunnerResponse{
			StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
		}},
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test", "./..."}}},
		Now:             fixedNow,
	})
	if err != nil {
		t.Fatalf("implementation: %v", err)
	}
	if runner.root != repoRoot {
		t.Fatalf("quality root = %q, want %q", runner.root, repoRoot)
	}
}

func TestEngineRequirementsDraftsAndAdvances(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Created)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageRequirements: {Text: "# Requirements: coupon flow\n\n## 目标行为\n\nimplement coupon check\n\n## 非目标范围\n\nnone\n\n## 业务规则\n\nonly active members\n\n## 用户/调用方影响\n\napi callers\n\n## 验收标准\n\nreturns coupon\n\n## 风险与歧义\n\nnone\n\n## 待确认问题\n\nnone\n\n## 人工确认记录\n\npending\n"},
	}}
	if err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageRequirements,
		Runner:   runner,
		Now:      fixedNow,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, err := engine.Store.LoadDemand("add-coupon-check")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if demand.State != string(workflow.RequirementsReview) {
		t.Fatalf("state = %q want requirements_review", demand.State)
	}
	if body := readArtifact(t, engine, artifacts.RequirementsFile); !strings.Contains(body, "implement coupon check") {
		t.Fatalf("requirements.md = %q", body)
	}
}

func TestEngineRunDetailedReportsResult(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Created)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageRequirements: {Text: "# Requirements: coupon flow\n\n## 目标行为\n\nimplement coupon check\n\n## 非目标范围\n\nnone\n\n## 业务规则\n\nonly active members\n\n## 用户/调用方影响\n\napi callers\n\n## 验收标准\n\nreturns coupon\n\n## 风险与歧义\n\nnone\n\n## 待确认问题\n\nnone\n\n## 人工确认记录\n\npending\n"},
	}}
	result, err := engine.RunDetailed(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageRequirements,
		Runner:   runner,
		Now:      fixedNow,
	})
	if err != nil {
		t.Fatalf("run detailed: %v", err)
	}
	if result.PreviousState != workflow.Created || result.CurrentState != workflow.RequirementsReview {
		t.Fatalf("states = %s -> %s", result.PreviousState, result.CurrentState)
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0] != artifacts.RequirementsFile {
		t.Fatalf("artifacts = %#v", result.Artifacts)
	}
	if len(result.NextActions) == 0 || result.NextActions[0].Label != "Confirm requirements" {
		t.Fatalf("next actions = %#v", result.NextActions)
	}
}

func TestEngineRunDetailedReportsFailedQualityGate(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 1, stderr: "test failed"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}
	result, err := engine.RunDetailed(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "quality gate failed") {
		t.Fatalf("err = %v want quality gate failed", err)
	}
	if result.CurrentState != workflow.FailedQualityGate {
		t.Fatalf("current state = %s want %s", result.CurrentState, workflow.FailedQualityGate)
	}
	if result.QualityPassed == nil || *result.QualityPassed {
		t.Fatalf("quality passed = %#v", result.QualityPassed)
	}
	if len(result.NextActions) == 0 || result.NextActions[0].Label != "Retry implementation" {
		t.Fatalf("next actions = %#v", result.NextActions)
	}
}
func TestEnginePlanDraftsAndAdvances(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.PlanDrafting)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StagePlan: {Text: "# Technical Plan: coupon flow\n\n## 当前实现与代码事实\n\nexisting\n\n## 目标设计\n\nplan body\n\n## 实施步骤\n\n- step\n\n## 改动范围\n\nscope\n\n## 数据结构/API/配置变化\n\nnone\n\n## 测试策略\n\ngo test ./...\n\n## 验收方式\n\nverification\n\n## 风险与回滚\n\nrevert\n\n## 不做事项\n\nnone\n\n## 人工确认记录\n\npending\n"},
	}}
	if err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StagePlan,
		Runner:   runner,
		Now:      fixedNow,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.PlanReview) {
		t.Fatalf("state = %q want plan_review", demand.State)
	}
	if body := readArtifact(t, engine, artifacts.PlanFile); !strings.Contains(body, "plan body") {
		t.Fatalf("plan.md = %q", body)
	}
}

func TestEngineImplementationPassesToMRReview(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented coupon check\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n", ToolSummary: []string{"edit file"}},
	}}
	if err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.MRReview) {
		t.Fatalf("state = %q want mr_review", demand.State)
	}
	if body := readArtifact(t, engine, artifacts.ProgressFile); !strings.Contains(body, "implemented coupon check") {
		t.Fatalf("progress.md = %q", body)
	}
}

func TestEngineImplementationFailsQualityGate(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 1, stderr: "test failed"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}
	err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "quality gate failed") {
		t.Fatalf("err = %v want quality gate failed", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.FailedQualityGate) {
		t.Fatalf("state = %q want failed_quality_gate", demand.State)
	}
}

func TestEngineImplementationRetriesFailedQualityGate(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.FailedQualityGate)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nretry fixed tests\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}
	if err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.MRReview) {
		t.Fatalf("state = %q want mr_review", demand.State)
	}
	if body := readArtifact(t, engine, artifacts.ProgressFile); !strings.Contains(body, "retry fixed tests") {
		t.Fatalf("progress.md = %q", body)
	}
	events, err := readDemandflowEventsFile(filepath.Join(engine.Store.DemandDir("add-coupon-check"), artifacts.EventsFile))
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	foundRetry := false
	for _, event := range events {
		if event.Type == "implementation.retry" {
			foundRetry = true
		}
	}
	if !foundRetry {
		t.Fatalf("events missing implementation.retry: %#v", events)
	}
}
func TestEngineVerificationStaysAndWritesArtifact(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Verification)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageVerification: {Text: "# Verification: coupon flow\n\n## 验收标准映射\n\nmapped\n\n## 自动化测试结果\n\npass\n\n## 手动验证记录\n\nnone\n\n## 接口/日志/监控证据\n\nnone\n\n## 未覆盖风险\n\nnone\n\n## 结论\n\nall green\n"},
	}}
	if err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageVerification,
		Runner:   runner,
		Now:      fixedNow,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.Verification) {
		t.Fatalf("state = %q want verification", demand.State)
	}
	if body := readArtifact(t, engine, artifacts.VerificationFile); !strings.Contains(body, "all green") {
		t.Fatalf("verification.md = %q", body)
	}
}

func TestEngineVerificationFailsQualityGate(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Verification)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 1, stderr: "verification failed"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageVerification: {Text: "# Verification: coupon flow\n\n## 验收标准映射\n\nmapped\n\n## 自动化测试结果\n\npass\n\n## 手动验证记录\n\nnone\n\n## 接口/日志/监控证据\n\nnone\n\n## 未覆盖风险\n\nnone\n\n## 结论\n\nneeds work\n"},
	}}
	result, err := engine.RunDetailed(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageVerification,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "quality gate failed") {
		t.Fatalf("err = %v want quality gate failed", err)
	}
	if result.CurrentState != workflow.FailedQualityGate {
		t.Fatalf("current state = %s want %s", result.CurrentState, workflow.FailedQualityGate)
	}
	if result.QualityPassed == nil || *result.QualityPassed {
		t.Fatalf("quality passed = %#v", result.QualityPassed)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.FailedQualityGate) {
		t.Fatalf("state = %q want failed_quality_gate", demand.State)
	}
}

func TestEngineCloseoutWritesCloseoutAndMemory(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Closeout)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageCloseout: {Text: "# Closeout: coupon flow\n\n## 需求结果\n\ndone\n\n## 关键产物链接\n\n- verification.md\n\n## MR 评论与处理摘要\n\nnone\n\n## 验收证据摘要\n\npass\n\n## 稳定知识候选\n\n- coupon flow knowledge\n\n## 流程改进候选\n\nnone\n\n## 一次性材料归档\n\nnone\n\n## 人工确认记录\n\npending\n\n---DEVFLOW-MEMORY-CANDIDATES---\n\n# Memory Candidates: coupon flow\n\n## 稳定知识候选\n\n- coupon flow knowledge\n\n## 流程改进候选\n\nnone\n\n## 不进入长期知识的材料\n\nnone\n"},
	}}
	if err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageCloseout,
		Runner:   runner,
		Now:      fixedNow,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.Closeout) {
		t.Fatalf("state = %q want closeout", demand.State)
	}
	if body := readArtifact(t, engine, artifacts.CloseoutFile); !strings.Contains(body, "done") {
		t.Fatalf("closeout.md = %q", body)
	}
	if body := readArtifact(t, engine, artifacts.MemoryCandidatesFile); !strings.Contains(body, "coupon flow knowledge") {
		t.Fatalf("memory-candidates.md = %q", body)
	}
}

func TestEngineCloseoutRejectsMissingMemoryMarker(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Closeout)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageCloseout: {Text: "# Closeout: coupon flow\n\n## 需求结果\n\ndone\n\n## 关键产物链接\n\n- verification.md\n\n## MR 评论与处理摘要\n\nnone\n\n## 验收证据摘要\n\npass\n\n## 稳定知识候选\n\n- rule\n\n## 流程改进候选\n\nnone\n\n## 一次性材料归档\n\nnone\n\n## 人工确认记录\n\npending\n"},
	}}

	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StageCloseout,
		Runner:   runner,
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "missing memory candidates marker") {
		t.Fatalf("err = %v want missing memory candidates marker", err)
	}
	demand, loadErr := engine.Store.LoadDemand("add-coupon-check")
	if loadErr != nil {
		t.Fatalf("load demand: %v", loadErr)
	}
	if demand.State != string(workflow.Closeout) {
		t.Fatalf("state = %s, want closeout", demand.State)
	}
}

func readDemandflowEventsFile(path string) ([]artifacts.Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []artifacts.Event
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event artifacts.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

type recordingDemandRunner struct {
	root string
}

func (r *recordingDemandRunner) Run(_ context.Context, req RunnerRequest) (RunnerResponse, error) {
	r.root = req.Root
	return RunnerResponse{Text: "# Requirements: runner root check\n\n## 目标行为\n\nrunner root recorded\n\n## 非目标范围\n\nnone\n\n## 业务规则\n\nrule\n\n## 用户/调用方影响\n\nimpact\n\n## 验收标准\n\ncriteria\n\n## 风险与歧义\n\nnone\n\n## 待确认问题\n\nnone\n\n## 人工确认记录\n\npending\n"}, nil
}

func TestRequirementsRunnerUsesRunnerRoot(t *testing.T) {
	artifactRoot := t.TempDir()
	codeRoot := t.TempDir()
	store := artifacts.NewStore(artifactRoot)
	if err := store.CreateDemand(artifacts.Demand{
		ID:          "runner-root-check",
		Title:       "Runner root check",
		Description: "Agent should run in code root",
		Source:      "test",
		State:       string(workflow.Created),
	}); err != nil {
		t.Fatalf("create demand: %v", err)
	}

	runner := &recordingDemandRunner{}
	engine := NewEngine(artifactRoot)
	err := engine.Run(context.Background(), Options{
		Root:       artifactRoot,
		RunnerRoot: codeRoot,
		DemandID:   "runner-root-check",
		Stage:      StageRequirements,
		Runner:     runner,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("requirements: %v", err)
	}
	if runner.root != codeRoot {
		t.Fatalf("runner root = %q, want %q", runner.root, codeRoot)
	}
}

type fakeMergeRequestSyncAdapter struct {
	result adapters.MergeRequestResult
	err    error
}

func (f fakeMergeRequestSyncAdapter) EnsureMergeRequest(_ context.Context, _ adapters.MergeRequestSpec) (adapters.MergeRequestResult, error) {
	return f.result, f.err
}

func TestEngineImplementationSyncMergeRequestPasses(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}

	fakeAdapter := fakeMergeRequestSyncAdapter{
		result: adapters.MergeRequestResult{
			IID: 42, WebURL: "https://gitlab.com/p/-/42", Title: "MR", State: "opened", WasCreated: false,
		},
	}

	if err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
		MergeRequest: MergeRequestOptions{
			Adapter: fakeAdapter,
			Spec:    adapters.MergeRequestSpec{Project: "p", SourceBranch: "s", TargetBranch: "t", Title: "MR"},
		},
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	body := readArtifact(t, engine, artifacts.ProgressFile)
	if !strings.Contains(body, "!42") {
		t.Fatalf("progress.md missing MR evidence:\n%s", body)
	}
}

func TestEngineImplementationSyncMergeRequestSkippedWhenNil(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}

	if err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.MRReview) {
		t.Fatalf("state = %q want mr_review", demand.State)
	}
}

func TestEngineImplementationSyncMergeRequestFailureBlocksPlatform(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}

	result, err := engine.RunDetailed(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
		MergeRequest: MergeRequestOptions{
			Adapter: fakeMergeRequestSyncAdapter{err: errors.New("gitlab unavailable")},
			Spec:    adapters.MergeRequestSpec{Project: "p", SourceBranch: "s", TargetBranch: "t", Title: "MR"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "blocked_need_platform") {
		t.Fatalf("err = %v, want blocked_need_platform", err)
	}
	if result.CurrentState != workflow.BlockedNeedPlatform {
		t.Fatalf("current state = %s, want %s", result.CurrentState, workflow.BlockedNeedPlatform)
	}
	demand, _ := engine.Store.LoadDemand("add-coupon-check")
	if demand.State != string(workflow.BlockedNeedPlatform) {
		t.Fatalf("state = %q want blocked_need_platform", demand.State)
	}
}

func TestEngineImplementationSyncChangeRequestOptionPasses(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}

	fakeAdapter := fakeMergeRequestSyncAdapter{
		result: adapters.MergeRequestResult{
			IID: 77, WebURL: "https://github.com/o/r/pull/77", Title: "CR", State: "open", WasCreated: true,
		},
	}

	if err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
		ChangeRequest: ChangeRequestOptions{
			Adapter: fakeAdapter,
			Spec:    adapters.ChangeRequestSpec{Project: "p", SourceBranch: "s", TargetBranch: "t", Title: "CR"},
		},
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	body := readArtifact(t, engine, artifacts.ProgressFile)
	if !strings.Contains(body, "!77") {
		t.Fatalf("progress.md missing change-request evidence:\n%s", body)
	}
}

func TestEngineChangeRequestOptionOverridesMergeRequest(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	engine.Gate = quality.Gate{Runner: fakeQualityRunner{exitCode: 0, stdout: "all tests pass"}}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n\n## 代码改动\n\n- file.go\n\n## 测试与验证\n\n- go test ./...\n\n## 遗留问题\n\nnone\n"},
	}}

	if err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test"}}},
		Now:             fixedNow,
		MergeRequest: MergeRequestOptions{
			Adapter: fakeMergeRequestSyncAdapter{result: adapters.MergeRequestResult{IID: 11, WebURL: "https://gitlab.com/p/-/11", Title: "old", State: "opened"}},
			Spec:    adapters.MergeRequestSpec{Project: "p", SourceBranch: "s", TargetBranch: "t", Title: "old"},
		},
		ChangeRequest: ChangeRequestOptions{
			Adapter: fakeMergeRequestSyncAdapter{result: adapters.MergeRequestResult{IID: 99, WebURL: "https://github.com/o/r/pull/99", Title: "new", State: "open", WasCreated: true}},
			Spec:    adapters.ChangeRequestSpec{Project: "p", SourceBranch: "s", TargetBranch: "t", Title: "new"},
		},
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	body := readArtifact(t, engine, artifacts.ProgressFile)
	if !strings.Contains(body, "!99") {
		t.Fatalf("progress.md should use change-request adapter (!99):\n%s", body)
	}
	if strings.Contains(body, "!11") {
		t.Fatalf("progress.md should not use merge-request adapter (!11):\n%s", body)
	}
}

func TestEnginePlanRejectsInvalidArtifactAndDoesNotAdvance(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.PlanDrafting)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StagePlan: {Text: "# Technical Plan: coupon flow\n\n## 目标设计\n\nmissing steps"},
	}}

	err := engine.Run(context.Background(), Options{
		Root:     root,
		DemandID: "add-coupon-check",
		Stage:    StagePlan,
		Runner:   runner,
		Now:      fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "plan.md invalid") {
		t.Fatalf("err = %v want plan.md invalid", err)
	}
	demand, loadErr := engine.Store.LoadDemand("add-coupon-check")
	if loadErr != nil {
		t.Fatalf("load demand: %v", loadErr)
	}
	if demand.State != string(workflow.PlanDrafting) {
		t.Fatalf("state = %s, want plan_drafting", demand.State)
	}
	planPath := filepath.Join(engine.Store.DemandDir("add-coupon-check"), artifacts.PlanFile)
	if body, readErr := os.ReadFile(planPath); readErr == nil && strings.Contains(string(body), "missing steps") {
		t.Fatalf("plan.md should not contain invalid artifact text, got %q", body)
	}
}

func TestEngineImplementationRejectsInvalidProgressBeforeQualityGate(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Implementation)
	qualityRunner := &recordingQualityRunner{}
	engine.Gate = quality.Gate{Runner: qualityRunner}
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageImplementation: {Text: "## 实现摘要\n\nmissing required sections"},
	}}

	err := engine.Run(context.Background(), Options{
		Root:            root,
		DemandID:        "add-coupon-check",
		Stage:           StageImplementation,
		Runner:          runner,
		QualityCommands: []quality.Command{{Name: "go", Args: []string{"test", "./..."}}},
		Now:             fixedNow,
	})
	if err == nil || !strings.Contains(err.Error(), "progress.md invalid") {
		t.Fatalf("err = %v want progress.md invalid", err)
	}
	if qualityRunner.root != "" {
		t.Fatalf("quality gate should not run for invalid progress, ran in %q", qualityRunner.root)
	}
	demand, loadErr := engine.Store.LoadDemand("add-coupon-check")
	if loadErr != nil {
		t.Fatalf("load demand: %v", loadErr)
	}
	if demand.State != string(workflow.Implementation) {
		t.Fatalf("state = %s, want implementation", demand.State)
	}
}
