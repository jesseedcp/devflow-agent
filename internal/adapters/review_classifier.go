package adapters

import "strings"

// ClassifyReviewComment maps a review comment to the earliest workflow surface
// that can resolve it. It is deterministic so release gates do not depend on an LLM.
func ClassifyReviewComment(body string, filePath string) CommentCategory {
	path := strings.ToLower(strings.TrimSpace(filePath))
	text := strings.ToLower(strings.TrimSpace(body))

	switch {
	case containsAny(path, "requirements.md", "requirement", "prd"):
		return CommentRequirements
	case containsAny(path, "plan.md", "design", "architecture"):
		return CommentPlan
	case containsAny(path, "_test.go", ".test.", ".spec.", "/test/", "\\test\\"):
		return CommentTest
	}

	switch {
	case containsAny(text, "requirement", "requirements", "acceptance criteria", "business rule", "scope", "需求", "验收", "业务规则"):
		return CommentRequirements
	case containsAny(text, "plan", "design", "architecture", "adapter boundary", "方案", "架构", "设计"):
		return CommentPlan
	case containsAny(text, "test", "tests", "coverage", "regression", "quality gate", "测试", "覆盖"):
		return CommentTest
	case containsAny(text, "nit", "style", "rename", "format", "readability", "typo", "命名", "格式"):
		return CommentStyle
	default:
		return CommentImplementation
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
