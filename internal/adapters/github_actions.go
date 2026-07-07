package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type DeploymentStatus string

const (
	DeploymentStatusPassed  DeploymentStatus = "passed"
	DeploymentStatusFailed  DeploymentStatus = "failed"
	DeploymentStatusPending DeploymentStatus = "pending"
	DeploymentStatusUnknown DeploymentStatus = "unknown"
)

type DeploymentRef struct {
	Repo        string
	WorkflowID  string
	Ref         string
	Environment string
	BaseURL     string
	Token       string
	Inputs      map[string]string
}

type DeploymentResult struct {
	Provider    string
	Repo        string
	WorkflowID  string
	Ref         string
	Environment string
	RunID       string
	RunURL      string
	HeadSHA     string
	Status      DeploymentStatus
	Conclusion  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Message     string
}

type DeploymentAdapter interface {
	TriggerDeployment(ctx context.Context, ref DeploymentRef) (DeploymentResult, error)
	GetDeployment(ctx context.Context, ref DeploymentRef) (DeploymentResult, error)
}

type GitHubActionsAdapter struct {
	Client       *http.Client
	PollInterval time.Duration
	Now          func() time.Time
}

type githubWorkflowRunsResponse struct {
	WorkflowRuns []githubWorkflowRun `json:"workflow_runs"`
}

