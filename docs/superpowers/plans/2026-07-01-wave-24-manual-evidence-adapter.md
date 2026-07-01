# Wave 24 Manual Evidence Adapter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a local manual evidence adapter so operators can record API/log/monitor/manual acceptance evidence against a demand without bypassing verification gates.

**Architecture:** Keep evidence as local demand material, not a new workflow state. `internal/demandflow` owns evidence recording, rendering, and workspace summaries; `internal/cli` exposes `devflow evidence add/list`; existing `status`, `console`, `workbench`, and `evaluate` surfaces read the same events and `verification.md`.

**Tech Stack:** Go standard library only, existing `internal/artifacts`, `internal/demandflow`, `internal/cli`, deterministic Markdown/event storage, PowerShell release readiness.

---

## Scope

Wave 24 implements local manual acceptance evidence only.

In scope:
- `devflow evidence add --demand <id> --type <api|log|monitor|manual|link> --criterion <text> --status <pass|fail|blocked> --summary <text> [--link <url>] [--by <name>]`
- `devflow evidence list --demand <id>`
- append evidence to `verification.md`
- append evidence event to `events.jsonl`
- surface evidence counts/latest items in `status`, `console`, and `workbench --snapshot`
- add deterministic verification evaluation checks for manual evidence presence and failed/blocked evidence
- release-readiness smoke and docs

Out of scope:
- no log platform adapter
- no monitoring query adapter
- no screenshots/files upload
- no LLM semantic evidence scoring
- no automatic verification confirmation
- no new workflow state

## File Structure

- Create `internal/demandflow/evidence.go`: evidence input validation, append-to-verification rendering, event writing, event summarization.
- Create `internal/demandflow/evidence_test.go`: unit tests for add/list/validation/rendering.
- Modify `internal/demandflow/workspace.go`: add `Evidence EvidenceSummary` to `WorkspaceSummary`, summarize evidence events, and prefer evidence-add next action when technical verification has passed but no manual evidence exists.
- Modify `internal/demandflow/workspace_test.go`: cover evidence summary and next-action ordering.
- Modify `internal/demandflow/evaluation.go`: add verification checks for manual evidence presence and failed/blocked manual evidence.
- Modify `internal/demandflow/evaluation_test.go`: cover new verification checks.
- Create `internal/cli/evidence.go`: parse `devflow evidence add/list` commands.
- Create `internal/cli/evidence_test.go`: CLI integration tests.
- Modify `internal/cli/cli.go`: route `evidence` command and update help text.
- Modify `internal/cli/status.go`: show evidence counts/latest manual evidence in `devflow status`.
- Modify `internal/cli/console.go`: show evidence counts/latest manual evidence in console Evidence section.
- Modify `internal/cli/workbench_snapshot.go` and `internal/cli/workbench_model.go`: show evidence in snapshot and detail view.
- Modify `internal/cli/status_test.go`, `internal/cli/console_test.go`, `internal/cli/workbench_test.go`: output coverage.
- Modify `scripts/release-readiness.ps1`: add manual evidence smoke inside deterministic release readiness.
- Modify `docs/user-guide/backend-demand-loop.md` and `docs/release/v0.1.md`: document Wave 24.

## Data Contract

Manual evidence is stored as `verification.evidence_recorded` events.

Event data fields:

```text
type       api | log | monitor | manual | link
criterion  acceptance criterion or business rule being proven
status     pass | fail | blocked
summary    concise evidence summary
link       optional URL/path/reference
by         optional recorder
evidence_file verification.md
```

`verification.md` gets an append-only section:

```markdown
## Manual Acceptance Evidence

- [PASS] api - Inactive users are blocked
  Summary: POST /coupon/claim returned COUPON_USER_INACTIVE.
  Link: https://example.test/log/123
  By: dd
```

Evidence does not replace technical verification. `verification.recorded` from `devflow verify` remains the technical PASS/FAIL gate. Manual evidence adds business proof and affects operator guidance.

## Task 1: Demandflow Evidence Core

**Files:**
- Create: `internal/demandflow/evidence.go`
- Create: `internal/demandflow/evidence_test.go`

- [ ] **Step 1: Write the failing demandflow evidence tests**

Create `internal/demandflow/evidence_test.go`:

