package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	memorystore "github.com/jesseedcp/devflow-agent/internal/memory"
)

func runMemory(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("memory subcommand is required")
	}
	switch args[0] {
	case "list":
		return runMemoryList(args[1:], stdout, stderr)
	case "promote":
		return runMemoryPromote(args[1:], stdout, stderr)
	case "reject":
		return runMemoryReject(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown memory subcommand %q", args[0])
	}
}

func runMemoryList(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("memory list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	candidates, err := memorystore.NewStore(root).ListCandidates(demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Memory candidates for %s\n", demandID)
	for _, candidate := range candidates {
		fmt.Fprintf(stdout, "%d. [%s] %s\n", candidate.Index, candidate.Status, candidate.Text)
		if candidate.StablePath != "" {
			fmt.Fprintf(stdout, "   stable: %s\n", candidate.StablePath)
		}
		if candidate.Reason != "" {
			fmt.Fprintf(stdout, "   reason: %s\n", candidate.Reason)
		}
	}
	return nil
}

func runMemoryPromote(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("memory promote", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, candidateRaw, name, description, by string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&candidateRaw, "candidate", "", "candidate number")
	fs.StringVar(&name, "name", "", "stable memory name")
	fs.StringVar(&description, "description", "", "stable memory description")
	fs.StringVar(&by, "by", "", "promoting person")
	if err := fs.Parse(args); err != nil {
		return err
	}
	candidateIndex, err := strconv.Atoi(strings.TrimSpace(candidateRaw))
	if err != nil || candidateIndex < 1 {
		return fmt.Errorf("--candidate must be a positive integer")
	}
	demandID = strings.TrimSpace(demandID)
	result, err := memorystore.NewStore(root).PromoteCandidate(memorystore.PromoteOptions{
		DemandID:       demandID,
		CandidateIndex: candidateIndex,
		Name:           name,
		Description:    description,
		By:             by,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "promoted candidate %d for %s\n%s\n", result.Candidate.Index, demandID, result.Path)
	return nil
}

func runMemoryReject(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("memory reject", flag.ContinueOnError)
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
	candidateIndex, err := strconv.Atoi(strings.TrimSpace(candidateRaw))
	if err != nil || candidateIndex < 1 {
		return fmt.Errorf("--candidate must be a positive integer")
	}
	demandID = strings.TrimSpace(demandID)
	candidate, err := memorystore.NewStore(root).RejectCandidate(memorystore.RejectOptions{
		DemandID:       demandID,
		CandidateIndex: candidateIndex,
		By:             by,
		Reason:         reason,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "rejected candidate %d for %s\n", candidate.Index, demandID)
	return nil
}
