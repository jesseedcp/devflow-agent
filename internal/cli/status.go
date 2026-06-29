package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runStatus(args []string, stdout io.Writer) error {
	opts, err := parseDemandLookupArgs("status", args)
	if err != nil {
		return err
	}
	report, err := demandflow.InspectStatus(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Demand: %s\n", report.Demand.ID)
	fmt.Fprintf(stdout, "Title: %s\n", report.Demand.Title)
	fmt.Fprintf(stdout, "State: %s\n", report.State)
	fmt.Fprintf(stdout, "Directory: %s\n\n", report.DemandDir)
	fmt.Fprintln(stdout, "Artifacts:")
	for _, artifact := range report.Artifacts {
		status := "missing"
		if artifact.Exists {
			status = fmt.Sprintf("%d bytes", artifact.Size)
		}
		fmt.Fprintf(stdout, "  - %s: %s\n", artifact.Name, status)
	}
	fmt.Fprintln(stdout, "\nNext actions:")
	for _, action := range report.Actions {
		fmt.Fprintf(stdout, "  - %s: %s\n", action.Label, action.Reason)
		if strings.TrimSpace(action.Command) != "" {
			fmt.Fprintf(stdout, "    %s\n", action.Command)
		}
	}
	return nil
}

func runNext(args []string, stdout io.Writer) error {
	opts, err := parseDemandLookupArgs("next", args)
	if err != nil {
		return err
	}
	report, err := demandflow.InspectStatus(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	if len(report.Actions) == 0 || strings.TrimSpace(report.Actions[0].Command) == "" {
		fmt.Fprintf(stdout, "No next command for %s in state %s\n", report.Demand.ID, report.State)
		return nil
	}
	fmt.Fprintln(stdout, report.Actions[0].Command)
	return nil
}

type demandLookupArgs struct {
	root     string
	demandID string
}

func parseDemandLookupArgs(name string, args []string) (demandLookupArgs, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts demandLookupArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	if err := fs.Parse(args); err != nil {
		return demandLookupArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.demandID == "" {
		return demandLookupArgs{}, fmt.Errorf("--demand is required")
	}
	return opts, nil
}