```go
package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestAddManualEvidenceAppendsVerificationAndEvent(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-demand", Title: "Evidence demand", Description: "Verify external behavior", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.VerificationFile, "# Verification: Evidence demand\n\n"); err != nil {
		t.Fatalf("WriteArtifact verification returned error: %v", err)
	}

	record, err := AddManualEvidence(AddManualEvidenceOptions{
		Root:      root,
		DemandID:  demand.ID,
		Type:      "api",
		Criterion: "Inactive users are blocked",
		Status:    "pass",
		Summary:   "POST /coupon/claim returned COUPON_USER_INACTIVE.",
		Link:      "https://example.test/log/123",
		By:        "dd",
	})
	if err != nil {
		t.Fatalf("AddManualEvidence returned error: %v", err)
	}
	if record.Status != "pass" || record.Type != "api" {
		t.Fatalf("record = %#v, want pass api", record)
	}

	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.VerificationFile))
	if err != nil {
		t.Fatalf("read verification: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"## Manual Acceptance Evidence",
		"[PASS] api - Inactive users are blocked",
		"POST /coupon/claim returned COUPON_USER_INACTIVE.",
		"Link: https://example.test/log/123",
		"By: dd",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("verification.md missing %q:\n%s", want, text)
		}
	}

	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	event := findEvidenceTestEvent(events, "verification.evidence_recorded")
	if event == nil {
		t.Fatalf("events missing verification.evidence_recorded: %#v", events)
	}
	if event.Data["status"] != "pass" || event.Data["criterion"] != "Inactive users are blocked" || event.Data["evidence_file"] != artifacts.VerificationFile {
		t.Fatalf("event data = %#v", event.Data)
	}
}

func TestAddManualEvidenceRequiresVerificationState(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-wrong-state", Title: "Wrong state", Description: "Wrong state", Source: "test", State: string(workflow.PlanReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	_, err := AddManualEvidence(AddManualEvidenceOptions{
		Root:      root,
		DemandID:  demand.ID,
		Type:      "api",
		Criterion: "Inactive users are blocked",
		Status:    "pass",
		Summary:   "Observed expected error code.",
	})
	if err == nil || !strings.Contains(err.Error(), "requires current state verification") {
		t.Fatalf("err = %v, want verification state error", err)
	}
}

func TestAddManualEvidenceValidatesRequiredFields(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-validation", Title: "Validation", Description: "Validation", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	_, err := AddManualEvidence(AddManualEvidenceOptions{
		Root:     root,
		DemandID: demand.ID,
		Type:     "spreadsheet",
		Status:   "maybe",
	})
	if err == nil || !strings.Contains(err.Error(), "--type must be one of") {
		t.Fatalf("err = %v, want type validation", err)
	}
}

func TestListManualEvidenceReturnsEventsInOrder(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-list", Title: "Evidence list", Description: "List evidence", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	for _, opts := range []AddManualEvidenceOptions{
		{Root: root, DemandID: demand.ID, Type: "api", Criterion: "Active user succeeds", Status: "pass", Summary: "200 OK"},
		{Root: root, DemandID: demand.ID, Type: "log", Criterion: "Inactive user blocked", Status: "blocked", Summary: "Waiting for log access"},
	} {
		if _, err := AddManualEvidence(opts); err != nil {
			t.Fatalf("AddManualEvidence returned error: %v", err)
		}
	}

	records, err := ListManualEvidence(root, demand.ID)
	if err != nil {
		t.Fatalf("ListManualEvidence returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].Criterion != "Active user succeeds" || records[1].Status != "blocked" {
		t.Fatalf("records = %#v", records)
	}
}

func findEvidenceTestEvent(events []artifacts.Event, eventType string) *artifacts.Event {
	for index := range events {
		if events[index].Type == eventType {
			return &events[index]
		}
	}
	return nil
}
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
go test ./internal/demandflow -run "TestAddManualEvidence|TestListManualEvidence" -count=1
```

Expected: compile failure with undefined `AddManualEvidence`, `AddManualEvidenceOptions`, and `ListManualEvidence`.

- [ ] **Step 3: Implement the demandflow evidence core**

Create `internal/demandflow/evidence.go`:

```go
package demandflow

import (
	"fmt"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type EvidenceRecord struct {
	Type      string
	Criterion string
	Status    string
	Summary   string
	Link      string
	By        string
}

type AddManualEvidenceOptions struct {
	Root      string
	DemandID  string
	Type      string
	Criterion string
	Status    string
	Summary   string
	Link      string
	By        string
	Now       func() time.Time
}

func AddManualEvidence(opts AddManualEvidenceOptions) (EvidenceRecord, error) {
	record, err := normalizeEvidenceRecord(opts)
	if err != nil {
		return EvidenceRecord{}, err
	}
	store := artifacts.NewStore(opts.Root)
	err = store.WithDemandLock(recordDemandID(opts.DemandID), func() error {
		demand, err := store.LoadDemand(recordDemandID(opts.DemandID))
		if err != nil {
			return err
		}
		if workflow.State(demand.State) != workflow.Verification {
			return fmt.Errorf("evidence add requires current state %s, got %s", workflow.Verification, demand.State)
		}
		if err := store.AppendToArtifact(demand.ID, artifacts.VerificationFile, renderManualEvidence(record)); err != nil {
			return err
		}
		now := time.Now().UTC()
		if opts.Now != nil {
			now = opts.Now().UTC()
		}
		return store.AppendEvent(demand.ID, artifacts.Event{
			Time:    now,
			Type:    "verification.evidence_recorded",
			Message: "manual verification evidence recorded",
			Data: map[string]string{
				"type":          record.Type,
				"criterion":     record.Criterion,
				"status":        record.Status,
				"summary":       record.Summary,
				"link":          record.Link,
				"by":            record.By,
				"evidence_file": artifacts.VerificationFile,
			},
		})
	})
	if err != nil {
		return EvidenceRecord{}, err
	}
	return record, nil
}

func ListManualEvidence(root, demandID string) ([]EvidenceRecord, error) {
	store := artifacts.NewStore(root)
	events, err := store.ReadEvents(strings.TrimSpace(demandID))
	if err != nil {
		return nil, err
	}
	var out []EvidenceRecord
	for _, event := range events {
		if event.Type != "verification.evidence_recorded" {
			continue
		}
		out = append(out, EvidenceRecord{
			Type:      event.Data["type"],
			Criterion: event.Data["criterion"],
			Status:    normalizeEvidenceStatus(event.Data["status"]),
			Summary:   event.Data["summary"],
			Link:      event.Data["link"],
			By:        event.Data["by"],
		})
	}
	return out, nil
}

func normalizeEvidenceRecord(opts AddManualEvidenceOptions) (EvidenceRecord, error) {
	record := EvidenceRecord{
		Type:      normalizeEvidenceType(opts.Type),
		Criterion: strings.Join(strings.Fields(opts.Criterion), " "),
		Status:    normalizeEvidenceStatus(opts.Status),
		Summary:   strings.Join(strings.Fields(opts.Summary), " "),
		Link:      strings.TrimSpace(opts.Link),
		By:        strings.Join(strings.Fields(opts.By), " "),
	}
	if record.Type == "" {
		return EvidenceRecord{}, fmt.Errorf("--type must be one of api, log, monitor, manual, link")
	}
	if record.Status == "" {
		return EvidenceRecord{}, fmt.Errorf("--status must be one of pass, fail, blocked")
	}
	if record.Criterion == "" || record.Summary == "" {
		return EvidenceRecord{}, fmt.Errorf("--criterion and --summary are required")
	}
	return record, nil
}

func normalizeEvidenceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "api", "log", "monitor", "manual", "link":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func normalizeEvidenceStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "pass", "passed", "success", "ok":
		return "pass"
	case "fail", "failed", "failure", "error":
		return "fail"
	case "blocked", "blocker":
		return "blocked"
	default:
		return ""
	}
}

func renderManualEvidence(record EvidenceRecord) string {
	var builder strings.Builder
	builder.WriteString("\n## Manual Acceptance Evidence\n\n")
	fmt.Fprintf(&builder, "- [%s] %s - %s\n", strings.ToUpper(record.Status), record.Type, record.Criterion)
	fmt.Fprintf(&builder, "  Summary: %s\n", record.Summary)
	if record.Link != "" {
		fmt.Fprintf(&builder, "  Link: %s\n", record.Link)
	}
	if record.By != "" {
		fmt.Fprintf(&builder, "  By: %s\n", record.By)
	}
	return builder.String()
}

func recordDemandID(value string) string {
	return strings.TrimSpace(value)
}
```

