# Wave 13 Demand Workspace Operator Loop Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an evidence-aware demand workspace summary behind `devflow status` and `devflow next`, including `status --all`.

**Architecture:** Add a read-only workspace summary layer in `internal/demandflow` that derives operational status from existing demand files, artifacts, events, verification evidence, MR evidence, and memory decisions. Keep workflow state authoritative in `demand.json`; Wave 13 only computes summaries and recommended commands, then the CLI formats those summaries.

**Tech Stack:** Go standard library, existing `internal/artifacts`, `internal/workflow`, `internal/memory`, existing CLI tests, PowerShell verification commands.

---

## Current Baseline

The repository is on `main` and Wave 12 is merged. The current implementation has:

- `internal/demandflow/status.go`
  - `InspectStatus(root, demandID) (StatusReport, error)`
  - `NextActions(state, demandID) []NextAction`
  - artifact display only reports missing or byte size.
- `internal/cli/status.go`
  - `devflow status --demand <id>`
  - `devflow next --demand <id>`
  - no `--all`.
- `internal/artifacts/store.go`
  - unexported `readEventsLogRecoverTrailing(path string) ([]Event, error)`.
- `internal/memory/decisions.go`
  - `Store.ListCandidates(demandID)` returns candidate statuses after reading memory decision events.

Wave 13 must not add a database, TUI, Web UI, GitLab API calls, auto execution, or workflow state machine changes.

## File Structure

Create or modify these files:

- Create `internal/demandflow/workspace.go`
  - Owns `WorkspaceSummary`, `StageSummary`, `ArtifactSummary`, `VerificationSummary`, `MergeRequestSummary`, `MemorySummary`, `ListWorkspaces`, and `InspectWorkspace`.
- Create `internal/demandflow/workspace_test.go`
  - Tests single-demand summary, evidence extraction, memory counts, MR evidence, list ordering, and artifact statuses.
- Modify `internal/demandflow/status.go`
  - Keep old public `StatusReport` shape for compatibility.
  - Delegate `InspectStatus` and evidence-aware actions to `InspectWorkspace`.
  - Keep `NextActions` as state-only fallback for old tests and callers.
- Modify `internal/demandflow/status_test.go`
  - Preserve existing state-only tests.
  - Add evidence-aware regression tests for `InspectStatus`.
- Modify `internal/artifacts/store.go`
  - Add exported read-only `ReadEvents(demandID string) ([]Event, error)`.
- Modify `internal/artifacts/store_test.go`
  - Add tests proving `ReadEvents` recovers trailing partial JSON but fails on malformed middle JSON.
- Modify `internal/cli/status.go`
  - Add `--all`.
  - Render detailed workspace summary for one demand.
  - Render compact list for all demands.
  - Make `next` use evidence-aware workspace actions.
- Modify `internal/cli/status_test.go`
  - Add CLI output tests for enhanced detail, `--all`, and evidence-aware `next`.
- Modify `internal/cli/cli.go`
  - Update help text for `status --all`.
- Modify docs:
  - `docs/backend-demand-loop.md`
  - `docs/release/v0.1.md`

## Data Contract

Use these exact strings for computed statuses so CLI tests remain stable:

- Stage statuses:
  - `pending`
  - `drafting`
  - `needs_confirmation`
  - `confirmed`
  - `completed`
  - `needs_review`
  - `cleared`
  - `needs_evidence`
  - `passed`
  - `failed`
  - `blocked`
- Artifact statuses:
  - `missing`
  - `template`
  - `present`
  - `confirmed`
  - `needs_review`
  - `has_pass_evidence`
  - `has_fail_evidence`
  - `read_error`
- MR statuses:
  - `not_started`
  - `needs_review`
  - `cleared`
  - `action_required`
- Verification statuses:
  - `none`
  - `pass`
  - `fail`
- Memory status:
  - `none`
  - `pending`
  - `settled`

The CLI should print these as user-readable phrases by replacing underscores with spaces.

## Task 1: Export Read-Only Event Loading

**Files:**
- Modify: `internal/artifacts/store.go`
- Modify: `internal/artifacts/store_test.go`

- [ ] **Step 1: Write failing tests for exported event loading**

Add these tests near existing event log tests in `internal/artifacts/store_test.go`:

