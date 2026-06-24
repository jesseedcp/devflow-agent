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
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

const helpText = `devflow - backend demand delivery agent

Usage:
  devflow help
  devflow start --title <title> --description <text>
  devflow confirm --demand <id> --stage <requirements|plan|verification|closeout> --by <name> --summary <text>

Commands:
  help     Show this help text
  start    Create a new demand workspace
  confirm  Record a human confirmation and advance the workflow gate
`

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" {
		_, err := fmt.Fprint(stdout, helpText)
		return err
	}

	switch args[0] {
	case "start":
		return runStart(args[1:], stdout)
	case "confirm":
		return runConfirm(args[1:], stdout)
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

func runConfirm(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("confirm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root string
	var demandID string
	var stage string
	var by string
	var summary string

	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&stage, "stage", "", "stage name")
	fs.StringVar(&by, "by", "", "confirming person")
	fs.StringVar(&summary, "summary", "", "confirmation summary")

	if err := fs.Parse(args); err != nil {
		return err
	}

	root = strings.TrimSpace(root)
	demandID = strings.TrimSpace(demandID)
	stage = strings.TrimSpace(stage)
	by = normalizeConfirmationText(by)
	summary = normalizeConfirmationText(summary)
	if root == "" {
		root = "."
	}
	if demandID == "" || stage == "" || by == "" || summary == "" {
		return fmt.Errorf("--demand, --stage, --by, and --summary are required")
	}

	artifactName, requiredCurrent, nextState, label, err := confirmationTarget(stage)
	if err != nil {
		return err
	}

	store := artifacts.NewStore(root)
	return store.WithDemandLock(demandID, func() error {
		demand, err := store.LoadDemand(demandID)
		if err != nil {
			return err
		}

		current := workflow.State(demand.State)
		if current != requiredCurrent {
			return fmt.Errorf("confirmation stage %q requires current state %s, got %s", stage, requiredCurrent, current)
		}

		advanced, err := workflow.Advance(current, nextState)
		if err != nil {
			return err
		}

		confirmedAt := time.Now().UTC()
		cycleToken := demand.UpdatedAt.UTC().Format(time.RFC3339Nano)
		confirmationID := confirmationID(demandID, stage, cycleToken, by, summary)
		record := fmt.Sprintf("- %s confirmed by %s at %s: %s\n", label, by, confirmedAt.Format(time.RFC3339), summary)
		if err := store.EnsureConfirmationEvidence(demandID, artifactName, confirmationID, record, artifacts.Event{
			Time:    confirmedAt,
			Type:    "stage.confirmed",
			Message: label + " confirmed",
			Data: map[string]string{
				"by":      by,
				"stage":   stage,
				"summary": summary,
			},
		}); err != nil {
			return err
		}

		demand.State = string(advanced)
		if err := store.SaveDemand(demand); err != nil {
			return err
		}

		_, err = fmt.Fprintf(stdout, "%s confirmed for %s\n", label, demandID)
		return err
	})
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

func confirmationTarget(stage string) (artifact string, requiredCurrent workflow.State, next workflow.State, label string, err error) {
	switch stage {
	case "requirements":
		return artifacts.RequirementsFile, workflow.RequirementsReview, workflow.PlanDrafting, "requirements", nil
	case "plan":
		return artifacts.PlanFile, workflow.PlanReview, workflow.Implementation, "plan", nil
	case "verification":
		return artifacts.VerificationFile, workflow.Verification, workflow.Closeout, "verification", nil
	case "closeout":
		return artifacts.CloseoutFile, workflow.Closeout, workflow.Completed, "closeout", nil
	default:
		return "", "", "", "", fmt.Errorf("unsupported confirmation stage %q", stage)
	}
}

func normalizeConfirmationText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func confirmationID(demandID, stage, cycleToken, by, summary string) string {
	normalizedDemandID := strings.ToLower(strings.TrimSpace(demandID))
	normalizedStage := strings.TrimSpace(stage)
	normalizedCycleToken := strings.TrimSpace(cycleToken)
	normalizedBy := normalizeConfirmationText(by)
	normalizedSummary := normalizeConfirmationText(summary)

	hash := sha256.Sum256([]byte(normalizedDemandID + "\x00" + normalizedStage + "\x00" + normalizedCycleToken + "\x00" + normalizedBy + "\x00" + normalizedSummary))
	return hex.EncodeToString(hash[:8])
}