- [ ] **Step 4: Run demandflow evidence tests and verify GREEN**

Run:

```powershell
gofmt -w internal/demandflow/evidence.go internal/demandflow/evidence_test.go
go test ./internal/demandflow -run "TestAddManualEvidence|TestListManualEvidence" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit demandflow evidence core**

Run:

```powershell
git add internal/demandflow/evidence.go internal/demandflow/evidence_test.go
git commit -m "Record manual verification evidence" -m "Verification needs business-facing proof beyond local command output. Demandflow now records local manual evidence as append-only verification material plus structured events." -m "Constraint: Evidence is local and deterministic; it must not advance workflow state or replace technical verification.`nRejected: External log or monitoring adapters | Wave 24 only defines the local evidence adapter seam.`nConfidence: high`nScope-risk: moderate`nDirective: Do not let manual evidence auto-confirm verification; confirmation remains a human gate.`nTested: go test ./internal/demandflow -run \"TestAddManualEvidence|TestListManualEvidence\" -count=1`nNot-tested: CLI surfaces"
```

## Task 2: CLI Evidence Command

**Files:**
- Create: `internal/cli/evidence.go`
- Create: `internal/cli/evidence_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write failing CLI tests**

Create `internal/cli/evidence_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestEvidenceAddRecordsManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-cli", Title: "Evidence CLI", Description: "CLI evidence", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{
		"evidence", "add",
		"--root", root,
		"--demand", demand.ID,
		"--type", "api",
		"--criterion", "Inactive users are blocked",
		"--status", "pass",
		"--summary", "POST /coupon/claim returned COUPON_USER_INACTIVE.",
		"--link", "https://example.test/log/123",
		"--by", "dd",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("evidence add returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "evidence recorded for evidence-cli: PASS api") {
		t.Fatalf("stdout = %q", stdout.String())
	}
	body, err := os.ReadFile(filepath.Join(store.DemandDir(demand.ID), artifacts.VerificationFile))
	if err != nil {
		t.Fatalf("read verification: %v", err)
	}
	if !strings.Contains(string(body), "[PASS] api - Inactive users are blocked") {
		t.Fatalf("verification.md missing evidence:\n%s", string(body))
	}
}

func TestEvidenceListPrintsManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "evidence-list-cli", Title: "Evidence list CLI", Description: "List", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := Run([]string{"evidence", "add", "--root", root, "--demand", demand.ID, "--type", "manual", "--criterion", "QA accepted", "--summary", "QA signed off", "--by", "dd"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("evidence add returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"evidence", "list", "--root", root, "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("evidence list returned error: %v", err)
	}
	for _, want := range []string{"Manual evidence: evidence-list-cli", "PASS manual QA accepted", "QA signed off"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("evidence list missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestEvidenceRejectsUnknownSubcommand(t *testing.T) {
	err := Run([]string{"evidence", "delete", "--root", t.TempDir(), "--demand", "x"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unknown evidence command") {
		t.Fatalf("err = %v, want unknown evidence command", err)
	}
}

func TestHelpIncludesEvidence(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	for _, want := range []string{"devflow evidence add --demand <id>", "evidence  Record and list manual verification evidence"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] **Step 2: Run the CLI tests and verify RED**

Run:

```powershell
go test ./internal/cli -run "TestEvidence|TestHelpIncludesEvidence" -count=1
```

Expected: FAIL because the `evidence` command is unknown.

- [ ] **Step 3: Implement CLI command routing**

Create `internal/cli/evidence.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runEvidence(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("evidence requires a subcommand: add or list")
	}
	switch args[0] {
	case "add":
		return runEvidenceAdd(args[1:], stdout, stderr)
	case "list":
		return runEvidenceList(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown evidence command %q", args[0])
	}
}

