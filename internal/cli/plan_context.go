package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
)

func runPlanContext(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("plan-context requires a subcommand: refresh")
	}
	switch args[0] {
	case "refresh":
		return runPlanContextRefresh(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown plan-context command %q", args[0])
	}
}

func runPlanContextRefresh(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("plan-context refresh", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	planContext, err := renderPlanContext(root, demand)
	if err != nil {
		return err
	}
	if err := store.WriteArtifact(demand.ID, artifacts.PlanContextFile, planContext); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "plan context refreshed for %s\n", demand.ID)
	return err
}

func renderPlanContext(root string, demand artifacts.Demand) (string, error) {
	demandDir := artifacts.NewStore(root).DemandDir(demand.ID)
	requirements, err := readPlanContextArtifact(demandDir, artifacts.RequirementsFile)
	if err != nil {
		return "", err
	}
	context, err := readPlanContextArtifact(demandDir, artifacts.ContextFile)
	if err != nil {
		return "", err
	}
	codemap, err := readPlanContextArtifact(demandDir, artifacts.CodemapFile)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Plan Context: %s\n\n", demand.Title)
	fmt.Fprintf(&builder, "Demand: %s\n\n", demand.ID)
	builder.WriteString("## Requirements\n\n")
	builder.WriteString(strings.TrimSpace(requirements))
	builder.WriteString("\n\n## Memory Context\n\n")
	builder.WriteString(strings.TrimSpace(context))
	builder.WriteString("\n\n## Codemap Context\n\n")
	builder.WriteString(strings.TrimSpace(codemap))
	builder.WriteString("\n")
	return builder.String(), nil
}

func readPlanContextArtifact(demandDir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(demandDir, name))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	return string(data), nil
}
