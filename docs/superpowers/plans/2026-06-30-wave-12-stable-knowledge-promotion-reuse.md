# Wave 12 Stable Knowledge Promotion And Reuse Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Wave 12 loop that lets users review demand-local `memory-candidates.md`, promote approved candidates into `.devflow/memory/*.md`, reject unsuitable candidates with audit events, and reuse approved stable memory in future requirements/plan prompts.

**Architecture:** Keep MewCode-derived runtime memory as the stable file format authority under `internal/runtime/memory`, while Devflow product memory orchestration lives in `internal/memory`, `internal/cli`, and `internal/demandflow`. Candidate memory and stable memory remain distinct: candidates come from `.devflow/demands/<id>/memory-candidates.md`; stable project knowledge lives in `.devflow/memory/*.md` and is rendered ahead of candidates in demand prompts.

**Tech Stack:** Go standard library, existing Devflow artifact store, existing runtime memory frontmatter format, existing CLI command dispatcher, PowerShell on Windows.

---

## Environment And Branch Setup

Run from:

```powershell
cd D:\Users\dd\Desktop\agent学习\mewcode-golang\devflow-agent
git status --short --branch
git pull --ff-only origin main
git switch -c feature/devflow-wave-12
```

Expected starting state:

```text
## main...origin/main
```

If using a Superpowers-owned worktree, create it from `main` at execution time and run the same commands inside that worktree. Do not implement directly on `main`.

## File Structure

Create:

- `internal/memory/candidates.go` - parses `memory-candidates.md` into deterministic numbered candidates.
- `internal/memory/candidates_test.go` - parser unit tests.
- `internal/memory/decisions.go` - reads/writes memory decision state through demand `events.jsonl`, builds stable memory markdown, and updates `MEMORY.md`.
- `internal/memory/decisions_test.go` - promote/reject/store tests.
- `internal/cli/memory.go` - `devflow memory list|promote|reject` CLI.
- `internal/cli/memory_test.go` - CLI tests for memory commands.

Modify:

- `internal/memory/store.go` - add stable memory search and extend `Result` with `Source`.
- `internal/memory/store_test.go` - add stable search tests; update existing assertions if they compare full `Result`.
- `internal/demandflow/types.go` - extend `MemoryHit` with `Source`.
- `internal/demandflow/context.go` - load stable memory first, then candidate memory excluding current demand.
- `internal/demandflow/context_test.go` - assert stable memory ordering and candidate exclusion.
- `internal/demandflow/prompts.go` - render stable and candidate memory under separate labels.
- `internal/demandflow/prompts_test.go` - add prompt rendering test if no existing prompt-specific test covers it.
- `internal/cli/cli.go` - add `memory` command to help text and command dispatch.
- `docs/user-guide/backend-demand-loop.md` - document the new memory review commands.
- `docs/release/v0.1.md` - add Wave 12 memory promotion note.

Do not modify `internal/runtime/memory` unless a compile-time import requires a small exported helper. The expected implementation can reuse its on-disk format without changing that package.

## Data Contracts

Use these types in `internal/memory`:

```go
type Source string

const (
	SourceCandidate Source = "candidate"
	SourceStable    Source = "stable"
)

type CandidateStatus string

const (
	CandidatePending  CandidateStatus = "pending"
	CandidatePromoted CandidateStatus = "promoted"
	CandidateRejected CandidateStatus = "rejected"
)

type Candidate struct {
	Index      int
	Text       string
	Status     CandidateStatus
	StablePath string
	Reason     string
}

type PromoteOptions struct {
	DemandID       string
	CandidateIndex int
	Name           string
	Description    string
	By             string
	Now            func() time.Time
}

type PromoteResult struct {
	Candidate Candidate
	Path      string
	IndexPath string
}

type RejectOptions struct {
	DemandID       string
	CandidateIndex int
	By             string
	Reason         string
	Now            func() time.Time
}
```

Extend existing `Result`:

```go
type Result struct {
	DemandID string
	Path     string
	Snippet  string
	Source   Source
}
```

Event types:

```text
memory.promoted
memory.rejected
```

Event data keys:

```text
candidate_index
candidate
by
stable_path
reason
```

Stable memory file frontmatter:

```markdown
---
name: coupon-eligibility-policy
description: Coupon eligibility must check active membership before applying discount rules.
type: project
source_demand: add-coupon-check
promoted_at: 2026-06-30T10:30:00+08:00
promoted_by: dd
---

# Coupon eligibility policy

Coupon eligibility must check active membership before applying discount rules.

Why: This rule was confirmed during demand add-coupon-check.

How to apply: Reuse this rule when generating requirements or plans for similar backend demand work.
```

---

### Task 1: Candidate Parser

**Files:**
- Create: `internal/memory/candidates.go`
- Create: `internal/memory/candidates_test.go`

- [ ] **Step 1: Write failing parser tests**

Create `internal/memory/candidates_test.go`:

```go
package memory

import (
	"reflect"
	"testing"
)

func TestParseCandidatesFromChineseStableKnowledgeSection(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates: Coupon

## 稳定知识候选

- Active membership must be checked before coupon discount rules.
- Coupon errors should preserve the original order validation message.

## 流程改进候选

- Keep review comments grouped by category.
`

	got := ParseCandidates(input)
	want := []Candidate{
		{Index: 1, Text: "Active membership must be checked before coupon discount rules.", Status: CandidatePending},
		{Index: 2, Text: "Coupon errors should preserve the original order validation message.", Status: CandidatePending},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCandidates() = %#v, want %#v", got, want)
	}
}

func TestParseCandidatesFromEnglishSection(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates

## Stable Knowledge Candidates

- Persist merge request routing decisions as progress evidence.
`
	got := ParseCandidates(input)
	want := []Candidate{{Index: 1, Text: "Persist merge request routing decisions as progress evidence.", Status: CandidatePending}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCandidates() = %#v, want %#v", got, want)
	}
}

func TestParseCandidatesFallsBackToTopLevelBullets(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates

- Stable fallback one.
  - nested detail should not become its own candidate.
- Stable fallback two.
`
	got := ParseCandidates(input)
	want := []Candidate{
		{Index: 1, Text: "Stable fallback one.", Status: CandidatePending},
		{Index: 2, Text: "Stable fallback two.", Status: CandidatePending},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCandidates() = %#v, want %#v", got, want)
	}
}

