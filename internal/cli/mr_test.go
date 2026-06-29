package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

func TestRunMREnsureReusesExisting(t *testing.T) {
	var buf bytes.Buffer
	stderr := new(bytes.Buffer)

	original := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = original }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter {
		return &fakeMergeRequestAdapter{result: adapters.MergeRequestResult{
			IID: 42, WebURL: "https://gitlab.com/p/-/42", Title: "MR", State: "opened",
		}}
	}

	err := runMREnsure([]string{
		"--gitlab-project", "p",
		"--source-branch", "feature/x",
		"--target-branch", "main",
		"--title", "MR",
	}, &buf, stderr)
	if err != nil {
		t.Fatalf("runMREnsure: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Reused") || !strings.Contains(out, "!42") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestRunMREnsureCreates(t *testing.T) {
	var buf bytes.Buffer
	stderr := new(bytes.Buffer)

	original := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = original }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter {
		return &fakeMergeRequestAdapter{result: adapters.MergeRequestResult{
			IID: 99, WebURL: "https://gitlab.com/p/-/99", Title: "New MR", State: "opened", WasCreated: true,
		}}
	}

	err := runMREnsure([]string{
		"--gitlab-project", "p",
		"--source-branch", "feature/y",
		"--target-branch", "main",
		"--title", "New MR",
	}, &buf, stderr)
	if err != nil {
		t.Fatalf("runMREnsure: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Created") || !strings.Contains(out, "!99") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestRunMREnsureRejectsBothDescriptionFlags(t *testing.T) {
	err := runMREnsure([]string{
		"--gitlab-project", "p",
		"--source-branch", "feature/x",
		"--target-branch", "main",
		"--title", "MR",
		"--description", "inline",
		"--description-file", "file.txt",
	}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "cannot both be set") {
		t.Fatalf("err = %v, want both-set error", err)
	}
}

func TestMergeRequestDescriptionReadsFile(t *testing.T) {
	desc, err := mergeRequestDescription("", "")
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if desc != "" {
		t.Fatalf("desc = %q, want empty", desc)
	}

	desc, err = mergeRequestDescription("inline text", "")
	if err != nil {
		t.Fatalf("inline: %v", err)
	}
	if desc != "inline text" {
		t.Fatalf("desc = %q, want inline text", desc)
	}
}

type fakeMergeRequestAdapter struct {
	result adapters.MergeRequestResult
	err    error
	spec   adapters.MergeRequestSpec
}

func (f *fakeMergeRequestAdapter) EnsureMergeRequest(_ context.Context, spec adapters.MergeRequestSpec) (adapters.MergeRequestResult, error) {
	f.spec = spec
	return f.result, f.err
}
