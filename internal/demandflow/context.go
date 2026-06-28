package demandflow

import (
	"os"
	"path/filepath"

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

	if hits, err := memorystore.NewStore(l.root).Search(demand.Title + " " + demand.Description); err == nil {
		for _, hit := range hits {
			if hit.DemandID == demand.ID {
				continue
			}
			snapshot.Memories = append(snapshot.Memories, MemoryHit{
				DemandID: hit.DemandID,
				Path:     hit.Path,
				Snippet:  hit.Snippet,
			})
		}
	}

	return snapshot, nil
}

func (l contextLoader) readArtifact(demandID, name string) string {
	path := filepath.Join(l.store.DemandDir(demandID), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
