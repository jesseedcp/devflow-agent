package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func githubReviewThreadsPayload(threads []map[string]any) map[string]any {
	nodes := make([]map[string]any, 0, len(threads))
	nodes = append(nodes, threads...)
	return map[string]any{
		"data": map[string]any{
			"repository": map[string]any{
				"pullRequest": map[string]any{
					"reviewThreads": map[string]any{
						"nodes": nodes,
					},
				},
			},
		},
	}
}

func reviewThread(id string, resolved bool, comment map[string]any) map[string]any {
	return map[string]any{
		"id":         id,
		"isResolved": resolved,
		"comments": map[string]any{
			"nodes": []map[string]any{comment},
		},
	}
}

func TestGitHubReviewListsUnresolvedThread(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(githubReviewThreadsPayload([]map[string]any{
			reviewThread("THREAD_1", false, map[string]any{
				"databaseId": 1001,
				"body":       "please add a regression test",
				"path":       "internal/service_test.go",
				"line":       42,
				"author":     map[string]any{"login": "reviewer"},
			}),
		}))
	}))
	defer server.Close()

	comments, err := (GitHubReviewAdapter{Client: server.Client()}).ListUnresolved(context.Background(), ReviewRef{
		Repo:        "owner/repo",
		PullRequest: "42",
		BaseURL:     server.URL,
		Token:       "secret-token",
	})
	if err != nil {
		t.Fatalf("ListUnresolved returned error: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want Bearer token", gotAuth)
	}
	if len(comments) != 1 {
		t.Fatalf("len(comments) = %d, want 1", len(comments))
	}
	if comments[0].Blocking != true {
		t.Fatalf("Blocking = false, want true")
	}
	if comments[0].Category == "" {
		t.Fatalf("Category is empty")
	}
	if comments[0].ID != "THREAD_1:1001" {
		t.Fatalf("ID = %q, want THREAD_1:1001", comments[0].ID)
	}
	if comments[0].FilePath != "internal/service_test.go" {
		t.Fatalf("FilePath = %q", comments[0].FilePath)
	}
	if comments[0].Line != 42 {
		t.Fatalf("Line = %d, want 42", comments[0].Line)
	}
	if comments[0].Author != "reviewer" {
		t.Fatalf("Author = %q, want reviewer", comments[0].Author)
	}
}

func TestGitHubReviewIgnoresResolvedThread(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(githubReviewThreadsPayload([]map[string]any{
			reviewThread("THREAD_RESOLVED", true, map[string]any{
				"databaseId": 2002,
				"body":       "resolved comment",
				"author":     map[string]any{"login": "reviewer"},
			}),
		}))
	}))
	defer server.Close()

	comments, err := (GitHubReviewAdapter{Client: server.Client()}).ListUnresolved(context.Background(), ReviewRef{
		Repo:        "owner/repo",
		PullRequest: "42",
		BaseURL:     server.URL,
		Token:       "secret-token",
	})
	if err != nil {
		t.Fatalf("ListUnresolved returned error: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("len(comments) = %d, want 0", len(comments))
	}
}

func TestGitHubReviewRequiresToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	_, err := (GitHubReviewAdapter{}).ListUnresolved(context.Background(), ReviewRef{
		Repo:        "owner/repo",
		PullRequest: "42",
	})
	if err == nil || !strings.Contains(err.Error(), "github token is required") {
		t.Fatalf("err = %v, want github token required", err)
	}
}

func TestGitHubReviewRequiresRepoAndPR(t *testing.T) {
	_, err := (GitHubReviewAdapter{}).ListUnresolved(context.Background(), ReviewRef{
		PullRequest: "42",
		Token:       "secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "github repo must be owner/repo") {
		t.Fatalf("err = %v, want repo validation", err)
	}
	_, err = (GitHubReviewAdapter{}).ListUnresolved(context.Background(), ReviewRef{
		Repo:  "owner/repo",
		Token: "secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "github pull request is required") {
		t.Fatalf("err = %v, want pr validation", err)
	}
}

func TestGitHubReviewReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := (GitHubReviewAdapter{Client: server.Client()}).ListUnresolved(context.Background(), ReviewRef{
		Repo:        "owner/repo",
		PullRequest: "42",
		BaseURL:     server.URL,
		Token:       "secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "github graphql returned 500") {
		t.Fatalf("err = %v, want github 500", err)
	}
}

func TestGitHubReviewReturnsGraphQLError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errors": []map[string]any{{"message": "bad query"}},
		})
	}))
	defer server.Close()

	_, err := (GitHubReviewAdapter{Client: server.Client()}).ListUnresolved(context.Background(), ReviewRef{
		Repo:        "owner/repo",
		PullRequest: "42",
		BaseURL:     server.URL,
		Token:       "secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "bad query") {
		t.Fatalf("err = %v, want graphql error", err)
	}
}

func TestGitHubReviewReplyUnsupported(t *testing.T) {
	err := (GitHubReviewAdapter{}).Reply(context.Background(), ReviewRef{}, "THREAD_1:1001", "thanks")
	if err == nil || !strings.Contains(err.Error(), "not implemented in Wave 26") {
		t.Fatalf("err = %v, want unsupported reply", err)
	}
}
