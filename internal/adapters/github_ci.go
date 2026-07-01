package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const defaultGitHubAPIBaseURL = "https://api.github.com"

type GitHubCIAdapter struct {
	Client *http.Client
}

type githubPullResponse struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type githubCheckRunsResponse struct {
	TotalCount int              `json:"total_count"`
	CheckRuns  []githubCheckRun `json:"check_runs"`
}

type githubCheckRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HTMLURL    string `json:"html_url"`
}

func (a GitHubCIAdapter) Check(ctx context.Context, ref CIRef) (CIResult, error) {
	repo := strings.TrimSpace(ref.Repo)
	pr := strings.TrimSpace(ref.PR)
	if repo == "" || !strings.Contains(repo, "/") {
		return CIResult{}, fmt.Errorf("github repo must be owner/repo")
	}
	if pr == "" {
		return CIResult{}, fmt.Errorf("github pr is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(ref.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGitHubAPIBaseURL
	}
	token := strings.TrimSpace(ref.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}

	var pull githubPullResponse
	if err := a.getJSON(ctx, baseURL+"/repos/"+githubRepoPath(repo)+"/pulls/"+url.PathEscape(pr), token, &pull); err != nil {
		return CIResult{}, fmt.Errorf("fetch github pull request: %w", err)
	}
	if strings.TrimSpace(pull.Head.SHA) == "" {
		return CIResult{}, fmt.Errorf("github pull request response missing head sha")
	}

	var checks githubCheckRunsResponse
	checksURL := baseURL + "/repos/" + githubRepoPath(repo) + "/commits/" + url.PathEscape(pull.Head.SHA) + "/check-runs"
	if err := a.getJSON(ctx, checksURL, token, &checks); err != nil {
		return CIResult{}, fmt.Errorf("fetch github check runs: %w", err)
	}

	result := CIResult{
		Provider: "github",
		Repo:     repo,
		PR:       pr,
		HeadSHA:  pull.Head.SHA,
		Checks:   make([]CICheck, 0, len(checks.CheckRuns)),
	}
	for _, check := range checks.CheckRuns {
		result.Checks = append(result.Checks, CICheck{
			Name:       check.Name,
			Status:     check.Status,
			Conclusion: check.Conclusion,
			URL:        check.HTMLURL,
		})
	}
	result.Status = normalizeGitHubCIStatus(result.Checks)
	result.Message = githubCIMessage(result)
	return result, nil
}

func githubRepoPath(repo string) string {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return url.PathEscape(repo)
	}
	return url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
}

func (a GitHubCIAdapter) getJSON(ctx context.Context, endpoint, token string, out any) error {
	client := a.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("github api returned %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

func normalizeGitHubCIStatus(checks []CICheck) CIStatus {
	if len(checks) == 0 {
		return CIStatusPending
	}
	pending := false
	for _, check := range checks {
		status := strings.ToLower(strings.TrimSpace(check.Status))
		conclusion := strings.ToLower(strings.TrimSpace(check.Conclusion))
		if status != "completed" || conclusion == "" {
			pending = true
			continue
		}
		switch conclusion {
		case "success", "neutral", "skipped":
		case "failure", "cancelled", "timed_out", "action_required":
			return CIStatusFailed
		default:
			return CIStatusUnknown
		}
	}
	if pending {
		return CIStatusPending
	}
	return CIStatusPassed
}

func githubCIMessage(result CIResult) string {
	switch result.Status {
	case CIStatusPassed:
		return "github ci passed"
	case CIStatusFailed:
		return "github ci failed"
	case CIStatusPending:
		return "github ci pending"
	default:
		return "github ci status unknown"
	}
}
