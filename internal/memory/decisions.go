package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

const memoryDirName = "memory"

var stableNamePattern = regexp.MustCompile(`[^a-z0-9]+`)

type PromoteOptions struct {
	DemandID       string
	CandidateIndex int
	Name           string
	Description    string
	By             string
	Now            func() time.Time
}

type PromoteResult struct {
	Candidate Candidate
	Path      string
	IndexPath string
}

type RejectOptions struct {
	DemandID       string
	CandidateIndex int
	By             string
	Reason         string
	Now            func() time.Time
}

func (s Store) ListCandidates(demandID string) ([]Candidate, error) {
	candidates, err := s.loadCandidates(demandID)
	if err != nil {
		return nil, err
	}
	decisions, err := s.loadDecisions(demandID)
	if err != nil {
		return nil, err
	}
	for index := range candidates {
		if decision, ok := decisions[candidates[index].Index]; ok {
			candidates[index].Status = decision.Status
			candidates[index].StablePath = decision.StablePath
			candidates[index].Reason = decision.Reason
		}
	}
	return candidates, nil
}

func (s Store) PromoteCandidate(opts PromoteOptions) (PromoteResult, error) {
	opts.By = strings.TrimSpace(opts.By)
	if opts.By == "" {
		return PromoteResult{}, fmt.Errorf("--by is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	candidate, err := s.candidateByIndex(opts.DemandID, opts.CandidateIndex)
	if err != nil {
		return PromoteResult{}, err
	}
	name := normalizeStableName(opts.Name)
	if name == "" {
		name = normalizeStableName(candidate.Text)
	}
	if name == "" {
		return PromoteResult{}, fmt.Errorf("stable memory name is required")
	}
	description := strings.Join(strings.Fields(opts.Description), " ")
	if description == "" {
		description = candidate.Text
	}

	memDir, err := s.ensureStableMemoryDir()
	if err != nil {
		return PromoteResult{}, err
	}
	fileName := name + ".md"
	path := filepath.Join(memDir, fileName)
	if _, err := os.Lstat(path); err == nil {
		fileName = name + "-" + opts.DemandID + ".md"
		path = filepath.Join(memDir, fileName)
	} else if err != nil && !os.IsNotExist(err) {
		return PromoteResult{}, fmt.Errorf("inspect stable memory path: %w", err)
	}

	promotedAt := now()
	body := stableMemoryBody(name, description, candidate.Text, opts.DemandID, opts.By, promotedAt)
	if err := writeTextAtomic(path, body); err != nil {
		return PromoteResult{}, fmt.Errorf("write stable memory: %w", err)
	}

	indexPath := filepath.Join(memDir, "MEMORY.md")
	if err := appendMemoryIndex(indexPath, name, fileName, description); err != nil {
		return PromoteResult{}, err
	}

	eventPath, err := s.eventsPath(opts.DemandID)
	if err != nil {
		return PromoteResult{}, err
	}
	if err := appendMemoryEvent(eventPath, artifacts.Event{
		Time:    promotedAt.UTC(),
		Type:    "memory.promoted",
		Message: "memory candidate promoted",
		Data: map[string]string{
			"candidate_index": strconv.Itoa(candidate.Index),
			"candidate":       candidate.Text,
			"by":              opts.By,
			"stable_path":     path,
		},
	}); err != nil {
		return PromoteResult{}, err
	}

	candidate.Status = CandidatePromoted
	candidate.StablePath = path
	return PromoteResult{Candidate: candidate, Path: path, IndexPath: indexPath}, nil
}

func (s Store) RejectCandidate(opts RejectOptions) (Candidate, error) {
	opts.By = strings.TrimSpace(opts.By)
	opts.Reason = strings.Join(strings.Fields(opts.Reason), " ")
	if opts.By == "" {
		return Candidate{}, fmt.Errorf("--by is required")
	}
	if opts.Reason == "" {
		return Candidate{}, fmt.Errorf("--reason is required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}

	candidate, err := s.candidateByIndex(opts.DemandID, opts.CandidateIndex)
	if err != nil {
		return Candidate{}, err
	}
	eventPath, err := s.eventsPath(opts.DemandID)
	if err != nil {
		return Candidate{}, err
	}
	rejectedAt := now()
	if err := appendMemoryEvent(eventPath, artifacts.Event{
		Time:    rejectedAt.UTC(),
		Type:    "memory.rejected",
		Message: "memory candidate rejected",
		Data: map[string]string{
			"candidate_index": strconv.Itoa(candidate.Index),
			"candidate":       candidate.Text,
			"by":              opts.By,
			"reason":          opts.Reason,
		},
	}); err != nil {
		return Candidate{}, err
	}

	candidate.Status = CandidateRejected
	candidate.Reason = opts.Reason
	return candidate, nil
}

type memoryDecision struct {
	Status     CandidateStatus
	StablePath string
	Reason     string
}

func (s Store) loadCandidates(demandID string) ([]Candidate, error) {
	if _, err := artifacts.NewStore(s.root).LoadDemand(demandID); err != nil {
		return nil, err
	}
	path := filepath.Join(s.root, ".devflow", "demands", demandID, memoryCandidatesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("memory candidates not found")
		}
		return nil, fmt.Errorf("read memory candidates: %w", err)
	}
	candidates := ParseCandidates(string(data))
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no memory candidates found")
	}
	return candidates, nil
}

func (s Store) candidateByIndex(demandID string, candidateIndex int) (Candidate, error) {
	if candidateIndex < 1 {
		return Candidate{}, fmt.Errorf("candidate index out of range")
	}
	candidates, err := s.loadCandidates(demandID)
	if err != nil {
		return Candidate{}, err
	}
	for _, candidate := range candidates {
		if candidate.Index == candidateIndex {
			return candidate, nil
		}
	}
	return Candidate{}, fmt.Errorf("candidate index out of range")
}

func (s Store) loadDecisions(demandID string) (map[int]memoryDecision, error) {
	eventPath, err := s.eventsPath(demandID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	decisions := map[int]memoryDecision{}
	for lineNo, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event artifacts.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("decode events line %d: %w", lineNo+1, err)
		}
		index, err := strconv.Atoi(event.Data["candidate_index"])
		if err != nil || index < 1 {
			continue
		}
		switch event.Type {
		case "memory.promoted":
			decisions[index] = memoryDecision{Status: CandidatePromoted, StablePath: event.Data["stable_path"]}
		case "memory.rejected":
			decisions[index] = memoryDecision{Status: CandidateRejected, Reason: event.Data["reason"]}
		}
	}
	return decisions, nil
}

func (s Store) eventsPath(demandID string) (string, error) {
	if _, err := artifacts.NewStore(s.root).LoadDemand(demandID); err != nil {
		return "", err
	}
	return filepath.Join(s.root, ".devflow", "demands", demandID, artifacts.EventsFile), nil
}

func (s Store) ensureStableMemoryDir() (string, error) {
	if s.root == "" {
		return "", fmt.Errorf("store root is required")
	}
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return "", fmt.Errorf("resolve store root: %w", err)
	}
	memDir := filepath.Join(rootAbs, ".devflow", memoryDirName)
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		return "", fmt.Errorf("create stable memory directory: %w", err)
	}
	return memDir, nil
}