func TestParseCandidatesReturnsEmptyForTemplateOnlyFile(t *testing.T) {
	t.Parallel()

	input := `# Memory Candidates: Empty

## 稳定知识候选

## 流程改进候选

- This process item is not a stable knowledge candidate.
`
	if got := ParseCandidates(input); len(got) != 0 {
		t.Fatalf("ParseCandidates() = %#v, want empty", got)
	}
}
```

- [ ] **Step 2: Run parser tests and verify failure**

Run:

```powershell
go test ./internal/memory -run TestParseCandidates -count=1
```

Expected: fail with `undefined: ParseCandidates` or `undefined: Candidate`.

- [ ] **Step 3: Implement parser**

Create `internal/memory/candidates.go`:

```go
package memory

import "strings"

type Source string

const (
	SourceCandidate Source = "candidate"
	SourceStable    Source = "stable"
)

type CandidateStatus string

const (
	CandidatePending  CandidateStatus = "pending"
	CandidatePromoted CandidateStatus = "promoted"
	CandidateRejected CandidateStatus = "rejected"
)

type Candidate struct {
	Index      int
	Text       string
	Status     CandidateStatus
	StablePath string
	Reason     string
}

func ParseCandidates(content string) []Candidate {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	sectionLines, found := stableCandidateSection(lines)
	if !found {
		sectionLines = lines
	}

	out := make([]Candidate, 0)
	for _, line := range sectionLines {
		if !isTopLevelBullet(line) {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if text == "" {
			continue
		}
		out = append(out, Candidate{
			Index:  len(out) + 1,
			Text:   text,
			Status: CandidatePending,
		})
	}
	return out
}

func stableCandidateSection(lines []string) ([]string, bool) {
	start := -1
	for index, line := range lines {
		if isStableCandidateHeading(line) {
			start = index + 1
			break
		}
	}
	if start < 0 {
		return nil, false
	}

	end := len(lines)
	for index := start; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "## ") {
			end = index
			break
		}
	}
	return lines[start:end], true
}

func isStableCandidateHeading(line string) bool {
	heading := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(heading, "## ") && (
		strings.Contains(heading, "稳定知识候选") ||
			strings.Contains(heading, "stable knowledge candidates") ||
			strings.Contains(heading, "memory candidates"))
}

func isTopLevelBullet(line string) bool {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(line), "- ")
}
```

- [ ] **Step 4: Fix Go syntax if needed and run parser tests**

The `return` expression in `isStableCandidateHeading` may need parentheses formatted by `gofmt`. Run:

```powershell
gofmt -w internal\memory\candidates.go internal\memory\candidates_test.go
go test ./internal/memory -run TestParseCandidates -count=1
```

Expected: pass.

- [ ] **Step 5: Commit parser**

```powershell
git add internal\memory\candidates.go internal\memory\candidates_test.go
git commit -m @'
Parse stable knowledge candidates deterministically

Wave 12 needs candidate extraction to be testable and independent of LLM wording so promotion can target numbered entries.

Constraint: Candidate parsing must handle current Chinese closeout templates and English plan/spec language
Rejected: Use an LLM parser | promotion must be deterministic and cheap
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/memory -run TestParseCandidates -count=1
'@
```

---

### Task 2: Memory Store Promotion And Rejection

**Files:**
- Create: `internal/memory/decisions.go`
- Create: `internal/memory/decisions_test.go`
- Modify: `internal/memory/store.go`

- [ ] **Step 1: Write failing store tests**

Create `internal/memory/decisions_test.go`:

```go
package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestStoreListCandidatesShowsDecisionState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	store := NewStore(root)
	if _, err := store.PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}
	if _, err := store.RejectCandidate(RejectOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 2,
		By:             "dd",
		Reason:         "too specific to one test fixture",
		Now:            fixedMemoryTime,
	}); err != nil {
		t.Fatalf("RejectCandidate returned error: %v", err)
	}

	got, err := store.ListCandidates("add-coupon-check")
	if err != nil {
		t.Fatalf("ListCandidates returned error: %v", err)
	}
	if got[0].Status != CandidatePromoted || !strings.Contains(got[0].StablePath, "coupon-eligibility-policy.md") {
		t.Fatalf("candidate 1 decision = %#v, want promoted with stable path", got[0])
	}
	if got[1].Status != CandidateRejected || got[1].Reason != "too specific to one test fixture" {
		t.Fatalf("candidate 2 decision = %#v, want rejected with reason", got[1])
	}
}

func TestStorePromoteCandidateWritesStableMemoryAndIndex(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	result, err := NewStore(root).PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	})
	if err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	body, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read stable memory: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"name: coupon-eligibility-policy",
		"description: membership gates coupon eligibility",
		"type: project",
		"source_demand: add-coupon-check",
		"promoted_by: dd",
		"Active membership must be checked before coupon discount rules.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("stable memory missing %q:\n%s", want, text)
		}
	}

	indexBody, err := os.ReadFile(result.IndexPath)
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	if !strings.Contains(string(indexBody), "[coupon-eligibility-policy](coupon-eligibility-policy.md)") {
		t.Fatalf("MEMORY.md missing pointer:\n%s", string(indexBody))
	}

	events := readMemoryTestEvents(t, root, "add-coupon-check")
	if !memoryTestHasEvent(events, "memory.promoted") {
		t.Fatalf("events missing memory.promoted: %#v", events)
	}
}

func TestStorePromoteCandidateUsesDemandSuffixOnFileConflict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)
	memDir := filepath.Join(root, ".devflow", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("MkdirAll memory dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "coupon-eligibility-policy.md"), []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing memory: %v", err)
	}

	result, err := NewStore(root).PromoteCandidate(PromoteOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 1,
		Name:           "coupon-eligibility-policy",
		Description:    "membership gates coupon eligibility",
		By:             "dd",
		Now:            fixedMemoryTime,
	})
	if err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}
	if filepath.Base(result.Path) != "coupon-eligibility-policy-add-coupon-check.md" {
		t.Fatalf("stable path = %s, want demand suffix", result.Path)
	}
}

