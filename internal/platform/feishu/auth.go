package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://open.feishu.cn"

type TenantTokenClient struct {
	Client    *http.Client
	BaseURL   string
	AppID     string
	AppSecret string

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type tenantTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

func (c *TenantTokenClient) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" && time.Now().Before(c.expiresAt) {
		token := c.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	appID := strings.TrimSpace(c.AppID)
	if appID == "" {
		appID = strings.TrimSpace(os.Getenv("FEISHU_APP_ID"))
	}
	appSecret := strings.TrimSpace(c.AppSecret)
	if appSecret == "" {
		appSecret = strings.TrimSpace(os.Getenv("FEISHU_APP_SECRET"))
	}
	if appID == "" || appSecret == "" {
		return "", fmt.Errorf("feishu tenant token requires app id and app secret")
	}

	payload, err := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	if err != nil {
		return "", fmt.Errorf("encode feishu tenant token request: %w", err)
	}
	endpoint := strings.TrimRight(c.BaseURL, "/")
	if endpoint == "" {
		endpoint = defaultBaseURL
	}
	endpoint += "/open-apis/auth/v3/tenant_access_token/internal"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build feishu tenant token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send feishu tenant token request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("feishu tenant token returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed tenantTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode feishu tenant token response: %w", err)
	}
	if parsed.Code != 0 {
		return "", fmt.Errorf("feishu tenant token error %d: %s", parsed.Code, parsed.Msg)
	}
	if parsed.TenantAccessToken == "" {
		return "", fmt.Errorf("feishu tenant token response missing token")
	}

	c.mu.Lock()
	c.token = parsed.TenantAccessToken
	ttl := time.Duration(parsed.Expire-300) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	c.expiresAt = time.Now().Add(ttl)
	c.mu.Unlock()
	return parsed.TenantAccessToken, nil
}
