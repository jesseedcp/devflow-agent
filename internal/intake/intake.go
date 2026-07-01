package intake

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Readiness string

const (
	ReadinessNeedsReview Readiness = "needs_review"
)

type Source struct {
	Path string
	URL  string
	Text string
}

type Result struct {
	Title                string
	SourcePath           string
	RawText              string
	Goals                []string
	NonGoals             []string
	Rules                []string
	AcceptanceCriteria   []string
	Risks                []string
	Questions            []string
	RequirementsMarkdown string
	Readiness            Readiness
}

type section struct {
	heading string
	lines   []string
}

var markdownHeading = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.+?)\s*$`)

func ParseMarkdown(src Source) Result {
	raw := normalizeNewlines(src.Text)
	title := extractTitle(raw)
	if title == "" {
		title = titleFromPath(src.Path)
	}
	sections := parseSections(raw)
	out := Result{
		Title:      title,
		SourcePath: sourcePath(src),
		RawText:    strings.TrimSpace(raw),
		Readiness:  ReadinessNeedsReview,
	}
	out.Goals = collectSections(sections, "目标", "需求", "背景", "goal", "objective", "behavior")
	out.NonGoals = collectSections(sections, "非目标", "不做", "out of scope", "non-goal")
	out.Rules = collectSections(sections, "业务规则", "规则", "business rule", "rule")
	out.AcceptanceCriteria = collectSections(sections, "验收", "acceptance", "criteria")
	out.Risks = collectSections(sections, "风险", "歧义", "risk", "ambiguity")
	out.Questions = collectSections(sections, "待确认", "问题", "question", "open")
	if len(out.Goals) == 0 {
		out.Goals = fallbackBody(raw)
	}
	if len(out.Questions) == 0 {
		out.Questions = []string{"请确认完整业务规则、边界条件、异常返回和验收口径是否准确。"}
	}
	out.RequirementsMarkdown = RenderRequirements(out)
	return out
}

func sourcePath(src Source) string {
	if strings.TrimSpace(src.Path) != "" {
		return strings.TrimSpace(src.Path)
	}
	return strings.TrimSpace(src.URL)
}

func RenderRequirements(result Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Requirements: %s\n\n", result.Title)
	writeBullets(&b, "## 目标行为", result.Goals)
	writeBullets(&b, "## 非目标范围", result.NonGoals)
	writeBullets(&b, "## 业务规则", result.Rules)
	writeBullets(&b, "## 用户/调用方影响", []string{"根据 intake 材料确认调用方、用户提示、错误码和兼容性影响。"})
	writeBullets(&b, "## 验收标准", result.AcceptanceCriteria)
	writeBullets(&b, "## 风险与歧义", result.Risks)
	writeBullets(&b, "## 待确认问题", result.Questions)
	b.WriteString("## 人工确认记录\n")
	return b.String()
}

func RenderSnapshot(result Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Intake: %s\n\n", result.Title)
	if result.SourcePath != "" {
		fmt.Fprintf(&b, "Source: `%s`\n\n", result.SourcePath)
	}
	fmt.Fprintf(&b, "Readiness: `%s`\n\n", result.Readiness)
	b.WriteString("## 原始需求材料\n\n")
	if strings.TrimSpace(result.RawText) == "" {
		b.WriteString("_empty intake text_\n")
	} else {
		b.WriteString(strings.TrimSpace(result.RawText))
		b.WriteString("\n")
	}
	return b.String()
}

func normalizeNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}

func extractTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		match := markdownHeading.FindStringSubmatch(line)
		if len(match) == 3 && match[1] == "#" {
			return strings.TrimSpace(strings.Trim(match[2], "#"))
		}
	}
	return ""
}

func titleFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.Join(strings.Fields(base), " ")
	if base == "" || base == "." {
		return "untitled demand"
	}
	return base
}

func parseSections(text string) []section {
	var sections []section
	current := section{heading: "body"}
	for _, line := range strings.Split(text, "\n") {
		if match := markdownHeading.FindStringSubmatch(line); len(match) == 3 {
			if len(current.lines) > 0 || current.heading != "body" {
				sections = append(sections, current)
			}
			current = section{heading: strings.ToLower(strings.TrimSpace(match[2]))}
			continue
		}
		current.lines = append(current.lines, line)
	}
	if len(current.lines) > 0 || current.heading != "body" {
		sections = append(sections, current)
	}
	return sections
}

func collectSections(sections []section, needles ...string) []string {
	var out []string
	for _, sec := range sections {
		heading := strings.ToLower(sec.heading)
		if !headingMatches(heading, needles) {
			continue
		}
		out = append(out, normalizeBullets(sec.lines)...)
	}
	return compactLines(out)
}

func headingMatches(heading string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(heading, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func normalizeBullets(lines []string) []string {
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "* ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func fallbackBody(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		lines = append(lines, trimmed)
	}
	return compactLines(lines)
}

func compactLines(lines []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

func writeBullets(b *strings.Builder, heading string, values []string) {
	b.WriteString(heading)
	b.WriteString("\n\n")
	if len(values) == 0 {
		b.WriteString("- 待人工补充。\n\n")
		return
	}
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", value)
	}
	b.WriteString("\n")
}
