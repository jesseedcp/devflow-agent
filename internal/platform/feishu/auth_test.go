package feishu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTenantTokenClientFetchesAndCachesToken(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/open-apis/auth/v3/tenant_access_token/internal" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"code":                0,
			"tenant_access_token": "tenant-token",
			"expire":              7200,
		})
	}))
	defer server.Close()

	client := TenantTokenClient{Client: server.Client(), BaseURL: server.URL, AppID: "cli_test", AppSecret: "secret"}
	first, err := client.Token(context.Background())
	if err != nil {
		t.Fatalf("Token returned error: %v", err)
	}
	second, err := client.Token(context.Background())
	if err != nil {
		t.Fatalf("Token returned error: %v", err)
	}
	if first != "tenant-token" || second != "tenant-token" {
		t.Fatalf("tokens = %q %q", first, second)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestTenantTokenClientRequiresCredentials(t *testing.T) {
	_, err := (&TenantTokenClient{}).Token(context.Background())
	if err == nil || err.Error() != "feishu tenant token requires app id and app secret" {
		t.Fatalf("err = %v", err)
	}
}
