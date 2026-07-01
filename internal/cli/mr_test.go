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

func TestRunChangeRequestEnsureViaDispatch(t *testing.T) {
	original := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = original }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter {
		return &fakeMergeRequestAdapter{result: adapters.MergeRequestResult{
			IID: 55, WebURL: "https://gitlab.com/p/-/55", Title: "CR", State: "opened",
		}}
	}

	var buf bytes.Buffer
	err := Run([]string{"change-request", "ensure", "--gitlab-project", "p", "--source-branch", "feature/x", "--target-branch", "main", "--title", "CR"}, &buf, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("change-request ensure: %v", err)
	}
	if !strings.Contains(buf.String(), "!55") {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestRunChangeRequestAliasCR(t *testing.T) {
	original := newMergeRequestAdapter
	defer func() { newMergeRequestAdapter = original }()
	newMergeRequestAdapter = func() adapters.MergeRequestAdapter {
		return &fakeMergeRequestAdapter{result: adapters.MergeRequestResult{
			IID: 56, WebURL: "https://gitlab.com/p/-/56", Title: "CR", State: "opened",
		}}
	}

	var buf bytes.Buffer
	err := Run([]string{"cr", "ensure", "--gitlab-project", "p", "--source-branch", "feature/x", "--target-branch", "main", "--title", "CR"}, &buf, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("cr ensure: %v", err)
	}
	if !strings.Contains(buf.String(), "!56") {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestChangeRequestHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(stdout.String(), "change-request Create or reuse GitLab MRs or GitHub PRs") {
		t.Fatalf("help missing change-request entry:\n%s", stdout.String())
	}
}

func TestRunMREnsureGitHubProviderCreates(t *testing.T) {
	original := newGitHubMergeRequestAdapter
	defer func() { newGitHubMergeRequestAdapter = original }()
	fake := &fakeMergeRequestAdapter{result: adapters.MergeRequestResult{
		IID: 12, WebURL: "https://github.com/owner/repo/pull/12", Title: "PR", State: "open", WasCreated: true,
	}}
	newGitHubMergeRequestAdapter = func() adapters.MergeRequestAdapter { return fake }

	var buf bytes.Buffer
	err := Run([]string{"mr", "ensure", "--provider", "github", "--github-repo", "owner/repo", "--source-branch", "feature/x", "--target-branch", "main", "--title", "PR"}, &buf, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("mr ensure github: %v", err)
	}
	if fake.spec.Provider != "github" || fake.spec.Repo != "owner/repo" {
		t.Fatalf("spec = %#v", fake.spec)
	}
	out := buf.String()
	if !strings.Contains(out, "Created pull request !12") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}

func TestRunChangeRequestEnsureGitHubProvider(t *testing.T) {
	original := newGitHubMergeRequestAdapter
	defer func() { newGitHubMergeRequestAdapter = original }()
	fake := &fakeMergeRequestAdapter{result: adapters.MergeRequestResult{
		IID: 8, WebURL: "https://github.com/owner/repo/pull/8", Title: "PR", State: "open",
	}}
	newGitHubMergeRequestAdapter = func() adapters.MergeRequestAdapter { return fake }

	var buf bytes.Buffer
	err := Run([]string{"change-request", "ensure", "--provider", "github", "--github-repo", "owner/repo", "--source-branch", "feature/x", "--target-branch", "main", "--title", "PR"}, &buf, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("change-request ensure github: %v", err)
	}
	if fake.spec.Provider != "github" || fake.spec.Repo != "owner/repo" {
		t.Fatalf("spec = %#v", fake.spec)
	}
	if !strings.Contains(buf.String(), "Reused pull request !8") {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestRunMREnsureGitHubProviderRequiresRepo(t *testing.T) {
	err := Run([]string{"mr", "ensure", "--provider", "github", "--source-branch", "s", "--target-branch", "t", "--title", "x"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--github-repo is required for provider github") {
		t.Fatalf("err = %v, want github repo required", err)
	}
}