```go
func TestReadEventsRecoversTrailingPartialEvent(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand, err := store.CreateDemand(CreateDemandOptions{
		ID:          "read-events-trailing",
		Title:       "Read events trailing",
		Description: "Exercise event reader",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	eventsPath := filepath.Join(root, ".devflow", "demands", demand.ID, EventsFile)
	if err := os.WriteFile(eventsPath, []byte(`{"time":"2026-06-30T01:02:03Z","type":"stage.confirmed","message":"requirements confirmed","data":{"stage":"requirements"}}`+"\n"+`{"time":`), 0o644); err != nil {
		t.Fatalf("write events log: %v", err)
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("ReadEvents returned %d events, want 1", len(events))
	}
	if events[0].Type != "stage.confirmed" || events[0].Data["stage"] != "requirements" {
		t.Fatalf("ReadEvents returned unexpected event: %#v", events[0])
	}
}

func TestReadEventsFailsOnMalformedMiddleEvent(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand, err := store.CreateDemand(CreateDemandOptions{
		ID:          "read-events-middle",
		Title:       "Read events middle",
		Description: "Exercise event reader",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	eventsPath := filepath.Join(root, ".devflow", "demands", demand.ID, EventsFile)
	body := strings.Join([]string{
		`{"time":"2026-06-30T01:02:03Z","type":"demand.created","message":"created"}`,
		`{"time":`,
		`{"time":"2026-06-30T01:03:03Z","type":"stage.confirmed","message":"requirements confirmed","data":{"stage":"requirements"}}`,
		"",
	}, "\n")
	if err := os.WriteFile(eventsPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write events log: %v", err)
	}

	_, err = store.ReadEvents(demand.ID)
	if err == nil {
		t.Fatal("ReadEvents returned nil error for malformed middle event")
	}
	if !strings.Contains(err.Error(), "decode event log line 2") {
		t.Fatalf("ReadEvents error = %q, want decode line context", err)
	}
}
```

- [ ] **Step 2: Run the failing artifact test**

Run:

```powershell
go test ./internal/artifacts -run "TestReadEvents" -count=1
```

Expected result before implementation:

```text
FAIL
store.ReadEvents undefined
```

- [ ] **Step 3: Implement `ReadEvents`**

In `internal/artifacts/store.go`, add this method near `AppendEvent`:

```go
func (s Store) ReadEvents(demandID string) ([]Event, error) {
	paths, err := s.prepareDemandPaths(demandID)
	if err != nil {
		return nil, err
	}
	events, err := readEventsLogRecoverTrailing(filepath.Join(paths.demandDir, EventsFile))
	if err != nil {
		return nil, err
	}
	return events, nil
}
```

Do not create or mutate files in this method.

- [ ] **Step 4: Run the artifact package tests**

Run:

```powershell
go test ./internal/artifacts -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/artifacts
```

- [ ] **Step 5: Commit Task 1**

Run:

```powershell
git add internal/artifacts/store.go internal/artifacts/store_test.go
git commit -m "Expose read-only demand event loading" -m "Wave 13 needs status summaries to inspect local event evidence without writing new state. This adds a Store.ReadEvents wrapper around the existing trailing-recovery parser instead of duplicating JSONL parsing in demandflow." -m "Constraint: Status must be read-only and must reuse existing event recovery semantics." -m "Rejected: Parse events directly in demandflow | would duplicate artifact log behavior and drift from confirmation repair tests." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/artifacts -count=1"
```

## Task 2: Build Workspace Summary Types and Single-Demand Inspection

**Files:**
- Create: `internal/demandflow/workspace.go`
- Create: `internal/demandflow/workspace_test.go`
- Modify: `internal/demandflow/status.go`

- [ ] **Step 1: Write failing workspace summary tests**

Create `internal/demandflow/workspace_test.go` with this content:

```go
package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/memory"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestInspectWorkspaceSummarizesEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand, err := store.CreateDemand(artifacts.CreateDemandOptions{
		ID:          "workspace-evidence",
		Title:       "Workspace evidence",
		Description: "Need an operator summary",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.UpdateState(demand.ID, string(workflow.Verification)); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.RequirementsFile, "\n- requirement detail\n"); err != nil {
		t.Fatalf("AppendToArtifact requirements returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.PlanFile, "\n- plan detail\n"); err != nil {
		t.Fatalf("AppendToArtifact plan returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.ProgressFile, "\nMR: !12 open\nreview gate cleared\n"); err != nil {
		t.Fatalf("AppendToArtifact progress returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.VerificationFile, "\nPASS go test ./...\n"); err != nil {
		t.Fatalf("AppendToArtifact verification returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.MemoryCandidatesFile, "\n- Reuse tenant validation rule\n- Reuse audit logging pattern\n"); err != nil {
		t.Fatalf("AppendToArtifact memory candidates returned error: %v", err)
	}
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "stage.confirmed", Message: "requirements confirmed", Data: map[string]string{"stage": "requirements"}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(time.Minute), Type: "stage.confirmed", Message: "plan confirmed", Data: map[string]string{"stage": "plan"}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(2 * time.Minute), Type: "implementation.completed", Message: "implementation completed"})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(3 * time.Minute), Type: "mr_review.cleared", Message: "review gate cleared", Data: map[string]string{"mr": "!12"}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(4 * time.Minute), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./...", "evidence_file": "verification.md"}})

	memStore := memory.NewStore(root)
	if _, err := memStore.PromoteCandidate(memory.PromoteOptions{DemandID: demand.ID, CandidateIndex: 1, Name: "tenant-validation", Description: "Tenant validation", By: "tester", Now: fixedWorkspaceTime}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}

	if summary.Demand.ID != demand.ID {
		t.Fatalf("Demand.ID = %q, want %q", summary.Demand.ID, demand.ID)
	}
	if summary.State != workflow.Verification {
		t.Fatalf("State = %q, want %q", summary.State, workflow.Verification)
	}
	assertStageStatus(t, summary, "requirements", "confirmed")
	assertStageStatus(t, summary, "plan", "confirmed")
	assertStageStatus(t, summary, "implementation", "completed")
	assertStageStatus(t, summary, "mr-review", "cleared")
	assertStageStatus(t, summary, "verification", "passed")
	assertArtifactStatus(t, summary, artifacts.RequirementsFile, "confirmed")
	assertArtifactStatus(t, summary, artifacts.VerificationFile, "has_pass_evidence")
	if summary.MergeRequest.Status != "cleared" {
		t.Fatalf("MergeRequest.Status = %q, want cleared", summary.MergeRequest.Status)
	}
	if summary.MergeRequest.Reference != "!12" {
		t.Fatalf("MergeRequest.Reference = %q, want !12", summary.MergeRequest.Reference)
	}
	if summary.Verification.Status != "pass" || summary.Verification.Command != "go test ./..." {
		t.Fatalf("Verification = %#v, want pass go test ./...", summary.Verification)
	}
	if summary.Memory.Pending != 1 || summary.Memory.Promoted != 1 || summary.Memory.Rejected != 0 {
		t.Fatalf("Memory = %#v, want 1 pending, 1 promoted, 0 rejected", summary.Memory)
	}
	if summary.Attention != "ready to confirm verification" {
		t.Fatalf("Attention = %q, want ready to confirm verification", summary.Attention)
	}
}

func TestInspectWorkspaceTemplateAndMissingArtifacts(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand, err := store.CreateDemand(artifacts.CreateDemandOptions{
		ID:          "workspace-template",
		Title:       "Workspace template",
		Description: "Template detection",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	demandDir := filepath.Join(root, ".devflow", "demands", demand.ID)
	if err := os.Remove(filepath.Join(demandDir, artifacts.CloseoutFile)); err != nil {
		t.Fatalf("remove closeout: %v", err)
	}

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}

	assertArtifactStatus(t, summary, artifacts.RequirementsFile, "template")
	assertArtifactStatus(t, summary, artifacts.PlanFile, "template")
	assertArtifactStatus(t, summary, artifacts.CloseoutFile, "missing")
}

func TestInspectWorkspaceFailVerificationEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand, err := store.CreateDemand(artifacts.CreateDemandOptions{
		ID:          "workspace-fail",
		Title:       "Workspace fail",
		Description: "Fail evidence",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.UpdateState(demand.ID, string(workflow.Verification)); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "verification.recorded", Message: "verification fail", Data: map[string]string{"status": "fail", "command": "go test ./...", "failure_kind": "exit_nonzero"}})

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}

	assertStageStatus(t, summary, "verification", "failed")
	assertArtifactStatus(t, summary, artifacts.VerificationFile, "has_fail_evidence")
	if summary.Verification.FailureKind != "exit_nonzero" {
		t.Fatalf("FailureKind = %q, want exit_nonzero", summary.Verification.FailureKind)
	}
	if summary.Attention != "verification failed" {
		t.Fatalf("Attention = %q, want verification failed", summary.Attention)
	}
}

func appendWorkspaceEvent(t *testing.T, store artifacts.Store, demandID string, event artifacts.Event) {
	t.Helper()
	if err := store.AppendEvent(demandID, event); err != nil {
		t.Fatalf("AppendEvent(%s) returned error: %v", event.Type, err)
	}
}

func fixedWorkspaceTime() time.Time {
	return time.Date(2026, 6, 30, 1, 2, 3, 0, time.UTC)
}

func assertStageStatus(t *testing.T, summary WorkspaceSummary, name, want string) {
	t.Helper()
	for _, stage := range summary.Stages {
		if stage.Name == name {
			if stage.Status != want {
				t.Fatalf("stage %s status = %q, want %q", name, stage.Status, want)
			}
			return
		}
	}
	t.Fatalf("stage %s not found in %#v", name, summary.Stages)
}

func assertArtifactStatus(t *testing.T, summary WorkspaceSummary, name, want string) {
	t.Helper()
	for _, artifact := range summary.Artifacts {
		if artifact.Name == name {
			if artifact.Status != want {
				t.Fatalf("artifact %s status = %q, want %q; notes=%s", name, artifact.Status, want, strings.Join(artifact.Notes, ", "))
			}
			return
		}
	}
	t.Fatalf("artifact %s not found in %#v", name, summary.Artifacts)
}
```

- [ ] **Step 2: Run failing workspace tests**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectWorkspace" -count=1
```

Expected result before implementation:

```text
FAIL
undefined: InspectWorkspace
undefined: WorkspaceSummary
```

- [ ] **Step 3: Add workspace summary implementation**

Create `internal/demandflow/workspace.go` with this complete implementation:

```go
package demandflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/memory"
	"github.com/jesseedcp/devflow-agent/internal/templates"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type StageSummary struct {
	Name     string
	Status   string
	Evidence string
}

type ArtifactSummary struct {
	Name   string
	Path   string
	Exists bool
	Size   int64
	Status string
	Notes  []string
	Error  string
}

type VerificationSummary struct {
	Status       string
	Command      string
	FailureKind string
	EvidenceFile string
	Excerpt      string
}

type MergeRequestSummary struct {
	Status    string
	Reference string
	Evidence  string
}

type MemorySummary struct {
	Status   string
	Pending  int
	Promoted int
	Rejected int
	Error    string
}

type WorkspaceSummary struct {
	Demand       artifacts.Demand
	State        workflow.State
	DemandDir    string
	Stages       []StageSummary
	Artifacts    []ArtifactSummary
	Verification VerificationSummary
	MergeRequest MergeRequestSummary
	Memory       MemorySummary
	Actions      []NextAction
	Attention    string
}

func InspectWorkspace(root, demandID string) (WorkspaceSummary, error) {
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return WorkspaceSummary{}, err
	}
	events, eventsErr := store.ReadEvents(demandID)
	demandDir := store.DemandDir(demandID)
	summary := WorkspaceSummary{
		Demand:    demand,
		State:     workflow.State(demand.State),
		DemandDir: demandDir,
	}
	summary.Verification = summarizeVerification(events)
	summary.MergeRequest = summarizeMergeRequest(events, readArtifactText(filepath.Join(demandDir, artifacts.ProgressFile)).text)
	summary.Memory = summarizeMemory(root, demandID)
	summary.Stages = summarizeStages(summary.State, events, summary.Verification, summary.MergeRequest)
	summary.Artifacts = summarizeArtifacts(demandDir, demand, eventsErr, summary)
	summary.Attention = workspaceAttention(summary, eventsErr)
	summary.Actions = WorkspaceNextActions(summary)
	return summary, nil
}

func summarizeStages(state workflow.State, events []artifacts.Event, verification VerificationSummary, mr MergeRequestSummary) []StageSummary {
	confirmed := confirmedStages(events)
	stageStatus := map[string]string{
		"requirements":   "pending",
		"plan":           "pending",
		"implementation": "pending",
		"mr-review":      "pending",
		"verification":   "pending",
		"closeout":       "pending",
	}
	if confirmed["requirements"] {
		stageStatus["requirements"] = "confirmed"
	}
	if confirmed["plan"] {
		stageStatus["plan"] = "confirmed"
	}
	if confirmed["verification"] {
		stageStatus["verification"] = "confirmed"
	}
	if confirmed["closeout"] {
		stageStatus["closeout"] = "confirmed"
	}
	for _, event := range events {
		if event.Type == "implementation.completed" {
			stageStatus["implementation"] = "completed"
		}
	}
	if mr.Status != "not_started" {
		stageStatus["mr-review"] = mr.Status
	}
	switch verification.Status {
	case "pass":
		stageStatus["verification"] = "passed"
	case "fail":
		stageStatus["verification"] = "failed"
	}
	switch state {
	case workflow.RequirementsDrafting:
		stageStatus["requirements"] = "drafting"
	case workflow.RequirementsReview:
		if !confirmed["requirements"] {
			stageStatus["requirements"] = "needs_confirmation"
		}
	case workflow.PlanDrafting:
		stageStatus["plan"] = "drafting"
	case workflow.PlanReview:
		if !confirmed["plan"] {
			stageStatus["plan"] = "needs_confirmation"
		}
	case workflow.Implementation, workflow.FailedQualityGate, workflow.ReturnedToRequirements, workflow.ReturnedToPlan:
		if stageStatus["implementation"] == "pending" {
			stageStatus["implementation"] = "drafting"
		}
	case workflow.MRReview:
		if stageStatus["mr-review"] == "pending" {
			stageStatus["mr-review"] = "needs_review"
		}
	case workflow.Verification:
		if verification.Status == "none" {
			stageStatus["verification"] = "needs_evidence"
		}
	case workflow.Closeout:
		if !confirmed["closeout"] {
			stageStatus["closeout"] = "needs_confirmation"
		}
	case workflow.BlockedNeedUser, workflow.BlockedNeedPlatform:
		stageStatus["implementation"] = "blocked"
	}
	names := []string{"requirements", "plan", "implementation", "mr-review", "verification", "closeout"}
	out := make([]StageSummary, 0, len(names))
	for _, name := range names {
		out = append(out, StageSummary{Name: name, Status: stageStatus[name]})
	}
	return out
}

func summarizeArtifacts(demandDir string, demand artifacts.Demand, eventsErr error, summary WorkspaceSummary) []ArtifactSummary {
	names := []string{
		artifacts.RequirementsFile,
		artifacts.PlanFile,
		artifacts.ProgressFile,
		artifacts.VerificationFile,
		artifacts.CloseoutFile,
		artifacts.MemoryCandidatesFile,
		artifacts.EventsFile,
	}
	out := make([]ArtifactSummary, 0, len(names))
	for _, name := range names {
		path := filepath.Join(demandDir, name)
		artifact := ArtifactSummary{Name: name, Path: path, Status: "missing"}
		stat, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				out = append(out, artifact)
				continue
			}
			artifact.Status = "read_error"
			artifact.Error = statErr.Error()
			out = append(out, artifact)
			continue
		}
		artifact.Exists = true
		artifact.Size = stat.Size()
		textResult := readArtifactText(path)
		if textResult.err != nil {
			artifact.Status = "read_error"
			artifact.Error = textResult.err.Error()
			out = append(out, artifact)
			continue
		}
		artifact.Status = artifactBaseStatus(name, textResult.text, demand)
		switch name {
		case artifacts.RequirementsFile:
			if stageStatus(summary, "requirements") == "confirmed" {
				artifact.Status = "confirmed"
			}
		case artifacts.PlanFile:
			if stageStatus(summary, "plan") == "confirmed" {
				artifact.Status = "confirmed"
			}
		case artifacts.VerificationFile:
			if summary.Verification.Status == "pass" {
				artifact.Status = "has_pass_evidence"
			}
			if summary.Verification.Status == "fail" {
				artifact.Status = "has_fail_evidence"
			}
		case artifacts.CloseoutFile:
			if stageStatus(summary, "closeout") == "confirmed" {
				artifact.Status = "confirmed"
			}
		case artifacts.MemoryCandidatesFile:
			if summary.Memory.Pending > 0 {
				artifact.Status = "needs_review"
				artifact.Notes = append(artifact.Notes, fmt.Sprintf("%d pending candidates", summary.Memory.Pending))
			}
		case artifacts.EventsFile:
			if eventsErr != nil {
				artifact.Status = "read_error"
				artifact.Error = eventsErr.Error()
			} else {
				artifact.Status = "present"
			}
		}
		out = append(out, artifact)
	}
	return out
}

