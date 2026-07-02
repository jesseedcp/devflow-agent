package codemap

import (
	"fmt"
	"sort"
	"strings"
)

func RenderSummary(index Index) string {
	var b strings.Builder
	b.WriteString("# Codemap Summary\n\n")
	fmt.Fprintf(&b, "Facts: %d\n\n", len(index.Facts))
	byFile := map[string][]CodeFact{}
	for _, fact := range index.Facts {
		byFile[fact.File] = append(byFile[fact.File], fact)
	}
	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		fmt.Fprintf(&b, "## %s\n\n", file)
		facts := byFile[file]
		sort.SliceStable(facts, func(i, j int) bool { return facts[i].Line < facts[j].Line })
		for _, fact := range facts {
			fmt.Fprintf(&b, "- %s `%s` line %d\n", fact.Kind, fact.Name, fact.Line)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func RenderDemandCodemap(demandID, query string, results []SearchResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Codemap Context: %s\n\n", demandID)
	fmt.Fprintf(&b, "Query: `%s`\n\n", strings.TrimSpace(query))
	b.WriteString("## Likely Impacted Code\n\n")
	if len(results) == 0 {
		b.WriteString("- No matching code facts found. Re-run `devflow codemap index` or broaden the query.\n")
		return b.String()
	}
	for _, result := range results {
		fact := result.Fact
		fmt.Fprintf(&b, "- `%s:%d` %s `%s` score=%d\n", fact.File, fact.Line, fact.Kind, fact.Name, result.Score)
	}
	return b.String()
}