func runEvidenceAdd(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("evidence add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, evidenceType, criterion, status, summary, link, by string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&evidenceType, "type", "", "evidence type: api, log, monitor, manual, link")
	fs.StringVar(&criterion, "criterion", "", "acceptance criterion or business rule")
	fs.StringVar(&status, "status", "pass", "evidence status: pass, fail, blocked")
	fs.StringVar(&summary, "summary", "", "evidence summary")
	fs.StringVar(&link, "link", "", "optional URL/path/reference")
	fs.StringVar(&by, "by", "", "recorder")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	record, err := demandflow.AddManualEvidence(demandflow.AddManualEvidenceOptions{
		Root:      root,
		DemandID:  demandID,
		Type:      evidenceType,
		Criterion: criterion,
		Status:    status,
		Summary:   summary,
		Link:      link,
		By:        by,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "evidence recorded for %s: %s %s\n", demandID, strings.ToUpper(record.Status), record.Type)
	return nil
}

func runEvidenceList(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDemandLookupArgs("evidence list", args)
	if err != nil {
		return err
	}
	records, err := demandflow.ListManualEvidence(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Manual evidence: %s\n", opts.demandID)
	if len(records) == 0 {
		fmt.Fprintln(stdout, "  none")
		return nil
	}
	for _, record := range records {
		fmt.Fprintf(stdout, "  %s %s %s\n", strings.ToUpper(record.Status), record.Type, record.Criterion)
		if record.Summary != "" {
			fmt.Fprintf(stdout, "    %s\n", record.Summary)
		}
		if record.Link != "" {
			fmt.Fprintf(stdout, "    link: %s\n", record.Link)
		}
		if record.By != "" {
			fmt.Fprintf(stdout, "    by: %s\n", record.By)
		}
	}
	return nil
}

func normalizedRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "."
	}
	return root
}
```

Modify `internal/cli/cli.go`:

```go
// In help usage, add:
//   devflow evidence add --demand <id> --type <api|log|monitor|manual|link> --criterion <text> --summary <text>
//   devflow evidence list --demand <id>

// In Commands, add:
//   evidence  Record and list manual verification evidence

// In Run switch, add:
case "evidence":
	return runEvidence(args[1:], stdout, stderr)
```

- [ ] **Step 4: Run CLI tests and verify GREEN**

Run:

```powershell
gofmt -w internal/cli/evidence.go internal/cli/evidence_test.go internal/cli/cli.go
go test ./internal/cli -run "TestEvidence|TestHelpIncludesEvidence" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit CLI evidence command**

Run:

```powershell
git add internal/cli/evidence.go internal/cli/evidence_test.go internal/cli/cli.go
git commit -m "Expose manual evidence commands" -m "Operators need a CLI surface to add and inspect acceptance evidence while staying inside the verification gate. The evidence command records structured local proof and lists it without mutating workflow state." -m "Constraint: Evidence commands only write verification.md and events; they do not call external systems.`nConfidence: high`nScope-risk: moderate`nTested: go test ./internal/cli -run \"TestEvidence|TestHelpIncludesEvidence\" -count=1`nNot-tested: status, console, workbench visibility"
```

## Task 3: Workspace Summary And Operator Surfaces

**Files:**
- Modify: `internal/demandflow/workspace.go`
- Modify: `internal/demandflow/workspace_test.go`
- Modify: `internal/cli/status.go`
- Modify: `internal/cli/status_test.go`
- Modify: `internal/cli/console.go`
- Modify: `internal/cli/console_test.go`
- Modify: `internal/cli/workbench_snapshot.go`
- Modify: `internal/cli/workbench_model.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Add failing workspace summary tests**

Append to `internal/demandflow/workspace_test.go`:

```go
func TestInspectWorkspaceSummarizesManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-evidence", Title: "Workspace evidence", Description: "Evidence", Source: "test", State: string(workflow.Verification)}
	createWorkspaceDemand(t, store, demand)
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}})
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime().Add(time.Minute), Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}})

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}
	if summary.Evidence.Pass != 1 || summary.Evidence.Fail != 0 || summary.Evidence.Blocked != 0 {
		t.Fatalf("Evidence = %#v, want one pass", summary.Evidence)
	}
	if len(summary.Evidence.Latest) != 1 || summary.Evidence.Latest[0].Criterion != "Inactive users are blocked" {
		t.Fatalf("Latest evidence = %#v", summary.Evidence.Latest)
	}
}

func TestWorkspaceNextActionsPreferEvidenceBeforeVerificationConfirmation(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workspace-evidence-next", Title: "Workspace evidence next", Description: "Evidence", Source: "test", State: string(workflow.Verification)}
	createWorkspaceDemand(t, store, demand)
	appendWorkspaceEvent(t, store, demand.ID, artifacts.Event{Time: fixedWorkspaceTime(), Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}})

	summary, err := InspectWorkspace(root, demand.ID)
	if err != nil {
		t.Fatalf("InspectWorkspace returned error: %v", err)
	}
	if len(summary.Actions) == 0 || summary.Actions[0].Label != "Add acceptance evidence" {
		t.Fatalf("Actions = %#v, want Add acceptance evidence first", summary.Actions)
	}
	if !strings.Contains(summary.Actions[0].Command, "devflow evidence add --demand workspace-evidence-next") {
		t.Fatalf("first command = %q", summary.Actions[0].Command)
	}
}
```

Add imports if missing:

```go
import (
	"strings"
	"time"
)
```

- [ ] **Step 2: Run workspace tests and verify RED**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectWorkspaceSummarizesManualEvidence|TestWorkspaceNextActionsPreferEvidence" -count=1
```

Expected: FAIL because `WorkspaceSummary.Evidence` does not exist.

- [ ] **Step 3: Implement workspace evidence summary**

Modify `internal/demandflow/workspace.go`:

```go
type WorkspaceSummary struct {
	Demand       artifacts.Demand
	State        workflow.State
	DemandDir    string
	Stages       []StageSummary
	Artifacts    []ArtifactSummary
	Verification VerificationSummary
	Evidence     EvidenceSummary
	MergeRequest MergeRequestSummary
	Memory       MemorySummary
	Actions      []NextAction
	Attention    string
}

type EvidenceSummary struct {
	Pass    int
	Fail    int
	Blocked int
	Latest  []EvidenceRecord
}
```

In `InspectWorkspace`, after verification summary:

```go
summary.Verification = summarizeVerification(events)
summary.Evidence = summarizeManualEvidence(events)
```

Add helper:

```go
func summarizeManualEvidence(events []artifacts.Event) EvidenceSummary {
	var summary EvidenceSummary
	for _, event := range events {
		if event.Type != "verification.evidence_recorded" {
			continue
		}
		record := EvidenceRecord{
			Type:      event.Data["type"],
			Criterion: event.Data["criterion"],
			Status:    normalizeEvidenceStatus(event.Data["status"]),
			Summary:   event.Data["summary"],
			Link:      event.Data["link"],
			By:        event.Data["by"],
		}
		switch record.Status {
		case "pass":
			summary.Pass++
		case "fail":
			summary.Fail++
		case "blocked":
			summary.Blocked++
		}
		summary.Latest = append(summary.Latest, record)
	}
	if len(summary.Latest) > 3 {
		summary.Latest = summary.Latest[len(summary.Latest)-3:]
	}
	return summary
}
```

Modify `WorkspaceNextActions` inside the `workflow.Verification` / `pass` case:

```go
case "pass":
	if summary.Evidence.Pass == 0 && summary.Evidence.Fail == 0 && summary.Evidence.Blocked == 0 {
		return []NextAction{
			{
				Label:   "Add acceptance evidence",
				Command: "devflow evidence add --demand " + idArg + " --type manual --criterion <criterion> --summary <summary> --by <name>",
				Reason: "Technical verification passed; add business acceptance evidence before confirmation.",
			},
			{Label: "Confirm verification", Command: "devflow confirm --demand " + idArg + " --stage verification --by <name> --summary <summary>", Reason: "PASS evidence is present and needs human confirmation."},
		}
	}
	return []NextAction{{Label: "Confirm verification", Command: "devflow confirm --demand " + idArg + " --stage verification --by <name> --summary <summary>", Reason: "PASS evidence is present and needs human confirmation."}}
```

- [ ] **Step 4: Update status/console/workbench output tests first**

Add focused assertions:

`internal/cli/status_test.go`:

```go
func TestRunStatusPrintsManualEvidenceSummary(t *testing.T) {
	root := t.TempDir()
	store := setupDemandAtState(t, root, workflow.Verification)
	if err := store.AppendEvent("add-coupon-check", artifacts.Event{Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	var out bytes.Buffer
	if err := runStatus([]string{"--root", root, "--demand", "add-coupon-check"}, &out); err != nil {
		t.Fatalf("runStatus returned error: %v", err)
	}
	for _, want := range []string{"Manual evidence:", "pass=1 fail=0 blocked=0", "PASS api Inactive users are blocked"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("status output missing %q:\n%s", want, out.String())
		}
	}
}
```

If helper names differ in the current file, use existing test helpers in `status_test.go` instead of adding duplicates.

`internal/cli/console_test.go`: add a console detail test that expects:

```text
manual        pass=1 fail=0 blocked=0
```

`internal/cli/workbench_test.go`: add a snapshot/detail test that expects:

```text
Evidence:
  manual        pass=1 fail=0 blocked=0
```

Run and expect RED until rendering is added:

```powershell
go test ./internal/cli -run "TestRunStatusPrintsManualEvidenceSummary|TestConsole|TestWorkbench" -count=1
```

- [ ] **Step 5: Implement operator output**

Modify `internal/cli/status.go`, inside `printWorkspaceDetail` after the existing `Verification:` block:

```go
fmt.Fprintln(stdout, "\nManual evidence:")
fmt.Fprintf(stdout, "  pass=%d fail=%d blocked=%d\n", summary.Evidence.Pass, summary.Evidence.Fail, summary.Evidence.Blocked)
for _, record := range summary.Evidence.Latest {
	fmt.Fprintf(stdout, "  %s %s %s\n", strings.ToUpper(record.Status), record.Type, record.Criterion)
	if strings.TrimSpace(record.Summary) != "" {
		fmt.Fprintf(stdout, "    %s\n", record.Summary)
	}
}
```

Modify `internal/cli/console.go`, inside `printConsoleEvidence` after verification:

```go
fmt.Fprintf(stdout, "  %-14s pass=%d fail=%d blocked=%d\n", "manual", workspace.Evidence.Pass, workspace.Evidence.Fail, workspace.Evidence.Blocked)
```

Modify `internal/cli/workbench_snapshot.go`, after `Attention`:

```go
fmt.Fprintln(&builder, "Evidence:")
fmt.Fprintf(&builder, "  %-14s pass=%d fail=%d blocked=%d\n", "manual", detail.Workspace.Evidence.Pass, detail.Workspace.Evidence.Fail, detail.Workspace.Evidence.Blocked)
```

Modify `internal/cli/workbench_model.go` detail rendering similarly in the selected demand details section.

- [ ] **Step 6: Run surface tests and verify GREEN**

Run:

```powershell
gofmt -w internal/demandflow/workspace.go internal/demandflow/workspace_test.go internal/cli/status.go internal/cli/status_test.go internal/cli/console.go internal/cli/console_test.go internal/cli/workbench_snapshot.go internal/cli/workbench_model.go internal/cli/workbench_test.go
go test ./internal/demandflow -run "TestInspectWorkspaceSummarizesManualEvidence|TestWorkspaceNextActionsPreferEvidence" -count=1
go test ./internal/cli -run "TestRunStatusPrintsManualEvidenceSummary|TestConsole|TestWorkbench" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit operator visibility**

Run:

```powershell
git add internal/demandflow/workspace.go internal/demandflow/workspace_test.go internal/cli/status.go internal/cli/status_test.go internal/cli/console.go internal/cli/console_test.go internal/cli/workbench_snapshot.go internal/cli/workbench_model.go internal/cli/workbench_test.go
git commit -m "Surface manual evidence in operator views" -m "Manual evidence should be visible wherever operators decide the next workflow step. Workspace summaries, status, console, and workbench now expose evidence counts and guide users to add acceptance evidence before verification confirmation." -m "Constraint: Evidence visibility is read-only in operator views; console --run-next must not execute evidence commands automatically.`nConfidence: high`nScope-risk: moderate`nTested: go test ./internal/demandflow -run \"TestInspectWorkspaceSummarizesManualEvidence|TestWorkspaceNextActionsPreferEvidence\" -count=1; go test ./internal/cli -run \"TestRunStatusPrintsManualEvidenceSummary|TestConsole|TestWorkbench\" -count=1`nNot-tested: release-readiness"
```

## Task 4: Verification Evaluation Checks

**Files:**
- Modify: `internal/demandflow/evaluation.go`
- Modify: `internal/demandflow/evaluation_test.go`
- Modify: `internal/cli/evaluate_test.go`

- [ ] **Step 1: Write failing evaluation tests**

Append to `internal/demandflow/evaluation_test.go`:

```go
func TestEvaluateVerificationWarnsWhenManualEvidenceMissing(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-manual-missing", Title: "Eval manual missing", Description: "Eval", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageVerification)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "verification.manual_evidence")
	if check.Status != EvaluationWarning {
		t.Fatalf("manual evidence status = %s, want warning", check.Status)
	}
}

func TestEvaluateVerificationFailsOnFailedManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "eval-manual-fail", Title: "Eval manual fail", Description: "Eval", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "PASS", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent verification returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Message: "manual evidence fail", Data: map[string]string{"status": "fail", "type": "api", "criterion": "Inactive users are blocked", "summary": "Unexpected success"}}); err != nil {
		t.Fatalf("AppendEvent manual evidence returned error: %v", err)
	}

	evaluation, err := EvaluateDemand(root, demand.ID, StageVerification)
	if err != nil {
		t.Fatalf("EvaluateDemand returned error: %v", err)
	}
	check := findEvaluationCheck(t, evaluation.Stages[0], "verification.manual_evidence_pass")
	if check.Status != EvaluationFail {
		t.Fatalf("manual evidence pass status = %s, want fail", check.Status)
	}
	if evaluation.Stages[0].Status != EvaluationFail {
		t.Fatalf("stage status = %s, want fail", evaluation.Stages[0].Status)
	}
}
```

Append to `internal/cli/evaluate_test.go` a CLI output assertion that `devflow evaluate --stage verification` prints:

```text
verification.manual_evidence
verification.manual_evidence_pass
```

- [ ] **Step 2: Run evaluation tests and verify RED**

Run:

```powershell
go test ./internal/demandflow -run "TestEvaluateVerification.*ManualEvidence" -count=1
go test ./internal/cli -run "TestEvaluateCommand" -count=1
```

Expected: demandflow tests fail because the checks do not exist.

- [ ] **Step 3: Implement evaluation checks**

Modify `evaluateVerification` in `internal/demandflow/evaluation.go`:

```go
manual := summarizeManualEvidence(events)
checks := []EvaluationCheck{
	statusCheck("verification.recorded", "verification evidence is recorded", latestStatus != "", "blocker", latestStatus),
	statusCheck("verification.pass", "latest verification status is pass", latestStatus == "pass", "blocker", latestStatus),
	statusCheck("verification.command", "verification command is recorded", latestCommand != "", "warning", latestCommand),
	manualEvidencePresenceCheck(manual),
	manualEvidencePassCheck(manual),
}
```

Add helpers:

```go
func manualEvidencePresenceCheck(summary EvidenceSummary) EvaluationCheck {
	total := summary.Pass + summary.Fail + summary.Blocked
	if total == 0 {
		return EvaluationCheck{
			ID:       "verification.manual_evidence",
			Label:    "manual acceptance evidence is recorded",
			Status:   EvaluationWarning,
			Severity: "warning",
			Evidence: "no manual acceptance evidence recorded",
		}
	}
	return statusCheck("verification.manual_evidence", "manual acceptance evidence is recorded", true, "warning", fmt.Sprintf("pass=%d fail=%d blocked=%d", summary.Pass, summary.Fail, summary.Blocked))
}

