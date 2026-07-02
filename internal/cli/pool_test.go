package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPoolListFeishuBitable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "tenant_access_token": "tenant-token", "expire": 7200})
		case "/open-apis/bitable/v1/apps/app_token/tables/tbl/records/search":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"items": []map[string]any{{"record_id": "rec1", "fields": map[string]any{"需求标题": "Coupon", "状态": "待澄清"}}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := Run([]string{"pool", "list", "--feishu-bitable", "app_token", "--table", "tbl", "--feishu-base-url", server.URL, "--feishu-app-id", "cli_test", "--feishu-app-secret", "secret"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("pool list returned error: %v\n%s", err, stdout.String())
	}
	for _, want := range []string{"record-id", "status", "title", "rec1", "待澄清", "Coupon"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}
