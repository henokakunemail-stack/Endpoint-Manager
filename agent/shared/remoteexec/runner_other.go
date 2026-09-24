//go:build !windows && !linux && !darwin

package remoteexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

type fallbackRunner struct{}

func newPlatformRunner() CommandRunner {
	return &fallbackRunner{}
}

func (r *fallbackRunner) RunCommand(ctx context.Context, shell, command string) (int, string, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)

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
