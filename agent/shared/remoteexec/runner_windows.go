//go:build windows

package remoteexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

type windowsRunner struct{}

func newPlatformRunner() CommandRunner {
	return &windowsRunner{}
}

func (r *windowsRunner) RunCommand(ctx context.Context, shell, command string) (int, string, error) {
	var cmd *exec.Cmd

	switch strings.ToLower(shell) {
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", command)
	default: // default to powershell
		cmd = exec.CommandContext(ctx, "powershell.exe",
			"-NoProfile",
			"-NonInteractive",
			"-ExecutionPolicy", "Bypass",
			"-Command", command,
		)
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	output := buf.String()

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return exitCode, output, err
}
