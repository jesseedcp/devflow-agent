package metrics

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func CollectProject(root string) (ProjectMetrics, error) {
	store := artifacts.NewStore(root)
	base := filepath.Join(root, ".devflow", "demands")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return ProjectMetrics{}, nil
		}
		return ProjectMetrics{}, err
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)

	out := ProjectMetrics{}
	for _, id := range ids {
		demand, err := store.LoadDemand(id)
		if err != nil {
			return ProjectMetrics{}, err
		}
		events, err := store.ReadEvents(id)
		if err != nil {
			return ProjectMetrics{}, err
		}
		metrics := CollectDemand(demand, events)
		out.Demands = append(out.Demands, metrics)
		applyDemandMetrics(&out, metrics)
		for _, event := range events {
			applyRuntimeEvent(&out, event)
		}
	}
	return out, nil
}

func applyDemandMetrics(out *ProjectMetrics, demand DemandMetrics) {
	out.DemandCount++
	if strings.EqualFold(demand.State, "completed") {
		out.CompletedCount++
	}
	if demand.BlockedEvents > 0 {
		out.BlockedCount++
	}
	if demand.PartialData {
		out.PartialDemandCount++
	}
	out.TotalHumanConfirmations += demand.HumanConfirmations
	out.TotalReviewReturns += demand.ReviewReturns
	out.TotalVerificationRuns += demand.VerificationRuns
	out.TotalVerificationPasses += demand.VerificationPasses
	out.TotalVerificationFailures += demand.VerificationFailures
	out.TotalAcceptancePasses += demand.AcceptancePasses
	out.TotalAcceptanceFailures += demand.AcceptanceFailures
	out.TotalAcceptanceBlocked += demand.AcceptanceBlocked
	out.TotalWikiCandidates += demand.WikiCandidatesDistilled
	out.TotalWikiPromoted += demand.WikiPromoted
	out.TotalWikiRejected += demand.WikiRejected
}

func ApplyForCLI(out *ProjectMetrics, demand DemandMetrics) {
	applyDemandMetrics(out, demand)
}

func ApplyRuntimeEvents(out *ProjectMetrics, events []artifacts.Event) {
	for _, event := range events {
		applyRuntimeEvent(out, event)
	}
}
