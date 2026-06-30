package cli

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
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

func TestWorkbenchDetailToggleShowsSelectedDemand(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "wb-detail", Title: "WB detail", Description: "Detail", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	model := newWorkbenchModel(workbenchOptions{root: root})
	next, _ := model.Update(workbenchLoadedMsg{demands: []workbenchDemand{{ID: demand.ID, State: string(workflow.Verification), Attention: "ready"}}})
	next, _ = next.(workbenchModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := next.(workbenchModel).View()
	for _, want := range []string{"Summary", "State:", "Attention:", "Next:", "Run-ready:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail view missing %q:\n%s", want, view)
		}
	}
}

func TestWorkbenchRunBlockedMessage(t *testing.T) {
	model := workbenchModel{opts: workbenchOptions{root: "."}, demands: []workbenchDemand{{ID: "blocked"}}}
	old := workbenchRunNext
	defer func() { workbenchRunNext = old }()
	workbenchRunNext = func(opts workbenchOptions, demandID string) string { return "Blocked: human confirmation is required" }

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	updated := next.(workbenchModel)
	if !strings.Contains(updated.message, "Blocked") {
		t.Fatalf("message = %q, want blocked", updated.message)
	}
}

func TestWorkbenchShortcutsUpdateMessage(t *testing.T) {
	model := workbenchModel{opts: workbenchOptions{root: "."}, demands: []workbenchDemand{{ID: "shortcut"}}}
	oldRun, oldDrive, oldEvaluate := workbenchRunNext, workbenchDrive, workbenchEvaluate
	defer func() { workbenchRunNext, workbenchDrive, workbenchEvaluate = oldRun, oldDrive, oldEvaluate }()
	workbenchRunNext = func(opts workbenchOptions, demandID string) string { return "run called " + demandID }
	workbenchDrive = func(opts workbenchOptions, demandID string) string { return "drive called " + demandID }
	workbenchEvaluate = func(opts workbenchOptions, demandID string) string { return "evaluate called " + demandID }

	cases := []struct {
		key  rune
		want string
	}{
		{'r', "run called shortcut"},
		{'d', "drive called shortcut"},
		{'e', "evaluate called shortcut"},
	}
	for _, tc := range cases {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		updated := next.(workbenchModel)
		if updated.message != tc.want {
			t.Fatalf("key %q message = %q, want %q", tc.key, updated.message, tc.want)
		}
	}
}

func TestWorkbenchRefreshKeyReloadsDemands(t *testing.T) {
	model := workbenchModel{demands: []workbenchDemand{{ID: "stale"}}}
	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	updated := next.(workbenchModel)
	if updated.message != "Refreshing demands..." {
		t.Fatalf("message = %q, want refresh message", updated.message)
	}
	if cmd == nil {
		t.Fatal("refresh key did not return a load command")
	}
}