func TestStoreRejectCandidateRecordsEventWithoutStableFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	seedDemandWithCandidates(t, root)

	got, err := NewStore(root).RejectCandidate(RejectOptions{
		DemandID:       "add-coupon-check",
		CandidateIndex: 2,
		By:             "dd",
		Reason:         "too specific to one test fixture",
		Now:            fixedMemoryTime,
	})
	if err != nil {
		t.Fatalf("RejectCandidate returned error: %v", err)
	}
	if got.Status != CandidateRejected || got.Reason != "too specific to one test fixture" {
		t.Fatalf("RejectCandidate = %#v, want rejected", got)
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".devflow", "memory")); err == nil && len(entries) != 0 {
		t.Fatalf("reject should not write stable memory, entries = %#v", entries)
	}

	events := readMemoryTestEvents(t, root, "add-coupon-check")
	if !memoryTestHasEvent(events, "memory.rejected") {
		t.Fatalf("events missing memory.rejected: %#v", events)
	}
}

func TestStoreCandidateErrorsAreClear(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "add-coupon-check", Title: "coupon", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact("add-coupon-check", artifacts.MemoryCandidatesFile, "# Memory Candidates\n\n"); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}

	_, err := NewStore(root).PromoteCandidate(PromoteOptions{DemandID: "add-coupon-check", CandidateIndex: 1, By: "dd", Now: fixedMemoryTime})
	if err == nil || !strings.Contains(err.Error(), "no memory candidates found") {
		t.Fatalf("PromoteCandidate error = %v, want no memory candidates found", err)
	}
}

func seedDemandWithCandidates(t *testing.T, root string) {
	t.Helper()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "add-coupon-check", Title: "coupon", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	body := `# Memory Candidates: coupon

## 稳定知识候选

- Active membership must be checked before coupon discount rules.
- Coupon errors should preserve the original order validation message.
`
	if err := store.WriteArtifact("add-coupon-check", artifacts.MemoryCandidatesFile, body); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
}

func fixedMemoryTime() time.Time {
	return time.Date(2026, 6, 30, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
}

func readMemoryTestEvents(t *testing.T, root, demandID string) []artifacts.Event {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demandID, artifacts.EventsFile))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	var events []artifacts.Event
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event artifacts.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode event %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func memoryTestHasEvent(events []artifacts.Event, eventType string) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run store tests and verify failure**

```powershell
go test ./internal/memory -run "TestStore(ListCandidates|PromoteCandidate|RejectCandidate|CandidateErrors)" -count=1
```

Expected: fail with missing `PromoteOptions`, `PromoteCandidate`, `RejectOptions`, `ListCandidates`.

- [ ] **Step 3: Implement promotion/rejection store**

Create `internal/memory/decisions.go` with these public methods and helpers:

```go
package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

const memoryDirName = "memory"

var stableNamePattern = regexp.MustCompile(`[^a-z0-9]+`)

type PromoteOptions struct {
	DemandID       string
	CandidateIndex int
	Name           string
	Description    string
	By             string
	Now            func() time.Time
}

type PromoteResult struct {
	Candidate Candidate
	Path      string
	IndexPath string
}

type RejectOptions struct {
	DemandID       string
	CandidateIndex int
	By             string
	Reason         string
	Now            func() time.Time
}

func (s Store) ListCandidates(demandID string) ([]Candidate, error) {
	candidates, err := s.loadCandidates(demandID)
	if err != nil {
		return nil, err
	}
	decisions, err := s.loadDecisions(demandID)
	if err != nil {
		return nil, err
	}
	for index := range candidates {
		if decision, ok := decisions[candidates[index].Index]; ok {
			candidates[index].Status = decision.Status
			candidates[index].StablePath = decision.StablePath
			candidates[index].Reason = decision.Reason
		}
	}
	return candidates, nil
}

func (s Store) PromoteCandidate(opts PromoteOptions) (PromoteResult, error) {
	opts.By = strings.TrimSpace(opts.By)
	if opts.By == "" {
		return PromoteResult{}, fmt.Errorf("--by is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	candidate, err := s.candidateByIndex(opts.DemandID, opts.CandidateIndex)
	if err != nil {
		return PromoteResult{}, err
	}
	name := normalizeStableName(opts.Name)
	if name == "" {
		name = normalizeStableName(candidate.Text)
	}
	if name == "" {
		return PromoteResult{}, fmt.Errorf("stable memory name is required")
	}
	description := strings.Join(strings.Fields(opts.Description), " ")
	if description == "" {
		description = candidate.Text
	}

	memDir, err := s.ensureStableMemoryDir()
	if err != nil {
		return PromoteResult{}, err
	}
	fileName := name + ".md"
	path := filepath.Join(memDir, fileName)
	if _, err := os.Lstat(path); err == nil {
		fileName = name + "-" + opts.DemandID + ".md"
		path = filepath.Join(memDir, fileName)
	} else if err != nil && !os.IsNotExist(err) {
		return PromoteResult{}, fmt.Errorf("inspect stable memory path: %w", err)
	}

	body := stableMemoryBody(name, description, candidate.Text, opts.DemandID, opts.By, now())
	if err := writeTextAtomic(path, body); err != nil {
		return PromoteResult{}, fmt.Errorf("write stable memory: %w", err)
	}

	indexPath := filepath.Join(memDir, "MEMORY.md")
	if err := appendMemoryIndex(indexPath, name, fileName, description); err != nil {
		return PromoteResult{}, err
	}

	eventPath, err := s.eventsPath(opts.DemandID)
	if err != nil {
		return PromoteResult{}, err
	}
	if err := appendMemoryEvent(eventPath, artifacts.Event{
		Time:    now().UTC(),
		Type:    "memory.promoted",
		Message: "memory candidate promoted",
		Data: map[string]string{
			"candidate_index": strconv.Itoa(candidate.Index),
			"candidate":       candidate.Text,
			"by":              opts.By,
			"stable_path":     path,
		},
	}); err != nil {
		return PromoteResult{}, err
	}

	candidate.Status = CandidatePromoted
	candidate.StablePath = path
	return PromoteResult{Candidate: candidate, Path: path, IndexPath: indexPath}, nil
}

func (s Store) RejectCandidate(opts RejectOptions) (Candidate, error) {
	opts.By = strings.TrimSpace(opts.By)
	opts.Reason = strings.Join(strings.Fields(opts.Reason), " ")
	if opts.By == "" {
		return Candidate{}, fmt.Errorf("--by is required")
	}
	if opts.Reason == "" {
		return Candidate{}, fmt.Errorf("--reason is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	candidate, err := s.candidateByIndex(opts.DemandID, opts.CandidateIndex)
	if err != nil {
		return Candidate{}, err
	}
	eventPath, err := s.eventsPath(opts.DemandID)
	if err != nil {
		return Candidate{}, err
	}
	if err := appendMemoryEvent(eventPath, artifacts.Event{
		Time:    now().UTC(),
		Type:    "memory.rejected",
		Message: "memory candidate rejected",
		Data: map[string]string{
			"candidate_index": strconv.Itoa(candidate.Index),
			"candidate":       candidate.Text,
			"by":              opts.By,
			"reason":          opts.Reason,
		},
	}); err != nil {
		return Candidate{}, err
	}

	candidate.Status = CandidateRejected
	candidate.Reason = opts.Reason
	return candidate, nil
}

type memoryDecision struct {
	Status     CandidateStatus
	StablePath string
	Reason     string
}
```

