//go:build windows

package software

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

type windowsRunner struct{}

func DefaultRunner() Runner {
	return &windowsRunner{}
}

func (r *windowsRunner) Run(ctx context.Context, filePath, packageType, installArgs string) (int, string, error) {
	var cmd *exec.Cmd

	switch strings.ToLower(packageType) {
	case "msi":
		args := []string{"/i", filePath}
		if installArgs == "" {
			args = append(args, "/qn", "/norestart")
		} else {
			args = append(args, strings.Fields(installArgs)...)
		}
		cmd = exec.CommandContext(ctx, "msiexec.exe", args...)

	case "exe":
		if installArgs != "" {
			args := append([]string{filePath}, strings.Fields(installArgs)...)
			cmd = exec.CommandContext(ctx, filePath, strings.Fields(installArgs)...)
			_ = args
		} else {
			cmd = exec.CommandContext(ctx, filePath)
		}

	case "script", "ps1":
		args := []string{"-ExecutionPolicy", "Bypass", "-NoProfile", "-NonInteractive", "-File", filePath}
		if installArgs != "" {
			args = append(args, strings.Fields(installArgs)...)
		}
		cmd = exec.CommandContext(ctx, "powershell.exe", args...)

	case "bat", "cmd":
		args := []string{"/c", filePath}
		if installArgs != "" {
			args = append(args, strings.Fields(installArgs)...)
		}
		cmd = exec.CommandContext(ctx, "cmd.exe", args...)

	default:
		return -1, "", fmt.Errorf("unsupported windows package type: %s", packageType)
	}

	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)

	if err == nil {
		return 0, outStr, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus(), outStr, err
		}
		return exitErr.ExitCode(), outStr, err
	}

	return -1, outStr, err
}
