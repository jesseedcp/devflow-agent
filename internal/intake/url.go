package intake

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxURLIntakeBytes int64 = 2 << 20

var (
	htmlScriptStyleBlock = regexp.MustCompile(`(?is)<(script|style|noscript)\b[^>]*>.*?</\s*(script|style|noscript)\s*>`)
	htmlTitle            = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title>`)
	htmlHeadingOpen      = regexp.MustCompile(`(?is)<h([1-6])\b[^>]*>`)
	htmlHeadingClose     = regexp.MustCompile(`(?is)</h[1-6]\s*>`)
	htmlListItemOpen     = regexp.MustCompile(`(?is)<li\b[^>]*>`)
	htmlBlockBreak       = regexp.MustCompile(`(?is)</?(p|div|section|article|main|header|footer|ul|ol|table|tr|blockquote)\b[^>]*>`)
	htmlLineBreak        = regexp.MustCompile(`(?is)<br\s*/?>`)
	htmlAnyTag           = regexp.MustCompile(`(?is)<[^>]+>`)
	blankLines           = regexp.MustCompile(`\n{3,}`)
)

func FetchURL(rawURL string) (Result, error) {
	trimmed := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" {
		return Result{}, fmt.Errorf("invalid intake URL: %s", rawURL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Result{}, fmt.Errorf("intake URL must use http or https")
	}
	if parsed.Host == "" {
		return Result{}, fmt.Errorf("invalid intake URL: %s", rawURL)
	}

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(trimmed)
	if err != nil {
		return Result{}, fmt.Errorf("fetch intake URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("fetch intake URL: unexpected status %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxURLIntakeBytes+1))
	if err != nil {
		return Result{}, fmt.Errorf("read intake URL: %w", err)
	}
	if int64(len(body)) > maxURLIntakeBytes {
		return Result{}, fmt.Errorf("read intake URL: response exceeds %d bytes", maxURLIntakeBytes)
	}
	return ParseHTML(Source{URL: trimmed, Text: string(body)}), nil
}

func ParseHTML(src Source) Result {
	return ParseMarkdown(Source{
		Path: src.Path,
		URL:  src.URL,
		Text: HTMLToText(src.Text),
	})
}

func HTMLToText(markup string) string {
	text := htmlScriptStyleBlock.ReplaceAllString(markup, "")
	title := extractHTMLTitle(text)
	text = htmlHeadingOpen.ReplaceAllStringFunc(text, func(tag string) string {
		match := htmlHeadingOpen.FindStringSubmatch(tag)
		if len(match) == 2 {
			return "\n" + strings.Repeat("#", headingLevel(match[1])) + " "
		}
		return "\n# "
	})
	text = htmlHeadingClose.ReplaceAllString(text, "\n")
	text = htmlListItemOpen.ReplaceAllString(text, "\n- ")
	text = strings.ReplaceAll(text, "</li>", "\n")
	text = htmlLineBreak.ReplaceAllString(text, "\n")
	text = htmlBlockBreak.ReplaceAllString(text, "\n")
	text = htmlAnyTag.ReplaceAllString(text, "")
	text = html.UnescapeString(text)
	text = normalizeExtractedLines(text)
	if extractTitle(text) == "" && title != "" {
		text = "# " + title + "\n\n" + text
	}
	return strings.TrimSpace(text)
}

func extractHTMLTitle(markup string) string {
	match := htmlTitle.FindStringSubmatch(markup)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(htmlAnyTag.ReplaceAllString(match[1], "")))
}

func headingLevel(level string) int {
	switch level {
	case "1", "2", "3", "4", "5", "6":
		return int(level[0] - '0')
	default:
		return 1
	}
}

func normalizeExtractedLines(text string) string {
	var lines []string
	for _, line := range strings.Split(normalizeNewlines(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, strings.Join(strings.Fields(trimmed), " "))
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}
