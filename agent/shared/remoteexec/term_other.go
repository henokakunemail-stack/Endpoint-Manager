//go:build !windows && !linux && !darwin

package remoteexec

import (
	"context"
	"os/exec"
)

func createShellCmd(ctx context.Context, shell string) *exec.Cmd {
	return exec.CommandContext(ctx, "/bin/sh", "-i")
}
