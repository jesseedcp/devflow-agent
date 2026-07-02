package feishu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jesseedcp/devflow-agent/internal/platform"
)

func TestBitableAdapterListsDemands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/bitable/v1/apps/app_token/tables/tbl/records/search":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": []map[string]any{{
				"record_id": "rec1",
				"fields": map[string]any{
					"需求标题": "Coupon",
					"需求描述": "Coupon eligibility",
					"状态":   "待澄清",
					"优先级":  "P1",
					"负责人":  "dd",
				},
			}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := BitableAdapter{Client: server.Client(), TokenClient: &TenantTokenClient{Client: server.Client(), BaseURL: server.URL, AppID: "cli_test", AppSecret: "secret"}, BaseURL: server.URL, Fields: DefaultBitableFields()}
	got, err := adapter.ListDemands(context.Background(), platform.IntakeRef{AppToken: "app_token", TableID: "tbl"})
	if err != nil {
		t.Fatalf("ListDemands returned error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "rec1" || got[0].Title != "Coupon" || got[0].Status != "待澄清" {
		t.Fatalf("demands = %#v", got)
	}
}

func TestBitableAdapterUpdatesDemandStatus(t *testing.T) {
	var patched string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/bitable/v1/apps/app_token/tables/tbl/records/rec1":
			body, _ := io.ReadAll(r.Body)
			patched = string(body)
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"record": map[string]any{"record_id": "rec1"}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := BitableAdapter{Client: server.Client(), TokenClient: &TenantTokenClient{Client: server.Client(), BaseURL: server.URL, AppID: "cli_test", AppSecret: "secret"}, BaseURL: server.URL, Fields: DefaultBitableFields()}
	err := adapter.UpdateDemand(context.Background(), platform.IntakeRef{AppToken: "app_token", TableID: "tbl", RecordID: "rec1"}, platform.DemandStatusUpdate{DemandID: "coupon", State: "verification", Summary: "PASS"})
	if err != nil {
		t.Fatalf("UpdateDemand returned error: %v", err)
	}
	for _, want := range []string{"Devflow Demand ID", "coupon", "Devflow 状态", "verification", "验收摘要", "PASS"} {
		if !strings.Contains(patched, want) {
			t.Fatalf("patched body missing %q: %s", want, patched)
		}
	}
}
