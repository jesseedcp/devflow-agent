package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	changescope "github.com/jesseedcp/devflow-agent/internal/scope"
)

type stringListFlag []string

func (flag *stringListFlag) String() string {
	return strings.Join(*flag, ",")
}

func (flag *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*flag = append(*flag, value)
	}
	return nil
}

func runScope(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("scope requires a subcommand: declare")
	}
	switch args[0] {
	case "declare":
		return runScopeDeclare(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown scope command %q", args[0])
	}
}

func runScopeDeclare(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("scope declare", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID string
	var sourceFiles, testFiles, outOfScope stringListFlag
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.Var(&sourceFiles, "source", "source file expected to change")
	fs.Var(&testFiles, "test", "test file expected to change")
	fs.Var(&outOfScope, "out-of-scope", "path expected to remain out of scope")
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
	decl := changescope.Declaration{SourceFiles: sourceFiles, TestFiles: testFiles, OutOfScope: outOfScope}
	if err := store.WriteArtifact(demand.ID, artifacts.ChangeScopeFile, changescope.RenderDeclaration(demand.Title, decl)); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "scope declared for %s\n", demand.ID)
	return err
}