func manualEvidencePassCheck(summary EvidenceSummary) EvaluationCheck {
	if summary.Fail > 0 || summary.Blocked > 0 {
		return EvaluationCheck{
			ID:       "verification.manual_evidence_pass",
			Label:    "manual acceptance evidence has no failures or blockers",
			Status:   EvaluationFail,
			Severity: "blocker",
			Evidence: fmt.Sprintf("pass=%d fail=%d blocked=%d", summary.Pass, summary.Fail, summary.Blocked),
		}
	}
	if summary.Pass == 0 {
		return EvaluationCheck{
			ID:       "verification.manual_evidence_pass",
			Label:    "manual acceptance evidence has no failures or blockers",
			Status:   EvaluationNotApplicable,
			Severity: "blocker",
			Evidence: "no manual acceptance evidence recorded",
		}
	}
	return statusCheck("verification.manual_evidence_pass", "manual acceptance evidence has no failures or blockers", true, "blocker", fmt.Sprintf("pass=%d fail=%d blocked=%d", summary.Pass, summary.Fail, summary.Blocked))
}
```

- [ ] **Step 4: Run evaluation tests and verify GREEN**

Run:

```powershell
gofmt -w internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go internal/cli/evaluate_test.go
go test ./internal/demandflow -run "TestEvaluateVerification.*ManualEvidence" -count=1
go test ./internal/cli -run "TestEvaluateCommand" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit evaluation checks**

