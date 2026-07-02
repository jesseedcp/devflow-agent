package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/artifacts"
	"github.com/jesseedcp/devflow-agent/internal/platform"
	platformfeishu "github.com/jesseedcp/devflow-agent/internal/platform/feishu"
	platformgithub "github.com/jesseedcp/devflow-agent/internal/platform/github"
)

func runSync(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var root, demandID, githubIssue, githubRepo, githubBaseURL, githubToken string
	var feishuBitable, feishuTable, feishuRecord, feishuBaseURL, feishuAppID, feishuAppSecret string
	var dryRun bool
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&demandID, "demand", "", "demand id")
	fs.StringVar(&githubIssue, "github-issue", "", "GitHub Issue reference in owner/repo#number form")
	fs.StringVar(&githubRepo, "github-repo", "", "GitHub repository in owner/repo form")
	fs.StringVar(&githubBaseURL, "github-base-url", "", "GitHub API base URL override")
	fs.StringVar(&githubToken, "github-token", "", "GitHub token override")
	fs.StringVar(&feishuBitable, "feishu-bitable", "", "Feishu Bitable app token")
	fs.StringVar(&feishuTable, "table", "", "Feishu Bitable table id")
	fs.StringVar(&feishuRecord, "record", "", "Feishu Bitable record id")
	fs.StringVar(&feishuBaseURL, "feishu-base-url", "", "Feishu OpenAPI base URL override")
	fs.StringVar(&feishuAppID, "feishu-app-id", "", "Feishu app id override")
	fs.StringVar(&feishuAppSecret, "feishu-app-secret", "", "Feishu app secret override")
	fs.BoolVar(&dryRun, "dry-run", false, "render sync without writing to the external platform")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(demandID) == "" {
		return fmt.Errorf("--demand is required")
	}
	store := artifacts.NewStore(root)
	demand, err := store.LoadDemand(demandID)
	if err != nil {
		return err
	}

	switch {
	case strings.TrimSpace(githubIssue) != "":
		return syncGitHubIssue(root, demand, githubRepo, githubIssue, githubBaseURL, githubToken, dryRun, store, stdout)
	case strings.TrimSpace(feishuBitable) != "":
		ref := platform.IntakeRef{
			Provider: platform.ProviderFeishu,
			Kind:     platform.SourceFeishuRecord,
			AppToken: strings.TrimSpace(feishuBitable),
			TableID:  strings.TrimSpace(feishuTable),
			RecordID: strings.TrimSpace(feishuRecord),
		}
		if err := syncFeishuBitable(root, demand, ref, feishuBaseURL, feishuAppID, feishuAppSecret, store); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "synced %s to feishu bitable record %s\n", demand.ID, ref.RecordID)
		return nil
	default:
		return fmt.Errorf("--github-issue or --feishu-bitable is required")
	}
}

func syncGitHubIssue(root string, demand artifacts.Demand, githubRepo, githubIssue, githubBaseURL, githubToken string, dryRun bool, store artifacts.Store, stdout io.Writer) error {
	repo, issue, err := parseGitHubIssueRef(githubRepo, githubIssue)
	if err != nil {
		return err
	}
	update := platform.ProgressUpdate{
		DemandID: demand.ID,
		Stage:    demand.State,
		State:    demand.State,
		Summary:  syncSummary(root, demand.ID, demand.State),
		Marker:   platform.SyncMarker(demand.ID, demand.State),
		DryRun:   dryRun,
	}
	if err := (platformgithub.IssueAdapter{}).PostProgress(context.Background(), platform.IntakeRef{
		Provider: platform.ProviderGitHub,
		Kind:     platform.SourceGitHubIssue,
		Repo:     repo,
		Issue:    issue,
		Token:    strings.TrimSpace(githubToken),
		BaseURL:  strings.TrimSpace(githubBaseURL),
	}, update); err != nil {
		_ = store.AppendEvent(demand.ID, artifacts.Event{
			Type:    "platform.sync_failed",
			Message: "platform sync failed",
			Data:    map[string]string{"provider": "github", "external_id": repo + "#" + issue, "reason": err.Error()},
		})
		return err
	}
	if !dryRun {
		if err := store.AppendEvent(demand.ID, artifacts.Event{
			Type:    "platform.sync_posted",
			Message: "platform sync posted",
			Data:    map[string]string{"provider": "github", "target": "issue_comment", "external_id": repo + "#" + issue, "stage": demand.State},
		}); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "synced %s to github issue %s#%s\n", demand.ID, repo, issue)
	return nil
}

func syncFeishuBitable(root string, demand artifacts.Demand, ref platform.IntakeRef, baseURL, appID, appSecret string, store artifacts.Store) error {
	adapter := platformfeishu.BitableAdapter{
		BaseURL: strings.TrimSpace(baseURL),
		TokenClient: &platformfeishu.TenantTokenClient{
			BaseURL:   strings.TrimSpace(baseURL),
			AppID:     strings.TrimSpace(appID),
			AppSecret: strings.TrimSpace(appSecret),
		},
	}
	summary := syncSummary(root, demand.ID, demand.State)
	if err := adapter.UpdateDemand(context.Background(), ref, platform.DemandStatusUpdate{
		DemandID:     demand.ID,
		State:        demand.State,
		Stage:        demand.State,
		Summary:      summary,
		Verification: summary,
	}); err != nil {
		_ = store.AppendEvent(demand.ID, artifacts.Event{Type: "platform.sync_failed", Message: "platform sync failed", Data: map[string]string{"provider": "feishu", "external_id": ref.RecordID, "reason": err.Error()}})
		return err
	}
	return store.AppendEvent(demand.ID, artifacts.Event{Type: "platform.sync_posted", Message: "platform sync posted", Data: map[string]string{"provider": "feishu", "target": "bitable_record", "external_id": ref.RecordID, "stage": demand.State}})
}

func syncSummary(root, demandID, state string) string {
	fileByState := map[string]string{
		"requirements_review": artifacts.RequirementsFile,
		"plan_review":         artifacts.PlanFile,
		"verification":        artifacts.VerificationFile,
		"closeout":            artifacts.CloseoutFile,
		"completed":           artifacts.CloseoutFile,
	}
	name := fileByState[state]
	if name == "" {
		name = artifacts.ProgressFile
	}
	body, err := os.ReadFile(filepath.Join(root, ".devflow", "demands", demandID, name))
	if err != nil {
		return "Devflow state: " + state
	}
	text := strings.TrimSpace(string(body))
	if len(text) > 2000 {
		return text[:2000]
	}
	return text
}
