package demandflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/memory"
	"github.com/jesseedcp/devflow-agent/internal/templates"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

type WorkspaceSummary struct {
	Demand       artifacts.Demand
	State        workflow.State
	DemandDir    string
	Stages       []StageSummary
	Artifacts    []ArtifactSummary
	Verification VerificationSummary
	MergeRequest MergeRequestSummary
	Memory       MemorySummary
	Actions      []NextAction
	Attention    string
}

type StageSummary struct {
	Name   string
	Status string
}

type ArtifactSummary struct {
	Name   string
	Path   string
	Exists bool
	Size   int64
	Status string
	Error  string
}

type VerificationSummary struct {
	Status       string
	Command      string
	EvidenceFile string
	Message      string
}

type MergeRequestSummary struct {
	Status    string
	Reference string
	URL       string
	Message   string
}

type MemorySummary struct {
	Status   string
	Pending  int
	Promoted int
	Rejected int
	Error    string
}

func InspectWorkspace(root, demandID string) (WorkspaceSummary, error) {
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return WorkspaceSummary{}, err
	}

	events, eventsErr := store.ReadEvents(demandID)
	demandDir := store.DemandDir(demandID)
	summary := WorkspaceSummary{
		Demand:    demand,
		State:     workflow.State(demand.State),
		DemandDir: demandDir,
	}
	progressText := readArtifactText(filepath.Join(demandDir, artifacts.ProgressFile)).text
	summary.Verification = summarizeVerification(events)
	summary.MergeRequest = summarizeMergeRequest(events, progressText)
	summary.Memory = summarizeMemory(root, demandID)
	summary.Stages = summarizeStages(summary.State, events, summary.Verification, summary.MergeRequest)
	summary.Artifacts = summarizeArtifacts(demandDir, demand, eventsErr, summary)
	summary.Attention = workspaceAttention(summary, eventsErr)
	summary.Actions = WorkspaceNextActions(summary)
	return summary, nil
}

