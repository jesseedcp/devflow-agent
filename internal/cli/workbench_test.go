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

func TestWorkbenchSnapshotRendersDemandList(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "snapshot-demand", Title: "Snapshot demand", Description: "Snapshot", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Devflow Workbench Snapshot", "snapshot-demand", "verification"} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot output missing %q:\n%s", want, got)
		}
	}
}

func TestWorkbenchSnapshotSelectedDemandShowsDetail(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "snapshot-detail", Title: "Snapshot detail", Description: "Snapshot", Source: "test", State: string(workflow.Created)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Summary", "State:", "Attention:", "Quality:", "Next:", "Run-ready:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot detail missing %q:\n%s", want, got)
		}
	}
}

func TestWorkbenchSnapshotPrintsCIGateStatus(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "snapshot-ci", Title: "Snapshot ci", Description: "Snapshot", Source: "test", State: string(workflow.MRReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "ci_gate.blocked", Message: "github ci pending", Data: map[string]string{"provider": "github", "repo": "owner/repo", "pr": "42", "status": "pending"}}); err != nil {
		t.Fatalf("AppendEvent ci returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "ci             owner/repo#42 pending") {
		t.Fatalf("snapshot detail missing ci gate status:\n%s", stdout.String())
	}
}
func TestWorkbenchSnapshotPrintsContextAwareRequirementChecks(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "workbench-quality-context", Title: "Workbench quality context", Description: "Evaluate context", Source: "test", State: string(workflow.RequirementsReview)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, "# Intake\n\n## 验收标准\n- Inactive users are blocked.\n"); err != nil {
		t.Fatalf("WriteArtifact intake returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ContextFile, "# Context\n\n## Approved Stable Memory\n\nNo approved stable memory recalled.\n\n## Historical Demand Candidates\n\n- `coupon-old`: Candidate needs confirmation.\n"); err != nil {
		t.Fatalf("WriteArtifact context returned error: %v", err)
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, "# Requirements\n\n## 业务规则\n\n- User status must be active.\n\n## 验收标准\n\n- Active users can claim coupons.\n\n## 待确认问题\n\n- 待人工补充。\n"); err != nil {
		t.Fatalf("WriteArtifact requirements returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"requirements.intake_coverage",
		"requirements.candidate_guard",
		"Inactive users are blocked",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("workbench snapshot missing %q:\n%s", want, got)
		}
	}
}

func TestWorkbenchSnapshotMissingDemandReturnsError(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(artifacts.Demand{ID: "known", Title: "Known", Source: "test", State: string(workflow.Created)}); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}

	err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", "missing"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `demand "missing" not found`) {
		t.Fatalf("err = %v, want missing demand error", err)
	}
}

func TestWorkbenchDriveUsesBackendDemandDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := writeBackendDemandDefaultsConfig(t, root)
	var got workbenchOptions
	old := workbenchDrive
	defer func() { workbenchDrive = old }()
	workbenchDrive = func(opts workbenchOptions, demandID string) string {
		got = opts
		return "drive called"
	}

	model := workbenchModel{opts: workbenchOptions{root: root, configPath: configPath}, demands: []workbenchDemand{{ID: "wb-defaults"}}}
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	updated := next.(workbenchModel)
	if updated.message != "drive called" {
		t.Fatalf("message = %q", updated.message)
	}
	if len(got.qualityCommand) != 1 || got.qualityCommand[0] != "go test ./..." {
		t.Fatalf("quality defaults = %#v", got.qualityCommand)
	}
}

func TestWorkbenchSnapshotShowsManualEvidence(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "snapshot-evidence", Title: "Snapshot evidence", Description: "Snapshot", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"Evidence:", "acceptance     pass=1 fail=0 blocked=0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot detail missing %q:\n%s", want, got)
		}
	}
}

func TestWorkbenchSnapshotPrintsWikiCounts(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "wiki-workbench", Title: "Wiki workbench", Description: "Closeout", Source: "test", State: string(workflow.Closeout)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatal(err)
	}
	wikiText := "# Wiki Candidates: Wiki workbench\n\n## Stable Business Knowledge\n\n- Active membership gates coupons. (source: memory-candidates.md)\n\n## Process Improvement Candidates\n\nNo process improvement candidates distilled yet.\n\n## Archive Only\n\nNo archive-only material distilled yet.\n"
	if err := store.WriteArtifact(demand.ID, artifacts.WikiCandidatesFile, wikiText); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--demand", demand.ID, "--snapshot"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "wiki           1 pending, 0 promoted, 0 rejected") {
		t.Fatalf("workbench snapshot missing wiki counts:\n%s", got)
	}
}

func TestWorkbenchSnapshotPrintsMetrics(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "metrics-workbench", Title: "Metrics workbench", Description: "Snapshot", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "stage.confirmed", Message: "requirements confirmed", Data: map[string]string{"stage": "requirements"}}); err != nil {
		t.Fatalf("AppendEvent stage returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent verification returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	var stdout bytes.Buffer
	if err := Run([]string{"workbench", "--root", root, "--snapshot", "--demand", demand.ID}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("workbench snapshot returned error: %v", err)
	}
	got := stdout.String()
	if !strings.Contains(got, "Metrics: human=1 review_returns=0 verification=1/1 acceptance=1/0/0 wiki=0/0") {
		t.Fatalf("workbench snapshot missing metrics summary:\n%s", got)
	}
}

func TestWorkbenchDetailToggleShowsMetrics(t *testing.T) {
	root := t.TempDir()
	store := artifacts.NewStore(root)
	demand := artifacts.Demand{ID: "wb-metrics", Title: "WB metrics", Description: "Detail", Source: "test", State: string(workflow.Verification)}
	if err := store.CreateDemand(demand); err != nil {
		t.Fatalf("CreateDemand returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "stage.confirmed", Message: "requirements confirmed", Data: map[string]string{"stage": "requirements"}}); err != nil {
		t.Fatalf("AppendEvent stage returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.recorded", Message: "verification pass", Data: map[string]string{"status": "pass", "command": "go test ./..."}}); err != nil {
		t.Fatalf("AppendEvent verification returned error: %v", err)
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{Type: "verification.evidence_recorded", Message: "manual evidence", Data: map[string]string{"status": "pass", "type": "api", "criterion": "Inactive users are blocked", "summary": "COUPON_USER_INACTIVE"}}); err != nil {
		t.Fatalf("AppendEvent evidence returned error: %v", err)
	}

	model := newWorkbenchModel(workbenchOptions{root: root})
	next, _ := model.Update(workbenchLoadedMsg{demands: []workbenchDemand{{ID: demand.ID, State: string(workflow.Verification), Attention: "ready"}}})
	next, _ = next.(workbenchModel).Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := next.(workbenchModel).View()
	if !strings.Contains(view, "Metrics: human=1 review_returns=0 verification=1/1 acceptance=1/0/0 wiki=0/0") {
		t.Fatalf("detail view missing metrics:\n%s", view)
	}
}
