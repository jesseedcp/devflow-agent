//go:build windows

package hooks

import (
	"context"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testSleepCommand(d time.Duration) string {
	return "Start-Sleep -Milliseconds " + strconv.FormatInt(d.Milliseconds(), 10)
}

func testExitCommand(code int) string {
	return "exit " + strconv.Itoa(code)
}

func testEchoCommand(value string) string {
	return "Write-Output '" + value + "'"
}

func testEnvEchoCommand() string {
	return `Write-Output "$env:DEVFLOW_EVENT|$env:MEWCODE_EVENT|$env:DEVFLOW_TOOL|$env:MEWCODE_TOOL|$env:DEVFLOW_FILE_PATH|$env:MEWCODE_FILE_PATH"`
}

func TestShellCommandContextUsesPowerShell(t *testing.T) {
	cmd := shellCommandContext(context.Background(), "Write-Output ok")
	if got := strings.ToLower(filepath.Base(cmd.Path)); got != "powershell.exe" {
		t.Fatalf("expected powershell.exe executable, got %q", cmd.Path)
	}
	wantArgs := []string{"powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "Write-Output ok"}
	if !reflect.DeepEqual(cmd.Args, wantArgs) {
		t.Fatalf("expected args %v, got %v", wantArgs, cmd.Args)
	}
}
