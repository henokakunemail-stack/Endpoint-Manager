//go:build windows

package remoteexec

import (
	"context"
	"os/exec"
	"strings"
)

func createShellCmd(ctx context.Context, shell string) *exec.Cmd {
	switch strings.ToLower(shell) {
	case "cmd":
		return exec.CommandContext(ctx, "cmd.exe")
	default:
		return exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "-")
	}
}
