package adapters

import "testing"

func TestClassifyReviewCommentByFilePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		path string
		want CommentCategory
	}{
		{name: "requirements file", body: "missing acceptance criteria", path: ".devflow/demands/add/requirements.md", want: CommentRequirements},
		{name: "plan file", body: "design misses rollback", path: ".devflow/demands/add/plan.md", want: CommentPlan},
		{name: "test file", body: "assert the boundary case", path: "internal/service/coupon_test.go", want: CommentTest},
		{name: "implementation file", body: "nil handling is wrong", path: "internal/service/coupon.go", want: CommentImplementation},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyReviewComment(tc.body, tc.path); got != tc.want {
				t.Fatalf("category = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestClassifyReviewCommentByBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want CommentCategory
	}{
		{name: "requirements keyword", body: "This changes the business rule and acceptance criteria.", want: CommentRequirements},
		{name: "plan keyword", body: "The architecture should use the existing adapter boundary.", want: CommentPlan},
		{name: "test keyword", body: "Please add regression coverage for the failure path.", want: CommentTest},
		{name: "style keyword", body: "nit: rename this helper for readability.", want: CommentStyle},
		{name: "implementation fallback", body: "This branch can panic when the user is nil.", want: CommentImplementation},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyReviewComment(tc.body, ""); got != tc.want {
				t.Fatalf("category = %s, want %s", got, tc.want)
			}
		})
	}
}
