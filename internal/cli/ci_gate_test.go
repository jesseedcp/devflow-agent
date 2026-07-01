package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

type fakeCIGateAdapter struct {
	result adapters.CIResult
	err    error
	ref    adapters.CIRef
}

func (f *fakeCIGateAdapter) Check(ctx context.Context, ref adapters.CIRef) (adapters.CIResult, error) {
	f.ref = ref
	return f.result, f.err
}

func TestRunCIGatePassesGitHubFlags(t *testing.T) {
	original := newCIGateAdapter
	defer func() { newCIGateAdapter = original }()
	fake := &fakeCIGateAdapter{result: adapters.CIResult{
		Provider: "github",
		Repo:     "owner/repo",
		PR:       "42",
		Status:   adapters.CIStatusPassed,
	}}
	newCIGateAdapter = func() adapters.CIGateAdapter { return fake }

	var stdout bytes.Buffer
	err := runCIGate([]string{"--github-repo", "owner/repo", "--github-pr", "42", "--github-base-url", "https://github.test"}, &stdout, ioDiscard{})
	if err != nil {
		t.Fatalf("runCIGate returned error: %v", err)
	}
	if fake.ref.Repo != "owner/repo" || fake.ref.PR != "42" || fake.ref.BaseURL != "https://github.test" {
		t.Fatalf("ref = %#v", fake.ref)
	}
	if !strings.Contains(stdout.String(), "ci gate passed for owner/repo#42") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCIGateFailsWhenChecksFail(t *testing.T) {
	original := newCIGateAdapter
	defer func() { newCIGateAdapter = original }()
	newCIGateAdapter = func() adapters.CIGateAdapter {
		return &fakeCIGateAdapter{result: adapters.CIResult{
			Provider: "github",
			Repo:     "owner/repo",
			PR:       "42",
			Status:   adapters.CIStatusFailed,
			Checks:   []adapters.CICheck{{Name: "Go verification", Status: "completed", Conclusion: "failure"}},
		}}
	}

	var stdout bytes.Buffer
	err := runCIGate([]string{"--github-repo", "owner/repo", "--github-pr", "42"}, &stdout, ioDiscard{})
	if err == nil || !strings.Contains(err.Error(), "ci gate blocked") {
		t.Fatalf("err = %v, want blocked", err)
	}
	if !strings.Contains(stdout.String(), "Go verification") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
