package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
		return fmt.Errorf("scope requires a subcommand: declare or diff")
	}
	switch args[0] {
	case "declare":
		return runScopeDeclare(args[1:], stdout, stderr)
	case "diff":
		return runScopeDiff(args[1:], stdout, stderr)
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
	if len(decl.SourceFiles)+len(decl.TestFiles) == 0 {
		return fmt.Errorf("at least one --source or --test file is required")
	}
	if err := store.WriteArtifact(demand.ID, artifacts.ChangeScopeFile, changescope.RenderDeclaration(demand.Title, decl)); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "scope declared for %s\n", demand.ID)
	return err
}

func runScopeDiff(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("scope diff", flag.ContinueOnError)
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
	store := artifacts.NewStore(root)
	if _, err := store.LoadDemand(demandID); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(store.DemandDir(demandID), artifacts.ChangeScopeFile))
	if err != nil {
		return fmt.Errorf("read change scope: %w", err)
	}
	if len(changedFiles) == 0 {
		changedFiles, err = gitChangedFiles(root)
		if err != nil {
			return err
		}
	}
	decl := changescope.ParseDeclaration(string(data))
	result := changescope.CompareChangedFiles(decl, changedFiles)
	renderScopeDiff(stdout, demandID, result)
	if len(result.OutOfScope) > 0 || len(result.MissingTests) > 0 {
		return fmt.Errorf("scope diff found out-of-scope changes or missing declared tests")
	}
	return nil
}

func renderScopeDiff(stdout io.Writer, demandID string, result changescope.DiffResult) {
	fmt.Fprintf(stdout, "scope diff for %s\n", demandID)
	fmt.Fprintf(stdout, "Summary: in_scope=%d out_of_scope=%d missing_tests=%d\n\n", len(result.InScope), len(result.OutOfScope), len(result.MissingTests))
	writeScopeDiffSection(stdout, "In Scope", result.InScope)
	writeScopeDiffSection(stdout, "Out Of Scope", result.OutOfScope)
	writeScopeDiffSection(stdout, "Missing Declared Tests", result.MissingTests)
	if len(result.OutOfScope) > 0 || len(result.MissingTests) > 0 {
		fmt.Fprintln(stdout, "Next: update change-scope.md or adjust implementation changes")
	}
}

func writeScopeDiffSection(stdout io.Writer, title string, files []string) {
	fmt.Fprintf(stdout, "## %s\n", title)
	if len(files) == 0 {
		fmt.Fprintln(stdout, "- none")
		fmt.Fprintln(stdout)
		return
	}
	for _, file := range files {
		fmt.Fprintf(stdout, "- %s\n", file)
	}
	fmt.Fprintln(stdout)
}

func gitChangedFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --name-only HEAD failed: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, filepath.ToSlash(line))
		}
	}
	return files, nil
}
