package demandflow

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestEngineRequirementsDraftsAndAdvances(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Created)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageRequirements: {Text: "# Requirements: coupon flow\n\n## 目标行为\n\nimplement coupon check\n"},
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
		StageRequirements: {Text: "# Requirements: coupon flow\n\n## 目标行为\n\nimplement coupon check\n"},
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
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n"},
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
		StagePlan: {Text: "# Technical Plan: coupon flow\n\n## 目标设计\n\nplan body\n"},
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
		StageImplementation: {Text: "## 实现摘要\n\nimplemented coupon check\n", ToolSummary: []string{"edit file"}},
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
		StageImplementation: {Text: "## 实现摘要\n\nimplemented\n"},
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
		StageImplementation: {Text: "## 实现摘要\n\nretry fixed tests\n"},
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
		StageVerification: {Text: "# Verification: coupon flow\n\n## 结论\n\nall green\n"},
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

func TestEngineCloseoutWritesCloseoutAndMemory(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Closeout)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageCloseout: {Text: "# Closeout: coupon flow\n\n## 需求结果\n\ndone\n\n---DEVFLOW-MEMORY-CANDIDATES---\n\n# Memory Candidates: coupon flow\n\n## 稳定知识候选\n\n- coupon flow knowledge\n"},
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

func TestEngineCloseoutWithoutMarkerKeepsMemoryNote(t *testing.T) {
	t.Parallel()
	engine, root := newTestEngine(t, workflow.Closeout)
	runner := &StaticRunner{Responses: map[Stage]RunnerResponse{
		StageCloseout: {Text: "# Closeout: coupon flow\n\n## 需求结果\n\ndone\n"},
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
	if body := readArtifact(t, engine, artifacts.MemoryCandidatesFile); !strings.Contains(body, "no stable candidates were generated") {
		t.Fatalf("memory-candidates.md = %q", body)
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
