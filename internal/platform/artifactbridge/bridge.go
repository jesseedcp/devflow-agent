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
	stageSummary := make(map[string]string, len(s.Stages))
	blockers, warnings := 0, 0
	for _, stage := range s.Stages {
		stageSummary[stage.Name] = stage.Status
		switch stage.Status {
		case "failed", "fail", "blocked", "needs_decision":
			blockers++
		case "pending", "drafting", "needs_confirmation", "needs_review", "needs_evidence", "needs_release", "needs_observation":
			warnings++
		}
	}
	return api.DemandDetail{
		DemandKey:   s.Demand.ID,
		Title:       s.Demand.Title,
		State:       s.Demand.State,
		Attention:   s.Attention,
		UpdatedAt:   s.Demand.UpdatedAt,
		Description: s.Demand.Description,
		Source:      s.Demand.Source,
		Artifacts:   toAPIArtifacts(s.Artifacts),
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
		Quality: api.QualitySummary{
			StageSummary: stageSummary,
			Blockers:     blockers,
			Warnings:     warnings,
		},
		NextActions: toAPINextActions(s.Actions),
	}
}

func toAPINextActions(actions []demandflow.NextAction) []api.NextAction {
	out := make([]api.NextAction, 0, len(actions))
	for _, a := range actions {
		out = append(out, api.NextAction{Label: a.Label, Command: a.Command, Reason: a.Reason})
	}
	return out
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
