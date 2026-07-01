package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/demandflow"
)

func runEvidence(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("evidence requires a subcommand: add or list")
	}
	switch args[0] {
	case "add":
		return runEvidenceAdd(args[1:], stdout, stderr)
	case "list":
		return runEvidenceList(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown evidence command %q", args[0])
	}
}

func runEvidenceAdd(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("evidence add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, evidenceType, criterion, status, summary, link, by string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&evidenceType, "type", "", "evidence type: api, log, monitor, manual, link")
	fs.StringVar(&criterion, "criterion", "", "acceptance criterion or business rule")
	fs.StringVar(&status, "status", "pass", "evidence status: pass, fail, blocked")
	fs.StringVar(&summary, "summary", "", "evidence summary")
	fs.StringVar(&link, "link", "", "optional URL/path/reference")
	fs.StringVar(&by, "by", "", "recorder")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	record, err := demandflow.AddManualEvidence(demandflow.AddManualEvidenceOptions{
		Root:      root,
		DemandID:  demandID,
		Type:      evidenceType,
		Criterion: criterion,
		Status:    status,
		Summary:   summary,
		Link:      link,
		By:        by,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "evidence recorded for %s: %s %s\n", demandID, strings.ToUpper(record.Status), record.Type)
	return nil
}

func runEvidenceList(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDemandLookupArgs("evidence list", args)
	if err != nil {
		return err
	}
	records, err := demandflow.ListManualEvidence(opts.root, opts.demandID)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Manual evidence: %s\n", opts.demandID)
	if len(records) == 0 {
		fmt.Fprintln(stdout, "  none")
		return nil
	}
	for _, record := range records {
		fmt.Fprintf(stdout, "  %s %s %s\n", strings.ToUpper(record.Status), record.Type, record.Criterion)
		if record.Summary != "" {
			fmt.Fprintf(stdout, "    %s\n", record.Summary)
		}
		if record.Link != "" {
			fmt.Fprintf(stdout, "    link: %s\n", record.Link)
		}
		if record.By != "" {
			fmt.Fprintf(stdout, "    by: %s\n", record.By)
		}
	}
	return nil
}

func normalizedRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "."
	}
	return root
}
