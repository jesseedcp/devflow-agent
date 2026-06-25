//go:build !windows

package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testSleepCommand(d time.Duration) string {
	return fmt.Sprintf("sleep %.3f", d.Seconds())
}

func testExitCommand(code int) string {
	return "exit " + strconv.Itoa(code)
}

func testEchoCommand(value string) string {
	return "printf '%s\\n' '" + value + "'"
}

func testEnvEchoCommand() string {
	return `printf '%s|%s|%s|%s|%s|%s' "$DEVFLOW_EVENT" "$MEWCODE_EVENT" "$DEVFLOW_TOOL" "$MEWCODE_TOOL" "$DEVFLOW_FILE_PATH" "$MEWCODE_FILE_PATH"`
}

func TestShellCommandContextUsesPreferredShell(t *testing.T) {
	cmd := shellCommandContext(context.Background(), "printf ok")
	expectedShell := "sh"
	if _, err := exec.LookPath("bash"); err == nil {
		expectedShell = "bash"
	} else if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("unexpected bash lookup error: %v", err)
	}
	if got := filepath.Base(cmd.Path); got != expectedShell {
		t.Fatalf("expected %s executable, got %q", expectedShell, cmd.Path)
	}
	wantArgs := []string{expectedShell, "-c", "printf ok"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("expected args %v, got %v", wantArgs, cmd.Args)
	}
}

func TestShellCommandContextRunsBashSpecificSyntaxWhenAvailable(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			t.Skip("bash not installed")
		}
		t.Fatalf("unexpected bash lookup error: %v", err)
	}

	cmd := shellCommandContext(context.Background(), `if [[ "ok" == "ok" ]]; then printf ok; fi`)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected bash-specific command to succeed, err=%v stderr=%q", err, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "ok" {
		t.Fatalf("expected bash-specific output %q, got %q", "ok", stdout.String())
	}
}
