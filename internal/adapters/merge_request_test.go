package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitLabEnsureMergeRequestReusesExisting(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "merge_requests") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]gitlabMergeRequest{{
				IID:    42,
				Title:  "Existing MR",
				State:  "opened",
				WebURL: "https://gitlab.com/test/project/-/merge_requests/42",
			}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	adapter := GitLabReviewAdapter{Client: server.Client()}
	result, err := adapter.EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Project:      "test/project",
		SourceBranch: "feature/x",
		TargetBranch: "main",
		Title:        "Existing MR",
		BaseURL:      server.URL,
		Token:        "glpat-test",
	})
	if err != nil {
		t.Fatalf("EnsureMergeRequest: %v", err)
	}
	if result.IID != 42 {
		t.Fatalf("iid = %d, want 42", result.IID)
	}
	if result.WasCreated {
		t.Fatal("WasCreated = true, want false (reused)")
	}
	if result.State != "opened" {
		t.Fatalf("state = %s, want opened", result.State)
	}
}

func TestGitLabEnsureMergeRequestCreatesWhenMissing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "merge_requests") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]gitlabMergeRequest{})
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "merge_requests") {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(gitlabMergeRequest{
				IID:    99,
				Title:  "New MR",
				State:  "opened",
				WebURL: "https://gitlab.com/test/project/-/merge_requests/99",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	adapter := GitLabReviewAdapter{Client: server.Client()}
	result, err := adapter.EnsureMergeRequest(context.Background(), MergeRequestSpec{
		Project:      "test/project",
		SourceBranch: "feature/x",
		TargetBranch: "main",
		Title:        "New MR",
		BaseURL:      server.URL,
		Token:        "glpat-test",
	})
	if err != nil {
		t.Fatalf("EnsureMergeRequest: %v", err)
	}
	if result.IID != 99 {
		t.Fatalf("iid = %d, want 99", result.IID)
	}
	if !result.WasCreated {
		t.Fatal("WasCreated = false, want true (created)")
	}
}

func TestValidateMergeRequestSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    MergeRequestSpec
		wantErr bool
	}{
		{name: "valid", spec: MergeRequestSpec{Project: "p", SourceBranch: "s", TargetBranch: "t", Title: "x"}, wantErr: false},
		{name: "missing project", spec: MergeRequestSpec{SourceBranch: "s", TargetBranch: "t", Title: "x"}, wantErr: true},
		{name: "missing source", spec: MergeRequestSpec{Project: "p", TargetBranch: "t", Title: "x"}, wantErr: true},
		{name: "missing target", spec: MergeRequestSpec{Project: "p", SourceBranch: "s", Title: "x"}, wantErr: true},
		{name: "missing title", spec: MergeRequestSpec{Project: "p", SourceBranch: "s", TargetBranch: "t"}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMergeRequestSpec(tc.spec)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
