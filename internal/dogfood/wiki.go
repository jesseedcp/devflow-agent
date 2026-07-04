package dogfood

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/wiki"
)

func distillAndDecideWiki(root, demandID string, now func() time.Time) error {
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	demandDir := store.DemandDir(demandID)
	events, err := store.ReadEvents(demandID)
	if err != nil {
		return err
	}
	eventInputs := make([]wiki.EventInput, 0, len(events))
	for _, event := range events {
		eventInputs = append(eventInputs, wiki.EventInput{
			Type:    event.Type,
			Message: event.Message,
			Data:    event.Data,
		})
	}
	result := wiki.Distill(wiki.DistillInput{
		Title:                demand.Title,
		Closeout:             readDogfoodArtifact(demandDir, artifacts.CloseoutFile),
		MemoryCandidates:     readDogfoodArtifact(demandDir, artifacts.MemoryCandidatesFile),
		ImplementationReview: readDogfoodArtifact(demandDir, artifacts.ImplementationReviewFile),
		Events:               eventInputs,
	})
	if err := store.WriteArtifact(demandID, artifacts.CloseoutRawLogFile, result.CloseoutRawLog); err != nil {
		return err
	}
	if err := store.WriteArtifact(demandID, artifacts.WikiCandidatesFile, wiki.RenderCandidates(demand.Title, result.Candidates)); err != nil {
		return err
	}
	if err := store.AppendEvent(demandID, artifacts.Event{
		Time:    now().UTC(),
		Type:    "wiki.candidates_distilled",
		Message: "dogfood wiki candidates distilled from closeout material",
		Data:    map[string]string{"candidates": strconv.Itoa(len(result.Candidates))},
	}); err != nil {
		return err
	}
	promoted := 0
	for _, candidate := range result.Candidates {
		if candidate.Status != wiki.StatusPending {
			continue
		}
		if candidate.Kind == wiki.KindBusiness && promoted == 0 {
			relPath, err := wiki.Promote(root, wiki.PromoteOptions{
				DemandID:       demandID,
				CandidateIndex: candidate.Index,
				Name:           "dogfood-stable-knowledge",
				By:             "devflow dogfood",
				Now:             now,
			}, candidate)
			if err != nil {
				return err
			}
			wiki.MarkCandidate(result.Candidates, candidate.Index, wiki.StatusPromoted, relPath, "")
			if err := store.AppendEvent(demandID, artifacts.Event{
				Time:    now().UTC(),
				Type:    "wiki.candidate_promoted",
				Message: "dogfood wiki candidate promoted",
				Data:    map[string]string{"candidate": strconv.Itoa(candidate.Index), "name": "dogfood-stable-knowledge", "by": "devflow dogfood", "path": relPath},
			}); err != nil {
				return err
			}
			promoted++
		} else {
			wiki.MarkCandidate(result.Candidates, candidate.Index, wiki.StatusRejected, "", "dogfood archive-only material")
			if err := store.AppendEvent(demandID, artifacts.Event{
				Time:    now().UTC(),
				Type:    "wiki.candidate_rejected",
				Message: "dogfood wiki candidate rejected",
				Data:    map[string]string{"candidate": strconv.Itoa(candidate.Index), "by": "devflow dogfood", "reason": "dogfood archive-only material"},
			}); err != nil {
				return err
			}
		}
	}
	return store.WriteArtifact(demandID, artifacts.WikiCandidatesFile, wiki.RenderCandidates(demand.Title, result.Candidates))
}

func readDogfoodArtifact(demandDir, name string) string {
	data, err := os.ReadFile(filepath.Join(demandDir, name))
	if err != nil {
		return ""
	}
	return string(data)
}