Add the remaining helpers in the same file:

```go
func (s Store) loadCandidates(demandID string) ([]Candidate, error) {
	if _, err := artifacts.NewStore(s.root).LoadDemand(demandID); err != nil {
		return nil, err
	}
	path := filepath.Join(s.root, ".devflow", "demands", demandID, memoryCandidatesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("memory candidates not found")
		}
		return nil, fmt.Errorf("read memory candidates: %w", err)
	}
	candidates := ParseCandidates(string(data))
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no memory candidates found")
	}
	return candidates, nil
}

func (s Store) candidateByIndex(demandID string, candidateIndex int) (Candidate, error) {
	if candidateIndex < 1 {
		return Candidate{}, fmt.Errorf("candidate index out of range")
	}
	candidates, err := s.loadCandidates(demandID)
	if err != nil {
		return Candidate{}, err
	}
	for _, candidate := range candidates {
		if candidate.Index == candidateIndex {
			return candidate, nil
		}
	}
	return Candidate{}, fmt.Errorf("candidate index out of range")
}

func (s Store) loadDecisions(demandID string) (map[int]memoryDecision, error) {
	eventPath, err := s.eventsPath(demandID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	decisions := map[int]memoryDecision{}
	for lineNo, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event artifacts.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode events line %d: %w", lineNo+1, err)
		}
		index, err := strconv.Atoi(event.Data["candidate_index"])
		if err != nil || index < 1 {
			continue
		}
		switch event.Type {
		case "memory.promoted":
			decisions[index] = memoryDecision{Status: CandidatePromoted, StablePath: event.Data["stable_path"]}
		case "memory.rejected":
			decisions[index] = memoryDecision{Status: CandidateRejected, Reason: event.Data["reason"]}
		}
	}
	return decisions, nil
}

func (s Store) eventsPath(demandID string) (string, error) {
	if _, err := artifacts.NewStore(s.root).LoadDemand(demandID); err != nil {
		return "", err
	}
	return filepath.Join(s.root, ".devflow", "demands", demandID, artifacts.EventsFile), nil
}

func (s Store) ensureStableMemoryDir() (string, error) {
	if s.root == "" {
		return "", fmt.Errorf("store root is required")
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve store root: %w", err)
	}
	memDir := filepath.Join(rootAbs, ".devflow", memoryDirName)
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return "", fmt.Errorf("create stable memory directory: %w", err)
	}
	return memDir, nil
}

func normalizeStableName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = stableNamePattern.ReplaceAllString(normalized, "-")
	return strings.Trim(normalized, "-")
}

func stableMemoryBody(name, description, candidate, demandID, by string, promotedAt time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + name + "\n")
	b.WriteString("description: " + description + "\n")
	b.WriteString("type: project\n")
	b.WriteString("source_demand: " + demandID + "\n")
	b.WriteString("promoted_at: " + promotedAt.Format(time.RFC3339) + "\n")
	b.WriteString("promoted_by: " + by + "\n")
	b.WriteString("---\n\n")
	b.WriteString("# " + name + "\n\n")
	b.WriteString(candidate + "\n\n")
	b.WriteString("Why: This rule was confirmed during demand " + demandID + ".\n\n")
	b.WriteString("How to apply: Reuse this rule when generating requirements or plans for similar backend demand work.\n")
	return b.String()
}

func appendMemoryIndex(indexPath, name, fileName, description string) error {
	entry := "- [" + name + "](" + fileName + ") - " + description
	data, err := os.ReadFile(indexPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read MEMORY.md: %w", err)
	}
	text := string(data)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	if strings.TrimSpace(text) == "" {
		text = entry + "\n"
	} else if strings.HasSuffix(text, "\n") {
		text += entry + "\n"
	} else {
		text += "\n" + entry + "\n"
	}
	return writeTextAtomic(indexPath, text)
}

func appendMemoryEvent(path string, event artifacts.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events log: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync events log: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close events log: %w", err)
	}
	return nil
}

func writeTextAtomic(path string, contents string) (err error) {
	tempFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp text file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err = tempFile.WriteString(contents); err != nil {
		tempFile.Close()
		return fmt.Errorf("write text file: %w", err)
	}
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("sync text file: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("close text file: %w", err)
	}
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename text file: %w", err)
	}
	return nil
}
```

Modify `internal/memory/store.go`:

```go
type Result struct {
	DemandID string
	Path     string
	Snippet  string
	Source   Source
}
```

Inside existing candidate `Search`, set:

```go
Source: SourceCandidate,
```

- [ ] **Step 4: Run memory package tests**

```powershell
gofmt -w internal\memory\candidates.go internal\memory\candidates_test.go internal\memory\decisions.go internal\memory\decisions_test.go internal\memory\store.go
go test ./internal/memory -count=1
```

Expected: pass.

- [ ] **Step 5: Commit store promotion/rejection**

```powershell
git add internal\memory
git commit -m @'
Promote approved memory candidates into project memory

Wave 12 needs a product-layer bridge from demand-local candidates to stable .devflow memory files with auditable human decisions.

Constraint: Stable memories must use the migrated MewCode-compatible .devflow/memory format
Rejected: Treat memory-candidates.md as stable memory | candidates are not human-approved
Confidence: high
Scope-risk: moderate
Directive: Do not bypass memory.promoted or memory.rejected events when changing promotion behavior
Tested: go test ./internal/memory -count=1
'@
```

