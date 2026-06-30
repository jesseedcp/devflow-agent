# Wave 17 TUI Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `devflow workbench`, an interactive product TUI for demand selection, summary inspection, and safe actions.

**Architecture:** Keep workflow rules in demandflow and action execution in CLI helpers. Add a small Bubble Tea model that consumes `ConsoleSummary`, `DemandEvaluation`, and drive/runner helpers through injectable functions so model tests do not launch a real terminal.

**Tech Stack:** Go, Bubble Tea already in repo, existing CLI patterns, existing console/evaluate/drive helpers.

---

## File Structure

- Create `internal/cli/workbench.go` for command parsing and program launch.
- Create `internal/cli/workbench_model.go` for Bubble Tea model/update/view.
- Create `internal/cli/workbench_test.go` for CLI dispatch and model behavior.
- Modify `internal/cli/cli.go` help and dispatch.
- Modify docs and release notes.

## Task 1: Workbench CLI Dispatch

**Files:**
- Create: `internal/cli/workbench.go`
- Create: `internal/cli/workbench_test.go`
- Modify: `internal/cli/cli.go`

- [ ] **Step 1: Write failing dispatch tests**

Create `internal/cli/workbench_test.go`:

```go
package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestWorkbenchDispatchesProgramRunner(t *testing.T) {
	old := runWorkbenchProgram
	defer func() { runWorkbenchProgram = old }()
	var called bool
	runWorkbenchProgram = func(opts workbenchOptions) error {
		called = true
		if opts.root != "repo" {
			t.Fatalf("root = %q, want repo", opts.root)
		}
		if !opts.noAltScreen {
			t.Fatal("noAltScreen = false, want true")
		}
		return nil
	}

	if err := Run([]string{"workbench", "--root", "repo", "--no-alt-screen"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench returned error: %v", err)
	}
	if !called {
		t.Fatal("runWorkbenchProgram was not called")
	}
}

func TestWorkbenchHelpIncludesCommand(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "devflow workbench") {
		t.Fatalf("help missing workbench:\n%s", stdout.String())
	}
}
```

- [ ] **Step 2: Run failing tests**

Run:

```powershell
go test ./internal/cli -run TestWorkbench -count=1
```

Expected: unknown command or missing symbols.

- [ ] **Step 3: Implement dispatch**

Create `internal/cli/workbench.go`:

```go
package cli

import (
	"flag"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type workbenchOptions struct {
	root           string
	configPath     string
	noAltScreen    bool
	qualityCommand stringSliceFlag
}

var runWorkbenchProgram = func(opts workbenchOptions) error {
	model := newWorkbenchModel(opts)
	options := []tea.ProgramOption{}
	if !opts.noAltScreen {
		options = append(options, tea.WithAltScreen())
	}
	program := tea.NewProgram(model, options...)
	_, err := program.Run()
	return err
}

func runWorkbench(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseWorkbenchArgs(args, stderr)
	if err != nil {
		return err
	}
	return runWorkbenchProgram(opts)
}

func parseWorkbenchArgs(args []string, stderr io.Writer) (workbenchOptions, error) {
	fs := flag.NewFlagSet("workbench", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var opts workbenchOptions
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.configPath, "config", "", "devflow config path")
	fs.BoolVar(&opts.noAltScreen, "no-alt-screen", false, "disable alternate screen")
	fs.Var(&opts.qualityCommand, "quality-command", "quality command for run actions")
	if err := fs.Parse(args); err != nil {
		return workbenchOptions{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	return opts, nil
}
```

Modify `cli.go` help and dispatch.

- [ ] **Step 4: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/workbench.go internal/cli/workbench_test.go internal/cli/cli.go
go test ./internal/cli -run TestWorkbench -count=1
git add internal/cli/workbench.go internal/cli/workbench_test.go internal/cli/cli.go
git commit -m "Add workbench command dispatch" -m "Wave 17 starts with a testable CLI entrypoint for the product workbench TUI." -m "Constraint: Dispatch tests stub the Bubble Tea runner and do not launch a terminal." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./internal/cli -run TestWorkbench -count=1"
```

## Task 2: Build Workbench Model

**Files:**
- Create: `internal/cli/workbench_model.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Add model tests**

Add:

```go
func TestWorkbenchModelRendersDemandList(t *testing.T) {
	model := workbenchModel{
		demands: []workbenchDemand{{ID: "a", State: "verification", Attention: "ready"}, {ID: "b", State: "completed", Attention: "complete"}},
	}
	view := model.View()
	for _, want := range []string{"Devflow Workbench", "a", "verification", "ready"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view missing %q:\n%s", want, view)
		}
	}
}

func TestWorkbenchModelSelectionMoves(t *testing.T) {
	model := workbenchModel{demands: []workbenchDemand{{ID: "a"}, {ID: "b"}}}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := next.(workbenchModel)
	if updated.selected != 1 {
		t.Fatalf("selected = %d, want 1", updated.selected)
	}
}
```

Add Bubble Tea import in test.

- [ ] **Step 2: Implement model**

Create `internal/cli/workbench_model.go`:

