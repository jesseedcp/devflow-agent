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
		fmt.Fprintf(&b, "- [%s] %s", status, c.Author)
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