Run:

```powershell
git add internal/demandflow/evaluation.go internal/demandflow/evaluation_test.go internal/cli/evaluate_test.go
git commit -m "Evaluate manual verification evidence" -m "Verification quality should distinguish technical command results from business acceptance proof. Evaluation now warns when manual evidence is absent and blocks confirmation quality when manual evidence records failures or blockers." -m "Constraint: Missing manual evidence is a warning, not a hard blocker, to preserve existing technical-only workflows while nudging operators toward stronger proof.`nConfidence: high`nScope-risk: moderate`nTested: go test ./internal/demandflow -run \"TestEvaluateVerification.*ManualEvidence\" -count=1; go test ./internal/cli -run \"TestEvaluateCommand\" -count=1`nNot-tested: release-readiness"
```

## Task 5: Release Gate And Docs

**Files:**
- Modify: `scripts/release-readiness.ps1`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Add release-readiness evidence smoke**

In `scripts/release-readiness.ps1`, inside the deterministic `"intake smoke"` or after operator dogfood, add a small verification workspace smoke:

```powershell
Invoke-Step "manual evidence smoke" {
    $evidenceRoot = Join-Path $readinessRoot 'manual-evidence-smoke'
    New-Item -ItemType Directory -Force $evidenceRoot | Out-Null
    $demandDir = Join-Path $evidenceRoot '.devflow\demands\manual-evidence-coupon'
    New-Item -ItemType Directory -Force $demandDir | Out-Null
    $now = (Get-Date).ToUniversalTime().ToString('o')
    @{
        id = 'manual-evidence-coupon'
        title = 'Manual evidence coupon'
        description = 'Inactive users are blocked'
        source = 'release-readiness'
        state = 'verification'
        created_at = $now
        updated_at = $now
    } | ConvertTo-Json | Set-Content -Encoding UTF8 (Join-Path $demandDir 'demand.json')
    "# Verification: Manual evidence coupon`n" | Set-Content -Encoding UTF8 (Join-Path $demandDir 'verification.md')
    '{"time":"2026-07-01T00:00:00Z","type":"verification.recorded","message":"verification pass","data":{"status":"PASS","command":"go test ./internal/version","evidence_file":"verification.md"}}' | Set-Content -Encoding UTF8 (Join-Path $demandDir 'events.jsonl')
    .\dist\devflow-windows-amd64.exe evidence add --root $evidenceRoot --demand manual-evidence-coupon --type api --criterion "Inactive users are blocked" --summary "POST /coupon/claim returned COUPON_USER_INACTIVE." --by readiness

    $evidenceList = .\dist\devflow-windows-amd64.exe evidence list --root $evidenceRoot --demand manual-evidence-coupon 2>&1
    $evidenceList | Tee-Object -FilePath (Join-Path $evidenceRoot 'evidence-list-output.txt') | Out-Host
    if (($evidenceList -join [Environment]::NewLine) -notmatch 'PASS api Inactive users are blocked') {
        throw "manual evidence missing from evidence list"
    }

    $statusOutput = .\dist\devflow-windows-amd64.exe status --root $evidenceRoot --demand manual-evidence-coupon 2>&1
    $statusOutput | Tee-Object -FilePath (Join-Path $evidenceRoot 'status-output.txt') | Out-Host
    if (($statusOutput -join [Environment]::NewLine) -notmatch 'pass=1 fail=0 blocked=0') {
        throw "manual evidence counts missing from status"
    }
}
```

- [ ] **Step 2: Update user guide**

In `docs/user-guide/backend-demand-loop.md`, under "Run Verification And Closeout", add:

```markdown
Record external or manual acceptance evidence while the demand is in `verification`:

