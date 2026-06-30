package demandflow

import (
	"fmt"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type ReviewActionPlan struct {
	Comments  []adapters.ReviewComment
	Counts    map[adapters.CommentCategory]int
	NextState workflow.State
	Message   string
}

func BuildReviewActionPlan(comments []adapters.ReviewComment) ReviewActionPlan {
	plan := ReviewActionPlan{
		Comments:  comments,
		Counts:    map[adapters.CommentCategory]int{},
		NextState: workflow.Verification,
		Message:   "mr review cleared, no blocking unresolved comments",
	}

	blockingRequirements := false
	blockingPlan := false
	blockingImplementation := false
	for i := range plan.Comments {
		comment := &plan.Comments[i]
		if comment.Category == "" {
			comment.Category = adapters.ClassifyReviewComment(comment.Body, comment.FilePath)
		}
		plan.Counts[comment.Category]++
		if !comment.Blocking {
			continue
		}
		switch comment.Category {
		case adapters.CommentRequirements:
			blockingRequirements = true
		case adapters.CommentPlan:
			blockingPlan = true
		default:
			blockingImplementation = true
		}
	}

	switch {
	case blockingRequirements:
		plan.NextState = workflow.ReturnedToRequirements
		plan.Message = "blocking review comments require requirements updates"
	case blockingPlan:
		plan.NextState = workflow.ReturnedToPlan
		plan.Message = "blocking review comments require plan updates"
	case blockingImplementation:
		plan.NextState = workflow.Implementation
		plan.Message = "blocking review comments require implementation updates"
	}
	return plan
}

func RenderReviewActionPlan(plan ReviewActionPlan) string {
	var b strings.Builder
	b.WriteString("## MR Review Action Plan\n\n")
	fmt.Fprintf(&b, "Next state: `%s`\n\n", plan.NextState)
	fmt.Fprintf(&b, "Decision: %s\n\n", plan.Message)
	b.WriteString("Category counts:\n")
	for _, category := range []adapters.CommentCategory{
		adapters.CommentRequirements,
		adapters.CommentPlan,
		adapters.CommentImplementation,
		adapters.CommentTest,
		adapters.CommentStyle,
	} {
		if plan.Counts[category] == 0 {
			continue
		}
		fmt.Fprintf(&b, "- %s: %d\n", category, plan.Counts[category])
	}
	if len(plan.Comments) == 0 {
		b.WriteString("- none: 0\n")
	}
	b.WriteString("\nActions:\n")
	if len(plan.Comments) == 0 {
		b.WriteString("- No unresolved review comments. Continue to verification.\n\n")
		return b.String()
	}
	for _, comment := range plan.Comments {
		location := strings.TrimSpace(comment.FilePath)
		if comment.Line > 0 {
			location = fmt.Sprintf("%s:%d", comment.FilePath, comment.Line)
		}
		if location == "" {
			location = "(no file location)"
		}
		status := "nonblocking"
		if comment.Blocking {
			status = "blocking"
		}
		fmt.Fprintf(&b, "- [%s][%s] %s by %s: %s\n",
			comment.Category,
			status,
			location,
			comment.Author,
			strings.TrimSpace(comment.Body),
		)
	}
	b.WriteString("\n")
	return b.String()
}
