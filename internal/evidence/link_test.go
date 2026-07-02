package evidence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLinkFetcherPassesWhenHEADAccessible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := LinkFetcher{Client: server.Client()}.Fetch(context.Background(), LinkFetchRequest{URL: server.URL, ExpectStatus: http.StatusOK, Timeout: time.Second})
	if result.Status != "pass" {
		t.Fatalf("status = %s summary=%s", result.Status, result.Summary)
	}
}

func TestLinkFetcherFallsBackToGETWhenHEADNotAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	result := LinkFetcher{Client: server.Client()}.Fetch(context.Background(), LinkFetchRequest{URL: server.URL, ExpectStatus: http.StatusOK, Timeout: time.Second})
	if result.Status != "pass" {
		t.Fatalf("status = %s summary=%s", result.Status, result.Summary)
	}
	if result.Method != http.MethodGet {
		t.Fatalf("method = %s, want GET fallback", result.Method)
	}
}
