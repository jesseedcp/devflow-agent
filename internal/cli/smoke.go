package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

var newSmokeRunner = func(configPath string) demandflow.Runner {
	return demandflow.RuntimeRunner{ConfigPath: configPath, MaxIterations: 8}
}

func runSmoke(args []string, stdout io.Writer, _ io.Writer) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root, title, description, configPath string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&title, "title", "", "smoke demand title")
	fs.StringVar(&description, "description", "", "smoke demand description")
	fs.StringVar(&configPath, "config", "", "devflow config path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return fmt.Errorf("--title is required")
	}
	if description == "" {
		return fmt.Errorf("--description is required")
	}

	demand := artifacts.Demand{
		ID:          slugify(title),
		Title:       title,
		Description: description,
		Source:      "smoke",
		State:       string(workflow.Created),
	}
	store := artifacts.NewStore(root)
	if err := store.CreateDemand(demand); err != nil {
		return err
	}

	engine := demandflow.NewEngine(root)
	result, err := engine.RunDetailed(context.Background(), demandflow.Options{
		Root:     root,
		DemandID: demand.ID,
		Stage:    demandflow.StageRequirements,
		Runner:   newSmokeRunner(configPath),
		Now:      time.Now,
	})
	if err != nil {
		if result.DemandID != "" {
			printRunResult(stdout, result)
		}
		return err
	}
	printRunResult(stdout, result)
	fmt.Fprintf(stdout, "smoke demand: %s\n", demand.ID)
	fmt.Fprintf(stdout, "requirements: %s\n", filepath.Join(store.DemandDir(demand.ID), artifacts.RequirementsFile))
	return nil
}
