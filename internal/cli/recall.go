package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runRecall(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("recall", flag.ContinueOnError)
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
	result, err := demandflow.WriteMemoryRecall(root, demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "context recalled for %s\n", result.DemandID)
	fmt.Fprintf(stdout, "context: %s\n", result.ContextPath)
	fmt.Fprintf(stdout, "memory: %d stable, %d candidate\n", result.StableCount, result.CandidateCount)
	return nil
}
