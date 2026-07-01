package intake

import (
	"net/http"
	"net/http/httptest"
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

func TestParseHTMLExtractsTitleAndRequirementSections(t *testing.T) {
	result := ParseHTML(Source{
		URL: "https://example.test/prd",
		Text: `<!doctype html>
<html>
<head><title>Coupon URL PRD</title><script>ignoreMe()</script></head>
<body>
	<h1>Coupon URL PRD</h1>
	<h2>目标</h2>
	<ul><li>Active members can claim URL coupons.</li></ul>
	<h2>业务规则</h2>
	<p>User status must be active.</p>
	<h2>验收标准</h2>
	<ol><li>Inactive users are blocked.</li></ol>
</body>
</html>`,
	})

	if result.Title != "Coupon URL PRD" {
		t.Fatalf("Title = %q, want Coupon URL PRD", result.Title)
	}
	if result.SourcePath != "https://example.test/prd" {
		t.Fatalf("SourcePath = %q, want URL", result.SourcePath)
	}
	for _, want := range []string{
		"Active members can claim URL coupons.",
		"User status must be active.",
		"Inactive users are blocked.",
	} {
		if !strings.Contains(result.RequirementsMarkdown, want) {
			t.Fatalf("requirements missing %q:\n%s", want, result.RequirementsMarkdown)
		}
	}
	if strings.Contains(result.RawText, "ignoreMe") {
		t.Fatalf("raw text should omit script content:\n%s", result.RawText)
	}
}

func TestFetchURLReadsHTMLAsParsedIntake(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Refund URL PRD</title></head><body><h1>Refund URL PRD</h1><h2>目标</h2><p>Refunds require an approved order.</p></body></html>`))
	}))
	defer server.Close()

	result, err := FetchURL(server.URL)
	if err != nil {
		t.Fatalf("FetchURL returned error: %v", err)
	}

	if result.Title != "Refund URL PRD" {
		t.Fatalf("Title = %q, want Refund URL PRD", result.Title)
	}
	if result.SourcePath != server.URL {
		t.Fatalf("SourcePath = %q, want %q", result.SourcePath, server.URL)
	}
	if !strings.Contains(result.RequirementsMarkdown, "Refunds require an approved order.") {
		t.Fatalf("requirements missing fetched body:\n%s", result.RequirementsMarkdown)
	}
}

func TestFetchURLRejectsNonHTTPURL(t *testing.T) {
	_, err := FetchURL("file:///tmp/prd.html")
	if err == nil || !strings.Contains(err.Error(), "http or https") {
		t.Fatalf("err = %v, want http/https rejection", err)
	}
}
