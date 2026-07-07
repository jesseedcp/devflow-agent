package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/evidence"
	"github.com/jesseedcp/devflow-agent/internal/releasecontrol"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func runObserve(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: devflow observe <refresh> ...")
	}
	switch args[0] {
	case "refresh":
		return runObserveRefresh(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown observe subcommand %q", args[0])
	}
}

func runObserveRefresh(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("observe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var root, demandID string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	var healthURL, healthMethod, expectContains string
	var expectStatus int
	var healthHeaders stringListFlag
	fs.StringVar(&healthURL, "health-url", "", "optional HTTP health URL to verify after deployment")
	fs.StringVar(&healthMethod, "health-method", http.MethodGet, "HTTP method for --health-url")
	fs.Var(&healthHeaders, "health-header", "HTTP health header in Name: value form; repeatable")
	fs.IntVar(&expectStatus, "expect-status", http.StatusOK, "expected HTTP status for --health-url")
	fs.StringVar(&expectContains, "expect-contains", "", "substring expected in HTTP health response")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	demandID = strings.TrimSpace(demandID)
	if demandID == "" {
		return fmt.Errorf("--demand is required")
	}

	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}
	if workflow.State(demand.State) != workflow.Observation {
		return fmt.Errorf("observe refresh requires state observation, got %s", demand.State)
	}

	deploymentText, err := os.ReadFile(filepath.Join(store.DemandDir(demandID), artifacts.DeploymentFile))
	if err != nil {
		return fmt.Errorf("read deployment evidence: %w", err)
	}
	deployment := releasecontrol.ParseDeployment(string(deploymentText))
	if strings.TrimSpace(deployment.RunID) == "" {
		return fmt.Errorf("no deployment evidence recorded; run devflow deploy trigger first")
	}

	if deployment.Status == releasecontrol.StatusPassed {
		if strings.TrimSpace(healthURL) != "" {
			result := evidence.HTTPFetcher{}.Fetch(context.Background(), evidence.HTTPFetchRequest{
				Method:         healthMethod,
				URL:            healthURL,
				Headers:        []string(healthHeaders),
				ExpectStatus:   expectStatus,
				ExpectContains: expectContains,
			})
			healthRecord := releasecontrol.ObservationFromHealthResult(result)
			healthRecord.Provider = deployment.Provider
			healthRecord.Repo = deployment.Repo
			healthRecord.RunID = deployment.RunID
			healthRecord.RunURL = deployment.RunURL
			healthRecord.DeploymentStatus = releasecontrol.StatusPassed
			healthRecord.ObservedAt = time.Now().UTC()
			if healthRecord.Status == releasecontrol.StatusPassed {
				healthRecord.Summary = "deployment evidence and HTTP health check passed"
				if err := store.WriteArtifact(demandID, artifacts.ObservationFile, releasecontrol.RenderObservation(demand.Title, healthRecord)); err != nil {
					return err
				}
				if err := store.AppendEvent(demandID, artifacts.Event{
					Time:    time.Now().UTC(),
					Type:    "observation.passed",
					Message: "observation passed with HTTP health check",
					Data: map[string]string{
						"health_url": result.URL,
						"status":     result.Status,
					},
				}); err != nil {
					return err
				}
				if err := advanceDemandState(store, demandID, workflow.Observation, workflow.Closeout); err != nil {
					return err
				}
				fmt.Fprintln(stdout, "observation passed")
				return nil
			}
			if err := store.WriteArtifact(demandID, artifacts.ObservationFile, releasecontrol.RenderObservation(demand.Title, healthRecord)); err != nil {
				return err
			}
			if err := store.AppendEvent(demandID, artifacts.Event{
				Time:    time.Now().UTC(),
				Type:    "observation.failed",
				Message: "observation failed HTTP health check",
				Data: map[string]string{
					"health_url": result.URL,
					"status":     result.Status,
				},
			}); err != nil {
				return err
			}
			if err := writeRollbackRecommendation(store, demandID, demand.Title, "post-deploy health check failed", "release evidence is not passing", "record a rollback decision"); err != nil {
				return err
			}
			if err := appendRollbackRecommended(store, demandID); err != nil {
				return err
			}
			if err := advanceDemandState(store, demandID, workflow.Observation, workflow.BlockedNeedReleaseDecision); err != nil {
				return err
			}
			fmt.Fprintln(stdout, "observation failed")
			return nil
		}
		record := releasecontrol.ObservationRecord{
			Provider:         deployment.Provider,
			Repo:             deployment.Repo,
			RunID:            deployment.RunID,
			RunURL:           deployment.RunURL,
			DeploymentStatus: releasecontrol.StatusPassed,
			Status:           releasecontrol.StatusPassed,
			Summary:          "deployment evidence is passing",
			EvidenceLinks:    evidenceLinksFor(deployment.RunURL),
			ObservedAt:       time.Now().UTC(),
		}
		if err := store.WriteArtifact(demandID, artifacts.ObservationFile, releasecontrol.RenderObservation(demand.Title, record)); err != nil {
			return err
		}
		if err := store.AppendEvent(demandID, artifacts.Event{Time: time.Now().UTC(), Type: "observation.passed", Message: "observation passed"}); err != nil {
			return err
		}
		if err := advanceDemandState(store, demandID, workflow.Observation, workflow.Closeout); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "observation passed")
		return nil
	}

	record := releasecontrol.ObservationRecord{
		Provider:         deployment.Provider,
		Repo:             deployment.Repo,
		RunID:            deployment.RunID,
		RunURL:           deployment.RunURL,
		DeploymentStatus: deployment.Status,
		Status:           releasecontrol.StatusFailed,
		Summary:          "deployment evidence is not passing",
		ObservedAt:       time.Now().UTC(),
	}
	if err := store.WriteArtifact(demandID, artifacts.ObservationFile, releasecontrol.RenderObservation(demand.Title, record)); err != nil {
		return err
	}
	if err := store.AppendEvent(demandID, artifacts.Event{Time: time.Now().UTC(), Type: "observation.failed", Message: "observation failed"}); err != nil {
		return err
	}
	if err := writeRollbackRecommendation(store, demandID, demand.Title, "deployment or observation failed", "release evidence is not passing", "record a rollback decision"); err != nil {
		return err
	}
	if err := appendRollbackRecommended(store, demandID); err != nil {
		return err
	}
	if err := advanceDemandState(store, demandID, workflow.Observation, workflow.BlockedNeedReleaseDecision); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "observation failed")
	return nil
}

func evidenceLinksFor(runURL string) []string {
	if strings.TrimSpace(runURL) == "" {
		return nil
	}
	return []string{runURL}
}
