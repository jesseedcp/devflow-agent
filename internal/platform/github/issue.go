package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/platform"
)

const defaultAPIBaseURL = "https://api.github.com"

type IssueAdapter struct {
	Client *http.Client
}

type issueResponse struct {
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	HTMLURL     string          `json:"html_url"`
	State       string          `json:"state"`
	User        userResponse    `json:"user"`
	Labels      []labelResponse `json:"labels"`
	PullRequest *struct{}       `json:"pull_request"`
}

type labelResponse struct {
	Name string `json:"name"`
}

type userResponse struct {
	Login string `json:"login"`
}

type commentResponse struct {
	ID        int64        `json:"id"`
	Body      string       `json:"body"`
	HTMLURL   string       `json:"html_url"`
	CreatedAt time.Time    `json:"created_at"`
	User      userResponse `json:"user"`
}

type commentListResponse struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

func (a IssueAdapter) FetchIntake(ctx context.Context, ref platform.IntakeRef) (platform.IntakeSnapshot, error) {
	repo, issue, err := normalizeIssueRef(ref)
	if err != nil {
		return platform.IntakeSnapshot{}, err
	}
	token, err := tokenFromRef(ref)
	if err != nil {
		return platform.IntakeSnapshot{}, err
	}
	baseURL := normalizedBaseURL(ref.BaseURL)
	repoPath := repoAPIPath(repo)

	var issueResp issueResponse
	if err := a.getJSON(ctx, baseURL+"/repos/"+repoPath+"/issues/"+url.PathEscape(issue), token, &issueResp); err != nil {
		return platform.IntakeSnapshot{}, fmt.Errorf("fetch github issue: %w", err)
	}
	if issueResp.PullRequest != nil {
		return platform.IntakeSnapshot{}, fmt.Errorf("github issue intake requires a GitHub Issue, got pull request #%d", issueResp.Number)
	}

	var comments []commentResponse
	if err := a.getJSON(ctx, baseURL+"/repos/"+repoPath+"/issues/"+url.PathEscape(issue)+"/comments", token, &comments); err != nil {
		return platform.IntakeSnapshot{}, fmt.Errorf("fetch github issue comments: %w", err)
	}

	labels := make([]string, 0, len(issueResp.Labels))
	for _, label := range issueResp.Labels {
		if strings.TrimSpace(label.Name) != "" {
			labels = append(labels, label.Name)
		}
	}
	externalComments := make([]platform.ExternalComment, 0, len(comments))
	for _, comment := range comments {
		externalComments = append(externalComments, platform.ExternalComment{
			ID:        strconv.FormatInt(comment.ID, 10),
			Author:    comment.User.Login,
			Body:      comment.Body,
			URL:       comment.HTMLURL,
			CreatedAt: comment.CreatedAt,
		})
	}

	return platform.IntakeSnapshot{
		Provider:   platform.ProviderGitHub,
		Kind:       platform.SourceGitHubIssue,
		ExternalID: repo + "#" + issue,
		Title:      issueResp.Title,
		Body:       issueResp.Body,
		URL:        issueResp.HTMLURL,
		Author:     issueResp.User.Login,
		Labels:     labels,
		Comments:   externalComments,
		Metadata: map[string]string{
			"state": issueResp.State,
		},
		FetchedAt: time.Now().UTC(),
	}, nil
}

func (a IssueAdapter) PostProgress(ctx context.Context, ref platform.IntakeRef, update platform.ProgressUpdate) error {
	repo, issue, err := normalizeIssueRef(ref)
	if err != nil {
		return err
	}
	token, err := tokenFromRef(ref)
	if err != nil {
		return err
	}
	if update.DryRun {
		return nil
	}
	body := platform.RenderProgressComment(update)
	marker := strings.TrimSpace(update.Marker)
	if marker == "" {
		marker = platform.SyncMarker(update.DemandID, update.Stage)
	}
	return a.upsertIssueComment(ctx, ref, repo, issue, token, marker, body)
}

func (a IssueAdapter) PostCloseout(ctx context.Context, ref platform.IntakeRef, update platform.CloseoutUpdate) error {
	repo, issue, err := normalizeIssueRef(ref)
	if err != nil {
		return err
	}
	token, err := tokenFromRef(ref)
	if err != nil {
		return err
	}
	if update.DryRun {
		return nil
	}
	body := platform.RenderCloseoutComment(update)
	marker := strings.TrimSpace(update.Marker)
	if marker == "" {
		marker = platform.SyncMarker(update.DemandID, "closeout")
	}
	return a.upsertIssueComment(ctx, ref, repo, issue, token, marker, body)
}

func (a IssueAdapter) upsertIssueComment(ctx context.Context, ref platform.IntakeRef, repo, issue, token, marker, body string) error {
	baseURL := normalizedBaseURL(ref.BaseURL)
	repoPath := repoAPIPath(repo)
	commentsURL := baseURL + "/repos/" + repoPath + "/issues/" + url.PathEscape(issue) + "/comments"
	var comments []commentListResponse
	if err := a.getJSON(ctx, commentsURL, token, &comments); err != nil {
		return fmt.Errorf("fetch github issue comments for sync: %w", err)
	}
	for _, comment := range comments {
		if strings.Contains(comment.Body, marker) {
			return a.writeComment(ctx, http.MethodPatch, baseURL+"/repos/"+repoPath+"/issues/comments/"+strconv.FormatInt(comment.ID, 10), token, body)
		}
	}
	return a.writeComment(ctx, http.MethodPost, commentsURL, token, body)
}

func (a IssueAdapter) writeComment(ctx context.Context, method, endpoint, token, body string) error {
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("encode github issue comment: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("build github issue comment request: %w", err)
	}
	setHeaders(req, token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("send github issue comment request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github issue comment returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func normalizeIssueRef(ref platform.IntakeRef) (string, string, error) {
	repo := strings.TrimSpace(ref.Repo)
	issue := strings.TrimSpace(ref.Issue)
	if repo == "" || issue == "" {
		return "", "", fmt.Errorf("github issue ref requires repo and issue")
	}
	if !strings.Contains(repo, "/") {
		return "", "", fmt.Errorf("github repo must be owner/repo")
	}
	return repo, issue, nil
}

func tokenFromRef(ref platform.IntakeRef) (string, error) {
	token := strings.TrimSpace(ref.Token)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}
	if token == "" {
		return "", fmt.Errorf("github token is required (set IntakeRef.Token or GITHUB_TOKEN)")
	}
	return token, nil
}

func normalizedBaseURL(raw string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}
	return baseURL
}

func repoAPIPath(repo string) string {
	parts := strings.SplitN(repo, "/", 2)
	return url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1])
}

func (a IssueAdapter) httpClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a IssueAdapter) getJSON(ctx context.Context, endpoint, token string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build github request: %w", err)
	}
	setHeaders(req, token)
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("send github request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github api returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

func setHeaders(req *http.Request, token string) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+token)
}
