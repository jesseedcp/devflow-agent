# Wave 21 Intake Memory Recall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make demand intake load reusable project memory into a visible `context.md` artifact so operators can see which stable knowledge and historical demand candidates influenced the requirements draft.

**Architecture:** Add a deterministic memory recall layer on top of the existing `demandflow` context loader. Wave 21 does not add network adapters or LLM summarization; it reuses existing `.devflow/memory/*.md` stable memory and historical `.devflow/demands/*/memory-candidates.md` search, renders a local `context.md`, writes it during `devflow intake`, and exposes `devflow recall --demand <id>` for recomputing the snapshot.

**Tech Stack:** Go 1.25, existing `internal/artifacts`, `internal/memory`, `internal/demandflow`, `internal/cli`, PowerShell release-readiness script.

---

## Product Decision

Wave 20 solved: `local PRD -> intake.md -> requirements.md -> requirements_review`.

Wave 21 solves the next missing article concept: `current demand material + long-term business memory`.

Operator flow after this wave:

```powershell
devflow intake --file docs/examples/demands/coupon-eligibility.md
devflow status --demand coupon-eligibility
devflow recall --demand coupon-eligibility
devflow console --demand coupon-eligibility
```

Expected artifacts:

```text
.devflow/demands/coupon-eligibility/intake.md
.devflow/demands/coupon-eligibility/context.md
.devflow/demands/coupon-eligibility/requirements.md
```

`context.md` is evidence, not a workflow stage. It must not auto-confirm requirements and must not promote candidate memory.

---

## Scope

In scope:

- Add `context.md` demand artifact.
- Build deterministic memory recall from existing stable memory and historical memory candidates.
- Write `context.md` during `devflow intake`.
- Add `devflow recall --demand <id>` to recompute `context.md`.
- Surface `context.md` in status/workspace summaries.
- Include recall in release-readiness smoke and docs.

Out of scope:

- URL, WeChat, Aone, DingTalk, or GitLab context adapters.
- Embeddings/vector search.
- LLM summarization of recalled memory.
- Automatic memory promotion.
- New workflow state before `requirements_review`.

---

## File Structure

- Modify: `internal/artifacts/model.go`
  - Add `ContextFile = "context.md"`.

- Modify: `internal/artifacts/store.go`
  - Create `context.md` for new demand workspaces.
  - Allow safe writes to `context.md`.

- Modify: `internal/artifacts/store_test.go`
  - Cover workspace file creation and `WriteArtifact` support.

- Modify: `internal/templates/templates.go`
  - Add `Context(title string) string`.

- Create: `internal/demandflow/recall.go`
  - Owns memory recall result types and Markdown rendering.
  - Reuses `newContextLoader(root).Load(demandID)` so search semantics stay in one place.

- Create: `internal/demandflow/recall_test.go`
  - Covers stable memory hits, historical candidate hits, no-hit output, and current-demand exclusion.

- Modify: `internal/cli/intake.go`
  - Writes `context.md` after demand creation.
  - Prints `context:` path.

- Modify: `internal/cli/intake_test.go`
  - Assert `context.md` is written and intake records recall event.

- Create: `internal/cli/recall.go`
  - Implements `devflow recall --demand <id>`.

- Create: `internal/cli/recall_test.go`
  - Covers recompute, missing demand, and help wiring.

- Modify: `internal/cli/cli.go`
  - Add command help and dispatch for `recall`.

- Modify: `internal/demandflow/workspace.go`
  - Include `context.md` in artifact summaries.

- Modify: `internal/demandflow/status_test.go`
  - Assert status sees `context.md`.

- Modify: `scripts/release-readiness.ps1`
  - Extend intake smoke to assert recall/context.

- Modify: `docs/user-guide/backend-demand-loop.md`
  - Document `context.md` and `devflow recall`.

- Modify: `docs/release/v0.1.md`
  - Add Wave 21 release note.

---

## Task 1: Add `context.md` Artifact Support

**Files:**
- Modify: `internal/artifacts/model.go`
- Modify: `internal/artifacts/store.go`
- Modify: `internal/artifacts/store_test.go`
- Modify: `internal/templates/templates.go`

- [ ] **Step 1: Add failing artifact tests**

In `internal/artifacts/store_test.go`, update the existing workspace creation expected file list to include `ContextFile` after `IntakeFile`:

