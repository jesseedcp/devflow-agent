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

	var root, filePath, title, demandID string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&filePath, "file", "", "local PRD or requirements markdown file")
	fs.StringVar(&title, "title", "", "override demand title")
	fs.StringVar(&demandID, "demand", "", "override demand id")

	if err := fs.Parse(args); err != nil {
		return err
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return fmt.Errorf("--file is required")
	}

	body, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read intake file: %w", err)
	}
	result := intake.ParseMarkdown(intake.Source{Path: filePath, Text: string(body)})
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
		Source:      "intake:file:" + filePath,
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
		Message: "local PRD intake created requirements draft",
		Data: map[string]string{
			"file":      filePath,
			"readiness": string(result.Readiness),
		},
	}); err != nil {
		return err
	}

	demandDir := store.DemandDir(demand.ID)
	fmt.Fprintf(stdout, "Created intake demand %s\n", demand.ID)
	fmt.Fprintf(stdout, "root: %s\n", displayPath(root))
	fmt.Fprintf(stdout, "intake: %s\n", filepath.Join(demandDir, artifacts.IntakeFile))
	fmt.Fprintf(stdout, "context: %s\n", recallResult.ContextPath)
	fmt.Fprintf(stdout, "memory: %d stable, %d candidate\n", recallResult.StableCount, recallResult.CandidateCount)
	fmt.Fprintf(stdout, "requirements: %s\n", filepath.Join(demandDir, artifacts.RequirementsFile))
	fmt.Fprintf(stdout, "state: %s\n", workflow.RequirementsReview)
	fmt.Fprintf(stdout, "next: devflow evaluate --demand %s --stage requirements --strict\n", demand.ID)
	fmt.Fprintf(stdout, "then: devflow confirm --demand %s --stage requirements --by dd --summary \"requirements accepted\"\n", demand.ID)
	return nil
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