type githubWorkflowRun struct {
	ID         int64     `json:"id"`
	HTMLURL    string    `json:"html_url"`
	HeadSHA    string    `json:"head_sha"`
	HeadBranch string    `json:"head_branch"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (a GitHubActionsAdapter) TriggerDeployment(ctx context.Context, ref DeploymentRef) (DeploymentResult, error) {
	if err := validateDeploymentRef(ref); err != nil {
		return DeploymentResult{}, err
	}
	baseURL, token, err := deploymentEndpointConfig(ref)
	if err != nil {
		return DeploymentResult{}, err
	}
	startedAt := a.now().Add(-5 * time.Second)
	if err := a.dispatchWorkflow(ctx, baseURL, token, ref); err != nil {
		return DeploymentResult{}, err
	}
	return a.fetchNewestRunAfter(ctx, baseURL, token, ref, startedAt)
}

func (a GitHubActionsAdapter) GetDeployment(ctx context.Context, ref DeploymentRef) (DeploymentResult, error) {
	if err := validateDeploymentRef(ref); err != nil {
		return DeploymentResult{}, err
	}
	baseURL, token, err := deploymentEndpointConfig(ref)
	if err != nil {
		return DeploymentResult{}, err
	}
	return a.fetchNewestRun(ctx, baseURL, token, ref)
}

func validateDeploymentRef(ref DeploymentRef) error {
	repo := strings.TrimSpace(ref.Repo)
	if repo == "" || !strings.Contains(repo, "/") {
		return fmt.Errorf("github repo must be owner/repo")
	}
	if strings.TrimSpace(ref.WorkflowID) == "" {
		return fmt.Errorf("github workflow id is required")
	}
	if strings.TrimSpace(ref.Ref) == "" {
		return fmt.Errorf("github ref is required")
	}
	return nil
}

func deploymentEndpointConfig(ref DeploymentRef) (string, string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(ref.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGitHubAPIBaseURL
	}
	token := strings.TrimSpace(ref.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token == "" {
		return "", "", fmt.Errorf("github token is required for deployment control")
	}
	return baseURL, token, nil
}

func (a GitHubActionsAdapter) dispatchWorkflow(ctx context.Context, baseURL, token string, ref DeploymentRef) error {
	client := a.client()
	inputs := map[string]string{}
	for key, value := range ref.Inputs {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		inputs[key] = value
	}
	if strings.TrimSpace(ref.Environment) != "" {
		inputs["environment"] = ref.Environment
	}

	payload := map[string]any{"ref": ref.Ref}
	if len(inputs) > 0 {
		payload["inputs"] = inputs
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode dispatch body: %w", err)
	}

	endpoint := baseURL + "/repos/" + githubRepoPath(ref.Repo) + "/actions/workflows/" + url.PathEscape(ref.WorkflowID) + "/dispatches"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch github actions workflow: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("github dispatch returned %d", resp.StatusCode)
	}
	return nil
}

func (a GitHubActionsAdapter) fetchNewestRun(ctx context.Context, baseURL, token string, ref DeploymentRef) (DeploymentResult, error) {
	result, _, err := a.fetchNewestRunWithRaw(ctx, baseURL, token, ref)
	return result, err
}

func (a GitHubActionsAdapter) fetchNewestRunWithRaw(ctx context.Context, baseURL, token string, ref DeploymentRef) (DeploymentResult, githubWorkflowRun, error) {
	client := a.client()
	runsURL := baseURL + "/repos/" + githubRepoPath(ref.Repo) + "/actions/workflows/" + url.PathEscape(ref.WorkflowID) + "/runs"
	query := url.Values{}
	query.Set("branch", ref.Ref)
	query.Set("event", "workflow_dispatch")
	query.Set("per_page", "20")
	runsURL = runsURL + "?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, runsURL, nil)
	if err != nil {
		return DeploymentResult{}, githubWorkflowRun{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return DeploymentResult{}, githubWorkflowRun{}, fmt.Errorf("fetch github actions runs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return DeploymentResult{}, githubWorkflowRun{}, fmt.Errorf("github runs returned %d", resp.StatusCode)
	}

	var runsResponse githubWorkflowRunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&runsResponse); err != nil {
		return DeploymentResult{}, githubWorkflowRun{}, fmt.Errorf("decode github runs response: %w", err)
	}
	if len(runsResponse.WorkflowRuns) == 0 {
		return DeploymentResult{}, githubWorkflowRun{}, fmt.Errorf("no github actions runs found for workflow %s ref %s", ref.WorkflowID, ref.Ref)
	}

	run := selectNewestRun(runsResponse.WorkflowRuns, ref.Ref)
	status := normalizeGitHubActionsDeploymentStatus(run.Status, run.Conclusion)
	return DeploymentResult{
		Provider:    "github_actions",
		Repo:        ref.Repo,
		WorkflowID:  ref.WorkflowID,
		Ref:         ref.Ref,
		Environment: ref.Environment,
		RunID:       strconv.FormatInt(run.ID, 10),
		RunURL:      run.HTMLURL,
		HeadSHA:     run.HeadSHA,
		Status:      status,
		Conclusion:  run.Conclusion,
		CreatedAt:   run.CreatedAt,
		UpdatedAt:   run.UpdatedAt,
		Message:     deploymentMessage(status),
	}, run, nil
}

func (a GitHubActionsAdapter) fetchNewestRunAfter(ctx context.Context, baseURL, token string, ref DeploymentRef, after time.Time) (DeploymentResult, error) {
	ticker := time.NewTicker(a.pollInterval())
	defer ticker.Stop()

	for {
		result, run, err := a.fetchNewestRunWithRaw(ctx, baseURL, token, ref)
		if err != nil {
			return DeploymentResult{}, err
		}
		if !run.CreatedAt.Before(after) {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return DeploymentResult{}, fmt.Errorf("wait for newly dispatched github actions run: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (a GitHubActionsAdapter) client() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a GitHubActionsAdapter) now() time.Time {
	if a.Now != nil {
		return a.Now().UTC()
	}
	return time.Now().UTC()
}

func (a GitHubActionsAdapter) pollInterval() time.Duration {
	if a.PollInterval > 0 {
		return a.PollInterval
	}
	return 2 * time.Second
}

func selectNewestRun(runs []githubWorkflowRun, ref string) githubWorkflowRun {
	var best githubWorkflowRun
	var bestSet bool
	for _, run := range runs {
		if run.HeadBranch != "" && run.HeadBranch != ref {
			continue
		}
		if !bestSet || run.CreatedAt.After(best.CreatedAt) {
			best = run
			bestSet = true
		}
	}
	if bestSet {
		return best
	}
	for _, run := range runs {
		if !bestSet || run.CreatedAt.After(best.CreatedAt) {
			best = run
			bestSet = true
		}
	}
	return best
}

func normalizeGitHubActionsDeploymentStatus(status, conclusion string) DeploymentStatus {
	status = strings.ToLower(strings.TrimSpace(status))
	conclusion = strings.ToLower(strings.TrimSpace(conclusion))
	if status != "completed" {
		return DeploymentStatusPending
	}
	switch conclusion {
	case "success", "neutral", "skipped":
		return DeploymentStatusPassed
	case "failure", "cancelled", "timed_out", "action_required":
		return DeploymentStatusFailed
	default:
		return DeploymentStatusUnknown
	}
}

func deploymentMessage(status DeploymentStatus) string {
	switch status {
	case DeploymentStatusPassed:
		return "deployment passed"
	case DeploymentStatusFailed:
		return "deployment failed"
	case DeploymentStatusPending:
		return "deployment pending"
	default:
		return "deployment status unknown"
	}
}