```go
expected := []string{
	DemandFile,
	IntakeFile,
	ContextFile,
	RequirementsFile,
	PlanFile,
	ProgressFile,
	VerificationFile,
	CloseoutFile,
	MemoryCandidatesFile,
	EventsFile,
}
```

Add this test near `TestWriteArtifactSupportsIntakeFile`:

```go
func TestWriteArtifactSupportsContextFile(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	demand := testDemand("context-artifact")
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	if err := store.WriteArtifact(demand.ID, ContextFile, "# Context\n\nmemory recall"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, ContextFile))
	if err != nil {
		t.Fatalf("ReadFile context returned error: %v", err)
	}
	if string(body) != "# Context\n\nmemory recall" {
		t.Fatalf("context.md = %q", string(body))
	}
}
```

- [ ] **Step 2: Run artifact tests and verify red**

Run:

```powershell
go test ./internal/artifacts -run "TestCreateDemandWorkspace|TestWriteArtifactSupportsContextFile" -count=1
```

Expected: FAIL because `ContextFile` is undefined or unsupported.

- [ ] **Step 3: Add artifact constant**

Modify `internal/artifacts/model.go`:

```go
const (
	DemandFile           = "demand.json"
	IntakeFile           = "intake.md"
	ContextFile          = "context.md"
	RequirementsFile     = "requirements.md"
	PlanFile             = "plan.md"
	ProgressFile         = "progress.md"
	VerificationFile     = "verification.md"
	CloseoutFile         = "closeout.md"
	MemoryCandidatesFile = "memory-candidates.md"
	EventsFile           = "events.jsonl"
)
```

- [ ] **Step 4: Add context template**

Modify `internal/templates/templates.go`:

```go
func Context(title string) string {
	return fmt.Sprintf(`# Context: %s

## Reusable Memory

No reusable memory recalled yet.

## Historical Demand Candidates

No historical candidate memory recalled yet.
`, title)
}
```

- [ ] **Step 5: Create context template during workspace creation**

Modify `internal/artifacts/store.go` inside `CreateDemand`, after writing `IntakeFile` and before writing `RequirementsFile`:

```go
if err := writeTextFile(filepath.Join(tempDir, ContextFile), templates.Context(demand.Title)); err != nil {
	return fmt.Errorf("write context template: %w", err)
}
```

Modify `validateAppendableArtifactName`:

```go
func validateAppendableArtifactName(name string) error {
	switch name {
	case IntakeFile,
		ContextFile,
		RequirementsFile,
		PlanFile,
		ProgressFile,
		VerificationFile,
		CloseoutFile,
		MemoryCandidatesFile:
		return nil
	default:
		return fmt.Errorf("unsupported artifact %q", name)
	}
}
```

- [ ] **Step 6: Run artifact tests and verify green**

Run:

