//go:build darwin

package remoteexec

import (
	"context"
	"os/exec"
	"strings"
)

func createShellCmd(ctx context.Context, shell string) *exec.Cmd {
	switch strings.ToLower(shell) {
	case "sh":
		return exec.CommandContext(ctx, "/bin/sh", "-i")
	case "bash":
		return exec.CommandContext(ctx, "/bin/bash", "-i")
	default:
		return exec.CommandContext(ctx, "/bin/zsh", "-i")
	}
}
