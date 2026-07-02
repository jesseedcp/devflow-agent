package codemap

import (
	"sort"
	"strings"
)

func Search(index Index, query string, limit int) []SearchResult {
	terms := queryTerms(query)
	if limit <= 0 {
		limit = 20
	}
	var results []SearchResult
	for _, fact := range index.Facts {
		score := scoreFact(fact, terms)
		if score <= 0 {
			continue
		}
		results = append(results, SearchResult{Fact: fact, Score: score})
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Fact.File != results[j].Fact.File {
			return results[i].Fact.File < results[j].Fact.File
		}
		return results[i].Fact.Line < results[j].Fact.Line
	})
	if len(results) > limit {
		return results[:limit]
	}
	return results
}

func queryTerms(query string) []string {
	seen := map[string]bool{}
	var out []string
	for _, term := range strings.Fields(strings.ToLower(query)) {
		term = strings.Trim(term, "`\"'.,:;()[]{}")
		if len(term) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out
}

func scoreFact(fact CodeFact, terms []string) int {
	haystack := strings.ToLower(strings.Join([]string{
		fact.Kind,
		fact.Package,
		fact.File,
		fact.Name,
		fact.Receiver,
		fact.Signature,
		fact.Text,
		strings.Join(fact.Tags, " "),
	}, " "))
	score := 0
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			score++
		}
	}
	return score
}
