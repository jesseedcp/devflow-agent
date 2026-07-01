package demandflow

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
)

type ReviewOptions struct {
	Adapter adapters.ReviewAdapter
	Ref     adapters.ReviewRef
}

func renderReviewSummary(comments []adapters.ReviewComment) string {
	var b strings.Builder
	b.WriteString("## MR 评审摘要\n\n")
	if len(comments) == 0 {
		b.WriteString("No unresolved review comments.\n\n")
		return b.String()
	}
	for _, c := range comments {
		status := "nonblocking"
		if c.Blocking {
			status = "blocking"
		}
		category := c.Category
		if category == "" {
			category = adapters.ClassifyReviewComment(c.Body, c.FilePath)
		}
		fmt.Fprintf(&b, "- [%s][%s] %s", category, status, c.Author)
		if c.FilePath != "" {
			fmt.Fprintf(&b, " (%s:%d)", c.FilePath, c.Line)
		}
		b.WriteString(": ")
		b.WriteString(strings.TrimSpace(c.Body))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

// MergeRequestOptions describes an optional merge request sync operation.
type MergeRequestOptions struct {
	Adapter adapters.MergeRequestAdapter
	Spec    adapters.MergeRequestSpec
}

// ChangeRequestOptions is a provider-neutral alias for MergeRequestOptions.
type ChangeRequestOptions = MergeRequestOptions

// resolveChangeRequestOptions returns the change-request sync options in effect.
// The provider-neutral ChangeRequest field wins when it carries an adapter,
// otherwise the legacy MergeRequest field is used for backward compatibility.
func resolveChangeRequestOptions(opts Options) MergeRequestOptions {
	if opts.ChangeRequest.Adapter != nil {
		return opts.ChangeRequest
	}
	return opts.MergeRequest
}