```powershell
go test ./internal/artifacts -run "TestCreateDemandWorkspace|TestWriteArtifactSupportsContextFile" -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit artifact support**

```powershell
git add internal/artifacts/model.go internal/artifacts/store.go internal/artifacts/store_test.go internal/templates/templates.go
git commit -m "Store recalled context with demand artifacts" -m "Demand intake needs a durable place for reusable memory evidence. Workspaces now include context.md and the artifact safety allowlist permits deterministic context writes." -m "Constraint: context.md is evidence, not a workflow stage.`nConfidence: high`nScope-risk: narrow`nDirective: Keep raw intake in intake.md, recalled memory in context.md, and approved requirements in requirements.md.`nTested: go test ./internal/artifacts -run \"TestCreateDemandWorkspace|TestWriteArtifactSupportsContextFile\" -count=1`nNot-tested: recall rendering"
```

---

## Task 2: Build Deterministic Memory Recall Renderer

**Files:**
- Create: `internal/demandflow/recall.go`
- Create: `internal/demandflow/recall_test.go`

- [ ] **Step 1: Write recall tests**

Create `internal/demandflow/recall_test.go`:

```go
package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/memory"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestBuildMemoryRecallIncludesStableAndCandidateMemory(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "coupon-new", Title: "Coupon eligibility", Description: "coupon active member", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand current returned error: %v", err)
	}
	if err := store.CreateDemand(artifacts.Demand{ID: "coupon-old", Title: "Old coupon", Description: "old", Source: "test", State: string(workflow.Completed)}); err != nil {
		t.Fatalf("CreateDemand old returned error: %v", err)
	}
	if err := store.WriteArtifact("coupon-old", artifacts.MemoryCandidatesFile, "# Memory Candidates\n\n## 稳定知识候选\n\n- coupon active member checks must happen before coupon claim writes\n"); err != nil {
		t.Fatalf("WriteArtifact memory candidates returned error: %v", err)
	}
	if _, err := memory.NewStore(root).PromoteCandidate(memory.PromoteOptions{
		DemandID:       "coupon-old",
		CandidateIndex: 1,
		Name:           "coupon-active-member",
		Description:    "coupon active member checks must happen before writes",
		By:             "tester",
	}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	recall, err := BuildMemoryRecall(root, "coupon-new")
	if err != nil {
		t.Fatalf("BuildMemoryRecall returned error: %v", err)
	}
	text := RenderMemoryRecall(recall)
	for _, want := range []string{
		"# Context: Coupon eligibility",
		"## Approved Stable Memory",
		"coupon active member checks must happen before writes",
		"## Historical Demand Candidates",
		"coupon-old",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("recall missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "coupon-new:") {
		t.Fatalf("recall should not include current demand as candidate:\n%s", text)
	}
}

func TestBuildMemoryRecallNoHitsStillRendersReviewableContext(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "empty-context", Title: "Empty context", Description: "nothing matches", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	recall, err := BuildMemoryRecall(root, "empty-context")
	if err != nil {
		t.Fatalf("BuildMemoryRecall returned error: %v", err)
	}
	text := RenderMemoryRecall(recall)
	for _, want := range []string{
		"# Context: Empty context",
		"No approved stable memory recalled.",
		"No historical candidate memory recalled.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("recall missing %q:\n%s", want, text)
		}
	}
}

func TestWriteMemoryRecallWritesContextAndEvent(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "write-context", Title: "Write context", Description: "context", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	result, err := WriteMemoryRecall(root, "write-context")
	if err != nil {
		t.Fatalf("WriteMemoryRecall returned error: %v", err)
	}
	if result.DemandID != "write-context" {
		t.Fatalf("DemandID = %q", result.DemandID)
	}
	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "write-context", artifacts.ContextFile))
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	if !strings.Contains(string(body), "# Context: Write context") {
		t.Fatalf("context body = %s", string(body))
	}
	events, err := store.ReadEvents("write-context")
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Type == "context.recalled" {
			found = true
			if event.Data["stable"] != "0" || event.Data["candidates"] != "0" {
				t.Fatalf("event data = %#v", event.Data)
			}
		}
	}
	if !found {
		t.Fatalf("context.recalled event missing: %#v", events)
	}
}
```

- [ ] **Step 2: Run recall tests and verify red**

Run:

```powershell
go test ./internal/demandflow -run "TestBuildMemoryRecall|TestWriteMemoryRecall" -count=1
```

Expected: FAIL because recall functions do not exist.

- [ ] **Step 3: Implement recall result and rendering**

Create `internal/demandflow/recall.go`:

```go
package demandflow

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

type MemoryRecall struct {
	DemandID  string
	Title     string
	Stable    []MemoryHit
	Candidates []MemoryHit
}

func BuildMemoryRecall(root, demandID string) (MemoryRecall, error) {
	snapshot, err := newContextLoader(root).Load(demandID)
	if err != nil {
		return MemoryRecall{}, err
	}
	recall := MemoryRecall{
		DemandID: demandID,
		Title:    snapshot.Demand.Title,
	}
	for _, hit := range snapshot.Memories {
		switch hit.Source {
		case "stable":
			recall.Stable = append(recall.Stable, hit)
		case "candidate":
			recall.Candidates = append(recall.Candidates, hit)
		}
	}
	return recall, nil
}

