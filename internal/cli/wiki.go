package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/wiki"
)

func runWiki(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("wiki subcommand is required: distill, list, promote, reject, or search")
	}
	switch args[0] {
	case "distill":
		return runWikiDistill(args[1:], stdout, stderr)
	case "list":
		return runWikiList(args[1:], stdout, stderr)
	case "promote":
		return runWikiPromote(args[1:], stdout, stderr)
	case "reject":
		return runWikiReject(args[1:], stdout, stderr)
	case "search":
		return runWikiSearch(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown wiki subcommand %q", args[0])
	}
}

func runWikiDistill(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDemandLookupArgs("wiki distill", args)
	if err != nil {
		return err
	}
	root := normalizedRoot(opts.root)
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(opts.demandID)
	if err != nil {
		return err
	}
	demandDir := store.DemandDir(opts.demandID)
	events, err := store.ReadEvents(opts.demandID)
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
		Closeout:             readDemandArtifact(demandDir, artifacts.CloseoutFile),
		MemoryCandidates:     readDemandArtifact(demandDir, artifacts.MemoryCandidatesFile),
		ImplementationReview: readDemandArtifact(demandDir, artifacts.ImplementationReviewFile),
		Events:               eventInputs,
	})
	if err := store.WriteArtifact(opts.demandID, artifacts.CloseoutRawLogFile, result.CloseoutRawLog); err != nil {
		return err
	}
	candidatesContent := wiki.RenderCandidates(demand.Title, result.Candidates)
	if err := store.WriteArtifact(opts.demandID, artifacts.WikiCandidatesFile, candidatesContent); err != nil {
		return err
	}
	if err := store.AppendEvent(opts.demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    "wiki.candidates_distilled",
		Message: "wiki candidates distilled from closeout material",
		Data: map[string]string{
			"candidates": strconv.Itoa(len(result.Candidates)),
		},
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wiki candidates distilled for %s: %d candidates\n", opts.demandID, len(result.Candidates))
	return nil
}

func runWikiList(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDemandLookupArgs("wiki list", args)
	if err != nil {
		return err
	}
	root := normalizedRoot(opts.root)
	store := artifacts.NewStore(root)
	demandDir := store.DemandDir(opts.demandID)
	text := readDemandArtifact(demandDir, artifacts.WikiCandidatesFile)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("wiki-candidates.md not found; run `devflow wiki distill --demand %s` first", opts.demandID)
	}
	candidates := wiki.ParseCandidates(text)
	fmt.Fprintf(stdout, "Wiki candidates for %s\n", opts.demandID)
	for _, candidate := range candidates {
		fmt.Fprintf(stdout, "%d. [%s] %s - %s\n", candidate.Index, candidate.Status, candidate.Kind, candidate.Text)
		if candidate.WikiPath != "" {
			fmt.Fprintf(stdout, "   promoted: %s\n", candidate.WikiPath)
		}
		if candidate.Reason != "" {
			fmt.Fprintf(stdout, "   reason: %s\n", candidate.Reason)
		}
	}
	return nil
}

