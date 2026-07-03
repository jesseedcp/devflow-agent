package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/implreview"
)

func runImplementationReview(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("implementation-review requires a subcommand: refresh or status")
	}
	switch args[0] {
	case "refresh":
		return runImplementationReviewRefresh(args[1:], stdout, stderr)
	case "status":
		return runImplementationReviewStatus(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown implementation-review command %q", args[0])
	}
}

func runImplementationReviewRefresh(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("implementation-review refresh", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID string
	var changedFiles stringListFlag
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.Var(&changedFiles, "changed", "changed file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	if len(changedFiles) == 0 {
		var err error
		changedFiles, err = gitChangedFiles(root)
		if err != nil {
			return err
		}
	}
	review, err := implreview.Collect(root, demandID, changedFiles)
	if err != nil {
		return err
	}
	store := artifacts.NewStore(root)
	if err := store.WriteArtifact(demandID, artifacts.ImplementationReviewFile, implreview.Render(review)); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "implementation review refreshed for %s: %s\n", demandID, review.Recommendation)
	return err
}

func runImplementationReviewStatus(args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseDemandLookupArgs("implementation-review status", args)
	if err != nil {
		return err
	}
	path := filepath.Join(artifacts.NewStore(opts.root).DemandDir(opts.demandID), artifacts.ImplementationReviewFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read implementation review: %w; run `devflow implementation-review refresh --demand %s` first", err, opts.demandID)
	}
	_, err = stdout.Write(data)
	return err
}
