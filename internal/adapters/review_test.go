package adapters

import (
	"context"
	"testing"
)

var _ ReviewAdapter = fakeReview{}

type fakeReviewState struct {
	comments   []ReviewComment
	listErr    error
	replyErr   error
	listedRefs []ReviewRef
	replyCalls []fakeReplyCall
}

type fakeReview struct {
	state *fakeReviewState
}

type fakeReplyCall struct {
	ref       ReviewRef
	commentID string
	body      string
}

func (f fakeReview) ListUnresolved(_ context.Context, ref ReviewRef) ([]ReviewComment, error) {
	f.state.listedRefs = append(f.state.listedRefs, ref)
	return f.state.comments, f.state.listErr
}

func (f fakeReview) Reply(_ context.Context, ref ReviewRef, commentID string, body string) error {
	f.state.replyCalls = append(f.state.replyCalls, fakeReplyCall{
		ref:       ref,
		commentID: commentID,
		body:      body,
	})
	return f.state.replyErr
}

func TestCommentCategoryStringsStable(t *testing.T) {
	t.Parallel()

	cases := map[CommentCategory]string{
		CommentRequirements:   "requirements",
		CommentPlan:           "plan",
		CommentImplementation: "implementation",
		CommentTest:           "test",
		CommentStyle:          "style",
	}

	for got, want := range cases {
		if string(got) != want {
			t.Fatalf("string(%q) = %q, want %q", want, string(got), want)
		}
	}
}

func TestReviewAdapterListUnresolvedContract(t *testing.T) {
	t.Parallel()

	ref := ReviewRef{
		Project:      "demo",
		MergeRequest: "42",
	}
	wantComment := ReviewComment{
		ID:       "note-1",
		Author:   "reviewer",
		Body:     "please add a test",
		FilePath: "internal/cli/cli.go",
		Line:     27,
		Blocking: true,
		Category: CommentTest,
	}

	state := &fakeReviewState{
		comments: []ReviewComment{wantComment},
	}
	adapter := fakeReview{state: state}

	gotComments, err := adapter.ListUnresolved(context.Background(), ref)
	if err != nil {
		t.Fatalf("ListUnresolved() error = %v, want nil", err)
	}
	if len(state.listedRefs) != 1 {
		t.Fatalf("len(fakeReviewState.listedRefs) = %d, want 1", len(state.listedRefs))
	}
	if state.listedRefs[0] != ref {
		t.Fatalf("fakeReviewState.listedRefs[0] = %#v, want %#v", state.listedRefs[0], ref)
	}
	if len(gotComments) != 1 {
		t.Fatalf("len(ListUnresolved()) = %d, want 1", len(gotComments))
	}
	if gotComments[0] != wantComment {
		t.Fatalf("ListUnresolved()[0] = %#v, want %#v", gotComments[0], wantComment)
	}
}

func TestReviewAdapterReplyContract(t *testing.T) {
	t.Parallel()

	ref := ReviewRef{
		Project:      "demo",
		MergeRequest: "42",
	}

	state := &fakeReviewState{}
	adapter := fakeReview{state: state}

	err := adapter.Reply(context.Background(), ref, "note-1", "resolved in c03190b")
	if err != nil {
		t.Fatalf("Reply() error = %v, want nil", err)
	}
	if len(state.replyCalls) != 1 {
		t.Fatalf("len(fakeReviewState.replyCalls) = %d, want 1", len(state.replyCalls))
	}

	got := state.replyCalls[0]
	if got.ref != ref {
		t.Fatalf("replyCalls[0].ref = %#v, want %#v", got.ref, ref)
	}
	if got.commentID != "note-1" {
		t.Fatalf("replyCalls[0].commentID = %q, want %q", got.commentID, "note-1")
	}
	if got.body != "resolved in c03190b" {
		t.Fatalf("replyCalls[0].body = %q, want %q", got.body, "resolved in c03190b")
	}
}
