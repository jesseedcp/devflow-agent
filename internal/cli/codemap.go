package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/codemap"
)

func runCodemap(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("codemap requires a subcommand: index or search")
	}
	switch args[0] {
	case "index":
		return runCodemapIndex(args[1:], stdout, stderr)
	case "search":
		return runCodemapSearch(args[1:], stdout, stderr)
	case "refresh":
		return runCodemapRefresh(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown codemap command %q", args[0])
	}
}

func runCodemapIndex(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("codemap index", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root string
	fs.StringVar(&root, "root", ".", "root directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	index, err := codemap.IndexGoFacts(root)
	if err != nil {
		return err
	}
	if err := codemap.WriteIndex(root, index); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "codemap indexed %d facts\n", len(index.Facts))
	return err
}

func readCodemapIndexForCLI(root string) (codemap.Index, error) {
	index, err := codemap.ReadIndex(root)
	if err != nil {
		return codemap.Index{}, fmt.Errorf("%w; run `devflow codemap index` first", err)
	}
	return index, nil
}

func runCodemapSearch(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("codemap search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root string
	var limit int
	fs.StringVar(&root, "root", ".", "root directory")
	fs.IntVar(&limit, "limit", 20, "maximum results")
	if err := fs.Parse(args); err != nil {
		return err
	}
	query := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("search query is required")
	}
	index, err := readCodemapIndexForCLI(normalizedRoot(root))
	if err != nil {
		return err
	}
	results := codemap.Search(index, query, limit)
	for _, result := range results {
		fact := result.Fact
		fmt.Fprintf(stdout, "%s:%d\n  %s %s score=%d\n", fact.File, fact.Line, fact.Kind, fact.Name, result.Score)
	}
	if len(results) == 0 {
		fmt.Fprintln(stdout, "no codemap results")
	}
	return nil
}

func runCodemapRefresh(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("codemap refresh", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID, query string
	var limit int
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&query, "query", "", "search query")
	fs.IntVar(&limit, "limit", 20, "maximum results")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = normalizedRoot(root)
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}
	if strings.TrimSpace(query) == "" {
		query = demandID
	}
	index, err := readCodemapIndexForCLI(root)
	if err != nil {
		return err
	}
	results := codemap.Search(index, query, limit)
	store := artifacts.NewStore(root)
	if err := store.WriteArtifact(demandID, artifacts.CodemapFile, codemap.RenderDemandCodemap(demandID, query, results)); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "codemap refreshed for %s: %d results\n", demandID, len(results))
	return err
}
