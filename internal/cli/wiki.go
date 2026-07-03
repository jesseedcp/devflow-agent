package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/wiki"
)

func runWiki(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("wiki subcommand is required: distill")
	}
	switch args[0] {
	case "distill":
		return runWikiDistill(args[1:], stdout, stderr)
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

func readDemandArtifact(demandDir, name string) string {
	data, err := os.ReadFile(filepath.Join(demandDir, name))
	if err != nil {
		return ""
	}
	return string(data)
}