package quality

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestGateHelperProcess(t *testing.T) {
	exitCodeText, ok := os.LookupEnv("DEVFLOW_GATE_HELPER_EXIT")
	if !ok {
		return
	}

	exitCode, err := strconv.Atoi(exitCodeText)
	if err != nil {
		os.Exit(2)
	}
	os.Exit(exitCode)
}

func TestGatePasses(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		results: []Result{
			{
				Command:  "echo",
				Args:     []string{"ok"},
				Dir:      "repo",
				ExitCode: 0,
				Stdout:   "ok\n",
			},
		},
	}

	got := (Gate{Runner: runner}).Run(
		context.Background(),
		"repo",
		Command{Name: "echo", Args: []string{"ok"}},
	)

	if !got.Passed {
		t.Fatal("Gate.Run().Passed = false, want true")
	}
	if len(got.Results) != 1 {
		t.Fatalf("len(Gate.Run().Results) = %d, want 1", len(got.Results))
	}
	if got.Results[0].Stdout != "ok\n" {
		t.Fatalf("Gate.Run().Results[0].Stdout = %q, want %q", got.Results[0].Stdout, "ok\n")
	}
}

func TestGateFailsAndContinues(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		results: []Result{
			{
				Command:  "go",
				Args:     []string{"test", "./..."},
				Dir:      "repo",
				ExitCode: 1,
				Stderr:   "tests failed\n",
			},
			{
				Command:  "go",
				Args:     []string{"vet", "./..."},
				Dir:      "repo",
				ExitCode: 0,
				Stdout:   "vet ok\n",
			},
		},
	}

	got := (Gate{Runner: runner}).Run(
		context.Background(),
		"repo",
		Command{Name: "go", Args: []string{"test", "./..."}},
		Command{Name: "go", Args: []string{"vet", "./..."}},
	)

	if got.Passed {
		t.Fatal("Gate.Run().Passed = true, want false")
	}
	if len(got.Results) != 2 {
		t.Fatalf("len(Gate.Run().Results) = %d, want 2", len(got.Results))
	}
	if got.Results[0].Stderr != "tests failed\n" {
		t.Fatalf("Gate.Run().Results[0].Stderr = %q, want %q", got.Results[0].Stderr, "tests failed\n")
	}
	if len(runner.calls) != 2 {
		t.Fatalf("len(fakeRunner.calls) = %d, want 2", len(runner.calls))
	}
	if runner.calls[1].name != "go" || strings.Join(runner.calls[1].args, " ") != "vet ./..." {
		t.Fatalf("fakeRunner.calls[1] = %#v, want second command to run", runner.calls[1])
	}
}

func TestGatePassesWithNoCommands(t *testing.T) {
	t.Parallel()

	got := Gate{}.Run(context.Background(), "repo")

	if !got.Passed {
		t.Fatal("Gate.Run().Passed = false, want true")
	}
	if len(got.Results) != 0 {
		t.Fatalf("len(Gate.Run().Results) = %d, want 0", len(got.Results))
	}
}

func TestGateUsesExecRunnerWhenRunnerIsNil(t *testing.T) {
	t.Setenv("DEVFLOW_GATE_HELPER_EXIT", "0")

	executable := testExecutable(t)
	got := Gate{}.Run(
		context.Background(),
		".",
		Command{Name: executable, Args: []string{"-test.run=^TestGateHelperProcess$"}},
	)

	if !got.Passed {
		t.Fatal("Gate.Run().Passed = false, want true")
	}
	if len(got.Results) != 1 {
		t.Fatalf("len(Gate.Run().Results) = %d, want 1", len(got.Results))
	}
}

func TestExecRunnerPreservesExitCode(t *testing.T) {
	t.Setenv("DEVFLOW_GATE_HELPER_EXIT", "7")

	got := ExecRunner{}.Run(
		context.Background(),
		".",
		testExecutable(t),
		"-test.run=^TestGateHelperProcess$",
	)

	if got.ExitCode != 7 {
		t.Fatalf("ExecRunner.Run().ExitCode = %d, want 7", got.ExitCode)
	}
}

func TestExecRunnerCopiesArgs(t *testing.T) {
	t.Setenv("DEVFLOW_GATE_HELPER_EXIT", "0")

	args := []string{"-test.run=^TestGateHelperProcess$"}
	got := ExecRunner{}.Run(context.Background(), ".", testExecutable(t), args...)
	args[0] = "changed"

	if len(got.Args) != 1 || got.Args[0] != "-test.run=^TestGateHelperProcess$" {
		t.Fatalf("ExecRunner.Run().Args = %q, want copied original args", got.Args)
	}
}

func TestExecRunnerCapturesCommandNotFound(t *testing.T) {
	t.Parallel()

	got := ExecRunner{}.Run(context.Background(), ".", "devflow-command-that-does-not-exist")

	if got.ExitCode == 0 {
		t.Fatalf("ExecRunner.Run().ExitCode = %d, want non-zero", got.ExitCode)
	}
	if got.Stderr == "" {
		t.Fatal("ExecRunner.Run().Stderr = empty, want error evidence")
	}
}

func TestExecRunnerCapturesContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := ExecRunner{}.Run(ctx, ".", testExecutable(t))

	if got.ExitCode == 0 {
		t.Fatalf("ExecRunner.Run().ExitCode = %d, want non-zero", got.ExitCode)
	}
	if got.Stderr == "" {
		t.Fatal("ExecRunner.Run().Stderr = empty, want cancellation evidence")
	}
	if !strings.Contains(strings.ToLower(got.Stderr), "context") && !strings.Contains(strings.ToLower(got.Stderr), "canceled") {
		t.Fatalf("ExecRunner.Run().Stderr = %q, want cancellation evidence", got.Stderr)
	}
}

func testExecutable(t *testing.T) string {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() returned error: %v", err)
	}
	return executable
}

type fakeRunner struct {
	results []Result
	calls   []fakeRunnerCall
}

type fakeRunnerCall struct {
	dir  string
	name string
	args []string
}

func (f *fakeRunner) Run(_ context.Context, dir string, name string, args ...string) Result {
	f.calls = append(f.calls, fakeRunnerCall{
		dir:  dir,
		name: name,
		args: append([]string(nil), args...),
	})

	return f.results[len(f.calls)-1]
}
