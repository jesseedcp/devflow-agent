package wiki

import (
	"fmt"
	"strings"
)

type EventInput struct {
	Type    string
	Message string
	Data    map[string]string
}

type DistillInput struct {
	Title                string
	Closeout             string
	MemoryCandidates     string
	ImplementationReview string
	Events               []EventInput
}

type DistillResult struct {
	CloseoutRawLog string
	Candidates     []Candidate
}

func Distill(in DistillInput) DistillResult {
	var candidates []Candidate
	index := 0

	for _, text := range extractBusinessCandidates(in.MemoryCandidates) {
		index++
		candidates = append(candidates, Candidate{
			Index:  index,
			Kind:   KindBusiness,
			Text:   text,
			Source: "memory-candidates.md",
			Status: StatusPending,
		})
	}

	rec := parseRecommendation(in.ImplementationReview)
	if rec != "" && rec != "ready_for_closeout" {
		index++
		candidates = append(candidates, Candidate{
			Index:  index,
			Kind:   KindProcess,
			Text:   "Implementation review recommended " + rec + ".",
			Source: "implementation-review.md",
			Status: StatusPending,
		})
	}

	for _, event := range in.Events {
		if matchesProcessEvent(event) {
			index++
			candidates = append(candidates, Candidate{
				Index:  index,
				Kind:   KindProcess,
				Text:   eventCandidateText(event),
				Source: "events.jsonl",
				Status: StatusPending,
			})
		}
	}

	if hasCloseoutMaterial(in, rec) {
		index++
		candidates = append(candidates, Candidate{
			Index:  index,
			Kind:   KindArchive,
			Text:   "Closeout raw material archived in closeout-raw-log.md.",
			Source: "closeout.md",
			Status: StatusPending,
		})
	}

	return DistillResult{
		CloseoutRawLog: buildCloseoutRawLog(in),
		Candidates:     candidates,
	}
}

func extractBusinessCandidates(text string) []string {
	var texts []string
	section := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "## Stable Knowledge Candidates", "## 稳定知识候选", "## Stable Business Knowledge":
			section = true
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			section = false
			continue
		}
		if !section || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if body == "" || strings.HasPrefix(body, "No ") {
			continue
		}
		texts = append(texts, body)
	}
	return texts
}

func parseRecommendation(text string) string {
	marker := "Recommendation: `"
	idx := strings.Index(text, marker)
	if idx == -1 {
		return ""
	}
	rest := text[idx+len(marker):]
	end := strings.Index(rest, "`")
	if end == -1 {
		return ""
	}
	return rest[:end]
}

func matchesProcessEvent(event EventInput) bool {
	haystack := event.Type + " " + event.Message
	for _, value := range event.Data {
		haystack += " " + value
	}
	for _, marker := range []string{"action_required", "returned_to_requirements", "returned_to_plan", "returned_to_implementation"} {
		if strings.Contains(haystack, marker) {
			return true
		}
	}
	return false
}

func eventCandidateText(event EventInput) string {
	if strings.TrimSpace(event.Message) != "" {
		return event.Message
	}
	return "Review event: " + event.Type
}

func hasCloseoutMaterial(in DistillInput, rec string) bool {
	return hasBullets(in.Closeout) ||
		hasBullets(in.MemoryCandidates) ||
		rec != "" ||
		hasDeliveryEvents(in.Events)
}

func hasBullets(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			return true
		}
	}
	return false
}

func hasDeliveryEvents(events []EventInput) bool {
	for _, event := range events {
		if event.Type != "demand.created" {
			return true
		}
	}
	return false
}

func buildCloseoutRawLog(in DistillInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Closeout Raw Log: %s\n\n", in.Title)
	writeRawSection(&b, "## Closeout", "No closeout material captured yet.", in.Closeout)
	writeRawSection(&b, "## Memory Candidates", "No memory candidate material captured yet.", in.MemoryCandidates)
	writeRawSection(&b, "## Implementation Review", "No implementation review captured yet.", in.ImplementationReview)
	b.WriteString("## Review And Verification Events\n\n")
	if len(in.Events) == 0 {
		b.WriteString("No events captured yet.\n\n")
	} else {
		for _, event := range in.Events {
			fmt.Fprintf(&b, "- %s: %s\n", event.Type, event.Message)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func writeRawSection(b *strings.Builder, heading, placeholder, content string) {
	fmt.Fprintf(b, "%s\n\n", heading)
	if strings.TrimSpace(content) == "" {
		fmt.Fprintf(b, "%s\n\n", placeholder)
		return
	}
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n")
}