func normalizeStableName(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = stableNamePattern.ReplaceAllString(normalized, "-")
	return strings.Trim(normalized, "-")
}

func stableMemoryBody(name, description, candidate, demandID, by string, promotedAt time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + name + "\n")
	b.WriteString("description: " + description + "\n")
	b.WriteString("type: project\n")
	b.WriteString("source_demand: " + demandID + "\n")
	b.WriteString("promoted_at: " + promotedAt.Format(time.RFC3339) + "\n")
	b.WriteString("promoted_by: " + by + "\n")
	b.WriteString("---\n\n")
	b.WriteString("# " + name + "\n\n")
	b.WriteString(candidate + "\n\n")
	b.WriteString("Why: This rule was confirmed during demand " + demandID + ".\n\n")
	b.WriteString("How to apply: Reuse this rule when generating requirements or plans for similar backend demand work.\n")
	return b.String()
}

func appendMemoryIndex(indexPath, name, fileName, description string) error {
	entry := "- [" + name + "](" + fileName + ") - " + description
	data, err := os.ReadFile(indexPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read MEMORY.md: %w", err)
	}
	text := string(data)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}
	if strings.TrimSpace(text) == "" {
		text = entry + "\n"
	} else if strings.HasSuffix(text, "\n") {
		text += entry + "\n"
	} else {
		text += "\n" + entry + "\n"
	}
	return writeTextAtomic(indexPath, text)
}

func appendMemoryEvent(path string, event artifacts.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open events log: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()
		return fmt.Errorf("append event: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync events log: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close events log: %w", err)
	}
	return nil
}

func writeTextAtomic(path string, contents string) (err error) {
	tempFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp text file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()
	if _, err = tempFile.WriteString(contents); err != nil {
		tempFile.Close()
		return fmt.Errorf("write text file: %w", err)
	}
	if err = tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("sync text file: %w", err)
	}
	if err = tempFile.Close(); err != nil {
		return fmt.Errorf("close text file: %w", err)
	}
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename text file: %w", err)
	}
	return nil
}