type artifactText struct {
	text string
	err  error
}

func readArtifactText(path string) artifactText {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifactText{err: err}
	}
	return artifactText{text: string(data)}
}

func artifactBaseStatus(name, text string, demand artifacts.Demand) string {
	if strings.TrimSpace(text) == "" {
		return "template"
	}
	expected := ""
	switch name {
	case artifacts.RequirementsFile:
		expected = templates.Requirements(demand.Title, demand.Description)
	case artifacts.PlanFile:
		expected = templates.Plan(demand.Title)
	case artifacts.VerificationFile:
		expected = templates.Verification(demand.Title)
	case artifacts.CloseoutFile:
		expected = templates.Closeout(demand.Title)
	case artifacts.MemoryCandidatesFile:
		expected = templates.MemoryCandidates(demand.Title)
	default:
		return "present"
	}
	if strings.TrimSpace(text) == strings.TrimSpace(expected) {
		return "template"
	}
	return "present"
}

func confirmedStages(events []artifacts.Event) map[string]bool {
	out := map[string]bool{}
	for _, event := range events {
		if event.Type != "stage.confirmed" {
			continue
		}
		stage := strings.TrimSpace(event.Data["stage"])
		if stage != "" {
			out[stage] = true
		}
	}
	return out
}

func summarizeVerification(events []artifacts.Event) VerificationSummary {
	out := VerificationSummary{Status: "none"}
	for _, event := range events {
		if event.Type != "verification.recorded" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(event.Data["status"]))
		if status != "pass" && status != "fail" {
			continue
		}
		out = VerificationSummary{
			Status:       status,
			Command:      strings.TrimSpace(event.Data["command"]),
			FailureKind: strings.TrimSpace(event.Data["failure_kind"]),
			EvidenceFile: strings.TrimSpace(event.Data["evidence_file"]),
			Excerpt:      strings.TrimSpace(event.Message),
		}
	}
	return out
}

