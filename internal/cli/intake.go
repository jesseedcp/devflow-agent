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
	"github.com/jesseedcp/devflow-agent/internal/demandflow"
	"github.com/jesseedcp/devflow-agent/internal/intake"
	"github.com/jesseedcp/devflow-agent/internal/platform"
	platformfeishu "github.com/jesseedcp/devflow-agent/internal/platform/feishu"
	platformgithub "github.com/jesseedcp/devflow-agent/internal/platform/github"
	"github.com/jesseedcp/devflow-agent/internal/workflow"
)

func runIntake(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("intake", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var root, filePath, rawURL, title, demandID string
	var githubIssue, githubRepo, githubBaseURL, githubToken string
	var feishuDoc, feishuBitable, feishuTable, feishuRecord, feishuBaseURL, feishuAppID, feishuAppSecret string
	fs.StringVar(&root, "root", ".", "root directory")
	fs.StringVar(&filePath, "file", "", "local PRD or requirements markdown file")
	fs.StringVar(&rawURL, "url", "", "HTTP(S) PRD or requirements URL")
	fs.StringVar(&title, "title", "", "override demand title")
	fs.StringVar(&demandID, "demand", "", "override demand id")
	fs.StringVar(&githubIssue, "github-issue", "", "GitHub Issue reference in owner/repo#number form or issue number with --github-repo")
	fs.StringVar(&githubRepo, "github-repo", "", "GitHub repository in owner/repo form for issue intake")
	fs.StringVar(&githubBaseURL, "github-base-url", "", "GitHub API base URL override")
	fs.StringVar(&githubToken, "github-token", "", "GitHub token override")
	fs.StringVar(&feishuDoc, "feishu-doc", "", "Feishu doc URL or token")
	fs.StringVar(&feishuBitable, "feishu-bitable", "", "Feishu Bitable app token")
	fs.StringVar(&feishuTable, "table", "", "Feishu Bitable table id")
	fs.StringVar(&feishuRecord, "record", "", "Feishu Bitable record id")
	fs.StringVar(&feishuBaseURL, "feishu-base-url", "", "Feishu OpenAPI base URL override")
	fs.StringVar(&feishuAppID, "feishu-app-id", "", "Feishu app id override")
	fs.StringVar(&feishuAppSecret, "feishu-app-secret", "", "Feishu app secret override")

	if err := fs.Parse(args); err != nil {
		return err
	}
	source, err := loadIntakeSource(intakeLoadOptions{
		FilePath:        filePath,
		RawURL:          rawURL,
		GitHubIssue:     githubIssue,
		GitHubRepo:      githubRepo,
		GitHubBaseURL:   githubBaseURL,
		GitHubToken:     githubToken,
		FeishuDoc:       feishuDoc,
		FeishuBitable:   feishuBitable,
		FeishuTable:     feishuTable,
		FeishuRecord:    feishuRecord,
		FeishuBaseURL:   feishuBaseURL,
		FeishuAppID:     feishuAppID,
		FeishuAppSecret: feishuAppSecret,
	})
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
	if source.snapshot != nil {
		if err := store.WriteArtifact(demand.ID, artifacts.IntakeFile, platform.RenderIntakeSnapshot(*source.snapshot)); err != nil {
			return err
		}
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
	if source.snapshot != nil {
		if err := store.AppendEvent(demand.ID, artifacts.Event{
			Type:    "platform.intake_fetched",
			Message: "platform intake fetched",
			Data: map[string]string{
				"provider":    string(source.snapshot.Provider),
				"kind":        string(source.snapshot.Kind),
				"external_id": source.snapshot.ExternalID,
				"url":         source.snapshot.URL,
			},
		}); err != nil {
			return err
		}
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
	kind     string
	label    string
	result   intake.Result
	snapshot *platform.IntakeSnapshot
}

type intakeLoadOptions struct {
	FilePath        string
	RawURL          string
	GitHubIssue     string
	GitHubRepo      string
	GitHubBaseURL   string
	GitHubToken     string
	FeishuDoc       string
	FeishuBitable   string
	FeishuTable     string
	FeishuRecord    string
	FeishuBaseURL   string
	FeishuAppID     string
	FeishuAppSecret string
}

func loadIntakeSource(opts intakeLoadOptions) (intakeSource, error) {
	sources := 0
	if strings.TrimSpace(opts.FilePath) != "" {
		sources++
	}
	if strings.TrimSpace(opts.RawURL) != "" {
		sources++
	}
	if strings.TrimSpace(opts.GitHubIssue) != "" {
		sources++
	}
	if strings.TrimSpace(opts.FeishuDoc) != "" {
		sources++
	}
	if strings.TrimSpace(opts.FeishuBitable) != "" {
		sources++
	}
	if sources != 1 {
		return intakeSource{}, fmt.Errorf("exactly one intake source is required")
	}
	if strings.TrimSpace(opts.FilePath) != "" {
		body, err := os.ReadFile(strings.TrimSpace(opts.FilePath))
		if err != nil {
			return intakeSource{}, fmt.Errorf("read intake file: %w", err)
		}
		return intakeSource{kind: "file", label: strings.TrimSpace(opts.FilePath), result: intake.ParseMarkdown(intake.Source{Path: strings.TrimSpace(opts.FilePath), Text: string(body)})}, nil
	}
	if strings.TrimSpace(opts.RawURL) != "" {
		result, err := intake.FetchURL(strings.TrimSpace(opts.RawURL))
		if err != nil {
			return intakeSource{}, err
		}
		return intakeSource{kind: "url", label: strings.TrimSpace(opts.RawURL), result: result}, nil
	}
	if strings.TrimSpace(opts.GitHubIssue) != "" {
		return loadGitHubIssueIntake(opts)
	}
	if strings.TrimSpace(opts.FeishuDoc) != "" {
		return loadFeishuDocIntake(opts)
	}
	return loadFeishuBitableIntake(opts)
}

func loadGitHubIssueIntake(opts intakeLoadOptions) (intakeSource, error) {
	repo, issue, err := parseGitHubIssueRef(opts.GitHubRepo, opts.GitHubIssue)
	if err != nil {
		return intakeSource{}, err
	}
	snapshot, err := (platformgithub.IssueAdapter{}).FetchIntake(context.Background(), platform.IntakeRef{
		Provider: platform.ProviderGitHub,
		Kind:     platform.SourceGitHubIssue,
		Repo:     repo,
		Issue:    issue,
		Token:    strings.TrimSpace(opts.GitHubToken),
		BaseURL:  strings.TrimSpace(opts.GitHubBaseURL),
	})
	if err != nil {
		return intakeSource{}, err
	}
	markdown := platform.RenderIntakeSnapshot(snapshot)
	result := intake.ParseMarkdown(intake.Source{Path: snapshot.ExternalID, Text: markdown})
	if strings.TrimSpace(snapshot.Title) != "" {
		result.Title = snapshot.Title
		result.RequirementsMarkdown = intake.RenderRequirements(result)
	}
	return intakeSource{
		kind:     string(platform.SourceGitHubIssue),
		label:    snapshot.ExternalID,
		result:   result,
		snapshot: &snapshot,
	}, nil
}

func parseGitHubIssueRef(repo, ref string) (string, string, error) {
	ref = strings.TrimSpace(ref)
	repo = strings.TrimSpace(repo)
	if strings.Contains(ref, "#") {
		parts := strings.SplitN(ref, "#", 2)
		repo = strings.TrimSpace(parts[0])
		ref = strings.TrimSpace(parts[1])
	}
	if repo == "" || ref == "" {
		return "", "", fmt.Errorf("--github-issue requires owner/repo#number or --github-repo with issue number")
	}
	return repo, ref, nil
}

func loadFeishuDocIntake(opts intakeLoadOptions) (intakeSource, error) {
	docRef := strings.TrimSpace(opts.FeishuDoc)
	adapter := platformfeishu.DocAdapter{
		BaseURL: strings.TrimSpace(opts.FeishuBaseURL),
		TokenClient: &platformfeishu.TenantTokenClient{
			BaseURL:   strings.TrimSpace(opts.FeishuBaseURL),
			AppID:     strings.TrimSpace(opts.FeishuAppID),
			AppSecret: strings.TrimSpace(opts.FeishuAppSecret),
		},
	}
	snapshot, err := adapter.FetchIntake(context.Background(), platform.IntakeRef{
		Provider: platform.ProviderFeishu,
		Kind:     platform.SourceFeishuDoc,
		Token:    docRef,
		URL:      docRef,
	})
	if err != nil {
		return intakeSource{}, err
	}
	markdown := platform.RenderIntakeSnapshot(snapshot)
	result := intake.ParseMarkdown(intake.Source{Path: snapshot.ExternalID, Text: markdown})
	if strings.TrimSpace(snapshot.Title) != "" {
		result.Title = snapshot.Title
		result.RequirementsMarkdown = intake.RenderRequirements(result)
	}
	return intakeSource{kind: string(platform.SourceFeishuDoc), label: snapshot.ExternalID, result: result, snapshot: &snapshot}, nil
}

func loadFeishuBitableIntake(opts intakeLoadOptions) (intakeSource, error) {
	adapter := platformfeishu.BitableAdapter{
		BaseURL: strings.TrimSpace(opts.FeishuBaseURL),
		TokenClient: &platformfeishu.TenantTokenClient{
			BaseURL:   strings.TrimSpace(opts.FeishuBaseURL),
			AppID:     strings.TrimSpace(opts.FeishuAppID),
			AppSecret: strings.TrimSpace(opts.FeishuAppSecret),
		},
	}
	record, err := adapter.FetchDemand(context.Background(), platform.IntakeRef{
		AppToken: strings.TrimSpace(opts.FeishuBitable),
		TableID:  strings.TrimSpace(opts.FeishuTable),
		RecordID: strings.TrimSpace(opts.FeishuRecord),
	})
	if err != nil {
		return intakeSource{}, err
	}
	body := strings.TrimSpace(record.Description)
	if body == "" {
		body = record.Title
	}
	snapshot := platform.IntakeSnapshot{
		Provider:   platform.ProviderFeishu,
		Kind:       platform.SourceFeishuRecord,
		ExternalID: record.ID,
		Title:      record.Title,
		Body:       body,
		Metadata: map[string]string{
			"status":   record.Status,
			"priority": record.Priority,
			"owner":    record.Owner,
		},
	}
	result := intake.ParseMarkdown(intake.Source{Path: record.ID, Text: platform.RenderIntakeSnapshot(snapshot)})
	result.Title = record.Title
	result.RequirementsMarkdown = intake.RenderRequirements(result)
	return intakeSource{kind: string(platform.SourceFeishuRecord), label: record.ID, result: result, snapshot: &snapshot}, nil
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
