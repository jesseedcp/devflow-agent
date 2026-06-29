package adapters

import "context"

// MergeRequestSpec describes a merge request to find or create.
type MergeRequestSpec struct {
	Project      string
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
	BaseURL      string
	Token        string
}

// MergeRequestResult describes the outcome of an EnsureMergeRequest call.
type MergeRequestResult struct {
	IID        int
	WebURL     string
	Title      string
	State      string
	WasCreated bool
}

// MergeRequestAdapter finds or creates a merge request.
type MergeRequestAdapter interface {
	EnsureMergeRequest(ctx context.Context, spec MergeRequestSpec) (MergeRequestResult, error)
}
