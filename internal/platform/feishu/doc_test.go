package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/platform"
)

func TestDocAdapterFetchesDocBlocksAsMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/docx/v1/documents/doc_token":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"document": map[string]any{"title": "Coupon PRD"}}})
		case "/open-apis/docx/v1/documents/doc_token/blocks/doc_token/children":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": []map[string]any{
				{"block_id": "b1", "block_type": 3, "heading1": map[string]any{"elements": []map[string]any{{"text_run": map[string]any{"content": "Goals"}}}}},
				{"block_id": "b2", "block_type": 2, "text": map[string]any{"elements": []map[string]any{{"text_run": map[string]any{"content": "Active users can claim coupons."}}}}},
				{"block_id": "b3", "block_type": 12, "bullet": map[string]any{"elements": []map[string]any{{"text_run": map[string]any{"content": "Inactive users are blocked."}}}}},
			}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := DocAdapter{Client: server.Client(), TokenClient: &TenantTokenClient{Client: server.Client(), BaseURL: server.URL, AppID: "cli_test", AppSecret: "secret"}, BaseURL: server.URL}
	got, err := adapter.FetchIntake(context.Background(), platform.IntakeRef{Kind: platform.SourceFeishuDoc, Token: "doc_token", URL: "https://example.feishu.cn/docx/doc_token"})
	if err != nil {
		t.Fatalf("FetchIntake returned error: %v", err)
	}
	if got.Title != "Coupon PRD" {
		t.Fatalf("Title = %q", got.Title)
	}
	for _, want := range []string{"# Goals", "Active users can claim coupons.", "- Inactive users are blocked."} {
		if !strings.Contains(got.Body, want) {
			t.Fatalf("body missing %q:\n%s", want, got.Body)
		}
	}
}
