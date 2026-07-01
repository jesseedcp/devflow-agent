package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubCIGatePassesWhenAllChecksSucceed(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/repos/owner/repo/pulls/42":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"number": 42,
				"head":   map[string]any{"sha": "abc123"},
			})
		case "/repos/owner/repo/commits/abc123/check-runs":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_count": 2,
				"check_runs": []map[string]any{
					{"name": "Go verification (ubuntu-latest)", "status": "completed", "conclusion": "success", "html_url": "https://github.test/checks/1"},
					{"name": "Go verification (windows-latest)", "status": "completed", "conclusion": "success", "html_url": "https://github.test/checks/2"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result, err := (GitHubCIAdapter{Client: server.Client()}).Check(context.Background(), CIRef{
		Repo:    "owner/repo",
		PR:      "42",
		BaseURL: server.URL,
		Token:   "secret-token",
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want Bearer token", gotAuth)
	}
	if result.Status != CIStatusPassed {
		t.Fatalf("Status = %s, want %s", result.Status, CIStatusPassed)
	}
	if result.HeadSHA != "abc123" {
		t.Fatalf("HeadSHA = %q, want abc123", result.HeadSHA)
	}
	if len(result.Checks) != 2 {
		t.Fatalf("Checks = %d, want 2", len(result.Checks))
	}
}

func TestNormalizeGitHubCIStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []CICheck
		want   CIStatus
	}{
		{name: "zero checks pending", checks: nil, want: CIStatusPending},
		{name: "failed conclusion", checks: []CICheck{{Status: "completed", Conclusion: "failure"}}, want: CIStatusFailed},
		{name: "cancelled conclusion", checks: []CICheck{{Status: "completed", Conclusion: "cancelled"}}, want: CIStatusFailed},
		{name: "in progress", checks: []CICheck{{Status: "in_progress"}}, want: CIStatusPending},
		{name: "queued", checks: []CICheck{{Status: "queued"}}, want: CIStatusPending},
		{name: "skipped is allowed", checks: []CICheck{{Status: "completed", Conclusion: "skipped"}}, want: CIStatusPassed},
		{name: "neutral is allowed", checks: []CICheck{{Status: "completed", Conclusion: "neutral"}}, want: CIStatusPassed},
		{name: "unknown conclusion", checks: []CICheck{{Status: "completed", Conclusion: "mystery"}}, want: CIStatusUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeGitHubCIStatus(tc.checks); got != tc.want {
				t.Fatalf("normalizeGitHubCIStatus() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestGitHubCIGateReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := (GitHubCIAdapter{Client: server.Client()}).Check(context.Background(), CIRef{
		Repo:    "owner/repo",
		PR:      "42",
		BaseURL: server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "github api returned 500") {
		t.Fatalf("err = %v, want github 500", err)
	}
}
