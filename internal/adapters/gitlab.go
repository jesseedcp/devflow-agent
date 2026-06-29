package adapters

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
)

const defaultGitLabBaseURL = "https://gitlab.com"

type GitLabReviewAdapter struct {
	Client *http.Client
}

func (a GitLabReviewAdapter) httpClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a GitLabReviewAdapter) baseURL(ref ReviewRef) string {
	if ref.BaseURL != "" {
		return strings.TrimRight(ref.BaseURL, "/")
	}
	return defaultGitLabBaseURL
}

func (a GitLabReviewAdapter) token(ref ReviewRef) (string, error) {
	if ref.Token != "" {
		return ref.Token, nil
	}
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("gitlab token is required (set ReviewRef.Token or GITLAB_TOKEN)")
}

func (a GitLabReviewAdapter) ListUnresolved(ctx context.Context, ref ReviewRef) ([]ReviewComment, error) {
	token, err := a.token(ref)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s/discussions",
		a.baseURL(ref),
		url.PathEscape(ref.Project),
		url.PathEscape(ref.MergeRequest),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build gitlab request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab list discussions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab list discussions status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var discussions []gitlabDiscussion
	if err := json.NewDecoder(resp.Body).Decode(&discussions); err != nil {
		return nil, fmt.Errorf("decode gitlab discussions: %w", err)
	}

	var comments []ReviewComment
	for _, discussion := range discussions {
		for _, note := range discussion.Notes {
			if !note.Resolvable || note.Resolved {
				continue
			}
			comment := ReviewComment{
				ID:       discussion.ID + ":" + strconv.FormatInt(note.ID, 10),
				Author:   note.Author.Username,
				Body:     note.Body,
				Blocking: true,
			}
			if note.Position != nil {
				if note.Position.NewPath != "" {
					comment.FilePath = note.Position.NewPath
				}
				if note.Position.NewLine != 0 {
					comment.Line = note.Position.NewLine
				}
			}
			comments = append(comments, comment)
		}
	}
	return comments, nil
}

func (a GitLabReviewAdapter) Reply(ctx context.Context, ref ReviewRef, commentID string, body string) error {
	token, err := a.token(ref)
	if err != nil {
		return err
	}

	parts := strings.SplitN(commentID, ":", 2)
	if len(parts) != 2 || parts[0] == "" {
		return fmt.Errorf("invalid gitlab comment id %q", commentID)
	}
	discussionID := parts[0]

	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%s/discussions/%s/notes",
		a.baseURL(ref),
		url.PathEscape(ref.Project),
		url.PathEscape(ref.MergeRequest),
		url.PathEscape(discussionID),
	)

	form := url.Values{}
	form.Set("body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build gitlab reply request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("gitlab reply: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitlab reply status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	io.Copy(io.Discard, resp.Body)
	return nil
}

type gitlabDiscussion struct {
	ID    string       `json:"id"`
	Notes []gitlabNote `json:"notes"`
}

type gitlabNote struct {
	ID         int64               `json:"id"`
	Body       string              `json:"body"`
	Resolved   bool                `json:"resolved"`
	Resolvable bool                `json:"resolvable"`
	Author     gitlabNoteAuthor    `json:"author"`
	Position   *gitlabNotePosition `json:"position"`
}

type gitlabNoteAuthor struct {
	Username string `json:"username"`
}

type gitlabNotePosition struct {
	NewPath string `json:"new_path"`
	NewLine int    `json:"new_line"`
}
