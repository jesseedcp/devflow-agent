package adapters

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitLabListUnresolvedSendsTokenAndFlattensNotes(t *testing.T) {
	t.Parallel()

	var gotAuth string
	var gotPath string
	payload := `[
		{"id":"abc","notes":[
			{"id":1,"body":"please add a test","resolved":false,"resolvable":true,"author":{"username":"reviewer"},"position":{"new_path":"main.go","new_line":42}},
			{"id":2,"body":"looks good","resolved":true,"resolvable":true,"author":{"username":"reviewer"}}
		]},
		{"id":"def","notes":[
			{"id":3,"body":"system note","resolved":false,"resolvable":false,"author":{"username":"bot"}}
		]}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("PRIVATE-TOKEN")
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, payload)
	}))
	defer srv.Close()

	adapter := GitLabReviewAdapter{}
	comments, err := adapter.ListUnresolved(context.Background(), ReviewRef{
		Project:      "group/project",
		MergeRequest: "123",
		BaseURL:      srv.URL,
		Token:        "secret-token",
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if gotAuth != "secret-token" {
		t.Fatalf("PRIVATE-TOKEN header = %q want secret-token", gotAuth)
	}
	if !strings.Contains(gotPath, "group%2Fproject") {
		t.Fatalf("path = %q want escaped project id", gotPath)
	}
	if len(comments) != 1 {
		t.Fatalf("comments = %d want 1 (resolved and non-resolvable notes ignored)", len(comments))
	}
	c := comments[0]
	if c.ID != "abc:1" {
		t.Fatalf("comment id = %q want abc:1", c.ID)
	}
	if c.Author != "reviewer" {
		t.Fatalf("author = %q want reviewer", c.Author)
	}
	if !c.Blocking {
		t.Fatalf("unresolved comment should be blocking")
	}
	if c.FilePath != "main.go" || c.Line != 42 {
		t.Fatalf("position = %s:%d want main.go:42", c.FilePath, c.Line)
	}
}

func TestGitLabReplyPostsToDiscussion(t *testing.T) {
	t.Parallel()

	var gotMethod string
	var gotPath string
	var gotBody string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.EscapedPath()
		gotAuth = r.Header.Get("PRIVATE-TOKEN")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	adapter := GitLabReviewAdapter{}
	err := adapter.Reply(context.Background(), ReviewRef{
		Project:      "group/project",
		MergeRequest: "123",
		BaseURL:      srv.URL,
		Token:        "secret-token",
	}, "abc:1", "resolved in c03190b")
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q want POST", gotMethod)
	}
	if gotAuth != "secret-token" {
		t.Fatalf("PRIVATE-TOKEN header = %q want secret-token", gotAuth)
	}
	if !strings.Contains(gotPath, "/discussions/abc/notes") {
		t.Fatalf("path = %q want discussion notes path", gotPath)
	}
	if !strings.Contains(gotBody, "body=") || !strings.Contains(gotBody, "resolved+in+c03190b") {
		t.Fatalf("body = %q want form body field", gotBody)
	}
}

func TestGitLabReplyRejectsInvalidCommentID(t *testing.T) {
	t.Parallel()

	adapter := GitLabReviewAdapter{}
	err := adapter.Reply(context.Background(), ReviewRef{
		Project:      "group/project",
		MergeRequest: "123",
		BaseURL:      "https://gitlab.example",
		Token:        "secret-token",
	}, "no-colon-here", "body")
	if err == nil || !strings.Contains(err.Error(), "invalid gitlab comment id") {
		t.Fatalf("err = %v want invalid gitlab comment id", err)
	}
}

func TestGitLabMissingTokenReturnsError(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")

	adapter := GitLabReviewAdapter{}
	_, err := adapter.ListUnresolved(context.Background(), ReviewRef{
		Project:      "group/project",
		MergeRequest: "123",
		BaseURL:      "https://gitlab.example",
	})
	if err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("err = %v want token error", err)
	}

	if err := adapter.Reply(context.Background(), ReviewRef{
		Project:      "group/project",
		MergeRequest: "123",
		BaseURL:      "https://gitlab.example",
	}, "abc:1", "body"); err == nil || !strings.Contains(err.Error(), "token") {
		t.Fatalf("reply err = %v want token error", err)
	}
}

func TestGitLabListUnresolvedClassifiesComments(t *testing.T) {
	t.Parallel()

	payload := `[
		{"id":"discussion-1","notes":[
			{"id":7,"body":"Please add regression coverage.","resolved":false,"resolvable":true,"author":{"username":"reviewer"},"position":{"new_path":"internal/service/coupon.go","new_line":12}}
		]}
	]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, payload)
	}))
	defer server.Close()

	adapter := GitLabReviewAdapter{}
	comments, err := adapter.ListUnresolved(context.Background(), ReviewRef{
		Project:      "group/project",
		MergeRequest: "1",
		BaseURL:      server.URL,
		Token:        "glpat-test",
	})
	if err != nil {
		t.Fatalf("ListUnresolved: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(comments))
	}
	if comments[0].Category != CommentTest {
		t.Fatalf("category = %s, want %s", comments[0].Category, CommentTest)
	}
}
