package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/intake"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func runIntake(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("intake", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root, filePath, rawURL, title, demandID string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&filePath, "file", "", "local PRD or requirements markdown file")
	fs.StringVar(&rawURL, "url", "", "HTTP(S) PRD or requirements URL")
	fs.StringVar(&title, "title", "", "override demand title")
	fs.StringVar(&demandID, "demand", "", "override demand id")

	if err := fs.Parse(args); err != nil {
		return err
	}
	source, err := loadIntakeSource(filePath, rawURL)
	if err != nil {
		return err
	}
	result := source.result
	if strings.TrimSpace(title) != "" {
		result.Title = strings.TrimSpace(title)
		result.RequirementsMarkdown = intake.RenderRequirements(result)
	}
	if strings.TrimSpace(demandID) == "" {
		demandID = slugify(result.Title)
	} else {
		demandID = strings.TrimSpace(demandID)
	}

	store := artifacts.NewStore(root)
	demand := artifacts.Demand{
		ID:          demandID,
		Title:       result.Title,
		Description: intakeDescription(result),
		Source:      "intake:" + source.kind + ":" + source.label,
		State:       string(workflow.Created),
	}
	if err := store.CreateDemand(demand); err != nil {
		return err
	}
	if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, intake.RenderSnapshot(result)); err != nil {
		return err
	}
	if err := store.WriteArtifact(demand.ID, artifacts.RequirementsFile, result.RequirementsMarkdown); err != nil {
		return err
	}
	recallResult, err := demandflow.WriteMemoryRecall(root, demand.ID)
	if err != nil {
		return err
	}
	demand.State = string(workflow.RequirementsReview)
	if err := store.SaveDemand(demand); err != nil {
		return err
	}
	if err := store.AppendEvent(demand.ID, artifacts.Event{
		Type:    "intake.created",
		Message: source.kind + " PRD intake created requirements draft",
		Data: map[string]string{
			source.kind: source.label,
			"readiness": string(result.Readiness),
		},
	}); err != nil {
		return err
	}

	demandDir := store.DemandDir(demand.ID)
	fmt.Fprintf(stdout, "Created intake demand %s\n", demand.ID)
	fmt.Fprintf(stdout, "root: %s\n", displayPath(root))
	fmt.Fprintf(stdout, "%s: %s\n", source.kind, source.label)
	fmt.Fprintf(stdout, "intake: %s\n", filepath.Join(demandDir, artifacts.IntakeFile))
	fmt.Fprintf(stdout, "context: %s\n", recallResult.ContextPath)
	fmt.Fprintf(stdout, "memory: %d stable, %d candidate\n", recallResult.StableCount, recallResult.CandidateCount)
	fmt.Fprintf(stdout, "requirements: %s\n", filepath.Join(demandDir, artifacts.RequirementsFile))
	fmt.Fprintf(stdout, "state: %s\n", workflow.RequirementsReview)
	fmt.Fprintf(stdout, "next: devflow evaluate --demand %s --stage requirements --strict\n", demand.ID)
	fmt.Fprintf(stdout, "then: devflow confirm --demand %s --stage requirements --by dd --summary \"requirements accepted\"\n", demand.ID)
	return nil
}

type intakeSource struct {
	kind   string
	label  string
	result intake.Result
}

func loadIntakeSource(filePath, rawURL string) (intakeSource, error) {
	filePath = strings.TrimSpace(filePath)
	rawURL = strings.TrimSpace(rawURL)
	if (filePath == "") == (rawURL == "") {
		return intakeSource{}, fmt.Errorf("exactly one of --file or --url is required")
	}
	if filePath != "" {
		body, err := os.ReadFile(filePath)
		if err != nil {
			return intakeSource{}, fmt.Errorf("read intake file: %w", err)
		}
		return intakeSource{kind: "file", label: filePath, result: intake.ParseMarkdown(intake.Source{Path: filePath, Text: string(body)})}, nil
	}
	result, err := intake.FetchURL(rawURL)
	if err != nil {
		return intakeSource{}, err
	}
	return intakeSource{kind: "url", label: rawURL, result: result}, nil
}

func intakeDescription(result intake.Result) string {
	if len(result.Goals) == 0 {
		return result.Title
	}
	return result.Goals[0]
}

func displayPath(root string) string {
	if strings.TrimSpace(root) == "." {
		cwd, err := os.Getwd()
		if err == nil {
			return cwd
		}
	}
	return root
}
