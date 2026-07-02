package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/platform"
)

func TestIssueAdapterFetchesIssueAndComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/repo/issues/123":
			json.NewEncoder(w).Encode(map[string]any{
				"number":   123,
				"title":    "Coupon issue",
				"body":     "Users need coupon eligibility.",
				"html_url": "https://github.com/owner/repo/issues/123",
				"user":     map[string]any{"login": "alice"},
				"labels":   []map[string]any{{"name": "backend"}, {"name": "priority-high"}},
				"state":    "open",
			})
		case "/repos/owner/repo/issues/123/comments":
			json.NewEncoder(w).Encode([]map[string]any{{
				"id":         10,
				"body":       "Remember inactive users.",
				"html_url":   "https://github.com/owner/repo/issues/123#issuecomment-10",
				"created_at": "2026-07-02T02:03:04Z",
				"user":       map[string]any{"login": "bob"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	got, err := (IssueAdapter{Client: server.Client()}).FetchIntake(context.Background(), platform.IntakeRef{
		Provider: platform.ProviderGitHub,
		Kind:     platform.SourceGitHubIssue,
		Repo:     "owner/repo",
		Issue:    "123",
		Token:    "token",
		BaseURL:  server.URL,
	})
	if err != nil {
		t.Fatalf("FetchIntake returned error: %v", err)
	}
	if got.ExternalID != "owner/repo#123" {
		t.Fatalf("ExternalID = %q", got.ExternalID)
	}
	if got.Title != "Coupon issue" || got.Author != "alice" {
		t.Fatalf("snapshot = %#v", got)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "backend" || got.Labels[1] != "priority-high" {
		t.Fatalf("Labels = %#v", got.Labels)
	}
	if len(got.Comments) != 1 || got.Comments[0].Author != "bob" {
		t.Fatalf("Comments = %#v", got.Comments)
	}
}

func TestIssueAdapterRejectsPullRequestIssue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"number":       123,
			"title":        "PR",
			"body":         "not an issue",
			"html_url":     "https://github.com/owner/repo/pull/123",
			"pull_request": map[string]any{"url": "https://api.github.com/repos/owner/repo/pulls/123"},
			"user":         map[string]any{"login": "alice"},
		})
	}))
	defer server.Close()

	_, err := (IssueAdapter{Client: server.Client()}).FetchIntake(context.Background(), platform.IntakeRef{
		Repo:    "owner/repo",
		Issue:   "123",
		Token:   "token",
		BaseURL: server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "github issue intake requires a GitHub Issue, got pull request #123") {
		t.Fatalf("err = %v", err)
	}
}

func TestIssueAdapterPostProgressCreatesComment(t *testing.T) {
	var created string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/issues/123/comments":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/issues/123/comments":
			body, _ := io.ReadAll(r.Body)
			created = string(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":11}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err := (IssueAdapter{Client: server.Client()}).PostProgress(context.Background(), platform.IntakeRef{
		Repo:    "owner/repo",
		Issue:   "123",
		Token:   "token",
		BaseURL: server.URL,
	}, platform.ProgressUpdate{
		DemandID: "coupon",
		Stage:    "plan",
		State:    "plan_review",
		Summary:  "Plan ready.",
		Marker:   platform.SyncMarker("coupon", "plan"),
	})
	if err != nil {
		t.Fatalf("PostProgress returned error: %v", err)
	}
	if !strings.Contains(created, "Devflow Update: plan") {
		t.Fatalf("created body missing update: %s", created)
	}
}

func TestIssueAdapterPostProgressUpdatesExistingComment(t *testing.T) {
	marker := platform.SyncMarker("coupon", "plan")
	var patched string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/issues/123/comments":
			_, _ = w.Write([]byte(`[{"id":42,"body":"` + marker + `\n\nold"}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/owner/repo/issues/comments/42":
			body, _ := io.ReadAll(r.Body)
			patched = string(body)
			_, _ = w.Write([]byte(`{"id":42}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	err := (IssueAdapter{Client: server.Client()}).PostProgress(context.Background(), platform.IntakeRef{
		Repo:    "owner/repo",
		Issue:   "123",
		Token:   "token",
		BaseURL: server.URL,
	}, platform.ProgressUpdate{
		DemandID: "coupon",
		Stage:    "plan",
		State:    "plan_review",
		Summary:  "Plan updated.",
		Marker:   marker,
	})
	if err != nil {
		t.Fatalf("PostProgress returned error: %v", err)
	}
	if !strings.Contains(patched, "Plan updated.") {
		t.Fatalf("patched body missing update: %s", patched)
	}
}
