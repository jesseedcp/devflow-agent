package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestDogfoodCommandCompletesFullLoop(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
	helper := testCLIExecutable(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"dogfood",
		"--root", root,
		"--quality-root", t.TempDir(),
		"--quality-command", `"` + helper + `" -test.run=^TestCLICommandHelper$`,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dogfood: %v\nstderr:\n%s", err, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"dogfood completed", "state: completed", "report:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}

func TestDogfoodHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"devflow dogfood", "dogfood  Run a deterministic full backend-demand loop"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help missing %q:\n%s", want, output)
		}
	}
}

func TestDogfoodRejectsUnknownScenario(t *testing.T) {
	err := Run([]string{"dogfood", "--root", t.TempDir(), "--scenario", "unknown"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unsupported dogfood scenario") {
		t.Fatalf("err = %v, want unsupported scenario", err)
	}
}

func TestDogfoodOperatorLoopCompletes(t *testing.T) {
	root := t.TempDir()
	t.Setenv("DEVFLOW_DOGFOOD_HELPER", "1")
	helper := testCLIExecutable(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run([]string{
		"dogfood",
		"--operator-loop",
		"--root", root,
		"--quality-root", t.TempDir(),
		"--quality-command", `"` + helper + `" -test.run=^TestCLICommandHelper$`,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("dogfood operator loop: %v\nstderr:\n%s", err, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"operator dogfood completed", "state: completed", "report:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}
