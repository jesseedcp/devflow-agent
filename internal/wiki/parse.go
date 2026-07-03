package wiki

import "strings"

func ParseCandidates(text string) []Candidate {
	var candidates []Candidate
	section := ""
	index := 0
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "## Stable Business Knowledge":
			section = "business"
			continue
		case "## Process Improvement Candidates":
			section = "process"
			continue
		case "## Archive Only":
			section = "archive"
			continue
		}
		if section == "" || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if body == "" {
			continue
		}
		index++
		c := Candidate{Index: index, Status: StatusPending}
		switch section {
		case "business":
			c.Kind = KindBusiness
		case "process":
			c.Kind = KindProcess
		case "archive":
			c.Kind = KindArchive
		}
		body, c.Status, c.WikiPath, c.Reason = extractStatus(body, c.Status)
		body, c.Source = extractSource(body)
		c.Text = strings.TrimSpace(body)
		candidates = append(candidates, c)
	}
	return candidates
}

func extractSource(body string) (string, string) {
	idx := strings.LastIndex(body, "(source: ")
	if idx == -1 {
		return body, ""
	}
	rest := body[idx+len("(source: "):]
	end := strings.Index(rest, ")")
	if end == -1 {
		return body, ""
	}
	return strings.TrimSpace(body[:idx]), strings.TrimSpace(rest[:end])
}

func extractStatus(body string, status CandidateStatus) (string, CandidateStatus, string, string) {
	wikiPath := ""
	reason := ""
	if idx := strings.LastIndex(body, "[promoted: "); idx != -1 {
		rest := body[idx+len("[promoted: "):]
		end := strings.Index(rest, "]")
		if end != -1 {
			wikiPath = strings.TrimSpace(rest[:end])
			body = strings.TrimSpace(body[:idx])
			status = StatusPromoted
		}
	} else if idx := strings.LastIndex(body, "[rejected: "); idx != -1 {
		rest := body[idx+len("[rejected: "):]
		end := strings.Index(rest, "]")
		if end != -1 {
			reason = strings.TrimSpace(rest[:end])
			body = strings.TrimSpace(body[:idx])
			status = StatusRejected
		}
	}
	return body, status, wikiPath, reason
}

func MarkCandidate(candidates []Candidate, index int, status CandidateStatus, wikiPath, reason string) bool {
	for i := range candidates {
		if candidates[i].Index == index {
			candidates[i].Status = status
			candidates[i].WikiPath = wikiPath
			candidates[i].Reason = reason
			return true
		}
	}
	return false
}