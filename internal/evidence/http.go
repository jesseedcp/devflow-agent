package evidence

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPFetchRequest struct {
	Method         string
	URL            string
	Headers        []string
	Body           string
	ExpectStatus   int
	ExpectContains string
	Timeout        time.Duration
}

type HTTPFetcher struct {
	Client *http.Client
}

func (f HTTPFetcher) Fetch(ctx context.Context, input HTTPFetchRequest) FetchResult {
	method := strings.ToUpper(strings.TrimSpace(input.Method))
	if method == "" {
		method = http.MethodGet
	}
	targetURL := strings.TrimSpace(input.URL)
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, targetURL, strings.NewReader(input.Body))
	if err != nil {
		return FetchResult{Status: "blocked", URL: Redact(targetURL), Method: method, RequestExcerpt: Excerpt(input.Body, 2048), Summary: "build request failed: " + Redact(err.Error())}
	}
	for _, header := range input.Headers {
		name, value, ok := strings.Cut(header, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		req.Header.Set(name, strings.TrimSpace(value))
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{Status: "blocked", URL: Redact(targetURL), Method: method, RequestExcerpt: Excerpt(input.Body, 2048), Summary: "request failed: " + Redact(err.Error())}
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	responseText := string(body)
	if readErr != nil {
		return FetchResult{Status: "blocked", URL: Redact(targetURL), Method: method, ActualStatus: resp.StatusCode, RequestExcerpt: Excerpt(input.Body, 2048), ResponseExcerpt: Excerpt(responseText, 4096), Summary: "read response failed: " + Redact(readErr.Error())}
	}
	expected := input.ExpectStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	status := "pass"
	if resp.StatusCode != expected {
		status = "fail"
	}
	if input.ExpectContains != "" && !strings.Contains(responseText, input.ExpectContains) {
		status = "fail"
	}
	summary := fmt.Sprintf("%s %s returned %d expected_status=%d", method, Redact(targetURL), resp.StatusCode, expected)
	if input.ExpectContains != "" {
		summary += fmt.Sprintf(" expect_contains=%q", Redact(input.ExpectContains))
	}
	return FetchResult{Status: status, Summary: summary, URL: Redact(targetURL), Method: method, ActualStatus: resp.StatusCode, RequestExcerpt: Excerpt(input.Body, 2048), ResponseExcerpt: Excerpt(responseText, 4096)}
}