func ListWorkspaces(root string) ([]WorkspaceSummary, error) {
	base := filepath.Join(root, ".devflow", "demands")
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	out := make([]WorkspaceSummary, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := strings.TrimSpace(entry.Name())
		if id == "" || strings.ContainsAny(id, `/\`) {
			continue
		}
		summary, err := InspectWorkspace(root, id)
		if err != nil {
			continue
		}
		out = append(out, summary)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := workspacePriority(out[i])
		right := workspacePriority(out[j])
		if left != right {
			return left < right
		}
		return out[i].Demand.ID < out[j].Demand.ID
	})
	return out, nil
}

func WorkspaceNextActions(summary WorkspaceSummary) []NextAction {
	idArg := shellQuote(summary.Demand.ID)
	if summary.Memory.Pending > 0 {
		return []NextAction{
			{Label: "Review memory candidates", Command: "devflow memory list --demand " + idArg, Reason: "Stable knowledge candidates are still pending."},
			{Label: "Promote memory candidate", Command: "devflow memory promote --demand " + idArg + " --candidate <index> --by <name>", Reason: "Promote reusable knowledge that should persist."},
			{Label: "Reject memory candidate", Command: "devflow memory reject --demand " + idArg + " --candidate <index> --by <name> --reason <reason>", Reason: "Reject candidates that should remain one-time material."},
		}
	}
	if summary.State == workflow.Verification {
		switch summary.Verification.Status {
		case "pass":
			return []NextAction{{Label: "Confirm verification", Command: "devflow confirm --demand " + idArg + " --stage verification --by <name> --summary <summary>", Reason: "PASS evidence is present and needs human confirmation."}}
		case "fail":
			return []NextAction{{Label: "Retry implementation", Command: "devflow run --demand " + idArg + " --stage implementation --permission-mode acceptEdits --quality-command \"go test ./...\"", Reason: "Verification evidence failed; fix implementation before confirmation."}}
		}
	}
	if summary.State == workflow.MRReview && summary.MergeRequest.Status == "cleared" {
		return []NextAction{{Label: "Draft verification", Command: "devflow run --demand " + idArg + " --stage verification --quality-command \"go test ./...\"", Reason: "MR review is clear and verification evidence should be generated."}}
	}
	return NextActions(summary.State, summary.Demand.ID)
}

func summarizeStages(state workflow.State, events []artifacts.Event, verification VerificationSummary, mr MergeRequestSummary) []StageSummary {
	confirmed := confirmedStages(events)
	statuses := map[string]string{
		"requirements":   "pending",
		"plan":           "pending",
		"implementation": "pending",
		"mr-review":      "pending",
		"verification":   "pending",
		"closeout":       "pending",
	}

	for stage := range confirmed {
		if _, ok := statuses[stage]; ok {
			statuses[stage] = "confirmed"
		}
	}
	for _, event := range events {
		switch event.Type {
		case "implementation.completed":
			statuses["implementation"] = "completed"
		}
	}
	if mr.Status != "not_started" {
		statuses["mr-review"] = mr.Status
	}
	switch verification.Status {
	case "pass":
		statuses["verification"] = "passed"
	case "fail":
		statuses["verification"] = "failed"
	}

	switch state {
	case workflow.RequirementsDrafting:
		statuses["requirements"] = "drafting"
	case workflow.RequirementsReview:
		if !confirmed["requirements"] {
			statuses["requirements"] = "needs_confirmation"
		}
	case workflow.PlanDrafting:
		statuses["plan"] = "drafting"
	case workflow.PlanReview:
		if !confirmed["plan"] {
			statuses["plan"] = "needs_confirmation"
		}
	case workflow.Implementation, workflow.FailedQualityGate, workflow.ReturnedToRequirements, workflow.ReturnedToPlan:
		if statuses["implementation"] == "pending" {
			statuses["implementation"] = "drafting"
		}
	case workflow.MRReview:
		if statuses["mr-review"] == "pending" {
			statuses["mr-review"] = "needs_review"
		}
	case workflow.Verification:
		if verification.Status == "none" {
			statuses["verification"] = "needs_evidence"
		}
	case workflow.Closeout:
		if !confirmed["closeout"] {
			statuses["closeout"] = "needs_confirmation"
		}
	case workflow.BlockedNeedUser, workflow.BlockedNeedPlatform:
		statuses["implementation"] = "blocked"
	}

	names := []string{"requirements", "plan", "implementation", "mr-review", "verification", "closeout"}
	out := make([]StageSummary, 0, len(names))
	for _, name := range names {
		out = append(out, StageSummary{Name: name, Status: statuses[name]})
	}
	return out
}

func summarizeArtifacts(demandDir string, demand artifacts.Demand, eventsErr error, summary WorkspaceSummary) []ArtifactSummary {
	names := []string{
		artifacts.RequirementsFile,
		artifacts.PlanFile,
		artifacts.ProgressFile,
		artifacts.VerificationFile,
		artifacts.CloseoutFile,
		artifacts.MemoryCandidatesFile,
		artifacts.EventsFile,
	}
	out := make([]ArtifactSummary, 0, len(names))
	for _, name := range names {
		path := filepath.Join(demandDir, name)
		artifact := ArtifactSummary{Name: name, Path: path, Status: "missing"}
		stat, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				out = append(out, artifact)
				continue
			}
			artifact.Status = "read_error"
			artifact.Error = err.Error()
			out = append(out, artifact)
			continue
		}
		artifact.Exists = true
		artifact.Size = stat.Size()
		textResult := readArtifactText(path)
		if textResult.err != nil {
			artifact.Status = "read_error"
			artifact.Error = textResult.err.Error()
			out = append(out, artifact)
			continue
		}
		artifact.Status = artifactBaseStatus(name, textResult.text, demand)
		switch name {
		case artifacts.RequirementsFile:
			if stageStatus(summary, "requirements") == "confirmed" {
				artifact.Status = "confirmed"
			}
		case artifacts.PlanFile:
			if stageStatus(summary, "plan") == "confirmed" {
				artifact.Status = "confirmed"
			}
		case artifacts.VerificationFile:
			switch summary.Verification.Status {
			case "pass":
				artifact.Status = "has_pass_evidence"
			case "fail":
				artifact.Status = "has_fail_evidence"
			}
		case artifacts.CloseoutFile:
			if stageStatus(summary, "closeout") == "confirmed" {
				artifact.Status = "confirmed"
			}
		case artifacts.EventsFile:
			if eventsErr != nil {
				artifact.Status = "read_error"
				artifact.Error = eventsErr.Error()
			}
		}
		out = append(out, artifact)
	}
	return out
}

func artifactBaseStatus(name, text string, demand artifacts.Demand) string {
	if strings.TrimSpace(text) == "" {
		return "template"
	}
	if strings.TrimSpace(text) == strings.TrimSpace(templateForArtifact(name, demand)) {
		return "template"
	}
	return "present"
}

func templateForArtifact(name string, demand artifacts.Demand) string {
	switch name {
	case artifacts.RequirementsFile:
		return templates.Requirements(demand.Title, demand.Description)
	case artifacts.PlanFile:
		return templates.Plan(demand.Title)
	case artifacts.ProgressFile:
		return "# Progress\n\n"
	case artifacts.VerificationFile:
		return templates.Verification(demand.Title)
	case artifacts.CloseoutFile:
		return templates.Closeout(demand.Title)
	case artifacts.MemoryCandidatesFile:
		return templates.MemoryCandidates(demand.Title)
	default:
		return ""
	}
}

func confirmedStages(events []artifacts.Event) map[string]bool {
	confirmed := map[string]bool{}
	for _, event := range events {
		if event.Type != "stage.confirmed" {
			continue
		}
		stage := normalizeStageName(firstNonEmpty(event.Data["stage"], event.Message))
		if stage != "" {
			confirmed[stage] = true
		}
	}
	return confirmed
}

func summarizeVerification(events []artifacts.Event) VerificationSummary {
	summary := VerificationSummary{Status: "none"}
	for _, event := range events {
		if event.Type != "verification.recorded" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(event.Data["status"]))
		switch status {
		case "pass", "passed", "success", "ok":
			summary.Status = "pass"
		case "fail", "failed", "failure", "error":
			summary.Status = "fail"
		default:
			continue
		}
		summary.Command = firstNonEmpty(event.Data["command"], event.Data["quality_command"])
		summary.EvidenceFile = firstNonEmpty(event.Data["evidence_file"], artifacts.VerificationFile)
		summary.Message = event.Message
	}
	return summary
}

func summarizeMergeRequest(events []artifacts.Event, progressText string) MergeRequestSummary {
	summary := MergeRequestSummary{Status: "not_started"}
	for _, event := range events {
		switch event.Type {
		case "merge_request.synced":
			summary.Status = "needs_review"
			summary.Reference = firstNonEmpty(referenceFromIID(event.Data["mr_iid"]), event.Data["mr"], extractMRReference(event.Message))
			summary.URL = event.Data["mr_url"]
			summary.Message = event.Message
		case "mr_review.cleared":
			summary.Status = "cleared"
			summary.Reference = firstNonEmpty(event.Data["mr"], summary.Reference, extractMRReference(event.Message), extractMRReference(progressText))
			summary.Message = event.Message
		case "mr_review.action_required":
			summary.Status = "action_required"
			summary.Reference = firstNonEmpty(event.Data["mr"], summary.Reference, extractMRReference(event.Message), extractMRReference(progressText))
			summary.Message = event.Message
		case "merge_request.sync_failed":
			summary.Status = "action_required"
			summary.Reference = firstNonEmpty(summary.Reference, extractMRReference(progressText))
			summary.Message = event.Message
		}
	}
	if summary.Reference == "" {
		summary.Reference = extractMRReference(progressText)
	}
	if summary.Status == "not_started" && summary.Reference != "" {
		summary.Status = "needs_review"
	}
	return summary
}

func summarizeMemory(root, demandID string) MemorySummary {
	summary := MemorySummary{Status: "none"}
	candidates, err := memory.NewStore(root).ListCandidates(demandID)
	if err != nil {
		if isNoMemoryCandidatesError(err) {
			return summary
		}
		summary.Error = err.Error()
		return summary
	}
	for _, candidate := range candidates {
		switch candidate.Status {
		case memory.CandidatePromoted:
			summary.Promoted++
		case memory.CandidateRejected:
			summary.Rejected++
		default:
			summary.Pending++
		}
	}
	if summary.Pending > 0 {
		summary.Status = "pending"
	} else if summary.Promoted > 0 || summary.Rejected > 0 {
		summary.Status = "settled"
	}
	return summary
}

func isNoMemoryCandidatesError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no memory candidates found") || strings.Contains(message, "memory candidates not found")
}

func workspaceAttention(summary WorkspaceSummary, eventsErr error) string {
	if eventsErr != nil {
		return "events error"
	}
	if summary.State == workflow.FailedQualityGate {
		return "quality gate failed"
	}
	if summary.State == workflow.ReturnedToRequirements {
		return "returned to requirements"
	}
	if summary.State == workflow.ReturnedToPlan {
		return "returned to plan"
	}
	if summary.State == workflow.MRReview {
		if summary.MergeRequest.Status == "cleared" {
			return "ready for verification"
		}
		return "needs MR review gate"
	}
	if summary.State == workflow.Verification {
		switch summary.Verification.Status {
		case "pass":
			return "ready to confirm verification"
		case "fail":
			return "verification failed"
		default:
			return "needs verification evidence"
		}
	}
	if summary.Memory.Pending > 0 {
		return "memory candidates pending"
	}
	if summary.State == workflow.Closeout {
		return "ready for closeout"
	}
	if summary.State == workflow.Completed {
		return "complete"
	}
	actions := WorkspaceNextActions(summary)
	if len(actions) > 0 {
		return actions[0].Reason
	}
	return "inspect manually"
}

func workspacePriority(summary WorkspaceSummary) int {
	switch summary.State {
	case workflow.BlockedNeedUser, workflow.BlockedNeedPlatform, workflow.FailedQualityGate, workflow.ReturnedToRequirements, workflow.ReturnedToPlan:
		return 0
	case workflow.MRReview, workflow.Verification, workflow.Closeout:
		return 1
	case workflow.RequirementsReview, workflow.PlanReview, workflow.Implementation, workflow.RequirementsDrafting, workflow.PlanDrafting, workflow.Created, workflow.ContextLoaded:
		return 2
	case workflow.Completed:
		if summary.Memory.Pending > 0 {
			return 1
		}
		return 3
	default:
		return 4
	}
}

func stageStatus(summary WorkspaceSummary, name string) string {
	for _, stage := range summary.Stages {
		if stage.Name == name {
			return stage.Status
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeStageName(value string) string {
	stage := strings.ToLower(strings.TrimSpace(value))
	stage = strings.ReplaceAll(stage, "_", "-")
	switch stage {
	case "requirements", "requirement":
		return "requirements"
	case "plan", "planning":
		return "plan"
	case "verification", "verify":
		return "verification"
	case "closeout", "close-out":
		return "closeout"
	default:
		return stage
	}
}

func referenceFromIID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "!") {
		return value
	}
	if _, err := strconv.Atoi(value); err == nil {
		return "!" + value
	}
	return ""
}

func extractMRReference(text string) string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune("()[]{}<>.,;:\"'", r)
	})
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if len(field) < 2 || field[0] != '!' {
			continue
		}
		digits := field[1:]
		allDigits := true
		for _, r := range digits {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return "!" + digits
		}
	}
	return ""
}

type artifactTextResult struct {
	text string
	err  error
}

func readArtifactText(path string) artifactTextResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifactTextResult{err: fmt.Errorf("read artifact: %w", err)}
	}
	return artifactTextResult{text: string(data)}
}
