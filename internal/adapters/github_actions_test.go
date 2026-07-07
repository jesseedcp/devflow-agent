package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubActionsDispatchFindsRun(t *testing.T) {
	var sawDispatch bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/actions/workflows/release.yml/dispatches":
			sawDispatch = true
			if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
				t.Fatalf("Authorization = %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/workflows/release.yml/runs":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"workflow_runs":[{"id":123,"html_url":"https://example/runs/123","head_sha":"abc123","head_branch":"main","status":"completed","conclusion":"success","created_at":"2026-07-05T10:00:00Z","updated_at":"2026-07-05T10:02:00Z"}]}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := (GitHubActionsAdapter{Client: server.Client()}).TriggerDeployment(context.Background(), DeploymentRef{
		Repo:       "owner/repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		BaseURL:    server.URL,
		Token:      "secret-token",
	})
	if err != nil {
		t.Fatalf("TriggerDeployment returned error: %v", err)
	}
	if !sawDispatch {
		t.Fatal("dispatch endpoint was not called")
	}
	if result.Status != DeploymentStatusPassed || result.RunID != "123" || result.HeadSHA != "abc123" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGitHubActionsGetDeploymentDoesNotDispatch(t *testing.T) {
	var sawDispatch bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost:
			sawDispatch = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/workflows/release.yml/runs":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"workflow_runs":[{"id":456,"html_url":"https://example/runs/456","head_sha":"def456","head_branch":"main","status":"completed","conclusion":"failure","created_at":"2026-07-05T11:00:00Z","updated_at":"2026-07-05T11:05:00Z"}]}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := (GitHubActionsAdapter{Client: server.Client()}).GetDeployment(context.Background(), DeploymentRef{
		Repo:       "owner/repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		BaseURL:    server.URL,
		Token:      "secret-token",
	})
	if err != nil {
		t.Fatalf("GetDeployment returned error: %v", err)
	}
	if sawDispatch {
		t.Fatal("GetDeployment should not dispatch")
	}
	if result.Status != DeploymentStatusFailed || result.RunID != "456" {
		t.Fatalf("result = %#v", result)
	}
}

func TestGitHubActionsPendingRunMapsToPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"workflow_runs":[{"id":789,"html_url":"https://example/runs/789","head_sha":"ghi789","head_branch":"main","status":"in_progress","conclusion":null,"created_at":"2026-07-05T12:00:00Z","updated_at":"2026-07-05T12:01:00Z"}]}`)
	}))
	defer server.Close()

	result, err := (GitHubActionsAdapter{Client: server.Client()}).TriggerDeployment(context.Background(), DeploymentRef{
		Repo:       "owner/repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		BaseURL:    server.URL,
		Token:      "secret-token",
	})
	if err != nil {
		t.Fatalf("TriggerDeployment returned error: %v", err)
	}
	if result.Status != DeploymentStatusPending {
		t.Fatalf("status = %s, want pending", result.Status)
	}
}

func TestGitHubActionsMissingTokenReturnsError(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	_, err := (GitHubActionsAdapter{}).TriggerDeployment(context.Background(), DeploymentRef{
		Repo:       "owner/repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		Token:      "",
	})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestGitHubActionsNon2xxReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := (GitHubActionsAdapter{Client: server.Client()}).TriggerDeployment(context.Background(), DeploymentRef{
		Repo:       "owner/repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		BaseURL:    server.URL,
		Token:      "secret-token",
	})
	if err == nil {
		t.Fatal("expected error for non-2xx response")
	}
}

func TestGitHubActionsRejectsInvalidRepo(t *testing.T) {
	_, err := (GitHubActionsAdapter{}).TriggerDeployment(context.Background(), DeploymentRef{
		Repo:       "not-a-repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		Token:      "secret-token",
	})
	if err == nil {
		t.Fatal("expected error for invalid repo")
	}
}

func TestGitHubActionsDispatchSendsInputs(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/actions/workflows/release.yml/dispatches":
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/actions/workflows/release.yml/runs":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"workflow_runs":[{"id":124,"html_url":"https://example/runs/124","head_sha":"abc124","head_branch":"main","status":"completed","conclusion":"success","created_at":"2099-01-01T00:00:00Z","updated_at":"2099-01-01T00:02:00Z"}]}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := (GitHubActionsAdapter{Client: server.Client()}).TriggerDeployment(context.Background(), DeploymentRef{
		Repo:       "owner/repo",
		WorkflowID: "release.yml",
		Ref:        "main",
		BaseURL:    server.URL,
		Token:      "secret-token",
		Inputs: map[string]string{
			"demand_id":    "release-dogfood",
			"release_note": "safe marker",
		},
	})
	if err != nil {
		t.Fatalf("TriggerDeployment returned error: %v", err)
	}
	if captured["ref"] != "main" {
		t.Fatalf("ref = %#v", captured["ref"])
	}
	inputs, ok := captured["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("inputs = %#v", captured["inputs"])
	}
	if inputs["demand_id"] != "release-dogfood" || inputs["release_note"] != "safe marker" {
		t.Fatalf("inputs = %#v", inputs)
	}
}
