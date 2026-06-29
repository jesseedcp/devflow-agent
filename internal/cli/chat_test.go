package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// isolateRuntimeDirs points config loading (which reads os.Getwd() and
// os.UserHomeDir()) at dir for the duration of the test, so no real user
// home configuration can leak into a test. It restores the originals on cleanup.
func isolateRuntimeDirs(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)
	t.Cleanup(func() { _ = os.Chdir(origDir) })
}

// withTestRuntimeConfig writes a minimal .devflow/config.yaml with a single
// provider into dir so config.LoadConfig("") succeeds from that directory.
func withTestRuntimeConfig(t *testing.T, dir string) {
	t.Helper()
	body := "providers:\n  - name: test-provider\n    protocol: openai-compat\n    base_url: https://example.invalid/v1\n    model: test-model\n"
	cfgDir := filepath.Join(dir, ".devflow")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// stubChatRunner replaces runTeaProgram with a no-op that counts invocations,
// so tests never take over the terminal. It returns a pointer that is
// incremented on each call and restores the original runner on cleanup.
func stubChatRunner(t *testing.T) *int {
	t.Helper()
	var calls int
	orig := runTeaProgram
	runTeaProgram = func(_ tea.Model) error {
		calls++
		return nil
	}
	t.Cleanup(func() { runTeaProgram = orig })
	return &calls
}

func TestChatLoadsConfigAndRunsTUI(t *testing.T) {
	dir := t.TempDir()
	withTestRuntimeConfig(t, dir)
	isolateRuntimeDirs(t, dir)
	calls := stubChatRunner(t)

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"chat"}, &stdout, &stderr); err != nil {
		t.Fatalf("chat returned error: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected runTeaProgram to be called once, got %d", *calls)
	}
}

func TestTUIAliasesChat(t *testing.T) {
	dir := t.TempDir()
	withTestRuntimeConfig(t, dir)
	isolateRuntimeDirs(t, dir)
	calls := stubChatRunner(t)

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"tui"}, &stdout, &stderr); err != nil {
		t.Fatalf("tui returned error: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected runTeaProgram to be called once for tui, got %d", *calls)
	}
}

func TestNoArgsLaunchesInteractive(t *testing.T) {
	dir := t.TempDir()
	withTestRuntimeConfig(t, dir)
	isolateRuntimeDirs(t, dir)
	calls := stubChatRunner(t)

	var stdout, stderr bytes.Buffer
	if err := Run(nil, &stdout, &stderr); err != nil {
		t.Fatalf("nil args returned error: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected runTeaProgram to be called once for nil args, got %d", *calls)
	}
}

func TestMissingConfigReturnsActionableError(t *testing.T) {
	dir := t.TempDir()
	isolateRuntimeDirs(t, dir)
	calls := stubChatRunner(t)

	var stdout, stderr bytes.Buffer
	err := Run([]string{"chat"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when config is missing")
	}
	msg := err.Error()
	if !strings.Contains(msg, ".devflow/config.yaml") {
		t.Fatalf("missing-config error should mention .devflow/config.yaml, got: %q", msg)
	}
	if !strings.Contains(msg, "devflow help") {
		t.Fatalf("missing-config error should mention devflow help, got: %q", msg)
	}
	if *calls != 0 {
		t.Fatalf("runTeaProgram must not run when config is missing, got %d calls", *calls)
	}
}

func TestHelpStillPrintsAndExitsNil(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &stderr); err != nil {
		t.Fatalf("help returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "devflow") {
		t.Fatalf("help output should mention devflow, got: %q", stdout.String())
	}
}

func TestUnknownCommandStillReturnsHelpError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"bogus"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown-command error, got: %v", err)
	}
}