---

### Task 3: Stable Memory Search And Demand Context Injection

**Files:**
- Modify: `internal/memory/store.go`
- Modify: `internal/memory/store_test.go`
- Modify: `internal/demandflow/types.go`
- Modify: `internal/demandflow/context.go`
- Modify: `internal/demandflow/context_test.go`
- Modify: `internal/demandflow/prompts.go`
- Create or modify: `internal/demandflow/prompts_test.go`

- [ ] **Step 1: Write failing stable search test**

Append to `internal/memory/store_test.go`:

```go
func TestStoreSearchStableMemory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	memDir := filepath.Join(root, ".devflow", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("MkdirAll memory dir: %v", err)
	}
	body := `---
name: coupon-eligibility-policy
description: membership gates coupon eligibility
type: project
---

# coupon-eligibility-policy

Active membership must be checked before coupon discount rules.
`
	if err := os.WriteFile(filepath.Join(memDir, "coupon-eligibility-policy.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write memory file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "MEMORY.md"), []byte("- [coupon](coupon-eligibility-policy.md)\n"), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	got, err := NewStore(root).SearchStable("membership coupon")
	if err != nil {
		t.Fatalf("SearchStable returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SearchStable returned %d results: %#v", len(got), got)
	}
	if got[0].Source != SourceStable {
		t.Fatalf("Source = %q, want stable", got[0].Source)
	}
	if got[0].DemandID != "" {
		t.Fatalf("DemandID = %q, want empty for stable memory", got[0].DemandID)
	}
	if !strings.Contains(got[0].Snippet, "membership gates coupon eligibility") {
		t.Fatalf("Snippet = %q, want description", got[0].Snippet)
	}
}
```

- [ ] **Step 2: Write failing context/prompt tests**

Modify `internal/demandflow/context_test.go` with a new test:

```go
func TestContextLoaderLoadsStableMemoryBeforeCandidateMemory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "prior-work", Title: "coupon flow", Description: "coupon flow", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create prior: %v", err)
	}
	if err := store.WriteArtifact("prior-work", artifacts.MemoryCandidatesFile, "## 稳定知识候选\n\n- coupon candidate knowledge\n"); err != nil {
		t.Fatalf("write prior memory: %v", err)
	}
	if err := store.CreateDemand(artifacts.Demand{ID: "add-coupon-check", Title: "coupon flow", Description: "coupon flow", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create current: %v", err)
	}
	memDir := filepath.Join(root, ".devflow", "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("MkdirAll memory dir: %v", err)
	}
	stable := `---
name: coupon-eligibility-policy
description: stable coupon policy
type: project
---

Stable coupon memory body.
`
	if err := os.WriteFile(filepath.Join(memDir, "coupon-eligibility-policy.md"), []byte(stable), 0o644); err != nil {
		t.Fatalf("write stable memory: %v", err)
	}

	snapshot, err := newContextLoader(root).Load("add-coupon-check")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(snapshot.Memories) < 2 {
		t.Fatalf("expected stable and candidate memories, got %#v", snapshot.Memories)
	}
	if snapshot.Memories[0].Source != "stable" {
		t.Fatalf("first memory source = %q, want stable; memories=%#v", snapshot.Memories[0].Source, snapshot.Memories)
	}
	if snapshot.Memories[1].Source != "candidate" {
		t.Fatalf("second memory source = %q, want candidate; memories=%#v", snapshot.Memories[1].Source, snapshot.Memories)
	}
}
```

Create `internal/demandflow/prompts_test.go`:

```go
package demandflow

import (
	"strings"
	"testing"
)

func TestRenderMemoryHitsSeparatesStableAndCandidate(t *testing.T) {
	t.Parallel()

	got := renderMemoryHits([]MemoryHit{
		{Source: "stable", Path: ".devflow/memory/coupon.md", Snippet: "stable coupon policy"},
		{Source: "candidate", DemandID: "prior-work", Path: ".devflow/demands/prior-work/memory-candidates.md", Snippet: "candidate coupon note"},
	})

	for _, want := range []string{
		"Approved stable memory:",
		"- .devflow/memory/coupon.md: stable coupon policy",
		"Unapproved candidate memory:",
		"- prior-work: candidate coupon note",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderMemoryHits missing %q:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 3: Run tests and verify failure**

```powershell
go test ./internal/memory -run TestStoreSearchStableMemory -count=1
go test ./internal/demandflow -run "TestContextLoaderLoadsStableMemoryBeforeCandidateMemory|TestRenderMemoryHitsSeparatesStableAndCandidate" -count=1
```

Expected: fail with missing `SearchStable`, missing `Source` field on `MemoryHit`, and old rendering.

- [ ] **Step 4: Implement stable search**

Add to `internal/memory/store.go`:

```go
func (s Store) SearchStable(query string) ([]Result, error) {
	if s.root == "" {
		return nil, fmt.Errorf("store root is required")
	}
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil, fmt.Errorf("query is required")
	}

	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return nil, fmt.Errorf("resolve store root: %w", err)
	}
	rootAbs = filepath.Clean(rootAbs)
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("resolve store root: %w", err)
	}
	memDir := filepath.Join(rootAbs, ".devflow", "memory")
	expectedMemDir := filepath.Join(rootResolved, ".devflow", "memory")
	exists, err := ensureSafePath(memDir, expectedMemDir)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	entries, err := os.ReadDir(memDir)
	if err != nil {
		return nil, fmt.Errorf("read stable memory directory: %w", err)
	}
	results := make([]Result, 0)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "MEMORY.md" || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(memDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read stable memory %s: %w", entry.Name(), err)
		}
		text := string(data)
		if !matchesAll(strings.ToLower(text), terms) {
			continue
		}
		results = append(results, Result{
			Path:    path,
			Snippet: stableSnippet(text),
			Source:  SourceStable,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})
	return results, nil
}

func stableSnippet(text string) string {
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "description:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		}
	}
	return firstLine(text)
}
```

- [ ] **Step 5: Implement demandflow memory source separation**

Modify `internal/demandflow/types.go`:

```go
type MemoryHit struct {
	DemandID string
	Path     string
	Snippet  string
	Source   string
}
```

Modify `internal/demandflow/context.go` memory loading block:

```go
memoryStore := memorystore.NewStore(l.root)
query := demand.Title + " " + demand.Description

