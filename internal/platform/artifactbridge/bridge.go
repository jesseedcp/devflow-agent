package artifactbridge

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/platform/api"
)

// ScanDemands lists demand workspaces under root by delegating to the existing
// demandflow inspector. It does not parse markdown beyond presence checks.
func ScanDemands(root string) ([]api.DemandSummary, error) {
	summaries, err := demandflow.ListWorkspaces(root)
	if err != nil {
		return nil, fmt.Errorf("artifactbridge: scan demands: %w", err)
	}
	out := make([]api.DemandSummary, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, toDemandSummary(s))
	}
	return out, nil
}

// GetDemand returns a single demand detail by key.
func GetDemand(root, demandKey string) (api.DemandDetail, error) {
	summary, err := demandflow.InspectWorkspace(root, demandKey)
	if err != nil {
		return api.DemandDetail{}, fmt.Errorf("artifactbridge: get demand: %w", err)
	}
	return toDemandDetail(summary), nil
}

// ReadArtifact returns the raw text of a named artifact for a demand. The name
// must be a known artifact file; unknown names are rejected to prevent path
// traversal.
func ReadArtifact(root, demandKey, name string) (string, error) {
	if !allowedArtifactName(name) {
		return "", fmt.Errorf("artifactbridge: unknown artifact %q", name)
	}
	path := filepath.Join(artifacts.NewStore(root).DemandDir(demandKey), name)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("artifactbridge: read artifact: %w", err)
	}
	return string(data), nil
}

func toDemandSummary(s demandflow.WorkspaceSummary) api.DemandSummary {
	return api.DemandSummary{
		DemandKey: s.Demand.ID,
		Title:     s.Demand.Title,
		State:     s.Demand.State,
		Attention: s.Attention,
		UpdatedAt: s.Demand.UpdatedAt,
		Artifacts: toAPIArtifacts(s.Artifacts),
	}
}

func toDemandDetail(s demandflow.WorkspaceSummary) api.DemandDetail {
	return api.DemandDetail{
		DemandKey: s.Demand.ID,
		Title:     s.Demand.Title,
		State:     s.Demand.State,
		Attention: s.Attention,
		UpdatedAt: s.Demand.UpdatedAt,
		Artifacts: toAPIArtifacts(s.Artifacts),
		Evidence: api.EvidenceSummary{
			Pass:    s.Evidence.Pass,
			Fail:    s.Evidence.Fail,
			Blocked: s.Evidence.Blocked,
		},
		Release: api.ReleaseSummary{
			DeploymentStatus:  s.Release.DeploymentStatus,
			ObservationStatus: s.Release.ObservationStatus,
			RollbackDecision:  s.Release.RollbackDecision,
			RunURL:            s.Release.RunURL,
		},
	}
}

func toAPIArtifacts(in []demandflow.ArtifactSummary) []api.ArtifactSummary {
	out := make([]api.ArtifactSummary, 0, len(in))
	for _, a := range in {
		out = append(out, api.ArtifactSummary{Name: a.Name, Exists: a.Exists})
	}
	return out
}

var allowedArtifacts = map[string]struct{}{
	artifacts.DemandFile:               {},
	artifacts.IntakeFile:               {},
	artifacts.ContextFile:              {},
	artifacts.CodemapFile:              {},
	artifacts.PlanContextFile:          {},
	artifacts.ChangeScopeFile:          {},
	artifacts.ImplementationReviewFile: {},
	artifacts.RequirementsFile:         {},
	artifacts.PlanFile:                 {},
	artifacts.ProgressFile:             {},
	artifacts.VerificationFile:         {},
	artifacts.DeploymentFile:           {},
	artifacts.ObservationFile:          {},
	artifacts.RollbackFile:             {},
	artifacts.CloseoutFile:             {},
	artifacts.MemoryCandidatesFile:     {},
	artifacts.CloseoutRawLogFile:       {},
	artifacts.WikiCandidatesFile:       {},
	artifacts.MetricsFile:              {},
	artifacts.EventsFile:               {},
}

func allowedArtifactName(name string) bool {
	_, ok := allowedArtifacts[name]
	return ok
}