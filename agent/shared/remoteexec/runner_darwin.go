//go:build darwin

package remoteexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
)

type darwinRunner struct{}

func newPlatformRunner() CommandRunner {
	return &darwinRunner{}
}

func (r *darwinRunner) RunCommand(ctx context.Context, shell, command string) (int, string, error) {
	var cmd *exec.Cmd

	switch strings.ToLower(shell) {
	case "sh":
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	case "bash":
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", command)
	default: // default to zsh on macOS
		cmd = exec.CommandContext(ctx, "/bin/zsh", "-c", command)
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
