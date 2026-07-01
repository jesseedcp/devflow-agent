package adapters

import "context"

type CommentCategory string

const (
	CommentRequirements   CommentCategory = "requirements"
	CommentPlan           CommentCategory = "plan"
	CommentImplementation CommentCategory = "implementation"
	CommentTest           CommentCategory = "test"
	CommentStyle          CommentCategory = "style"
)

type ReviewRef struct {
	Provider     string
	Project      string
	MergeRequest string
	Repo         string
	PullRequest  string
	BaseURL      string
	Token        string
}

type ReviewComment struct {
	ID       string
	Author   string
	Body     string
	FilePath string
	Line     int
	Blocking bool
	Category CommentCategory
}

type ReviewAdapter interface {
	ListUnresolved(ctx context.Context, ref ReviewRef) ([]ReviewComment, error)
	Reply(ctx context.Context, ref ReviewRef, commentID string, body string) error
}