```go
package cli

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

type workbenchDemand struct {
	ID        string
	State     string
	Attention string
}

type workbenchModel struct {
	opts     workbenchOptions
	demands  []workbenchDemand
	selected int
	message  string
	err      string
}

func newWorkbenchModel(opts workbenchOptions) workbenchModel {
	summaries, err := demandflow.ListConsole(opts.root)
	model := workbenchModel{opts: opts}
	if err != nil {
		model.err = err.Error()
		return model
	}
	for _, summary := range summaries {
		model.demands = append(model.demands, workbenchDemand{
			ID:        summary.Workspace.Demand.ID,
			State:     string(summary.Workspace.State),
			Attention: summary.Workspace.Attention,
		})
	}
	return model
}

func (m workbenchModel) Init() tea.Cmd {
	return nil
}

func (m workbenchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "down", "j":
			if m.selected < len(m.demands)-1 {
				m.selected++
			}
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "R":
			return newWorkbenchModel(m.opts), nil
		}
	}
	return m, nil
}

func (m workbenchModel) View() string {
	var b strings.Builder
	b.WriteString("Devflow Workbench\n\n")
	if m.err != "" {
		b.WriteString("Error: " + m.err + "\n")
		return b.String()
	}
	if len(m.demands) == 0 {
		b.WriteString("No demands found\n")
		return b.String()
	}
	for index, demand := range m.demands {
		prefix := "  "
		if index == m.selected {
			prefix = "> "
		}
		b.WriteString(fmt.Sprintf("%s%-24s %-18s %s\n", prefix, demand.ID, demand.State, demand.Attention))
	}
	b.WriteString("\nKeys: up/down select  R refresh  q quit\n")
	return b.String()
}
```

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/workbench_model.go internal/cli/workbench_test.go
go test ./internal/cli -run TestWorkbench -count=1
git add internal/cli/workbench_model.go internal/cli/workbench_test.go
git commit -m "Render demand list in workbench model" -m "The product TUI now has a small testable Bubble Tea model for demand list selection and refresh." -m "Constraint: Workbench view consumes ConsoleSummary and does not own workflow rules." -m "Confidence: medium" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -run TestWorkbench -count=1"
```

## Task 3: Add Detail And Actions

**Files:**
- Modify: `internal/cli/workbench_model.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Add tests for detail and blocked action messages**

Test that Enter toggles detail and view includes `Next`. Test that `r` on a human gate sets message `Blocked`.

- [ ] **Step 2: Implement details**

Extend model to load selected `demandflow.InspectConsole(root, id)` for detail. Render:

```text
Summary
State:
Attention:
Next:
Run-ready:
```

For `r`, call an injectable function:

```go
var workbenchRunNext = func(opts workbenchOptions, demandID string) string
```

First implementation can return block messages using `InspectConsole` without executing. Execution wiring can be added once model behavior is stable.

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/workbench_model.go internal/cli/workbench_test.go
go test ./internal/cli -run TestWorkbench -count=1
git add internal/cli/workbench_model.go internal/cli/workbench_test.go
git commit -m "Show selected demand details in workbench" -m "Workbench can now toggle a concise selected-demand detail view and show action block messages without duplicating workflow rules." -m "Constraint: Detail view uses InspectConsole rather than recomputing status." -m "Confidence: medium" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -run TestWorkbench -count=1"
```

## Task 4: Wire Run, Drive, Evaluate Shortcuts

**Files:**
- Modify: `internal/cli/workbench_model.go`
- Modify: `internal/cli/workbench_test.go`

- [ ] **Step 1: Add shortcut tests**

Add tests for keys:

- `r` calls `workbenchRunNext`;
- `d` calls `workbenchDrive`;
- `e` calls `workbenchEvaluate`;
- each updates `message`.

- [ ] **Step 2: Implement injectable actions**

Add package variables:

```go
var workbenchRunNext = func(opts workbenchOptions, demandID string) string { return "run-next not configured" }
var workbenchDrive = func(opts workbenchOptions, demandID string) string { return "drive not configured" }
var workbenchEvaluate = func(opts workbenchOptions, demandID string) string { return "evaluate not configured" }
```

Wire keys `r`, `d`, `e`. If Wave 15/16 commands are already implemented, call their helpers directly. If implementation order runs Wave 17 before 15/16, keep injected functions returning an explanatory message and add wiring after those waves land.

- [ ] **Step 3: Verify and commit**

Run:

```powershell
gofmt -w internal/cli/workbench_model.go internal/cli/workbench_test.go
go test ./internal/cli -run TestWorkbench -count=1
git add internal/cli/workbench_model.go internal/cli/workbench_test.go
git commit -m "Add workbench action shortcuts" -m "The workbench model now exposes keyboard hooks for run-next, drive, and evaluate through injectable functions so behavior remains testable." -m "Constraint: Shortcuts must refresh or report blocked actions without bypassing console safety." -m "Confidence: medium" -m "Scope-risk: moderate" -m "Tested: go test ./internal/cli -run TestWorkbench -count=1"
```

## Task 5: Docs And Full Verification

- [ ] Add user guide section:

```markdown
### Workbench TUI

Use `devflow workbench` for an interactive demand list and selected-demand operator view.
```

- [ ] Add release note:

```markdown
### Wave 17 - TUI Workbench

- Adds `devflow workbench`, a product demand TUI built on ConsoleSummary.
- Supports demand selection, detail view, refresh, and action shortcuts.
```

- [ ] Run:

```powershell
go test ./internal/cli -run TestWorkbench -count=1
go test ./internal/cli -count=1
go vet ./...
go build ./cmd/devflow
go test ./... -count=1 -timeout 5m
git diff --check
```

- [ ] Commit docs:

```powershell
git add docs/user-guide/backend-demand-loop.md docs/release/v0.1.md
git commit -m "Document the product workbench TUI" -m "Wave 17 adds an interactive demand workbench, so the docs now describe when to use it and how it relates to console and chat." -m "Constraint: Workbench does not replace the existing runtime chat TUI." -m "Confidence: high" -m "Scope-risk: narrow" -m "Tested: go test ./... -count=1 -timeout 5m; go vet ./...; go build ./cmd/devflow; git diff --check"
```

## Final Checklist

- `devflow workbench` dispatches.
- model renders demand list.
- selection works.
- detail view works.
- shortcuts are tested.
- no workflow rules are duplicated in TUI.
- full verification passes.