```powershell
devflow evidence add --demand add-coupon-eligibility-check `
  --type api `
  --criterion "Inactive users are blocked" `
  --summary "POST /coupon/claim returned COUPON_USER_INACTIVE." `
  --link "https://example.test/log/123" `
  --by dd

devflow evidence list --demand add-coupon-eligibility-check
```

Manual evidence is local and reviewable. It can reference API calls, logs, monitor links, QA notes, or other acceptance proof, but Devflow does not fetch those external systems in Wave 24. Manual evidence does not auto-confirm verification.
```

- [ ] **Step 3: Update release notes**

In `docs/release/v0.1.md`, add:

```markdown
- Adds `devflow evidence add/list` for local manual acceptance evidence. Operators can record API, log, monitor, manual, or link evidence in `verification.md`; status, console, workbench, and evaluate surface the evidence without auto-confirming verification.
```

Add limitation:

```markdown
- Manual evidence in Wave 24 stores operator-provided references only. It does not query log, monitor, API, or screenshot systems automatically.
```

- [ ] **Step 4: Run docs and release-readiness checks**

Run:

```powershell
rg -n "evidence add|Manual evidence|manual acceptance|verification.manual_evidence" docs scripts internal
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave24
```

Expected: release readiness PASS.

- [ ] **Step 5: Commit docs and release gate**

