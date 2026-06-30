package memory

import "strings"

type Source string

const (
	SourceCandidate Source = "candidate"
	SourceStable    Source = "stable"
)

type CandidateStatus string

const (
	CandidatePending  CandidateStatus = "pending"
	CandidatePromoted CandidateStatus = "promoted"
	CandidateRejected CandidateStatus = "rejected"
)

type Candidate struct {
	Index      int
	Text       string
	Status     CandidateStatus
	StablePath string
	Reason     string
}

func ParseCandidates(content string) []Candidate {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	sectionLines, found := stableCandidateSection(lines)
	if !found {
		sectionLines = lines
	}

	out := make([]Candidate, 0)
	for _, line := range sectionLines {
		if !isTopLevelBullet(line) {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if text == "" {
			continue
		}
		out = append(out, Candidate{
			Index:  len(out) + 1,
			Text:   text,
			Status: CandidatePending,
		})
	}
	return out
}

func stableCandidateSection(lines []string) ([]string, bool) {
	start := -1
	for index, line := range lines {
		if isStableCandidateHeading(line) {
			start = index + 1
			break
		}
	}
	if start < 0 {
		return nil, false
	}

	end := len(lines)
	for index := start; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "## ") {
			end = index
			break
		}
	}
	return lines[start:end], true
}

func isStableCandidateHeading(line string) bool {
	heading := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(heading, "## ") && (strings.Contains(heading, "稳定知识候选") ||
		strings.Contains(heading, "stable knowledge candidates") ||
		strings.Contains(heading, "memory candidates"))
}

func isTopLevelBullet(line string) bool {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(line), "- ")
}
