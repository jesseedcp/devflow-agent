package cli

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestServerRequiresDSN(t *testing.T) {
	old := os.Getenv("DEVFLOW_DATABASE_DSN")
	os.Unsetenv("DEVFLOW_DATABASE_DSN")
	defer os.Setenv("DEVFLOW_DATABASE_DSN", old)

	err := runServer([]string{}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected error when no dsn is configured")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "dsn") {
		t.Fatalf("expected a dsn error, got %v", err)
	}
}

func TestServerBadDSNReachesOpen(t *testing.T) {
	// A DSN pointing at a closed port must fail at Open, not at the "required"
	// check, proving --database-dsn overrides the env requirement.
	err := runServer([]string{
		"--database-dsn", "devflow:devflow@tcp(127.0.0.1:1)/devflow?parseTime=true",
		"--addr", "127.0.0.1:0",
	}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected open error for unreachable database")
	}
	if strings.Contains(strings.ToLower(err.Error()), "required") {
		t.Fatalf("should not be the required-dsn error, got %v", err)
	}
}