package platform

import (
	"context"
	"time"
)

type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderFeishu Provider = "feishu"
)

type SourceKind string

const (
	SourceGitHubIssue  SourceKind = "github_issue"
	SourceFeishuDoc    SourceKind = "feishu_doc"
	SourceFeishuRecord SourceKind = "feishu_bitable_record"
)

type Capability string

const (
	CapabilityReadIntake Capability = "read_intake"
	CapabilityWriteSync  Capability = "write_sync"
	CapabilityReadPool   Capability = "read_pool"
	CapabilityWritePool  Capability = "write_pool"
)

type IntakeRef struct {
	Provider   Provider
	Kind       SourceKind
	Repo       string
	Issue      string
	URL        string
	Token      string
	AppToken   string
	TableID    string
	RecordID   string
	BaseURL    string
	Properties map[string]string
}

type IntakeSnapshot struct {
	Provider   Provider
	Kind       SourceKind
	ExternalID string
	Title      string
	Body       string
	URL        string
	Author     string
	Labels     []string
	Comments   []ExternalComment
	Metadata   map[string]string
	FetchedAt  time.Time
}

type ExternalComment struct {
	ID        string
	Author    string
	Body      string
	URL       string
	CreatedAt time.Time
}

type ExternalDemand struct {
	ID          string
	Title       string
	Description string
	Status      string
	Priority    string
	Owner       string
	URL         string
	Metadata    map[string]string
}

type DemandStatusUpdate struct {
	DemandID     string
	State        string
	Stage        string
	Summary      string
	Verification string
	Closeout     string
	URL          string
}

type ProgressUpdate struct {
	DemandID string
	Stage    string
	State    string
	Summary  string
	URL      string
	Marker   string
	DryRun   bool
}

type CloseoutUpdate struct {
	DemandID     string
	Result       string
	Verification string
	Knowledge    string
	URL          string
	Marker       string
	DryRun       bool
}

type IntakeAdapter interface {
	FetchIntake(ctx context.Context, ref IntakeRef) (IntakeSnapshot, error)
}

type DemandPoolAdapter interface {
	ListDemands(ctx context.Context, ref IntakeRef) ([]ExternalDemand, error)
	FetchDemand(ctx context.Context, ref IntakeRef) (ExternalDemand, error)
	UpdateDemand(ctx context.Context, ref IntakeRef, update DemandStatusUpdate) error
}

type SyncAdapter interface {
	PostProgress(ctx context.Context, ref IntakeRef, update ProgressUpdate) error
	PostCloseout(ctx context.Context, ref IntakeRef, update CloseoutUpdate) error
}
