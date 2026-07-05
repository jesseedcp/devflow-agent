package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runStatus(args []string, stdout io.Writer) error {
	opts, err := parseStatusArgs(args)
	if err != nil {
		return err
	}
	if opts.all {
		summaries, err := demandflow.ListWorkspaces(opts.root)
		if err != nil {
			return err
		}
		printWorkspaceList(stdout, summaries)
		return nil
	}
	summary, err := demandflow.InspectWorkspace(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	printWorkspaceDetail(stdout, summary)
	return nil
}

func runNext(args []string, stdout io.Writer) error {
	opts, err := parseDemandLookupArgs("next", args)
	if err != nil {
		return err
	}
	summary, err := demandflow.InspectWorkspace(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	if len(summary.Actions) == 0 || strings.TrimSpace(summary.Actions[0].Command) == "" {
		fmt.Fprintf(stdout, "No next command for %s in state %s\n", summary.Demand.ID, summary.State)
		return nil
	}
	fmt.Fprintln(stdout, summary.Actions[0].Command)
	return nil
}

type statusArgs struct {
	root     string
	demandID string
	all      bool
}

func parseStatusArgs(args []string) (statusArgs, error) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts statusArgs
	fs.StringVar(&opts.root, "root", ".", "root directory")
	fs.StringVar(&opts.demandID, "demand", "", "demand id")
	fs.BoolVar(&opts.all, "all", false, "list all demands")
	if err := fs.Parse(args); err != nil {
		return statusArgs{}, err
	}
	opts.root = strings.TrimSpace(opts.root)
	if opts.root == "" {
		opts.root = "."
	}
	opts.demandID = strings.TrimSpace(opts.demandID)
	if opts.all {
		if opts.demandID != "" {
			return statusArgs{}, fmt.Errorf("--all cannot be combined with --demand")
		}
		return opts, nil
	}
	if opts.demandID == "" {
		return statusArgs{}, fmt.Errorf("--demand is required")
	}
	return opts, nil
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

func printWorkspaceDetail(stdout io.Writer, summary demandflow.WorkspaceSummary) {
	fmt.Fprintf(stdout, "Demand: %s\n", summary.Demand.ID)
	fmt.Fprintf(stdout, "Title: %s\n", summary.Demand.Title)
	fmt.Fprintf(stdout, "State: %s\n", summary.State)
	fmt.Fprintf(stdout, "Attention: %s\n", summary.Attention)
	fmt.Fprintf(stdout, "Directory: %s\n\n", summary.DemandDir)

	fmt.Fprintln(stdout, "Stage summary:")
	for _, stage := range summary.Stages {
		fmt.Fprintf(stdout, "  %-14s %s\n", stage.Name, humanStatus(stage.Status))
	}

	fmt.Fprintln(stdout, "\nArtifacts:")
	for _, artifact := range summary.Artifacts {
		detail := humanStatus(artifact.Status)
		if artifact.Error != "" {
			detail += ", " + artifact.Error
		}
		fmt.Fprintf(stdout, "  %-22s %s\n", artifact.Name, detail)
	}

	fmt.Fprintln(stdout, "\nMR:")
	mrLine := humanStatus(summary.MergeRequest.Status)
	if summary.MergeRequest.Reference != "" {
		mrLine = summary.MergeRequest.Reference + " " + mrLine
	}
	fmt.Fprintf(stdout, "  %s\n", mrLine)
	if summary.MergeRequest.URL != "" {
		fmt.Fprintf(stdout, "  url: %s\n", summary.MergeRequest.URL)
	}
	if summary.MergeRequest.Message != "" {
		fmt.Fprintf(stdout, "  evidence: %s\n", summary.MergeRequest.Message)
	}

	fmt.Fprintln(stdout, "\nVerification:")
	switch summary.Verification.Status {
	case "pass":
		fmt.Fprintf(stdout, "  latest: PASS %s\n", summary.Verification.Command)
	case "fail":
		fmt.Fprintf(stdout, "  latest: FAIL %s\n", summary.Verification.Command)
		if summary.Verification.FailureKind != "" {
			fmt.Fprintf(stdout, "  failure_kind: %s\n", summary.Verification.FailureKind)
		}
	default:
		fmt.Fprintln(stdout, "  latest: none")
	}

	fmt.Fprintln(stdout, "\nAcceptance evidence:")
	fmt.Fprintf(stdout, "  pass=%d fail=%d blocked=%d\n", summary.Evidence.Pass, summary.Evidence.Fail, summary.Evidence.Blocked)
	for _, record := range summary.Evidence.Latest {
		fmt.Fprintf(stdout, "  %s %s %s\n", strings.ToUpper(record.Status), record.Type, record.Criterion)
		if strings.TrimSpace(record.Summary) != "" {
			fmt.Fprintf(stdout, "    %s\n", record.Summary)
		}
	}

	fmt.Fprintln(stdout, "\nMemory:")
	if summary.Memory.Error != "" && summary.Memory.Status == "none" {
		fmt.Fprintln(stdout, "  candidates: none")
	} else {
		fmt.Fprintf(stdout, "  candidates: %d pending, %d promoted, %d rejected\n", summary.Memory.Pending, summary.Memory.Promoted, summary.Memory.Rejected)
	}

	fmt.Fprintf(stdout, "\nWiki: pending=%d promoted=%d rejected=%d\n", summary.Wiki.Pending, summary.Wiki.Promoted, summary.Wiki.Rejected)
	fmt.Fprintf(stdout, "Metrics: human=%d review_returns=%d verification=%d/%d acceptance=%d/%d/%d wiki=%d/%d\n",
		summary.Metrics.HumanConfirmations,
		summary.Metrics.ReviewReturns,
		summary.Metrics.VerificationPasses,
		summary.Metrics.VerificationRuns,
		summary.Metrics.AcceptancePasses,
		summary.Metrics.AcceptanceFailures,
		summary.Metrics.AcceptanceBlocked,
		summary.Metrics.WikiPromoted,
		summary.Metrics.WikiRejected,
	)

	fmt.Fprintln(stdout, "\nNext:")
	printActions(stdout, summary.Actions)
}

func printActions(stdout io.Writer, actions []demandflow.NextAction) {
	for _, action := range actions {
		fmt.Fprintf(stdout, "  - %s: %s\n", action.Label, action.Reason)
		if strings.TrimSpace(action.Command) != "" {
			fmt.Fprintf(stdout, "    %s\n", action.Command)
		}
	}
}

func printWorkspaceList(stdout io.Writer, summaries []demandflow.WorkspaceSummary) {
	if len(summaries) == 0 {
		fmt.Fprintln(stdout, "No demands found")
		return
	}
	fmt.Fprintln(stdout, "Demand status:")
	for _, summary := range summaries {
		fmt.Fprintf(stdout, "  %-24s %-22s %s\n", summary.Demand.ID, summary.State, summary.Attention)
	}
}

func humanStatus(status string) string {
	return strings.ReplaceAll(status, "_", " ")
}