func summarizeMergeRequest(events []artifacts.Event, progressText string) MergeRequestSummary {
	out := MergeRequestSummary{Status: "not_started"}
	for _, event := range events {
		switch event.Type {
		case "mr_review.action_required":
			out.Status = "action_required"
			out.Evidence = strings.TrimSpace(event.Message)
			out.Reference = firstNonEmpty(event.Data["mr"], event.Data["iid"], event.Data["url"], out.Reference)
		case "mr_review.cleared":
			out.Status = "cleared"
			out.Evidence = strings.TrimSpace(event.Message)
			out.Reference = firstNonEmpty(event.Data["mr"], event.Data["iid"], event.Data["url"], out.Reference)
		}
	}
	if out.Reference == "" {
		out.Reference = extractMRReference(progressText)
	}
	if out.Status == "not_started" && strings.Contains(strings.ToLower(progressText), "review gate cleared") {
		out.Status = "cleared"
		out.Evidence = "progress.md review gate cleared"
	}
	if out.Status == "not_started" && out.Reference != "" {
		out.Status = "needs_review"
	}
	return out
}

func extractMRReference(text string) string {
	for _, field := range strings.Fields(text) {
		cleaned := strings.Trim(field, ".,;:()[]")
		if strings.HasPrefix(cleaned, "!") && len(cleaned) > 1 {
			return cleaned
		}
	}
	return ""
}

func summarizeMemory(root, demandID string) MemorySummary {
	candidates, err := memory.NewStore(root).ListCandidates(demandID)
	if err != nil {
		return MemorySummary{Status: "none", Error: err.Error()}
	}
	out := MemorySummary{}
	for _, candidate := range candidates {
		switch candidate.Status {
		case memory.CandidatePromoted:
			out.Promoted++
		case memory.CandidateRejected:
			out.Rejected++
		default:
			out.Pending++
		}
	}
	if out.Pending > 0 {
		out.Status = "pending"
	} else {
		out.Status = "settled"
	}
	return out
}

func WorkspaceNextActions(summary WorkspaceSummary) []NextAction {
	idArg := shellQuote(summary.Demand.ID)
	if summary.Memory.Pending > 0 && (summary.State == workflow.Closeout || summary.State == workflow.Completed) {
		return []NextAction{
			{Label: "Review memory candidates", Command: "devflow memory list --demand " + idArg, Reason: "Stable knowledge candidates are still pending."},
			{Label: "Promote memory candidate", Command: "devflow memory promote --demand " + idArg + " --candidate <index> --by <name>", Reason: "Promote reusable knowledge that should persist."},
			{Label: "Reject memory candidate", Command: "devflow memory reject --demand " + idArg + " --candidate <index> --by <name> --reason <reason>", Reason: "Reject candidates that should remain one-time material."},
		}
	}
	if summary.State == workflow.Verification {
		if summary.Verification.Status == "pass" {
			return []NextAction{{Label: "Confirm verification", Command: "devflow confirm --demand " + idArg + " --stage verification --by <name> --summary <summary>", Reason: "PASS evidence is present and needs human confirmation."}}
		}
		if summary.Verification.Status == "fail" {
			return []NextAction{{Label: "Retry implementation", Command: "devflow run --demand " + idArg + " --stage implementation --permission-mode acceptEdits --quality-command \"go test ./...\"", Reason: "Verification evidence failed; fix implementation before confirmation."}}
		}
	}
	if summary.State == workflow.MRReview && summary.MergeRequest.Status == "cleared" {
		return []NextAction{{Label: "Draft verification", Command: "devflow run --demand " + idArg + " --stage verification --quality-command \"go test ./...\"", Reason: "MR review is clear and verification evidence should be generated."}}
	}
	return NextActions(summary.State, summary.Demand.ID)
}