func RenderMemoryRecall(recall MemoryRecall) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Context: %s\n\n", recall.Title)
	fmt.Fprintf(&b, "Demand: `%s`\n\n", recall.DemandID)
	b.WriteString("## Approved Stable Memory\n\n")
	if len(recall.Stable) == 0 {
		b.WriteString("No approved stable memory recalled.\n\n")
	} else {
		for _, hit := range recall.Stable {
			fmt.Fprintf(&b, "- `%s`", hit.Path)
			if strings.TrimSpace(hit.Snippet) != "" {
				fmt.Fprintf(&b, ": %s", strings.TrimSpace(hit.Snippet))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Historical Demand Candidates\n\n")
	if len(recall.Candidates) == 0 {
		b.WriteString("No historical candidate memory recalled.\n\n")
	} else {
		for _, hit := range recall.Candidates {
			fmt.Fprintf(&b, "- `%s`", hit.DemandID)
			if strings.TrimSpace(hit.Snippet) != "" {
				fmt.Fprintf(&b, ": %s", strings.TrimSpace(hit.Snippet))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Operator Notes\n\n")
	b.WriteString("- Review this context before confirming requirements.\n")
	b.WriteString("- Candidate memory is unapproved and must not be treated as stable truth.\n")
	return b.String()
}

type RecallWriteResult struct {
	DemandID       string
	ContextPath    string
	StableCount    int
	CandidateCount int
}

func WriteMemoryRecall(root, demandID string) (RecallWriteResult, error) {
	recall, err := BuildMemoryRecall(root, demandID)
	if err != nil {
		return RecallWriteResult{}, err
	}
	store := artifacts.NewStore(root)
	if err := store.WriteArtifact(demandID, artifacts.ContextFile, RenderMemoryRecall(recall)); err != nil {
		return RecallWriteResult{}, err
	}
	if err := store.AppendEvent(demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    "context.recalled",
		Message: "reusable memory context recalled",
		Data: map[string]string{
			"stable":     fmt.Sprintf("%d", len(recall.Stable)),
			"candidates": fmt.Sprintf("%d", len(recall.Candidates)),
		},
	}); err != nil {
		return RecallWriteResult{}, err
	}
	return RecallWriteResult{
		DemandID:       demandID,
		ContextPath:    filepath.Join(store.DemandDir(demandID), artifacts.ContextFile),
		StableCount:    len(recall.Stable),
		CandidateCount: len(recall.Candidates),
	}, nil
}
```

- [ ] **Step 4: Run recall tests and verify green**

Run:

```powershell
gofmt -w internal/demandflow/recall.go internal/demandflow/recall_test.go
go test ./internal/demandflow -run "TestBuildMemoryRecall|TestWriteMemoryRecall" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit recall renderer**

```powershell
git add internal/demandflow/recall.go internal/demandflow/recall_test.go
git commit -m "Render reusable memory context snapshots" -m "Intake should make historical knowledge visible instead of hiding it inside runner prompts. Demandflow now builds and writes a deterministic context.md from stable memory and historical candidates." -m "Constraint: Candidate memory remains unapproved evidence and is labeled as such.`nConfidence: high`nScope-risk: moderate`nDirective: Keep recall deterministic; add embeddings or LLM summarization only behind a separate adapter plan.`nTested: go test ./internal/demandflow -run \"TestBuildMemoryRecall|TestWriteMemoryRecall\" -count=1`nNot-tested: CLI recall command"
```

---

## Task 3: Write Context During Intake

**Files:**
- Modify: `internal/cli/intake.go`
- Modify: `internal/cli/intake_test.go`

- [ ] **Step 1: Add intake expectations for context**

In `internal/cli/intake_test.go`, inside `TestIntakeFileCreatesReviewReadyDemand`, after reading `intake.md`, add:

```go
	contextBody, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demand.ID, artifacts.ContextFile))
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	if !strings.Contains(string(contextBody), "# Context: Coupon eligibility") {
		t.Fatalf("context missing heading:\n%s", string(contextBody))
	}
	if !strings.Contains(stdout.String(), "context: ") {
		t.Fatalf("stdout missing context path:\n%s", stdout.String())
	}
```

Add this event assertion near the end of the test:

```go
	events, err := store.ReadEvents(demand.ID)
	if err != nil {
		t.Fatalf("ReadEvents returned error: %v", err)
	}
	if !cliTestHasEvent(events, "context.recalled") {
		t.Fatalf("events missing context.recalled: %#v", events)
	}
```

If `cliTestHasEvent` does not exist in `internal/cli` tests, add this helper at the bottom of `intake_test.go`:

```go
func cliTestHasEvent(events []artifacts.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run intake test and verify red**

Run:

```powershell
go test ./internal/cli -run TestIntakeFileCreatesReviewReadyDemand -count=1
```

Expected: FAIL because intake does not call `WriteMemoryRecall` or print `context:`.

- [ ] **Step 3: Call recall from intake**

Modify imports in `internal/cli/intake.go` to add demandflow:

```go
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
```

After writing `requirements.md` and before setting `demand.State`, add:

```go
	recallResult, err := demandflow.WriteMemoryRecall(root, demand.ID)
	if err != nil {
		return err
	}
```

In stdout output, add after the `intake:` line:

```go
	fmt.Fprintf(stdout, "context: %s\n", recallResult.ContextPath)
	fmt.Fprintf(stdout, "memory: %d stable, %d candidate\n", recallResult.StableCount, recallResult.CandidateCount)
```

- [ ] **Step 4: Run intake test and verify green**

Run:

```powershell
gofmt -w internal/cli/intake.go internal/cli/intake_test.go
go test ./internal/cli -run TestIntakeFileCreatesReviewReadyDemand -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit intake integration**

```powershell
git add internal/cli/intake.go internal/cli/intake_test.go
git commit -m "Recall memory during local PRD intake" -m "Local intake now captures reusable memory context at the same time it creates requirements, making the demand entry snapshot reviewable before human confirmation." -m "Constraint: Recall failure should fail intake because silent missing context would be misleading.`nConfidence: high`nScope-risk: moderate`nDirective: Do not auto-confirm requirements even when memory recall finds stable matches.`nTested: go test ./internal/cli -run TestIntakeFileCreatesReviewReadyDemand -count=1`nNot-tested: explicit recall command"
```

---

## Task 4: Add `devflow recall --demand`

**Files:**
- Create: `internal/cli/recall.go`
- Create: `internal/cli/recall_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write recall CLI tests**

Create `internal/cli/recall_test.go`:

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

func TestRecallCommandRewritesContext(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "recall-demand", Title: "Recall demand", Description: "memory", Source: "test", State: string(workflow.RequirementsReview)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact("recall-demand", artifacts.ContextFile, "stale"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}

	var stdout bytes.Buffer
	err := Run([]string{"recall", "--root", root, "--demand", "recall-demand"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("recall returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", "recall-demand", artifacts.ContextFile))
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	if !strings.Contains(string(body), "# Context: Recall demand") {
		t.Fatalf("context not rewritten:\n%s", string(body))
	}
	if !strings.Contains(stdout.String(), "context recalled for recall-demand") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestRecallCommandRequiresDemand(t *testing.T) {
	err := Run([]string{"recall"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--demand is required") {
		t.Fatalf("err = %v, want --demand is required", err)
	}
}

func TestHelpIncludesRecall(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	for _, want := range []string{"devflow recall --demand <id>", "recall   Rebuild reusable memory context for a demand"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
```

- [ ] **Step 2: Run recall CLI tests and verify red**

Run:

```powershell
go test ./internal/cli -run "TestRecall|TestHelpIncludesRecall" -count=1
```

Expected: FAIL because `recall` command is unknown.

- [ ] **Step 3: Implement recall command**

Create `internal/cli/recall.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runRecall(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("recall", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	result, err := demandflow.WriteMemoryRecall(root, demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "context recalled for %s\n", result.DemandID)
	fmt.Fprintf(stdout, "context: %s\n", result.ContextPath)
	fmt.Fprintf(stdout, "memory: %d stable, %d candidate\n", result.StableCount, result.CandidateCount)
	return nil
}
```

- [ ] **Step 4: Wire help and dispatch**

Modify `internal/cli/cli.go`.

In usage, add after `devflow intake --file <path>`:

```text
  devflow recall --demand <id>
```

In commands list, add after `intake`:

```text
  recall   Rebuild reusable memory context for a demand
```

In `Run`, add:

```go
	case "recall":
		return runRecall(args[1:], stdout, stderr)
```

- [ ] **Step 5: Run recall CLI tests and verify green**

Run:

```powershell
gofmt -w internal/cli/recall.go internal/cli/recall_test.go internal/cli/cli.go
go test ./internal/cli -run "TestRecall|TestHelpIncludesRecall" -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit recall CLI**

```powershell
git add internal/cli/recall.go internal/cli/recall_test.go internal/cli/cli.go
git commit -m "Expose memory recall as an operator command" -m "Operators need to refresh context.md after memory changes without recreating a demand. The recall command rebuilds reusable memory context while preserving the existing workflow state." -m "Constraint: recall is read-only except for context.md and the context.recalled event.`nConfidence: high`nScope-risk: narrow`nTested: go test ./internal/cli -run \"TestRecall|TestHelpIncludesRecall\" -count=1`nNot-tested: release-readiness integration"
```

---

## Task 5: Surface `context.md` In Status And Workspace

**Files:**
- Modify: `internal/demandflow/workspace.go`
- Modify: `internal/demandflow/status_test.go`
- Optionally modify: `internal/demandflow/workspace_test.go`

- [ ] **Step 1: Add status expectation**

In `internal/demandflow/status_test.go`, update artifact expectations that already include `intake.md` to also include `context.md`.

Add this assertion beside the intake assertion:

```go
if !foundContext {
	t.Fatalf("context artifact missing from %#v", report.Artifacts)
}
```

Set `foundContext = true` when:

```go
if artifact.Name == artifacts.ContextFile {
	foundContext = true
}
```

- [ ] **Step 2: Run status tests and verify red**

Run:

```powershell
go test ./internal/demandflow -run "TestInspectStatus|TestWorkspace" -count=1
```

Expected: FAIL because `context.md` is not in workspace artifact summaries.

- [ ] **Step 3: Include context artifact**

Modify `internal/demandflow/workspace.go`.

In the artifact file list, add `artifacts.ContextFile` immediately after `artifacts.IntakeFile`:

```go
files := []string{
	artifacts.IntakeFile,
	artifacts.ContextFile,
	artifacts.RequirementsFile,
	artifacts.PlanFile,
	artifacts.ProgressFile,
	artifacts.VerificationFile,
	artifacts.CloseoutFile,
	artifacts.MemoryCandidatesFile,
	artifacts.EventsFile,
}
```

In artifact status classification, add:

```go
case artifacts.ContextFile:
	lower := strings.ToLower(text)
	if strings.Contains(lower, "approved stable memory") || strings.Contains(lower, "historical demand candidates") {
		return "present"
	}
	return "template"
```

- [ ] **Step 4: Run status tests and verify green**

Run:

```powershell
gofmt -w internal/demandflow/workspace.go internal/demandflow/status_test.go internal/demandflow/workspace_test.go
go test ./internal/demandflow -run "TestInspectStatus|TestWorkspace" -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit status surfacing**

```powershell
git add internal/demandflow/workspace.go internal/demandflow/status_test.go internal/demandflow/workspace_test.go
git commit -m "Surface recalled context in demand status" -m "Context recall is now a first-class demand artifact in local operator views, while remaining outside the workflow stage machine." -m "Constraint: context.md does not create a new human confirmation gate.`nConfidence: high`nScope-risk: narrow`nTested: go test ./internal/demandflow -run \"TestInspectStatus|TestWorkspace\" -count=1`nNot-tested: workbench snapshot rendering beyond shared workspace summary"
```

---

## Task 6: Add Release Readiness And Documentation

**Files:**
- Modify: `scripts/release-readiness.ps1`
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`

- [ ] **Step 1: Extend release-readiness intake smoke**

Open `scripts/release-readiness.ps1`. In the existing `"intake smoke"` step added by Wave 20, after the `intake` command and before `evaluate`, add:

```powershell
    .\dist\devflow-windows-amd64.exe recall --root $intakeRoot --demand coupon-eligibility | Tee-Object -FilePath (Join-Path $intakeRoot 'recall-output.txt') | Out-Host
    $contextPath = Join-Path $intakeRoot '.devflow\demands\coupon-eligibility\context.md'
    if (-not (Test-Path $contextPath)) {
        throw "context.md was not created by intake recall"
    }
```

- [ ] **Step 2: Update user guide**

In `docs/user-guide/backend-demand-loop.md`, after the intake section that explains `intake.md`, add:

```markdown
`intake` also writes `context.md`. This file is the reusable-memory snapshot for the demand. It lists approved stable memory separately from historical demand candidates, because candidate memory is useful context but not approved truth.

Rebuild the context snapshot after promoting or rejecting memory:

```powershell
devflow recall --demand coupon-eligibility
```
```

- [ ] **Step 3: Update release notes**

In `docs/release/v0.1.md`, add:

```markdown
- Adds `context.md` memory recall for demand intake. `devflow intake` and `devflow recall` now make stable memory and historical candidates visible before requirements confirmation.
```

Under limitations, add:

```markdown
- Memory recall is deterministic keyword search in Wave 21. Embeddings, ranked semantic search, and external wiki adapters remain future work.
```

- [ ] **Step 4: Run docs and release smoke**

Run:

```powershell
rg -n "context.md|devflow recall|Memory recall|Wave 21" docs\user-guide\backend-demand-loop.md docs\release\v0.1.md
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave21
```

Expected:

- `rg` finds the new docs.
- Release readiness exits 0 and includes intake smoke plus recall output.

- [ ] **Step 5: Commit docs and release gate**

```powershell
git add scripts/release-readiness.ps1 docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document and gate intake memory recall" -m "Wave 21 extends intake from source capture to reusable context recall. The user guide, release notes, and release-readiness smoke now cover context.md and the recall command." -m "Constraint: Recall remains deterministic and credential-free.`nConfidence: high`nScope-risk: narrow`nTested: rg docs; scripts/release-readiness.ps1 -Version 0.1.0-wave21`nNot-tested: external wiki or semantic search"
```

---

## Final Verification

Run from the Wave 21 worktree:

```powershell
go test ./internal/artifacts ./internal/demandflow ./internal/cli -count=1
go vet ./...
go build ./cmd/devflow
git diff --check
go test ./... -count=1 -timeout 5m
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\release-readiness.ps1 -Version 0.1.0-wave21
```

Manual smoke with reusable memory:

```powershell
$tmp = Join-Path $env:TEMP "devflow-wave21-memory-smoke"
Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $tmp | Out-Null
$old = Join-Path $tmp "old-coupon.md"
@"
# Old coupon

## 目标
- coupon active member historical rule

## 验收标准
- active member rule exists
"@ | Set-Content -Encoding UTF8 $old
go run ./cmd/devflow intake --root $tmp --file $old
go run ./cmd/devflow closeout --root $tmp --demand old-coupon --result "done" --knowledge "coupon active member checks must happen before coupon claim writes"
go run ./cmd/devflow memory promote --root $tmp --demand old-coupon --candidate 1 --by dd
$new = Join-Path $tmp "coupon-eligibility.md"
@"
# Coupon eligibility

## 目标
- coupon active member eligibility

## 验收标准
- inactive members are blocked
"@ | Set-Content -Encoding UTF8 $new
go run ./cmd/devflow intake --root $tmp --file $new
go run ./cmd/devflow recall --root $tmp --demand coupon-eligibility
Get-Content (Join-Path $tmp ".devflow\demands\coupon-eligibility\context.md")
```

Expected:

- `context.md` exists for `coupon-eligibility`.
- It contains `Approved Stable Memory`.
- It contains the stable memory description from the old demand.
- Current demand remains in `requirements_review`.

Open PR:

```powershell
git push -u origin wave21-intake-memory-recall
gh pr create --base main --head wave21-intake-memory-recall --title "Wave 21 intake memory recall" --body "Adds context.md memory recall for intake and a devflow recall command. Verification: go test ./...; go vet ./...; go build ./cmd/devflow; git diff --check; release-readiness 0.1.0-wave21."
```

Wait for CI:

```powershell
gh pr view --json number,state,mergeable,statusCheckRollup,url
```

Expected:

- PR is mergeable.
- Ubuntu and Windows Go verification pass.

---

## Self-Review

- Scope coverage: This plan implements visible reusable memory context for intake and recall. It does not attempt URL intake, external wiki adapters, embeddings, or LLM summarization.
- Safety: `context.md` is an artifact, not a workflow state. Requirements still require human confirmation before planning.
- Reuse: The plan reuses existing `contextLoader`, `memory.SearchStable`, and `memory.Search` semantics instead of inventing a second memory search engine.
- Data separation: `intake.md` remains raw source material; `context.md` is recalled knowledge; `requirements.md` is review material.
- Test coverage: Artifact tests, recall renderer tests, intake integration tests, recall CLI tests, status tests, release-readiness, and manual memory smoke are specified.
- Placeholder scan: Command examples use concrete file names, demand IDs, and function names. Angle-bracket notation appears only in user-facing help syntax and PR examples.
