package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

type fakeReviewGateAdapter struct {
	comments []adapters.ReviewComment
	ref      adapters.ReviewRef
}

func (f *fakeReviewGateAdapter) ListUnresolved(_ context.Context, ref adapters.ReviewRef) ([]adapters.ReviewComment, error) {
	f.ref = ref
	return f.comments, nil
}

func (f *fakeReviewGateAdapter) Reply(context.Context, adapters.ReviewRef, string, string) error {
	return nil
}

func TestReviewGatePassesWithoutUnresolvedComments(t *testing.T) {
	adapter := &fakeReviewGateAdapter{}
	original := newReviewAdapter
	defer func() { newReviewAdapter = original }()
	newReviewAdapter = func() adapters.ReviewAdapter { return adapter }

	var stdout bytes.Buffer
	err := Run([]string{"review-gate", "--gitlab-project", "group/project", "--gitlab-mr", "123", "--gitlab-base-url", "https://gitlab.example.com"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("review gate: %v", err)
	}
	if adapter.ref.Project != "group/project" || adapter.ref.MergeRequest != "123" || adapter.ref.BaseURL != "https://gitlab.example.com" {
		t.Fatalf("ref = %#v", adapter.ref)
	}
	if !strings.Contains(stdout.String(), "review gate passed") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestReviewGateBlocksOnUnresolvedComments(t *testing.T) {
	adapter := &fakeReviewGateAdapter{comments: []adapters.ReviewComment{{
		ID:       "discussion:1",
		Author:   "reviewer",
		Body:     "fix nil handling",
		FilePath: "internal/service.go",
		Line:     42,
		Blocking: true,
		Category: adapters.CommentTest,
	}}}
	original := newReviewAdapter
	defer func() { newReviewAdapter = original }()
	newReviewAdapter = func() adapters.ReviewAdapter { return adapter }

	var stdout bytes.Buffer
	err := Run([]string{"review-gate", "--gitlab-project", "group/project", "--gitlab-mr", "123"}, &stdout, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "review gate blocked") {
		t.Fatalf("err = %v, want blocked", err)
	}
	output := stdout.String()
	for _, want := range []string{"review gate blocked", "[test]", "internal/service.go:42", "fix nil handling"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}

func TestReviewGateRequiresGitLabRef(t *testing.T) {
	err := Run([]string{"review-gate", "--gitlab-project", "group/project"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--gitlab-project and --gitlab-mr are required") {
		t.Fatalf("err = %v, want required ref", err)
	}
}

func TestReviewGateHelpIsListed(t *testing.T) {
	var stdout bytes.Buffer
	if err := Run([]string{"help"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("help: %v", err)
	}
	for _, want := range []string{"devflow review-gate", "review-gate Check unresolved GitLab MR comments directly"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, stdout.String())
		}
	}
}
