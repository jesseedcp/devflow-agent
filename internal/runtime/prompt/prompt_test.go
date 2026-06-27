package prompt

import (
	"runtime"
	"strings"
	"testing"
)

func TestIdentitySectionUsesDevflowBrand(t *testing.T) {
	content := IdentitySection().Content
	if !strings.Contains(content, "Devflow") {
		t.Fatalf("expected Devflow identity, got %q", content)
	}
	if strings.Contains(content, "MewCode") {
		t.Fatalf("identity should not expose legacy MewCode brand: %q", content)
	}
}

func TestBuilderOrdersSectionsByPriority(t *testing.T) {
	got := NewBuilder().
		Add(Section{Name: "late", Priority: 20, Content: "late"}).
		Add(Section{Name: "early", Priority: 10, Content: "early"}).
		Build()
	if got != "early\n\nlate" {
		t.Fatalf("unexpected section order: %q", got)
	}
}

func TestBuildSystemPromptIncludesSkillAndEnvironment(t *testing.T) {
	prompt := BuildSystemPrompt(EnvironmentContext{
		WorkDir:   "/repo",
		OS:        "windows",
		Arch:      "amd64",
		Shell:     "powershell",
		IsGitRepo: true,
		GitBranch: "main",
		Date:      "2026-06-27",
	}, BuildOptions{SkillSection: "skill text"})

	for _, want := range []string{"Devflow", "skill text", "工作目录: /repo", "Shell: powershell", "Git 分支: main"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}

func TestDefaultShell(t *testing.T) {
	want := "bash"
	if runtime.GOOS == "windows" {
		want = "powershell"
	}
	if got := defaultShell(); got != want {
		t.Fatalf("defaultShell() = %q, want %q", got, want)
	}
}

func TestDetectEnvironmentAppliesDefaultShellWhenUnset(t *testing.T) {
	t.Setenv("SHELL", "")
	env := DetectEnvironment(t.TempDir())
	want := "bash"
	if runtime.GOOS == "windows" {
		want = "powershell"
	}
	if env.Shell != want {
		t.Fatalf("DetectEnvironment shell = %q, want %q", env.Shell, want)
	}
}

func TestBuildPlanModeReminderCadence(t *testing.T) {
	cases := []struct {
		iteration  int
		full       bool
		planExists bool
	}{
		{iteration: 1, full: true},
		{iteration: 2, full: true},
		{iteration: 5, full: true},
		{iteration: 6, full: false},
		{iteration: 25, full: false},
		{iteration: 26, full: true},
		{iteration: 1, full: true, planExists: true},
		{iteration: 6, full: false, planExists: true},
	}

	for _, tc := range cases {
		got := BuildPlanModeReminder("/tmp/plan.md", tc.planExists, tc.iteration)
		hasFullMarker := strings.Contains(got, "## Plan Workflow")
		if hasFullMarker != tc.full {
			t.Fatalf("iteration %d planExists %v full=%v, want %v\n%s", tc.iteration, tc.planExists, hasFullMarker, tc.full, got)
		}
		if tc.planExists && tc.full && !strings.Contains(got, "A plan file already exists") {
			t.Fatalf("expected existing-plan full reminder, got:\n%s", got)
		}
		if !tc.planExists && tc.full && !strings.Contains(got, "No plan file exists yet") {
			t.Fatalf("expected missing-plan full reminder, got:\n%s", got)
		}
	}
}