func runWikiPromote(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("wiki promote", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, candidateRaw, name, by string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&candidateRaw, "candidate", "", "candidate number")
	fs.StringVar(&name, "name", "", "wiki entry slug name")
	fs.StringVar(&by, "by", "", "promoting person")
	if err := fs.Parse(args); err != nil {
		return err
	}
	demandID = strings.TrimSpace(demandID)
	candidateIndex, err := strconv.Atoi(strings.TrimSpace(candidateRaw))
	if err != nil || candidateIndex < 1 {
		return fmt.Errorf("--candidate must be a positive integer")
	}
	name = strings.TrimSpace(name)
	by = strings.TrimSpace(by)
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if by == "" {
		return fmt.Errorf("--by is required")
	}
	root = normalizedRoot(root)
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	demandDir := store.DemandDir(demandID)
	text := readDemandArtifact(demandDir, artifacts.WikiCandidatesFile)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("wiki-candidates.md not found; run `devflow wiki distill --demand %s` first", demandID)
	}
	candidates := wiki.ParseCandidates(text)
	var candidate wiki.Candidate
	found := false
	for _, c := range candidates {
		if c.Index == candidateIndex {
			candidate = c
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("candidate %d not found", candidateIndex)
	}
	relPath, err := wiki.Promote(root, wiki.PromoteOptions{
		DemandID:       demandID,
		CandidateIndex: candidateIndex,
		Name:           name,
		By:             by,
	}, candidate)
	if err != nil {
		return err
	}
	wiki.MarkCandidate(candidates, candidateIndex, wiki.StatusPromoted, relPath, "")
	rendered := wiki.RenderCandidates(demand.Title, candidates)
	if err := store.WriteArtifact(demandID, artifacts.WikiCandidatesFile, rendered); err != nil {
		return err
	}
	if err := store.AppendEvent(demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    "wiki.candidate_promoted",
		Message: "wiki candidate promoted",
		Data: map[string]string{
			"candidate": strconv.Itoa(candidateIndex),
			"name":      name,
			"by":        by,
			"path":      relPath,
		},
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "promoted candidate %d for %s\n%s\n", candidateIndex, demandID, relPath)
	return nil
}

func runWikiReject(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("wiki reject", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, candidateRaw, by, reason string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&candidateRaw, "candidate", "", "candidate number")
	fs.StringVar(&by, "by", "", "rejecting person")
	fs.StringVar(&reason, "reason", "", "rejection reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	demandID = strings.TrimSpace(demandID)
	candidateIndex, err := strconv.Atoi(strings.TrimSpace(candidateRaw))
	if err != nil || candidateIndex < 1 {
		return fmt.Errorf("--candidate must be a positive integer")
	}
	by = strings.TrimSpace(by)
	reason = strings.TrimSpace(reason)
	if by == "" {
		return fmt.Errorf("--by is required")
	}
	if reason == "" {
		return fmt.Errorf("--reason is required")
	}
	root = normalizedRoot(root)
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	demandDir := store.DemandDir(demandID)
	text := readDemandArtifact(demandDir, artifacts.WikiCandidatesFile)
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("wiki-candidates.md not found; run `devflow wiki distill --demand %s` first", demandID)
	}
	candidates := wiki.ParseCandidates(text)
	if !wiki.MarkCandidate(candidates, candidateIndex, wiki.StatusRejected, "", reason) {
		return fmt.Errorf("candidate %d not found", candidateIndex)
	}
	rendered := wiki.RenderCandidates(demand.Title, candidates)
	if err := store.WriteArtifact(demandID, artifacts.WikiCandidatesFile, rendered); err != nil {
		return err
	}
	if err := store.AppendEvent(demandID, artifacts.Event{
		Time:    time.Now().UTC(),
		Type:    "wiki.candidate_rejected",
		Message: "wiki candidate rejected",
		Data: map[string]string{
			"candidate": strconv.Itoa(candidateIndex),
			"by":        by,
			"reason":    reason,
		},
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "rejected candidate %d for %s\n", candidateIndex, demandID)
	return nil
}

func runWikiSearch(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("wiki search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root string
	fs.StringVar(&root, "root", ".", "root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	if fs.NArg() == 0 {
		return fmt.Errorf("search query is required")
	}
	query := strings.Join(fs.Args(), " ")
	hits, err := wiki.Search(root, query)
	if err != nil {
		return err
	}
	if len(hits) == 0 {
		fmt.Fprintln(stdout, "No wiki entries matched")
		return nil
	}
	for _, hit := range hits {
		fmt.Fprintf(stdout, "%s\n  title: %s\n  snippet: %s\n", hit.Path, hit.Title, hit.Snippet)
	}
	return nil
}

func readDemandArtifact(demandDir, name string) string {
	data, err := os.ReadFile(filepath.Join(demandDir, name))
	if err != nil {
		return ""
	}
	return string(data)
}