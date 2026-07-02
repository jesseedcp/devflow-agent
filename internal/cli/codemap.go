package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

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
	index, err := codemap.ReadIndex(normalizedRoot(root))
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
