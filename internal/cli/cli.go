package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

const helpText = `devflow - backend demand delivery agent

Usage:
  devflow help
  devflow start --title <title> --description <text>

Commands:
  help    Show this help text
  start   Create a new demand workspace
`

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" {
		_, err := fmt.Fprint(stdout, helpText)
		return err
	}

	switch args[0] {
	case "start":
		return runStart(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText)
	}
}

var slugPattern = regexp.MustCompile(`[^a-z0-9]+`)

func runStart(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root string
	var title string
	var description string

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&title, "title", "", "demand title")
	fs.StringVar(&description, "description", "", "demand description")

	if err := fs.Parse(args); err != nil {
		return err
	}

	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return fmt.Errorf("--title is required")
	}

	demand := artifacts.Demand{
		ID:          slugify(title),
		Title:       title,
		Description: description,
		Source:      "manual",
		State:       string(workflow.Created),
	}

	store := artifacts.NewStore(root)
	if err := store.CreateDemand(demand); err != nil {
		return err
	}

	displayRoot := root
	if root == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		displayRoot = cwd
	}

	_, err := fmt.Fprintf(stdout, "Created demand %s under %s\n", demand.ID, displayRoot)
	return err
}

func slugify(title string) string {
	normalized := strings.ToLower(strings.TrimSpace(title))
	slug := slugPattern.ReplaceAllString(normalized, "-")
	slug = strings.Trim(slug, "-")

	hash := sha256.Sum256([]byte(normalized))
	suffix := hex.EncodeToString(hash[:6])

	if slug == "" {
		return "demand-" + suffix
	}
	if containsNonASCII(normalized) {
		return slug + "-" + suffix
	}
	return slug
}

func containsNonASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return true
		}
	}
	return false
}
