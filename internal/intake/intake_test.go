package intake

import (
	"strings"
	"testing"
)

func TestParseMarkdownPRDExtractsRequirementsDraft(t *testing.T) {
	input := `# Coupon eligibility

## 背景
Marketing wants to block inactive users from claiming coupons.

## 目标
- Active members can claim eligible coupons.
- Inactive members are blocked with a clear reason.

## 非目标
- Do not redesign coupon creation.

## 业务规则
- User status must be active.
- Coupon must be inside the claim window.

## 验收标准
- Given an active member, claim succeeds.
- Given an inactive member, claim fails.

## 待确认
- Should expired coupons return a business code or generic error?
`

	result := ParseMarkdown(Source{
		Path: "docs/examples/demands/coupon-eligibility.md",
		Text: input,
	})

	if result.Title != "Coupon eligibility" {
		t.Fatalf("Title = %q, want Coupon eligibility", result.Title)
	}
	for _, want := range []string{
		"Active members can claim eligible coupons.",
		"User status must be active.",
		"Given an inactive member, claim fails.",
		"Should expired coupons return a business code or generic error?",
	} {
		if !strings.Contains(result.RequirementsMarkdown, want) {
			t.Fatalf("requirements missing %q:\n%s", want, result.RequirementsMarkdown)
		}
	}
	if result.Readiness != ReadinessNeedsReview {
		t.Fatalf("Readiness = %q, want %q", result.Readiness, ReadinessNeedsReview)
	}
}

func TestParseMarkdownPRDUsesFileNameWhenHeadingMissing(t *testing.T) {
	result := ParseMarkdown(Source{
		Path: "docs/examples/demands/refund-policy.md",
		Text: "Refunds should be rejected after the configured window.",
	})

	if result.Title != "refund policy" {
		t.Fatalf("Title = %q, want refund policy", result.Title)
	}
	if !strings.Contains(result.RequirementsMarkdown, "Refunds should be rejected after the configured window.") {
		t.Fatalf("requirements missing raw text:\n%s", result.RequirementsMarkdown)
	}
	if !strings.Contains(result.RequirementsMarkdown, "请确认完整业务规则") {
		t.Fatalf("requirements missing fallback confirmation question:\n%s", result.RequirementsMarkdown)
	}
}

func TestRenderIntakeSnapshotRecordsSourceAndRawText(t *testing.T) {
	result := ParseMarkdown(Source{
		Path: "prd.md",
		Text: "# Title\n\nBody",
	})

	snapshot := RenderSnapshot(result)
	for _, want := range []string{
		"# Intake: Title",
		"Source: `prd.md`",
		"## 原始需求材料",
		"Body",
	} {
		if !strings.Contains(snapshot, want) {
			t.Fatalf("snapshot missing %q:\n%s", want, snapshot)
		}
	}
}
