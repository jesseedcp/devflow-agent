//go:build !windows

package hooks

import (
	"context"
	"os/exec"
)

func shellCommandContext(ctx context.Context, command string) *exec.Cmd {
	if _, err := exec.LookPath("bash"); err == nil {
		return exec.CommandContext(ctx, "bash", "-c", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}