func workspaceAttention(summary WorkspaceSummary, eventsErr error) string {
	if eventsErr != nil {
		return "events error"
	}
	if summary.State == workflow.FailedQualityGate {
		return "quality gate failed"
	}
	if summary.State == workflow.ReturnedToRequirements {
		return "returned to requirements"
	}
	if summary.State == workflow.ReturnedToPlan {
		return "returned to plan"
	}
	if summary.State == workflow.MRReview {
		if summary.MergeRequest.Status == "cleared" {
			return "ready for verification"
		}
		return "needs MR review gate"
	}
	if summary.State == workflow.Verification {
		switch summary.Verification.Status {
		case "pass":
			return "ready to confirm verification"
		case "fail":
			return "verification failed"
		default:
			return "needs verification evidence"
		}
	}
	if summary.Memory.Pending > 0 {
		return "memory candidates pending"
	}
	if summary.State == workflow.Closeout {
		return "ready for closeout"
	}
	if summary.State == workflow.Completed {
		return "complete"
	}
	actions := WorkspaceNextActions(summary)
	if len(actions) > 0 {
		return actions[0].Reason
	}
	return "inspect manually"
}

func stageStatus(summary WorkspaceSummary, name string) string {
	for _, stage := range summary.Stages {
		if stage.Name == name {
			return stage.Status
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ListWorkspaces(root string) ([]WorkspaceSummary, error) {
	base := filepath.Join(root, ".devflow", "demands")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]WorkspaceSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		if strings.TrimSpace(id) == "" || strings.ContainsAny(id, `/\`) {
			continue
		}
		summary, err := InspectWorkspace(root, id)
		if err != nil {
			continue
		}
		out = append(out, summary)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := workspacePriority(out[i])
		right := workspacePriority(out[j])
		if left != right {
			return left < right
		}
		return out[i].Demand.ID < out[j].Demand.ID
	})
	return out, nil
}

func workspacePriority(summary WorkspaceSummary) int {
	switch summary.State {
	case workflow.BlockedNeedUser, workflow.BlockedNeedPlatform, workflow.FailedQualityGate, workflow.ReturnedToRequirements, workflow.ReturnedToPlan:
		return 0
	case workflow.MRReview, workflow.Verification, workflow.Closeout:
		return 1
	case workflow.RequirementsReview, workflow.PlanReview, workflow.Implementation, workflow.RequirementsDrafting, workflow.PlanDrafting, workflow.Created, workflow.ContextLoaded:
		return 2
	case workflow.Completed:
		if summary.Memory.Pending > 0 {
			return 1
		}
		return 3
	default:
		return 4
	}
}
```

- [ ] **Step 4: Delegate `InspectStatus` to `InspectWorkspace`**

In `internal/demandflow/status.go`, replace `InspectStatus` with:

```go
func InspectStatus(root, demandID string) (StatusReport, error) {
	summary, err := InspectWorkspace(root, demandID)
	if err != nil {
		return StatusReport{}, err
	}
	artifacts := make([]ArtifactInfo, 0, len(summary.Artifacts))
	for _, artifact := range summary.Artifacts {
		artifacts = append(artifacts, ArtifactInfo{
			Name:   artifact.Name,
			Path:   artifact.Path,
			Exists: artifact.Exists,
			Size:   artifact.Size,
		})
	}
	return StatusReport{
		Demand:    summary.Demand,
		State:     summary.State,
		DemandDir: summary.DemandDir,
		Artifacts: artifacts,
		Actions:   summary.Actions,
	}, nil
}
```

Leave `NextActions`, `ArtifactInfo`, and `StatusReport` in place.

- [ ] **Step 5: Run workspace tests**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectWorkspace" -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] **Step 6: Run demandflow package tests**

Run:

```powershell
go test ./internal/demandflow -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] **Step 7: Commit Task 2**

Run:

```powershell
git add internal/demandflow/workspace.go internal/demandflow/workspace_test.go internal/demandflow/status.go
git commit -m "Summarize demand workspace evidence" -m "Wave 13 needs a reusable operator summary for CLI status, next, and a future TUI. The summary derives stage, artifact, MR, verification, and memory status from existing demand artifacts and events without adding another source of truth." -m "Constraint: demand.json remains the workflow state authority." -m "Rejected: Put event parsing and memory counting in CLI | would make future surfaces repeat business logic." -m "Confidence: medium" -m "Scope-risk: moderate" -m "Tested: go test ./internal/demandflow -count=1"
```

## Task 3: Add Evidence-Aware Next Regression Coverage

**Files:**
- Modify: `internal/demandflow/status_test.go`

- [ ] **Step 1: Add failing regression tests for evidence-aware `InspectStatus`**

Append these tests to `internal/demandflow/status_test.go`:

```go
func TestInspectStatusVerificationPassPrefersConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand, err := store.CreateDemand(artifacts.CreateDemandOptions{
		ID:          "status-pass-next",
		Title:       "Status pass next",
		Description: "Pass evidence next action",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.UpdateState(demand.ID, string(workflow.Verification)); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 2, 0, 0, 0, time.UTC), Type: "verification.recorded", Message: "pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	report, err := InspectStatus(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectStatus returned error: %v", err)
	}
	if len(report.Actions) == 0 {
		t.Fatal("InspectStatus returned no actions")
	}
	if report.Actions[0].Label != "Confirm verification" {
		t.Fatalf("first action = %#v, want Confirm verification", report.Actions[0])
	}
}

func TestInspectStatusCompletedWithPendingMemoryPrefersMemoryReview(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand, err := store.CreateDemand(artifacts.CreateDemandOptions{
		ID:          "status-memory-next",
		Title:       "Status memory next",
		Description: "Memory next action",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.UpdateState(demand.ID, string(workflow.Completed)); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}
	if err := store.AppendToArtifact(demand.ID, artifacts.MemoryCandidatesFile, "\n- Reuse this product rule\n"); err != nil {
		t.Fatalf("AppendToArtifact returned error: %v", err)
	}

	report, err := InspectStatus(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectStatus returned error: %v", err)
	}
	if len(report.Actions) == 0 {
		t.Fatal("InspectStatus returned no actions")
	}
	if report.Actions[0].Label != "Review memory candidates" {
		t.Fatalf("first action = %#v, want Review memory candidates", report.Actions[0])
	}
}
```

Ensure the file imports `time` if it does not already. It already imports `artifacts` and `workflow`; keep imports sorted by `gofmt`.

- [ ] **Step 2: Run the regression tests**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectStatusVerificationPassPrefersConfirmation|TestInspectStatusCompletedWithPendingMemoryPrefersMemoryReview" -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] **Step 3: Run all demandflow tests**

Run:

```powershell
go test ./internal/demandflow -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
```

- [ ] **Step 4: Commit Task 3**

Run:

```powershell
git add internal/demandflow/status_test.go
git commit -m "Lock evidence-aware next actions" -m "The state-only next action table remains useful as a fallback, but InspectStatus now has enough workspace evidence to recommend confirmation or memory review at the right time. These tests protect that operator-facing behavior." -m "Constraint: devflow next must keep printing one scriptable command." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/demandflow -count=1"
```

## Task 4: Enhance CLI Status and Add `status --all`

**Files:**
- Modify: `internal/cli/status.go`
- Modify: `internal/cli/status_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Add failing CLI tests**

Add these tests to `internal/cli/status_test.go`:

```go
func TestRunStatusPrintsWorkspaceSummary(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand, err := store.CreateDemand(artifacts.CreateDemandOptions{
		ID:          "cli-workspace",
		Title:       "CLI workspace",
		Description: "Show summary",
		Source:      "test",
	})
	if err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.UpdateState(demand.ID, string(workflow.Verification)); err != nil {
		t.Fatalf("UpdateState returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 3, 0, 0, 0, time.UTC), Type: "stage.confirmed", Message: "requirements confirmed", Data: map[string]string{"stage": "requirements"}}); err != nil {
		t.Fatalf("AppendEvent requirements returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Time: time.Date(2026, 6, 30, 3, 1, 0, 0, time.UTC), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent verification returned error: %v", err)
	}

	var out strings.Builder
	if err := runStatus([]string{"--root", root, "--demand", demand.ID}, &out); err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	got := out.String()
	wantParts := []string{
		"Demand: cli-workspace",
		"Stage summary:",
		"requirements   confirmed",
		"verification   passed",
		"Artifacts:",
		"requirements.md",
		"Verification:",
		"latest: PASS go test ./...",
		"Memory:",
		"Next:",
		"devflow confirm --demand cli-workspace --stage verification",
	}
	for _, want := range wantParts {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
}

func TestRunStatusAllPrintsSortedWorkspaceList(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	completed, err := store.CreateDemand(artifacts.CreateDemandOptions{ID: "z-complete", Title: "Z complete", Description: "Done", Source: "test"})
	if err != nil {
		t.Fatalf("CreateDemand completed returned error: %v", err)
	}
	if err := store.UpdateState(completed.ID, string(workflow.Completed)); err != nil {
		t.Fatalf("UpdateState completed returned error: %v", err)
	}
	failed, err := store.CreateDemand(artifacts.CreateDemandOptions{ID: "a-failed", Title: "A failed", Description: "Failed", Source: "test"})
	if err != nil {
		t.Fatalf("CreateDemand failed returned error: %v", err)
	}
	if err := store.UpdateState(failed.ID, string(workflow.FailedQualityGate)); err != nil {
		t.Fatalf("UpdateState failed returned error: %v", err)
	}
	verify, err := store.CreateDemand(artifacts.CreateDemandOptions{ID: "b-verify", Title: "B verify", Description: "Verify", Source: "test"})
	if err != nil {
		t.Fatalf("CreateDemand verify returned error: %v", err)
	}
	if err := store.UpdateState(verify.ID, string(workflow.Verification)); err != nil {
		t.Fatalf("UpdateState verify returned error: %v", err)
	}

	var out strings.Builder
	if err := runStatus([]string{"--root", root, "--all"}, &out); err != nil {
		t.Fatalf("runStatus --all returned error: %v", err)
	}
	got := out.String()
	failedIndex := strings.Index(got, "a-failed")
	verifyIndex := strings.Index(got, "b-verify")
	completedIndex := strings.Index(got, "z-complete")
	if failedIndex < 0 || verifyIndex < 0 || completedIndex < 0 {
		t.Fatalf("status --all output missing demand rows:\n%s", got)
	}
	if !(failedIndex < verifyIndex && verifyIndex < completedIndex) {
		t.Fatalf("status --all output not sorted by attention priority:\n%s", got)
	}
	if !strings.Contains(got, "quality gate failed") || !strings.Contains(got, "needs verification evidence") {
		t.Fatalf("status --all output missing attention:\n%s", got)
	}
}

func TestRunStatusAllEmptyWorkspace(t *testing.T) {
	root := t.TempDir()
	var out strings.Builder
	if err := runStatus([]string{"--root", root, "--all"}, &out); err != nil {
		t.Fatalf("runStatus --all returned error: %v", err)
	}
	if !strings.Contains(out.String(), "No demands found") {
		t.Fatalf("empty status --all output = %q, want No demands found", out.String())
	}
}
```

Ensure imports include `time`, `strings`, `artifacts`, and `workflow` once.

- [ ] **Step 2: Run failing CLI tests**

Run:

```powershell
go test ./internal/cli -run "TestRunStatusPrintsWorkspaceSummary|TestRunStatusAllPrintsSortedWorkspaceList|TestRunStatusAllEmptyWorkspace" -count=1
```

Expected result before implementation:

```text
FAIL
flag provided but not defined: -all
```

- [ ] **Step 3: Implement CLI parsing and formatting**

Replace `internal/cli/status.go` with this implementation:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runStatus(args []string, stdout io.Writer) error {
	opts, err := parseStatusArgs(args)
	if err != nil {
		return err
	}
	if opts.all {
		summaries, err := demandflow.ListWorkspaces(opts.root)
		if err != nil {
			return err
		}
		printWorkspaceList(stdout, summaries)
		return nil
	}
	summary, err := demandflow.InspectWorkspace(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	printWorkspaceDetail(stdout, summary)
	return nil
}

func runNext(args []string, stdout io.Writer) error {
	opts, err := parseDemandLookupArgs("next", args)
	if err != nil {
		return err
	}
	summary, err := demandflow.InspectWorkspace(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	if len(summary.Actions) == 0 || strings.TrimSpace(summary.Actions[0].Command) == "" {
		fmt.Fprintf(stdout, "No next command for %s in state %s\n", summary.Demand.ID, summary.State)
		return nil
	}
	fmt.Fprintln(stdout, summary.Actions[0].Command)
	return nil
}

type statusArgs struct {
	root     string
	demandID string
	all      bool
}

func parseStatusArgs(args []string) (statusArgs, error) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts statusArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.BoolVar(&opts.all, "all", false, "list all demands")
	if err := fs.Parse(args); err != nil {
		return statusArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.all {
		if opts.demandID != "" {
			return statusArgs{}, fmt.Errorf("--all cannot be combined with --demand")
		}
		return opts, nil
	}
	if opts.demandID == "" {
		return statusArgs{}, fmt.Errorf("--demand is required")
	}
	return opts, nil
}

type demandLookupArgs struct {
	root     string
	demandID string
}

func parseDemandLookupArgs(name string, args []string) (demandLookupArgs, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts demandLookupArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	if err := fs.Parse(args); err != nil {
		return demandLookupArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.demandID == "" {
		return demandLookupArgs{}, fmt.Errorf("--demand is required")
	}
	return opts, nil
}

func printWorkspaceDetail(stdout io.Writer, summary demandflow.WorkspaceSummary) {
	fmt.Fprintf(stdout, "Demand: %s\n", summary.Demand.ID)
	fmt.Fprintf(stdout, "Title: %s\n", summary.Demand.Title)
	fmt.Fprintf(stdout, "State: %s\n", summary.State)
	fmt.Fprintf(stdout, "Attention: %s\n", summary.Attention)
	fmt.Fprintf(stdout, "Directory: %s\n\n", summary.DemandDir)

	fmt.Fprintln(stdout, "Stage summary:")
	for _, stage := range summary.Stages {
		fmt.Fprintf(stdout, "  %-14s %s\n", stage.Name, humanStatus(stage.Status))
	}

	fmt.Fprintln(stdout, "\nArtifacts:")
	for _, artifact := range summary.Artifacts {
		detail := humanStatus(artifact.Status)
		if len(artifact.Notes) > 0 {
			detail += ", " + strings.Join(artifact.Notes, ", ")
		}
		if artifact.Error != "" {
			detail += ", " + artifact.Error
		}
		fmt.Fprintf(stdout, "  %-22s %s\n", artifact.Name, detail)
	}

	fmt.Fprintln(stdout, "\nMR:")
	mrLine := humanStatus(summary.MergeRequest.Status)
	if summary.MergeRequest.Reference != "" {
		mrLine = summary.MergeRequest.Reference + " " + mrLine
	}
	fmt.Fprintf(stdout, "  %s\n", mrLine)
	if summary.MergeRequest.Evidence != "" {
		fmt.Fprintf(stdout, "  evidence: %s\n", summary.MergeRequest.Evidence)
	}

	fmt.Fprintln(stdout, "\nVerification:")
	switch summary.Verification.Status {
	case "pass":
		fmt.Fprintf(stdout, "  latest: PASS %s\n", summary.Verification.Command)
	case "fail":
		fmt.Fprintf(stdout, "  latest: FAIL %s\n", summary.Verification.Command)
		if summary.Verification.FailureKind != "" {
			fmt.Fprintf(stdout, "  failure_kind: %s\n", summary.Verification.FailureKind)
		}
	default:
		fmt.Fprintln(stdout, "  latest: none")
	}

	fmt.Fprintln(stdout, "\nMemory:")
	if summary.Memory.Error != "" && summary.Memory.Status == "none" {
		fmt.Fprintln(stdout, "  candidates: none")
	} else {
		fmt.Fprintf(stdout, "  candidates: %d pending, %d promoted, %d rejected\n", summary.Memory.Pending, summary.Memory.Promoted, summary.Memory.Rejected)
	}

	fmt.Fprintln(stdout, "\nNext:")
	for _, action := range summary.Actions {
		fmt.Fprintf(stdout, "  - %s: %s\n", action.Label, action.Reason)
		if strings.TrimSpace(action.Command) != "" {
			fmt.Fprintf(stdout, "    %s\n", action.Command)
		}
	}
}

func printWorkspaceList(stdout io.Writer, summaries []demandflow.WorkspaceSummary) {
	if len(summaries) == 0 {
		fmt.Fprintln(stdout, "No demands found")
		return
	}
	fmt.Fprintln(stdout, "Demand status:")
	for _, summary := range summaries {
		fmt.Fprintf(stdout, "  %-24s %-22s %s\n", summary.Demand.ID, summary.State, summary.Attention)
	}
}

func humanStatus(status string) string {
	return strings.ReplaceAll(status, "_", " ")
}
```

- [ ] **Step 4: Update CLI help**

In `internal/cli/cli.go`, update the help text line for status from:

```text
devflow status --demand <id>
```

to:

```text
devflow status --demand <id>
devflow status --all
```

Keep `devflow next --demand <id>` unchanged.

- [ ] **Step 5: Run CLI tests**

Run:

```powershell
go test ./internal/cli -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 6: Run demandflow and CLI together**

Run:

```powershell
go test ./internal/demandflow ./internal/cli -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 7: Commit Task 4**

Run:

```powershell
git add internal/cli/status.go internal/cli/status_test.go internal/cli/cli.go
git commit -m "Turn status into a demand workspace view" -m "Operators need one command that explains where a demand is stuck and one scriptable next command. The CLI now renders the shared WorkspaceSummary for detail views and lists all demands with attention reasons." -m "Constraint: Keep the command surface to status and next; do not introduce an inspect command." -m "Rejected: Print raw JSON summary | harder for the current CLI-first operator loop." -m "Confidence: medium" -m "Scope-risk: moderate" -m "Tested: go test ./internal/demandflow ./internal/cli -count=1"
```

## Task 5: Document the Operator Loop

**Files:**
- Modify: `docs/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Update backend demand loop docs**

In `docs/backend-demand-loop.md`, add this section near the CLI workflow description:

```markdown
### Demand Workspace Status

Use `devflow status` as the operator checkpoint before deciding the next command.

```powershell
devflow status --demand add-coupon-check
devflow next --demand add-coupon-check
devflow status --all
```

`status --demand` reads only local demand materials under `.devflow/demands/<id>` and summarizes:

- workflow state from `demand.json`;
- confirmation evidence from `events.jsonl`;
- artifact state for requirements, plan, progress, verification, closeout, memory candidates, and events;
- local MR review evidence from events and `progress.md`;
- latest verification PASS/FAIL evidence;
- stable memory candidate counts;
- the recommended next command.

`status --all` scans `.devflow/demands` and sorts demands that need attention ahead of completed work. It does not call GitLab and does not mutate any artifact.
```

If the file already has a status section, replace that section with the text above instead of duplicating it.

- [ ] **Step 2: Update release notes**

In `docs/release/v0.1.md`, add this bullet under the unreleased or Wave 13 section:

```markdown
- Wave 13 adds a demand workspace operator loop: `devflow status --demand` now shows stage, artifact, MR, verification, memory, and next-action evidence, while `devflow status --all` lists all local demands by attention priority.
```

If there is no Wave 13 heading, add:

```markdown
### Wave 13 - Demand Workspace Operator Loop

- `devflow status --demand <id>` now renders an evidence-aware workspace summary.
- `devflow status --all` lists local demands by attention priority.
- `devflow next --demand <id>` now prefers evidence-aware commands such as confirming PASS verification or reviewing pending memory candidates.
```

- [ ] **Step 3: Run doc smoke checks**

Run:

```powershell
rg -n "status --all|Demand Workspace|Wave 13" docs
git diff --check
```

Expected result:

```text
docs/backend-demand-loop.md:<line>:### Demand Workspace Status
docs/release/v0.1.md:<line>:### Wave 13 - Demand Workspace Operator Loop
```

`git diff --check` must exit with code 0.

- [ ] **Step 4: Commit Task 5**

Run:

```powershell
git add docs/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document the demand workspace loop" -m "Wave 13 changes the operator flow from remembering separate commands to checking status, next, and all-demand attention lists. The docs now explain the local-only evidence model and command usage." -m "Constraint: Documentation must state that status is read-only and does not call GitLab." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: rg -n \"status --all|Demand Workspace|Wave 13\" docs; git diff --check"
```

## Task 6: Full Verification, PR, and Merge Gate

**Files:**
- No planned source edits.
- Possible generated changes only from `gofmt`.

- [ ] **Step 1: Format changed Go files**

Run:

```powershell
gofmt -w internal/artifacts/store.go internal/artifacts/store_test.go internal/demandflow/status.go internal/demandflow/status_test.go internal/demandflow/workspace.go internal/demandflow/workspace_test.go internal/cli/status.go internal/cli/status_test.go internal/cli/cli.go
```

- [ ] **Step 2: Run package verification**

Run:

```powershell
go test ./internal/artifacts -count=1
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
```

Expected result:

```text
ok  	github.com/jesseedcp/devflow-agent/internal/artifacts
ok  	github.com/jesseedcp/devflow-agent/internal/demandflow
ok  	github.com/jesseedcp/devflow-agent/internal/cli
```

- [ ] **Step 3: Run full repository verification**

Run:

```powershell
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git diff --check
```

Expected result:

```text
go vet exits 0
go build exits 0
go test exits 0
git diff --check exits 0
```

- [ ] **Step 4: Manual smoke test on a temporary root**

Run:

```powershell
$tmp = Join-Path $env:TEMP ("devflow-wave13-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tmp | Out-Null
go run ./cmd/devflow create --root $tmp --id wave13-smoke --title "Wave 13 smoke" --description "Check operator loop"
go run ./cmd/devflow status --root $tmp --demand wave13-smoke
go run ./cmd/devflow status --root $tmp --all
go run ./cmd/devflow next --root $tmp --demand wave13-smoke
```

Expected output contains:

```text
Demand: wave13-smoke
Stage summary:
Artifacts:
Next:
Demand status:
devflow run --demand wave13-smoke --stage requirements
```

- [ ] **Step 5: Commit formatting-only changes if present**

Run:

```powershell
git status --short
```

If only formatting changes remain, commit them:

```powershell
git add internal/artifacts/store.go internal/artifacts/store_test.go internal/demandflow/status.go internal/demandflow/status_test.go internal/demandflow/workspace.go internal/demandflow/workspace_test.go internal/cli/status.go internal/cli/status_test.go internal/cli/cli.go
git commit -m "Normalize Wave 13 Go formatting" -m "The final verification pass ran gofmt across Wave 13 touched files so generated diffs are stable before PR review." -m "Constraint: Formatting only; no behavior changes." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go vet ./...; go build ./cmd/devflow; go test ./... -count=1 -timeout 5m; git diff --check"
```

If `git status --short` is clean, do not create an empty commit.

- [ ] **Step 6: Review the diff**

Run:

```powershell
git diff --stat main...HEAD
git diff main...HEAD -- internal/demandflow internal/cli internal/artifacts docs
```

Check these points:

- `status` reads local data only.
- `status --all` does not fail the whole command because one demand has missing memory candidates.
- `next` remains one-line command output.
- No GitLab API call was introduced.
- No workflow state transition was added.

- [ ] **Step 7: Push and open PR**

Run:

```powershell
git push -u origin HEAD
gh pr create --base main --head HEAD --title "Wave 13 demand workspace operator loop" --body "## Summary`n- Adds WorkspaceSummary for local demand evidence`n- Enhances status/next with evidence-aware actions`n- Adds status --all attention list`n`n## Verification`n- go vet ./...`n- go build ./cmd/devflow`n- go test ./... -count=1 -timeout 5m`n- git diff --check"
```

If `gh pr create --head HEAD` rejects the branch name, run:

```powershell
$branch = git branch --show-current
gh pr create --base main --head $branch --title "Wave 13 demand workspace operator loop" --body "## Summary`n- Adds WorkspaceSummary for local demand evidence`n- Enhances status/next with evidence-aware actions`n- Adds status --all attention list`n`n## Verification`n- go vet ./...`n- go build ./cmd/devflow`n- go test ./... -count=1 -timeout 5m`n- git diff --check"
```

- [ ] **Step 8: Merge only after CI is green**

After PR CI passes on Ubuntu and Windows, use `superpowers:finishing-a-development-branch`.

Recommended merge path:

```powershell
git switch main
git pull --ff-only origin main
git merge --no-ff <wave-13-branch>
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git push origin main
```

Clean the Wave 13 worktree and branch only after the merge is pushed.

## Self-Review Checklist

- Spec coverage:
  - Single-demand workspace summary: Task 2 and Task 4.
  - `status --all`: Task 2 `ListWorkspaces`, Task 4 CLI.
  - Evidence-aware next: Task 2 `WorkspaceNextActions`, Task 3 regressions, Task 4 `runNext`.
  - Artifact status: Task 2 `summarizeArtifacts`.
  - Confirmation evidence: Task 2 `confirmedStages`.
  - Verification PASS/FAIL evidence: Task 2 `summarizeVerification`.
  - MR local evidence: Task 2 `summarizeMergeRequest`.
  - Memory counts: Task 2 `summarizeMemory`.
  - Docs and release notes: Task 5.
- Placeholder scan:
  - The plan contains concrete tests, commands, and implementation snippets without deferred-work markers.
- Type consistency:
  - `WorkspaceSummary`, `StageSummary`, `ArtifactSummary`, `VerificationSummary`, `MergeRequestSummary`, `MemorySummary`, and `WorkspaceNextActions` are defined before use.
  - CLI uses `demandflow.InspectWorkspace` and `demandflow.ListWorkspaces`.
  - Compatibility wrapper `InspectStatus` remains available for existing callers.

## Execution Notes

Use a feature branch or isolated worktree for implementation:

```powershell
git switch main
git pull --ff-only origin main
git switch -c feature/devflow-wave-13
```

If the branch already exists:

```powershell
git switch feature/devflow-wave-13
git pull --ff-only origin feature/devflow-wave-13
```

Do not edit MewCode runtime code for Wave 13. This wave only changes `devflow-agent` and keeps the MewCode integration boundary at local artifacts and future reusable summaries.
