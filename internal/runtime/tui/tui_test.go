package tui

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/runtime/config"
	"github.com/jesseedcp/devflow-agent/internal/runtime/permissions"
)

func TestNewOneProviderStartsInChat(t *testing.T) {
	m := New([]config.ProviderConfig{{Name: "test"}}, nil, nil)
	if m.state != stateChat {
		t.Fatalf("expected single-provider New to start in chat state, got %v", m.state)
	}
}

func TestNewMultipleProvidersStaysInSelect(t *testing.T) {
	m := New([]config.ProviderConfig{{Name: "a"}, {Name: "b"}}, nil, nil)
	if m.state != stateProviderSelect {
		t.Fatalf("expected multi-provider New to stay in provider select, got %v", m.state)
	}
}

func TestPermissionModeInfoLabels(t *testing.T) {
	cases := []struct {
		mode  permissions.PermissionMode
		label string
	}{
		{permissions.ModeDefault, "Default"},
		{permissions.ModeAcceptEdits, "Accept Edits"},
		{permissions.ModePlan, "Plan"},
		{permissions.ModeBypass, "YOLO"},
	}
	for _, c := range cases {
		label, desc := permissionModeInfo(c.mode)
		if label != c.label {
			t.Errorf("mode %v: label = %q, want %q", c.mode, label, c.label)
		}
		if desc == "" {
			t.Errorf("mode %v: expected non-empty description", c.mode)
		}
	}
}

func TestNextPermissionModeCycles(t *testing.T) {
	want := []permissions.PermissionMode{
		permissions.ModeAcceptEdits,
		permissions.ModePlan,
		permissions.ModeBypass,
		permissions.ModeDefault,
	}
	current := permissions.ModeDefault
	for _, expected := range want {
		got := nextPermissionMode(current)
		if got != expected {
			t.Fatalf("nextPermissionMode(%v) = %v, want %v", current, got, expected)
		}
		current = got
	}
}

func TestCoordinatorToolFilterNilReturnsNil(t *testing.T) {
	if got := coordinatorToolFilter(nil); got != nil {
		t.Fatal("expected nil filter for nil team manager")
	}
}

func TestIsCollapsibleTool(t *testing.T) {
	collapsible := []string{"ReadFile", "Glob", "Grep", "ToolSearch"}
	for _, name := range collapsible {
		if !isCollapsibleTool(name) {
			t.Errorf("expected %q to be collapsible", name)
		}
	}
	if isCollapsibleTool("WriteFile") {
		t.Errorf("WriteFile should not be collapsible")
	}
}

func TestRenderToolGroupSummary(t *testing.T) {
	tools := []toolBlockInfo{
		{toolName: "ReadFile", elapsed: 0.5},
		{toolName: "Glob", elapsed: 1.5},
	}
	got := renderToolGroupSummary(tools)
	if !strings.Contains(got, "2 tool uses") {
		t.Fatalf("expected 2 tool uses in summary, got %q", got)
	}
	if !strings.Contains(got, "2.0s") {
		t.Fatalf("expected 2.0s elapsed in summary, got %q", got)
	}
	if strings.Contains(got, "error") {
		t.Fatalf("did not expect error wording for clean run, got %q", got)
	}

	errTools := []toolBlockInfo{
		{toolName: "ReadFile", elapsed: 0.5, isError: true},
	}
	got = renderToolGroupSummary(errTools)
	if !strings.Contains(got, "1 error") {
		t.Fatalf("expected error wording, got %q", got)
	}
}
