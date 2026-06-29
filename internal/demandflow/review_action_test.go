package demandflow

import (
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/adapters"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func TestBuildReviewActionPlanRoutesEarliestBlockingCategory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		comments []adapters.ReviewComment
		want     workflow.State
	}{
		{
			name: "requirements wins over implementation",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "business rule changed", Blocking: true, Category: adapters.CommentRequirements},
				{ID: "2", Body: "nil handling", Blocking: true, Category: adapters.CommentImplementation},
			},
			want: workflow.ReturnedToRequirements,
		},
		{
			name: "plan routes to returned_to_plan",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "adapter boundary is wrong", Blocking: true, Category: adapters.CommentPlan},
			},
			want: workflow.ReturnedToPlan,
		},
		{
			name: "test routes to implementation",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "missing regression coverage", Blocking: true, Category: adapters.CommentTest},
			},
			want: workflow.Implementation,
		},
		{
			name: "no blocking comments routes to verification",
			comments: []adapters.ReviewComment{
				{ID: "1", Body: "nit rename", Blocking: false, Category: adapters.CommentStyle},
			},
			want: workflow.Verification,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := BuildReviewActionPlan(tc.comments)
			if plan.NextState != tc.want {
				t.Fatalf("next state = %s, want %s", plan.NextState, tc.want)
			}
		})
	}
}

func TestRenderReviewActionPlanIncludesEvidence(t *testing.T) {
	t.Parallel()

	plan := BuildReviewActionPlan([]adapters.ReviewComment{
		{ID: "1", Author: "reviewer", Body: "missing acceptance criteria", Blocking: true, Category: adapters.CommentRequirements, FilePath: "requirements.md", Line: 3},
	})

	body := RenderReviewActionPlan(plan)
	for _, want := range []string{"## MR Review Action Plan", "returned_to_requirements", "[requirements]", "missing acceptance criteria"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}
