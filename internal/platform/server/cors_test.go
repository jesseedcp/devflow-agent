package server

import (
	"net/http"
	"testing"
)

func TestCORSPreflightReturnsNoContent(t *testing.T) {
	ts := newTestServer(t, newFakeStore())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/api/workspaces", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight expected 204, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("expected reflected origin, got %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("expected Access-Control-Allow-Headers to be set")
	}
}

func TestCORSHeadersOnGet(t *testing.T) {
	ts := newTestServer(t, newFakeStore())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:5173")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected reflected origin, got %q", got)
	}
}