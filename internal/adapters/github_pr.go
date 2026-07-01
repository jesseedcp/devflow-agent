package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// GitHubPullRequestAdapter finds or creates a GitHub pull request through the
// same MergeRequestAdapter contract that GitLab merge requests use.
type GitHubPullRequestAdapter struct {
	Client *http.Client
}

func (a GitHubPullRequestAdapter) httpClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

type githubPullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	Title   string `json:"title"`
	State   string `json:"state"`
}

func (a GitHubPullRequestAdapter) EnsureMergeRequest(ctx context.Context, spec MergeRequestSpec) (MergeRequestResult, error) {
	owner, name, err := splitGitHubRepo(spec.Repo)
	if err != nil {
		return MergeRequestResult{}, err
	}
	if strings.TrimSpace(spec.SourceBranch) == "" {
		return MergeRequestResult{}, fmt.Errorf("source branch is required")
	}
	if strings.TrimSpace(spec.TargetBranch) == "" {
		return MergeRequestResult{}, fmt.Errorf("target branch is required")
	}
	if strings.TrimSpace(spec.Title) == "" {
		return MergeRequestResult{}, fmt.Errorf("title is required")
	}

	token := strings.TrimSpace(spec.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token == "" {
		return MergeRequestResult{}, fmt.Errorf("github token is required (set MergeRequestSpec.Token or GITHUB_TOKEN)")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(spec.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGitHubAPIBaseURL
	}
	repoPath := url.PathEscape(owner) + "/" + url.PathEscape(name)

	existing, err := a.findOpenPullRequest(ctx, baseURL, repoPath, owner, token, spec.SourceBranch, spec.TargetBranch)
	if err != nil {
		return MergeRequestResult{}, err
	}
	if existing != nil {
		return MergeRequestResult{
			IID:        existing.Number,
			WebURL:     existing.HTMLURL,
			Title:      existing.Title,
			State:      existing.State,
			WasCreated: false,
		}, nil
	}

	created, err := a.createPullRequest(ctx, baseURL, repoPath, token, spec)
	if err != nil {
		return MergeRequestResult{}, err
	}
	return MergeRequestResult{
		IID:        created.Number,
		WebURL:     created.HTMLURL,
		Title:      created.Title,
		State:      created.State,
		WasCreated: true,
	}, nil
}

func (a GitHubPullRequestAdapter) findOpenPullRequest(ctx context.Context, baseURL, repoPath, owner, token, source, target string) (*githubPullRequest, error) {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", owner+":"+source)
	query.Set("base", target)
	endpoint := baseURL + "/repos/" + repoPath + "/pulls?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build github list pulls request: %w", err)
	}
	a.setHeaders(req, token)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("github list pull requests: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github list pull requests status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var pulls []githubPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pulls); err != nil {
		return nil, fmt.Errorf("decode github pull requests: %w", err)
	}
	for i := range pulls {
		if strings.EqualFold(pulls[i].State, "open") {
			return &pulls[i], nil
		}
	}
	if len(pulls) > 0 {
		return &pulls[0], nil
	}
	return nil, nil
}

func (a GitHubPullRequestAdapter) createPullRequest(ctx context.Context, baseURL, repoPath, token string, spec MergeRequestSpec) (*githubPullRequest, error) {
	payload := map[string]any{
		"title": spec.Title,
		"head":  spec.SourceBranch,
		"base":  spec.TargetBranch,
	}
	if strings.TrimSpace(spec.Description) != "" {
		payload["body"] = spec.Description
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode github create pull request: %w", err)
	}

	endpoint := baseURL + "/repos/" + repoPath + "/pulls"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build github create pull request: %w", err)
	}
	a.setHeaders(req, token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("github create pull request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github create pull request status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var pr githubPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode created github pull request: %w", err)
	}
	return &pr, nil
}

func (a GitHubPullRequestAdapter) setHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)
}
