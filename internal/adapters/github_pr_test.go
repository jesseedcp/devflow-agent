package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubPREnsureReusesExistingOpenPR(t *testing.T) {
	var gotAuth, gotMethod, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotQuery = r.URL.RawQuery
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"number": 7, "html_url": "https://github.com/owner/repo/pull/7", "title": "Existing", "state": "open"},
			})
			return
		}
		http.Error(w, "should not create", http.StatusInternalServerError)
	}))
	defer server.Close()

	result, err := (GitHubPullRequestAdapter{Client: server.Client()}).EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Repo:         "owner/repo",
		SourceBranch: "feature/x",
		TargetBranch: "main",
		Title:        "Add feature",
		BaseURL:      server.URL,
		Token:        "secret-token",
	})
	if err != nil {
		t.Fatalf("EnsureMergeRequest returned error: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want Bearer token", gotAuth)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if !strings.Contains(gotQuery, "head=owner%3Afeature%2Fx") || !strings.Contains(gotQuery, "base=main") || !strings.Contains(gotQuery, "state=open") {
		t.Fatalf("query = %q missing head/base/state", gotQuery)
	}
	if result.IID != 7 || result.WasCreated {
		t.Fatalf("result = %#v, want reused PR 7", result)
	}
	if result.WebURL != "https://github.com/owner/repo/pull/7" {
		t.Fatalf("WebURL = %q", result.WebURL)
	}
}

func TestGitHubPREnsureCreatesWhenMissing(t *testing.T) {
	var postBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&postBody)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 12, "html_url": "https://github.com/owner/repo/pull/12", "title": "Add feature", "state": "open",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result, err := (GitHubPullRequestAdapter{Client: server.Client()}).EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Repo:         "owner/repo",
		SourceBranch: "feature/y",
		TargetBranch: "main",
		Title:        "Add feature",
		Description:  "body text",
		BaseURL:      server.URL,
		Token:        "secret-token",
	})
	if err != nil {
		t.Fatalf("EnsureMergeRequest returned error: %v", err)
	}
	if result.IID != 12 || !result.WasCreated {
		t.Fatalf("result = %#v, want created PR 12", result)
	}
	if postBody["head"] != "feature/y" || postBody["base"] != "main" || postBody["title"] != "Add feature" || postBody["body"] != "body text" {
		t.Fatalf("post body = %#v", postBody)
	}
}

func TestGitHubPREnsureValidatesSpec(t *testing.T) {
	adapter := GitHubPullRequestAdapter{}
	cases := []struct {
		name string
		spec MergeRequestSpec
		want string
	}{
		{"repo", MergeRequestSpec{SourceBranch: "s", TargetBranch: "t", Title: "x", Token: "tok"}, "github repo must be owner/repo"},
		{"source", MergeRequestSpec{Repo: "owner/repo", TargetBranch: "t", Title: "x", Token: "tok"}, "source branch is required"},
		{"target", MergeRequestSpec{Repo: "owner/repo", SourceBranch: "s", Title: "x", Token: "tok"}, "target branch is required"},
		{"title", MergeRequestSpec{Repo: "owner/repo", SourceBranch: "s", TargetBranch: "t", Token: "tok"}, "title is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := adapter.EnsureMergeRequest(context.Background(), tc.spec)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestGitHubPREnsureRequiresToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	_, err := (GitHubPullRequestAdapter{}).EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Repo:         "owner/repo",
		SourceBranch: "s",
		TargetBranch: "t",
		Title:        "x",
	})
	if err == nil || !strings.Contains(err.Error(), "github token is required") {
		t.Fatalf("err = %v, want github token required", err)
	}
}

func TestGitHubPREnsureReadsTokenFromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"number": 1, "html_url": "https://github.com/owner/repo/pull/1", "title": "x", "state": "open"},
		})
	}))
	defer server.Close()

	_, err := (GitHubPullRequestAdapter{Client: server.Client()}).EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Repo:         "owner/repo",
		SourceBranch: "s",
		TargetBranch: "t",
		Title:        "x",
		BaseURL:      server.URL,
	})
	if err != nil {
		t.Fatalf("EnsureMergeRequest returned error: %v", err)
	}
	if gotAuth != "Bearer env-token" {
		t.Fatalf("Authorization = %q, want env token", gotAuth)
	}
}

func TestGitHubPREnsureSurfacesAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		}
		http.Error(w, `{"message":"Validation Failed"}`, http.StatusUnprocessableEntity)
	}))
	defer server.Close()

	_, err := (GitHubPullRequestAdapter{Client: server.Client()}).EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Repo:         "owner/repo",
		SourceBranch: "s",
		TargetBranch: "t",
		Title:        "x",
		BaseURL:      server.URL,
		Token:        "secret-token",
	})
	if err == nil || !strings.Contains(err.Error(), "Validation Failed") {
		t.Fatalf("err = %v, want validation failed body", err)
	}
}
