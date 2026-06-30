package demandflow

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	memorystore "github.com/jesseedcp/devflow-agent/internal/memory"
)

type contextLoader struct {
	store artifacts.Store
	root  string
}

func newContextLoader(root string) contextLoader {
	return contextLoader{
		store: artifacts.NewStore(root),
		root:  root,
	}
}

func (l contextLoader) Load(demandID string) (ContextSnapshot, error) {
	demand, err := l.store.LoadDemand(demandID)
	if err != nil {
		return ContextSnapshot{}, err
	}

	snapshot := ContextSnapshot{Demand: demand}
	snapshot.Artifacts.Requirements = l.readArtifact(demandID, artifacts.RequirementsFile)
	snapshot.Artifacts.Plan = l.readArtifact(demandID, artifacts.PlanFile)
	snapshot.Artifacts.Progress = l.readArtifact(demandID, artifacts.ProgressFile)
	snapshot.Artifacts.Verification = l.readArtifact(demandID, artifacts.VerificationFile)
	snapshot.Artifacts.Closeout = l.readArtifact(demandID, artifacts.CloseoutFile)
	snapshot.Artifacts.MemoryCandidates = l.readArtifact(demandID, artifacts.MemoryCandidatesFile)

	memoryStore := memorystore.NewStore(l.root)
	query := demand.Title + " " + demand.Description

	for _, hit := range searchMemoryQueries(query, memoryStore.SearchStable) {
		snapshot.Memories = append(snapshot.Memories, MemoryHit{
			Path:    hit.Path,
			Snippet: hit.Snippet,
			Source:  string(hit.Source),
		})
	}

	for _, hit := range searchCandidateMemoryQueries(query, demand.ID, memoryStore.Search) {
		snapshot.Memories = append(snapshot.Memories, MemoryHit{
			DemandID: hit.DemandID,
			Path:     hit.Path,
			Snippet:  hit.Snippet,
			Source:   string(hit.Source),
		})
	}
	return snapshot, nil
}

func searchMemoryQueries(query string, search func(string) ([]memorystore.Result, error)) []memorystore.Result {
	for _, candidateQuery := range memoryQueries(query) {
		hits, err := search(candidateQuery)
		if err != nil || len(hits) == 0 {
			continue
		}
		return hits
	}
	return nil
}

func searchCandidateMemoryQueries(query string, currentDemandID string, search func(string) ([]memorystore.Result, error)) []memorystore.Result {
	for _, candidateQuery := range memoryQueries(query) {
		hits, err := search(candidateQuery)
		if err != nil || len(hits) == 0 {
			continue
		}
		filtered := make([]memorystore.Result, 0, len(hits))
		for _, hit := range hits {
			if hit.DemandID == currentDemandID {
				continue
			}
			filtered = append(filtered, hit)
		}
		if len(filtered) > 0 {
			return filtered
		}
	}
	return nil
}

func memoryQueries(query string) []string {
	queries := []string{query}
	seenQuery := map[string]struct{}{query: {}}
	for _, term := range strings.Fields(query) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if _, ok := seenQuery[term]; ok {
			continue
		}
		seenQuery[term] = struct{}{}
		queries = append(queries, term)
	}
	return queries
}

func (l contextLoader) readArtifact(demandID, name string) string {
	path := filepath.Join(l.store.DemandDir(demandID), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