Run:

```powershell
git add scripts/release-readiness.ps1 docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document and gate manual evidence workflow" -m "Wave 24 adds the missing business acceptance proof layer to verification. Release readiness and user docs now cover local manual evidence capture and make clear that external systems are referenced, not queried." -m "Constraint: Release readiness must stay deterministic and credential-free.`nConfidence: high`nScope-risk: narrow`nTested: rg docs/scripts; scripts/release-readiness.ps1 -Version 0.1.0-wave24`nNot-tested: live log or monitoring platforms"
```

## Task 6: Final Verification And PR

**Files:**
- No code edits unless verification exposes a defect.

- [ ] **Step 1: Run focused tests**

```powershell
go test ./internal/demandflow ./internal/cli -count=1
```

Expected: PASS.

- [ ] **Step 2: Run static checks and build**

```powershell
go vet ./...
go build ./cmd/devflow
git diff --check
```

Expected: all exit 0.

- [ ] **Step 3: Run full suite**

```powershell
go test ./... -count=1 -timeout 5m
```

Expected: all packages PASS.

- [ ] **Step 4: Run release readiness**

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave24
```

Expected: PASS, including manual evidence smoke.

- [ ] **Step 5: Manual smoke**

Run:

```powershell
$tmp = Join-Path $env:TEMP "devflow-wave24-evidence-smoke"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $tmp | Out-Null

go run ./cmd/devflow start --root $tmp --title "Manual evidence coupon" --description "Inactive users are blocked"
go run ./cmd/devflow run --root $tmp --demand manual-evidence-coupon --stage requirements
go run ./cmd/devflow confirm --root $tmp --demand manual-evidence-coupon --stage requirements --by dd --summary "requirements accepted"
go run ./cmd/devflow run --root $tmp --demand manual-evidence-coupon --stage plan
go run ./cmd/devflow confirm --root $tmp --demand manual-evidence-coupon --stage plan --by dd --summary "plan accepted"
go run ./cmd/devflow run --root $tmp --demand manual-evidence-coupon --stage implementation --quality-command "go test ./internal/version"
go run ./cmd/devflow run --root $tmp --demand manual-evidence-coupon --stage mr-review
go run ./cmd/devflow run --root $tmp --demand manual-evidence-coupon --stage verification --quality-command "go test ./internal/version"
go run ./cmd/devflow evidence add --root $tmp --demand manual-evidence-coupon --type api --criterion "Inactive users are blocked" --summary "POST /coupon/claim returned COUPON_USER_INACTIVE." --by dd
go run ./cmd/devflow evidence list --root $tmp --demand manual-evidence-coupon
go run ./cmd/devflow evaluate --root $tmp --demand manual-evidence-coupon --stage verification
go run ./cmd/devflow status --root $tmp --demand manual-evidence-coupon
go run ./cmd/devflow console --root $tmp --demand manual-evidence-coupon
go run ./cmd/devflow workbench --root $tmp --snapshot --demand manual-evidence-coupon
```

Expected:
- `verification.md` contains `## Manual Acceptance Evidence`.
- `evidence list` prints `PASS api Inactive users are blocked`.
- `evaluate --stage verification` includes `verification.manual_evidence` and `verification.manual_evidence_pass`.
- `status`, `console`, and `workbench --snapshot` show `pass=1 fail=0 blocked=0`.
- demand remains in `verification` until `devflow confirm --stage verification`.

- [ ] **Step 6: Push and open PR**

```powershell
git status --short --branch
git push -u origin wave24-manual-evidence-adapter
gh pr create --base main --head wave24-manual-evidence-adapter --title "Wave 24 manual evidence adapter" --body "Adds local manual verification evidence capture and surfaces it in status, console, workbench, and evaluation. Verification: go test ./...; go vet ./...; go build ./cmd/devflow; git diff --check; release-readiness 0.1.0-wave24."
```

- [ ] **Step 7: Wait for CI**

```powershell
gh pr view --json number,state,mergeable,statusCheckRollup,url
```

Expected:
- PR is `OPEN`.
- `mergeable` is `MERGEABLE`.
- Ubuntu and Windows Go verification checks pass.

## Self-Review

- Spec coverage: The plan implements local manual evidence capture, storage, listing, operator visibility, quality evaluation, release-readiness, and docs.
- Placeholder scan: No step uses incomplete-marker instructions. Release-readiness has one concrete credential-free path that creates a temporary verification workspace and records evidence against it.
- Type consistency: `EvidenceRecord`, `EvidenceSummary`, `AddManualEvidenceOptions`, `AddManualEvidence`, and `ListManualEvidence` are introduced before use.
- Scope control: No external adapters, no new workflow states, no automatic confirmation, no dependencies.