if hits, err := memoryStore.SearchStable(query); err == nil {
	for _, hit := range hits {
		snapshot.Memories = append(snapshot.Memories, MemoryHit{
			Path:    hit.Path,
			Snippet: hit.Snippet,
			Source: string(hit.Source),
		})
	}
}

if hits, err := memoryStore.Search(query); err == nil {
	for _, hit := range hits {
		if hit.DemandID == demand.ID {
			continue
		}
		snapshot.Memories = append(snapshot.Memories, MemoryHit{
			DemandID: hit.DemandID,
			Path:     hit.Path,
			Snippet:  hit.Snippet,
			Source:   string(hit.Source),
		})
	}
}
```

Modify `internal/demandflow/prompts.go` `renderMemoryHits`:

```go
func renderMemoryHits(hits []MemoryHit) string {
	if len(hits) == 0 {
		return "(none)"
	}

	var stable []MemoryHit
	var candidates []MemoryHit
	for _, hit := range hits {
		if hit.Source == "stable" {
			stable = append(stable, hit)
			continue
		}
		candidates = append(candidates, hit)
	}

	var b strings.Builder
	if len(stable) > 0 {
		b.WriteString("Approved stable memory:\n")
		for _, hit := range stable {
			b.WriteString("- ")
			b.WriteString(hit.Path)
			if strings.TrimSpace(hit.Snippet) != "" {
				b.WriteString(": ")
				b.WriteString(strings.TrimSpace(hit.Snippet))
			}
			b.WriteString("\n")
		}
	}
	if len(candidates) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Unapproved candidate memory:\n")
		for _, hit := range candidates {
			b.WriteString("- ")
			b.WriteString(hit.DemandID)
			if strings.TrimSpace(hit.Snippet) != "" {
				b.WriteString(": ")
				b.WriteString(strings.TrimSpace(hit.Snippet))
			}
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
```

- [ ] **Step 6: Run memory and demandflow tests**

```powershell
gofmt -w internal\memory\store.go internal\memory\store_test.go internal\demandflow\types.go internal\demandflow\context.go internal\demandflow\context_test.go internal\demandflow\prompts.go internal\demandflow\prompts_test.go
go test ./internal/memory -count=1
go test ./internal/demandflow -count=1
```

Expected: pass.

- [ ] **Step 7: Commit context reuse**

```powershell
git add internal\memory internal\demandflow
git commit -m @'
Surface approved memory before candidate recall

Future demands need stable project memory ahead of unapproved demand-local candidates so prompts preserve the trust boundary from Wave 12.

Constraint: Existing demand candidate recall must continue excluding the current demand
Rejected: Merge stable and candidate memories into one list without labels | prompts would obscure approval status
Confidence: high
Scope-risk: moderate
Directive: Keep stable memory rendered before candidate memory in requirements and plan prompts
Tested: go test ./internal/memory -count=1; go test ./internal/demandflow -count=1
'@
```

---

### Task 4: CLI Memory Commands

**Files:**
- Create: `internal/cli/memory.go`
- Create: `internal/cli/memory_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write failing CLI tests**

Create `internal/cli/memory_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func TestMemoryListShowsNumberedCandidates(t *testing.T) {
	t.Parallel()

	root := seedCLIMemoryDemand(t)
	var stdout bytes.Buffer

	err := Run([]string{"memory", "list", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run memory list returned error: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Memory candidates for add-coupon-check",
		"1. [pending] Active membership must be checked before coupon discount rules.",
		"2. [pending] Coupon errors should preserve the original order validation message.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("memory list missing %q:\n%s", want, output)
		}
	}
}

func TestMemoryPromoteWritesStableMemory(t *testing.T) {
	t.Parallel()

	root := seedCLIMemoryDemand(t)
	var stdout bytes.Buffer

	err := Run([]string{
		"memory", "promote",
		"--root", root,
		"--demand", "add-coupon-check",
		"--candidate", "1",
		"--name", "coupon-eligibility-policy",
		"--description", "membership gates coupon eligibility",
		"--by", "dd",
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run memory promote returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "promoted candidate 1 for add-coupon-check") {
		t.Fatalf("promote output = %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".devflow", "memory", "coupon-eligibility-policy.md")); err != nil {
		t.Fatalf("stable memory file missing: %v", err)
	}
}

func TestMemoryRejectUpdatesListStatus(t *testing.T) {
	t.Parallel()

	root := seedCLIMemoryDemand(t)
	if err := Run([]string{
		"memory", "reject",
		"--root", root,
		"--demand", "add-coupon-check",
		"--candidate", "2",
		"--by", "dd",
		"--reason", "too specific",
	}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run memory reject returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"memory", "list", "--root", root, "--demand", "add-coupon-check"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run memory list returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "2. [rejected] Coupon errors should preserve the original order validation message.") {
		t.Fatalf("list output missing rejected status:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "reason: too specific") {
		t.Fatalf("list output missing rejection reason:\n%s", stdout.String())
	}
}

func TestMemoryCommandRequiresSubcommand(t *testing.T) {
	t.Parallel()

	err := Run([]string{"memory"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "memory subcommand is required") {
		t.Fatalf("Run memory error = %v, want subcommand required", err)
	}
}

func seedCLIMemoryDemand(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "add-coupon-check", Title: "coupon", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	body := `# Memory Candidates: coupon

## 稳定知识候选

- Active membership must be checked before coupon discount rules.
- Coupon errors should preserve the original order validation message.
`
	if err := store.WriteArtifact("add-coupon-check", artifacts.MemoryCandidatesFile, body); err != nil {
		t.Fatalf("WriteArtifact returned error: %v", err)
	}
	return root
}
```

- [ ] **Step 2: Run CLI tests and verify failure**

```powershell
go test ./internal/cli -run TestMemory -count=1
```

Expected: fail with unknown command `memory`.

- [ ] **Step 3: Implement CLI command**

Create `internal/cli/memory.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	memorystore "github.com/jesseedcp/devflow-agent/internal/memory"
)

func runMemory(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("memory subcommand is required")
	}
	switch args[0] {
	case "list":
		return runMemoryList(args[1:], stdout, stderr)
	case "promote":
		return runMemoryPromote(args[1:], stdout, stderr)
	case "reject":
		return runMemoryReject(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown memory subcommand %q", args[0])
	}
}

func runMemoryList(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("memory list", flag.ContinueOnError)
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
	candidates, err := memorystore.NewStore(root).ListCandidates(demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Memory candidates for %s\n", demandID)
	for _, candidate := range candidates {
		fmt.Fprintf(stdout, "%d. [%s] %s\n", candidate.Index, candidate.Status, candidate.Text)
		if candidate.StablePath != "" {
			fmt.Fprintf(stdout, "   stable: %s\n", candidate.StablePath)
		}
		if candidate.Reason != "" {
			fmt.Fprintf(stdout, "   reason: %s\n", candidate.Reason)
		}
	}
	return nil
}

func runMemoryPromote(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("memory promote", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, candidateRaw, name, description, by string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&candidateRaw, "candidate", "", "candidate number")
	fs.StringVar(&name, "name", "", "stable memory name")
	fs.StringVar(&description, "description", "", "stable memory description")
	fs.StringVar(&by, "by", "", "promoting person")
	if err := fs.Parse(args); err != nil {
		return err
	}
	candidateIndex, err := strconv.Atoi(strings.TrimSpace(candidateRaw))
	if err != nil || candidateIndex < 1 {
		return fmt.Errorf("--candidate must be a positive integer")
	}
	result, err := memorystore.NewStore(root).PromoteCandidate(memorystore.PromoteOptions{
		DemandID:       strings.TrimSpace(demandID),
		CandidateIndex: candidateIndex,
		Name:           name,
		Description:    description,
		By:             by,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "promoted candidate %d for %s\n%s\n", result.Candidate.Index, strings.TrimSpace(demandID), result.Path)
	return nil
}

func runMemoryReject(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("memory reject", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, candidateRaw, by, reason string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&candidateRaw, "candidate", "", "candidate number")
	fs.StringVar(&by, "by", "", "rejecting person")
	fs.StringVar(&reason, "reason", "", "rejection reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	candidateIndex, err := strconv.Atoi(strings.TrimSpace(candidateRaw))
	if err != nil || candidateIndex < 1 {
		return fmt.Errorf("--candidate must be a positive integer")
	}
	candidate, err := memorystore.NewStore(root).RejectCandidate(memorystore.RejectOptions{
		DemandID:       strings.TrimSpace(demandID),
		CandidateIndex: candidateIndex,
		By:             by,
		Reason:         reason,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "rejected candidate %d for %s\n", candidate.Index, strings.TrimSpace(demandID))
	return nil
}
```

Modify `internal/cli/cli.go` help text:

```text
  devflow memory list --demand <id>
  devflow memory promote --demand <id> --candidate <n> --by <name>
  devflow memory reject --demand <id> --candidate <n> --by <name> --reason <text>
```

Add command description:

```text
  memory    Review, promote, and reject stable knowledge candidates
```

Add dispatch:

```go
case "memory":
	return runMemory(args[1:], stdout, stderr)
```

- [ ] **Step 4: Run CLI tests**

```powershell
gofmt -w internal\cli\cli.go internal\cli\memory.go internal\cli\memory_test.go
go test ./internal/cli -run TestMemory -count=1
```

Expected: pass.

- [ ] **Step 5: Commit CLI**

```powershell
git add internal\cli
git commit -m @'
Expose memory review commands in the CLI

Users need a concrete promote and reject surface so stable knowledge only enters project memory after explicit human action.

Constraint: Wave 12 promotion must be operable without the TUI
Rejected: Hide promotion behind closeout | automatic promotion would blur the human gate
Confidence: high
Scope-risk: moderate
Directive: Keep devflow memory commands deterministic and scriptable
Tested: go test ./internal/cli -run TestMemory -count=1
'@
```

---

### Task 5: Documentation And Dogfood Evidence

**Files:**
- Modify: `docs/user-guide/backend-demand-loop.md`
- Modify: `docs/release/v0.1.md`
- Modify: `internal/demandflow/e2e_test.go` or create `internal/demandflow/memory_e2e_test.go`

- [ ] **Step 1: Add an end-to-end memory reuse test**

Create `internal/demandflow/memory_e2e_test.go`:

```go
package demandflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	memorystore "github.com/jesseedcp/devflow-agent/internal/memory"
)

func TestStableMemoryPromotedFromOneDemandAppearsInNextDemandPrompt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "first-coupon-demand", Title: "coupon flow", Description: "coupon membership", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create first demand: %v", err)
	}
	if err := store.WriteArtifact("first-coupon-demand", artifacts.MemoryCandidatesFile, "## 稳定知识候选\n\n- Active membership must be checked before coupon discount rules.\n"); err != nil {
		t.Fatalf("write first candidates: %v", err)
	}
	if _, err := memorystore.NewStore(root).PromoteCandidate(memorystore.PromoteOptions{
		DemandID:       "first-coupon-demand",
		CandidateIndex: 1,
		Name:           "coupon-membership-gate",
		Description:    "membership gates coupon discount rules",
		By:             "dd",
		Now:            func() time.Time { return time.Date(2026, 6, 30, 10, 30, 0, 0, time.UTC) },
	}); err != nil {
		t.Fatalf("PromoteCandidate returned error: %v", err)
	}

	if err := store.CreateDemand(artifacts.Demand{ID: "second-coupon-demand", Title: "coupon discount", Description: "coupon membership", Source: "manual", State: "created"}); err != nil {
		t.Fatalf("create second demand: %v", err)
	}
	snapshot, err := newContextLoader(root).Load("second-coupon-demand")
	if err != nil {
		t.Fatalf("load second context: %v", err)
	}
	prompt := requirementsPrompt(snapshot)
	for _, want := range []string{
		"Approved stable memory:",
		"membership gates coupon discount rules",
		"Unapproved candidate memory:",
		"first-coupon-demand",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("requirements prompt missing %q:\n%s", want, prompt)
		}
	}

	if _, err := os.Stat(filepath.Join(root, ".devflow", "memory", "MEMORY.md")); err != nil {
		t.Fatalf("MEMORY.md missing: %v", err)
	}
}
```

- [ ] **Step 2: Run e2e test and verify failure or pass**

```powershell
go test ./internal/demandflow -run TestStableMemoryPromotedFromOneDemandAppearsInNextDemandPrompt -count=1
```

Expected after Tasks 2-4: pass. If it fails because stable search requires all query words and the description lacks one term, adjust the test description or stable body so both `coupon` and `membership` appear.

- [ ] **Step 3: Document the user workflow**

In `docs/user-guide/backend-demand-loop.md`, add this section after closeout guidance:

```markdown
### Stable knowledge review

After closeout, Devflow writes reviewable knowledge candidates to `memory-candidates.md`. These are not stable memory until a human promotes them.

List candidates:

```powershell
devflow memory list --demand add-coupon-check
```

Promote an approved candidate into project memory:

```powershell
devflow memory promote --demand add-coupon-check --candidate 1 --name coupon-eligibility-policy --description "membership gates coupon eligibility" --by dd
```

Reject a candidate that is too narrow or stale:

```powershell
devflow memory reject --demand add-coupon-check --candidate 2 --by dd --reason "too specific to one fixture"
```

Promoted memories are stored under `.devflow/memory/` and indexed in `.devflow/memory/MEMORY.md`. Future requirements and plan stages render approved stable memory before unapproved candidate memory.
```

In `docs/release/v0.1.md`, add one bullet:

```markdown
- Wave 12 adds human-gated stable knowledge promotion: `devflow memory list`, `devflow memory promote`, and `devflow memory reject` connect closeout candidates to `.devflow/memory/` reuse.
```

- [ ] **Step 4: Run docs/e2e verification**

```powershell
gofmt -w internal\demandflow\memory_e2e_test.go
go test ./internal/demandflow -run TestStableMemoryPromotedFromOneDemandAppearsInNextDemandPrompt -count=1
git diff --check
```

Expected: pass.

- [ ] **Step 5: Commit docs and dogfood evidence**

```powershell
git add internal\demandflow\memory_e2e_test.go docs\user-guide\backend-demand-loop.md docs\release\v0.1.md
git commit -m @'
Document stable memory promotion workflow

Wave 12 needs visible dogfood evidence and user-facing commands so closeout knowledge candidates become a repeatable review loop.

Constraint: Promoted memory must appear in the next demand prompt before candidates
Rejected: Rely on unit tests only | this feature is a cross-stage product loop
Confidence: high
Scope-risk: narrow
Tested: go test ./internal/demandflow -run TestStableMemoryPromotedFromOneDemandAppearsInNextDemandPrompt -count=1; git diff --check
'@
```

---

### Task 6: Full Verification, Fixes, And PR

**Files:**
- Modify only files needed to fix verification failures.

- [ ] **Step 1: Run package-level tests**

```powershell
go test ./internal/memory -count=1
go test ./internal/demandflow -count=1
go test ./internal/cli -count=1
```

Expected: all pass.

- [ ] **Step 2: Run repository verification**

```powershell
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git diff --check
```

Expected: all pass. If `go test ./...` hits the known Windows transient mailbox file-lock test, rerun the failing package once; only treat it as acceptable if the rerun passes without code changes.

- [ ] **Step 3: Manual CLI smoke test**

Use a temporary root:

```powershell
$root = Join-Path $env:TEMP "devflow-wave12-smoke"
Remove-Item -Recurse -Force $root -ErrorAction SilentlyContinue
go run ./cmd/devflow start --root $root --title "Coupon memory smoke" --description "coupon membership"
go run ./cmd/devflow closeout --root $root --demand coupon-memory-smoke --result "done" --knowledge "Active membership must be checked before coupon discount rules."
go run ./cmd/devflow memory list --root $root --demand coupon-memory-smoke
go run ./cmd/devflow memory promote --root $root --demand coupon-memory-smoke --candidate 1 --name coupon-membership-gate --description "membership gates coupon discounts" --by dd
Get-Content (Join-Path $root ".devflow\memory\MEMORY.md")
```

Expected:

```text
Memory candidates for coupon-memory-smoke
1. [pending] Active membership must be checked before coupon discount rules.
promoted candidate 1 for coupon-memory-smoke
- [coupon-membership-gate](coupon-membership-gate.md) - membership gates coupon discounts
```

- [ ] **Step 4: Commit verification fixes if needed**

If Step 1-3 required code changes, commit them:

```powershell
git add internal\memory internal\demandflow internal\cli docs\user-guide\backend-demand-loop.md docs\release\v0.1.md
git commit -m @'
Stabilize Wave 12 memory promotion verification

Final verification exposed small integration issues in the memory promotion loop, so this change keeps the delivered behavior aligned with the Wave 12 spec.

Constraint: Repository-wide Go verification must pass on Windows
Confidence: high
Scope-risk: narrow
Tested: go vet ./...; go build ./cmd/devflow; go test ./... -count=1 -timeout 5m; git diff --check
'@
```

If no code changes were needed, skip this commit.

- [ ] **Step 5: Push branch and open PR**

```powershell
git status --short --branch
git push -u origin feature/devflow-wave-12
gh pr create --base main --head feature/devflow-wave-12 --title "Wave 12 stable knowledge promotion" --body "Adds human-gated stable knowledge promotion and reuse for Devflow memory candidates."
```

Expected:

```text
https://github.com/jesseedcp/devflow-agent/pull/<number>
```

If `gh` is not authenticated, push the branch and report the PR creation blocker with the exact `gh auth status` output.

## Final Self-Review Checklist

- [ ] Spec section 2 product chain is covered by Tasks 2-5.
- [ ] Spec section 3 MewCode reuse is preserved by storing approved memory in `.devflow/memory/` and not changing `internal/runtime/memory`.
- [ ] Spec section 4 candidate/stable separation is covered by `SourceCandidate`, `SourceStable`, `memory.promoted`, and prompt labels.
- [ ] Spec section 5 CLI commands are covered by Task 4.
- [ ] Spec section 6 file format is covered by `stableMemoryBody`.
- [ ] Spec section 7 next-demand reuse is covered by Task 3 and Task 5 e2e.
- [ ] Spec section 10 error cases are covered by store and CLI validation tests.
- [ ] Spec section 11 verification commands are covered by Task 6.

## Handoff Notes

- Keep commits small. Do not combine parser, store, CLI, and docs in one commit.
- Do not auto-promote candidates during `closeout`.
- Do not write `~/.devflow/memory/` in this wave.
- Do not add a vector database or new dependency.
- Use `gofmt` after each Go edit.
- Use Lore Commit Protocol for every commit.
