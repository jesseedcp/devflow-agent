package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const defaultGitHubGraphQLURL = "https://api.github.com/graphql"

// GitHubReviewAdapter reads unresolved GitHub pull request review threads and
// maps them into the provider-neutral ReviewComment contract.
type GitHubReviewAdapter struct {
	Client *http.Client
}

func (a GitHubReviewAdapter) httpClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a GitHubReviewAdapter) graphqlURL(ref ReviewRef) string {
	base := strings.TrimSpace(ref.BaseURL)
	if base == "" {
		return defaultGitHubGraphQLURL
	}
	return strings.TrimRight(base, "/")
}

func (a GitHubReviewAdapter) token(ref ReviewRef) (string, error) {
	if token := strings.TrimSpace(ref.Token); token != "" {
		return token, nil
	}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("github token is required (set ReviewRef.Token or GITHUB_TOKEN)")
}

const githubReviewThreadsQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewThreads(first:100){nodes{id isResolved comments(first:1){nodes{databaseId body path line originalLine author{login}}}}}}}}`

type githubGraphQLError struct {
	Message string `json:"message"`
}

type githubReviewThreadsResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					Nodes []githubReviewThread `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []githubGraphQLError `json:"errors"`
}

type githubReviewThread struct {
	ID         string `json:"id"`
	IsResolved bool   `json:"isResolved"`
	Comments   struct {
		Nodes []githubReviewComment `json:"nodes"`
	} `json:"comments"`
}

type githubReviewComment struct {
	DatabaseID   int64  `json:"databaseId"`
	Body         string `json:"body"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
	OriginalLine int    `json:"originalLine"`
	Author       struct {
		Login string `json:"login"`
	} `json:"author"`
}

func (a GitHubReviewAdapter) ListUnresolved(ctx context.Context, ref ReviewRef) ([]ReviewComment, error) {
	owner, name, err := splitGitHubRepo(ref.Repo)
	if err != nil {
		return nil, err
	}
	pr := strings.TrimSpace(ref.PullRequest)
	if pr == "" {
		return nil, fmt.Errorf("github pull request is required")
	}
	number, err := strconv.Atoi(pr)
	if err != nil {
		return nil, fmt.Errorf("github pull request must be a number: %w", err)
	}

	token, err := a.token(ref)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"query": githubReviewThreadsQuery,
		"variables": map[string]any{
			"owner":  owner,
			"name":   name,
			"number": number,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode github graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.graphqlURL(ref), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build github graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("github graphql request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("github graphql returned %d", resp.StatusCode)
	}

	var decoded githubReviewThreadsResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode github graphql response: %w", err)
	}
	if len(decoded.Errors) > 0 {
		return nil, fmt.Errorf("github graphql error: %s", decoded.Errors[0].Message)
	}

	var comments []ReviewComment
	for _, thread := range decoded.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if thread.IsResolved {
			continue
		}
		if len(thread.Comments.Nodes) == 0 {
			continue
		}
		first := thread.Comments.Nodes[0]
		line := first.Line
		if line == 0 {
			line = first.OriginalLine
		}
		comments = append(comments, ReviewComment{
			ID:       thread.ID + ":" + strconv.FormatInt(first.DatabaseID, 10),
			Author:   first.Author.Login,
			Body:     first.Body,
			FilePath: first.Path,
			Line:     line,
			Blocking: true,
			Category: ClassifyReviewComment(first.Body, first.Path),
		})
	}
	return comments, nil
}

func (a GitHubReviewAdapter) Reply(ctx context.Context, ref ReviewRef, commentID string, body string) error {
	return fmt.Errorf("github review replies are not implemented in Wave 26")
}

func splitGitHubRepo(repo string) (string, string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" || !strings.Contains(repo, "/") {
		return "", "", fmt.Errorf("github repo must be owner/repo")
	}
	parts := strings.SplitN(repo, "/", 2)
	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return "", "", fmt.Errorf("github repo must be owner/repo")
	}
	return owner, name, nil
}
