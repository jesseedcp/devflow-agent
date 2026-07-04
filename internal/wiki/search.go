package wiki

import (
	"os"
	"path/filepath"
	"strings"
)

type SearchHit struct {
	Path    string
	Title   string
	Snippet string
}

func Search(root, query string) ([]SearchHit, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, nil
	}
	wikiRoot := WikiRoot(root)
	entries, err := os.ReadDir(wikiRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var hits []SearchHit
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".md") || name == "WIKI.md" {
			continue
		}
		path := filepath.Join(wikiRoot, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if !strings.Contains(strings.ToLower(text), query) {
			continue
		}
		title, snippet := extractTitleAndSnippet(text, query)
		hits = append(hits, SearchHit{
			Path:    ".devflow/wiki/" + name,
			Title:   title,
			Snippet: snippet,
		})
	}
	return hits, nil
}

func extractTitleAndSnippet(text, query string) (string, string) {
	title := ""
	var matchingLines []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if title == "" && strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
		if strings.Contains(strings.ToLower(line), query) {
			matchingLines = append(matchingLines, trimmed)
		}
	}
	snippet := ""
	for _, line := range matchingLines {
		if line != "" && !strings.HasPrefix(line, "#") {
			snippet = line
			break
		}
	}
	if snippet == "" && len(matchingLines) > 0 {
		snippet = matchingLines[0]
	}
	if snippet == "" {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				snippet = trimmed
				break
			}
		}
	}
	return title, snippet
}