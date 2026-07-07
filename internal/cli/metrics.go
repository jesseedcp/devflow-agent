package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/metrics"
)

func runMetrics(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("metrics requires a subcommand: report")
	}
	switch args[0] {
	case "report":
		return runMetricsReport(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown metrics command %q", args[0])
	}
}

func runMetricsReport(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("metrics report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "workspace root")
	demandID := fs.String("demand", "", "optional demand id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	store := artifacts.NewStore(*root)
	var report metrics.ProjectMetrics
	if *demandID != "" {
		demand, err := store.LoadDemand(*demandID)
		if err != nil {
			return err
		}
		events, err := store.ReadEvents(*demandID)
		if err != nil {
			return err
		}
		demandMetrics := metrics.CollectDemand(demand, events)
		report.Demands = []metrics.DemandMetrics{demandMetrics}
		metrics.ApplyForCLI(&report, demandMetrics)
		metrics.ApplyRuntimeEvents(&report, events)
	} else {
		collected, err := metrics.CollectProject(*root)
		if err != nil {
			return err
		}
		report = collected
	}
	body := metrics.RenderProject(report)
	fmt.Fprint(stdout, body)
	for _, demand := range report.Demands {
		if err := store.WriteArtifact(demand.DemandID, artifacts.MetricsFile, body); err != nil {
			return err
		}
	}
	return nil
}
