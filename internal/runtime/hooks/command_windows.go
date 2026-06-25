//go:build windows

package hooks

import (
	"context"
	"os/exec"
)

func shellCommandContext(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command)
}
