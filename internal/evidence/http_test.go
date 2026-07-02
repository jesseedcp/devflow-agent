package evidence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPFetcherPassesOnExpectedStatusAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("Authorization = %q", got)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":"COUPON_USER_INACTIVE"}`))
	}))
	defer server.Close()

	result := HTTPFetcher{Client: server.Client()}.Fetch(context.Background(), HTTPFetchRequest{
		Method:         http.MethodPost,
		URL:            server.URL + "?token=abc",
		Headers:        []string{"Authorization: Bearer secret-token"},
		Body:           `{"password":"pw"}`,
		ExpectStatus:   http.StatusForbidden,
		ExpectContains: "COUPON_USER_INACTIVE",
		Timeout:        time.Second,
	})
	if result.Status != "pass" {
		t.Fatalf("status = %s summary=%s", result.Status, result.Summary)
	}
	for _, leaked := range []string{"secret-token", "token=abc", `"password":"pw"`} {
		if strings.Contains(result.Summary+result.URL+result.RequestExcerpt, leaked) {
			t.Fatalf("result leaked %q: %#v", leaked, result)
		}
	}
}

func TestHTTPFetcherFailsOnUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	result := HTTPFetcher{Client: server.Client()}.Fetch(context.Background(), HTTPFetchRequest{URL: server.URL, ExpectStatus: http.StatusForbidden, Timeout: time.Second})
	if result.Status != "fail" {
		t.Fatalf("status = %s summary=%s", result.Status, result.Summary)
	}
}

func TestHTTPFetcherBlocksOnRequestError(t *testing.T) {
	result := HTTPFetcher{}.Fetch(context.Background(), HTTPFetchRequest{URL: "http://127.0.0.1:1", Timeout: 10 * time.Millisecond})
	if result.Status != "blocked" {
		t.Fatalf("status = %s summary=%s", result.Status, result.Summary)
	}
}
