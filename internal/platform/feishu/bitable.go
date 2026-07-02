package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jesseedcp/devflow-agent/internal/platform"
)

type BitableFields struct {
	Title           string
	Description     string
	Status          string
	Priority        string
	Owner           string
	DevflowDemandID string
	DevflowState    string
	Verification    string
	Closeout        string
}

func DefaultBitableFields() BitableFields {
	return BitableFields{
		Title:           "需求标题",
		Description:     "需求描述",
		Status:          "状态",
		Priority:        "优先级",
		Owner:           "负责人",
		DevflowDemandID: "Devflow Demand ID",
		DevflowState:    "Devflow 状态",
		Verification:    "验收摘要",
		Closeout:        "交付总结",
	}
}

type BitableAdapter struct {
	Client      *http.Client
	TokenClient *TenantTokenClient
	BaseURL     string
	Fields      BitableFields
}

type bitableRecordsResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Items     []bitableRecord `json:"items"`
		HasMore   bool            `json:"has_more"`
		PageToken string          `json:"page_token"`
	} `json:"data"`
}

type bitableRecord struct {
	RecordID string         `json:"record_id"`
	Fields   map[string]any `json:"fields"`
}

func (a BitableAdapter) ListDemands(ctx context.Context, ref platform.IntakeRef) ([]platform.ExternalDemand, error) {
	records, err := a.listRecords(ctx, ref)
	if err != nil {
		return nil, err
	}
	fields := a.fields()
	out := make([]platform.ExternalDemand, 0, len(records))
	for _, record := range records {
		out = append(out, platform.ExternalDemand{
			ID:          record.RecordID,
			Title:       fieldString(record.Fields, fields.Title),
			Description: fieldString(record.Fields, fields.Description),
			Status:      fieldString(record.Fields, fields.Status),
			Priority:    fieldString(record.Fields, fields.Priority),
			Owner:       fieldString(record.Fields, fields.Owner),
			Metadata:    map[string]string{"record_id": record.RecordID},
		})
	}
	return out, nil
}

func (a BitableAdapter) FetchDemand(ctx context.Context, ref platform.IntakeRef) (platform.ExternalDemand, error) {
	demands, err := a.ListDemands(ctx, ref)
	if err != nil {
		return platform.ExternalDemand{}, err
	}
	for _, demand := range demands {
		if demand.ID == ref.RecordID {
			return demand, nil
		}
	}
	return platform.ExternalDemand{}, fmt.Errorf("feishu bitable record %s not found", ref.RecordID)
}

func (a BitableAdapter) UpdateDemand(ctx context.Context, ref platform.IntakeRef, update platform.DemandStatusUpdate) error {
	if strings.TrimSpace(ref.AppToken) == "" || strings.TrimSpace(ref.TableID) == "" || strings.TrimSpace(ref.RecordID) == "" {
		return fmt.Errorf("feishu bitable update requires app token, table id, and record id")
	}
	token, err := a.tenantToken(ctx)
	if err != nil {
		return err
	}
	fields := a.fields()
	payloadFields := map[string]any{
		fields.DevflowDemandID: update.DemandID,
		fields.DevflowState:    update.State,
	}
	if update.Verification != "" || update.Summary != "" {
		payloadFields[fields.Verification] = firstNonEmpty(update.Verification, update.Summary)
	}
	if update.Closeout != "" {
		payloadFields[fields.Closeout] = update.Closeout
	}
	payload, err := json.Marshal(map[string]any{"fields": payloadFields})
	if err != nil {
		return fmt.Errorf("encode feishu bitable update: %w", err)
	}
	endpoint := a.normalizedBaseURL() + "/open-apis/bitable/v1/apps/" + url.PathEscape(ref.AppToken) + "/tables/" + url.PathEscape(ref.TableID) + "/records/" + url.PathEscape(ref.RecordID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build feishu bitable update request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("send feishu bitable update request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu bitable update returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (a BitableAdapter) listRecords(ctx context.Context, ref platform.IntakeRef) ([]bitableRecord, error) {
	if strings.TrimSpace(ref.AppToken) == "" || strings.TrimSpace(ref.TableID) == "" {
		return nil, fmt.Errorf("feishu bitable list requires app token and table id")
	}
	token, err := a.tenantToken(ctx)
	if err != nil {
		return nil, err
	}
	var records []bitableRecord
	pageToken := ""
	for {
		parsed, err := a.searchRecords(ctx, ref, token, pageToken)
		if err != nil {
			return nil, err
		}
		if parsed.Code != 0 {
			return nil, fmt.Errorf("feishu bitable list error %d: %s", parsed.Code, parsed.Msg)
		}
		records = append(records, parsed.Data.Items...)
		if !parsed.Data.HasMore || strings.TrimSpace(parsed.Data.PageToken) == "" {
			break
		}
		pageToken = parsed.Data.PageToken
	}
	return records, nil
}

func (a BitableAdapter) searchRecords(ctx context.Context, ref platform.IntakeRef, token, pageToken string) (bitableRecordsResponse, error) {
	endpoint := a.normalizedBaseURL() + "/open-apis/bitable/v1/apps/" + url.PathEscape(ref.AppToken) + "/tables/" + url.PathEscape(ref.TableID) + "/records/search"
	payloadBody := map[string]any{"page_size": 500}
	if strings.TrimSpace(pageToken) != "" {
		payloadBody["page_token"] = pageToken
	}
	payload, err := json.Marshal(payloadBody)
	if err != nil {
		return bitableRecordsResponse{}, fmt.Errorf("encode feishu bitable search request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return bitableRecordsResponse{}, fmt.Errorf("build feishu bitable search request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	var parsed bitableRecordsResponse
	if err := a.doJSON(req, &parsed); err != nil {
		return bitableRecordsResponse{}, err
	}
	return parsed, nil
}

func (a BitableAdapter) tenantToken(ctx context.Context) (string, error) {
	client := a.TokenClient
	if client == nil {
		client = &TenantTokenClient{Client: a.Client, BaseURL: a.BaseURL}
	}
	return client.Token(ctx)
}

func (a BitableAdapter) normalizedBaseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(a.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return baseURL
}

func (a BitableAdapter) httpClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return http.DefaultClient
}

func (a BitableAdapter) fields() BitableFields {
	fields := a.Fields
	defaults := DefaultBitableFields()
	if fields.Title == "" {
		fields.Title = defaults.Title
	}
	if fields.Description == "" {
		fields.Description = defaults.Description
	}
	if fields.Status == "" {
		fields.Status = defaults.Status
	}
	if fields.Priority == "" {
		fields.Priority = defaults.Priority
	}
	if fields.Owner == "" {
		fields.Owner = defaults.Owner
	}
	if fields.DevflowDemandID == "" {
		fields.DevflowDemandID = defaults.DevflowDemandID
	}
	if fields.DevflowState == "" {
		fields.DevflowState = defaults.DevflowState
	}
	if fields.Verification == "" {
		fields.Verification = defaults.Verification
	}
	if fields.Closeout == "" {
		fields.Closeout = defaults.Closeout
	}
	return fields
}

func (a BitableAdapter) getJSON(ctx context.Context, endpoint, token string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build feishu bitable request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return a.doJSON(req, target)
}

func (a BitableAdapter) doJSON(req *http.Request, target any) error {
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("send feishu bitable request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("feishu bitable returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode feishu bitable response: %w", err)
	}
	return nil
}

func fieldString(fields map[string]any, name string) string {
	value, ok := fields[name]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(value)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
