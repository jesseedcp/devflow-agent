package evidence

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type LinkFetchRequest struct {
	URL          string
	ExpectStatus int
	Timeout      time.Duration
}

type LinkFetcher struct {
	Client *http.Client
}

func (f LinkFetcher) Fetch(ctx context.Context, input LinkFetchRequest) FetchResult {
	result, retryWithGet := f.fetchWithMethod(ctx, http.MethodHead, input)
	if retryWithGet {
		result, _ = f.fetchWithMethod(ctx, http.MethodGet, input)
	}
	return result
}

func (f LinkFetcher) fetchWithMethod(ctx context.Context, method string, input LinkFetchRequest) (FetchResult, bool) {
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	targetURL := strings.TrimSpace(input.URL)
	req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
	if err != nil {
		return FetchResult{Status: "blocked", URL: Redact(targetURL), Method: method, Summary: "build link request failed: " + Redact(err.Error())}, false
	}
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return FetchResult{Status: "blocked", URL: Redact(targetURL), Method: method, Summary: "link request failed: " + Redact(err.Error())}, false
	}
	defer resp.Body.Close()
	if method == http.MethodHead && resp.StatusCode == http.StatusMethodNotAllowed {
		return FetchResult{}, true
	}
	expected := input.ExpectStatus
	if expected == 0 {
		expected = http.StatusOK
	}
	status := "pass"
	if resp.StatusCode != expected {
		status = "fail"
	}
	return FetchResult{Status: status, URL: Redact(targetURL), Method: method, ActualStatus: resp.StatusCode, Summary: fmt.Sprintf("%s %s returned %d expected_status=%d", method, Redact(targetURL), resp.StatusCode, expected)}, false
}
