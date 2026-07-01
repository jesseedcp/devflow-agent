package demandflow

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

type MemoryRecall struct {
	DemandID   string
	Title      string
	Stable     []MemoryHit
	Candidates []MemoryHit
}

func BuildMemoryRecall(root, demandID string) (MemoryRecall, error) {
	snapshot, err := newContextLoader(root).Load(demandID)
	if err != nil {
		return MemoryRecall{}, err
	}
	recall := MemoryRecall{
		DemandID: demandID,
		Title:    snapshot.Demand.Title,
	}
	for _, hit := range snapshot.Memories {
		switch hit.Source {
		case "stable":
			recall.Stable = append(recall.Stable, hit)
		case "candidate":
			recall.Candidates = append(recall.Candidates, hit)
		}
	}
	return recall, nil
}

func RenderMemoryRecall(recall MemoryRecall) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Context: %s\n\n", recall.Title)
	fmt.Fprintf(&b, "Demand: `%s`\n\n", recall.DemandID)
	b.WriteString("## Approved Stable Memory\n\n")
	if len(recall.Stable) == 0 {
		b.WriteString("No approved stable memory recalled.\n\n")
	} else {
		for _, hit := range recall.Stable {
			fmt.Fprintf(&b, "- `%s`", hit.Path)
			if strings.TrimSpace(hit.Snippet) != "" {
				fmt.Fprintf(&b, ": %s", strings.TrimSpace(hit.Snippet))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Historical Demand Candidates\n\n")
	if len(recall.Candidates) == 0 {
		b.WriteString("No historical candidate memory recalled.\n\n")
	} else {
		for _, hit := range recall.Candidates {
			fmt.Fprintf(&b, "- `%s`", hit.DemandID)
			if strings.TrimSpace(hit.Snippet) != "" {
				fmt.Fprintf(&b, ": %s", strings.TrimSpace(hit.Snippet))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	b.WriteString("## Operator Notes\n\n")
	b.WriteString("- Review this context before confirming requirements.\n")
	b.WriteString("- Candidate memory is unapproved and must not be treated as stable truth.\n")
	return b.String()
}

type RecallWriteResult struct {
	DemandID       string
	ContextPath    string
	StableCount    int
	CandidateCount int
}

func WriteMemoryRecall(root, demandID string) (RecallWriteResult, error) {
	recall, err := BuildMemoryRecall(root, demandID)
	if err != nil {
		return RecallWriteResult{}, err
	}
	store := artifacts.NewStore(root)
	if err := store.WriteArtifact(demandID, artifacts.ContextFile, RenderMemoryRecall(recall)); err != nil {
		return RecallWriteResult{}, err
	}
	if err := store.AppendEvent(demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    "context.recalled",
		Message: "reusable memory context recalled",
		Data: map[string]string{
			"stable":     fmt.Sprintf("%d", len(recall.Stable)),
			"candidates": fmt.Sprintf("%d", len(recall.Candidates)),
		},
	}); err != nil {
		return RecallWriteResult{}, err
	}
	return RecallWriteResult{
		DemandID:       demandID,
		ContextPath:    filepath.Join(store.DemandDir(demandID), artifacts.ContextFile),
		StableCount:    len(recall.Stable),
		CandidateCount: len(recall.Candidates),
	}, nil
}
