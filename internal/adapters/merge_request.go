package adapters

import "context"

// MergeRequestSpec describes a merge request or pull request to find or create.
type MergeRequestSpec struct {
	Provider     string
	Project      string
	Repo         string
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

// Provider-neutral aliases. GitLab merge requests and GitHub pull requests are
// both change requests; these names let callers use provider-neutral vocabulary
// without breaking existing merge-request consumers.
type ChangeRequestSpec = MergeRequestSpec
type ChangeRequestResult = MergeRequestResult
type ChangeRequestAdapter = MergeRequestAdapter
