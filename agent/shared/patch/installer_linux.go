//go:build linux

package patch

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

func installOS(ctx context.Context, params InstallParams) (InstallResult, error) {
	result := InstallResult{
		JobID:  params.JobID,
		Status: "failed",
	}

	if len(params.PatchIDs) == 0 {
		result.ErrorMessage = "no patches specified"
		return result, nil
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	var cmd *exec.Cmd
	if _, err := exec.LookPath("apt-get"); err == nil {
		args := append([]string{"install", "-y", "--only-upgrade"}, params.PatchIDs...)
		cmd = exec.CommandContext(ctxTimeout, "apt-get", args...)
		cmd.Env = append(cmd.Environ(), "DEBIAN_FRONTEND=noninteractive")
	} else if _, err := exec.LookPath("dnf"); err == nil {
		args := append([]string{"update", "-y"}, params.PatchIDs...)
		cmd = exec.CommandContext(ctxTimeout, "dnf", args...)
	} else if _, err := exec.LookPath("yum"); err == nil {
		args := append([]string{"update", "-y"}, params.PatchIDs...)
		cmd = exec.CommandContext(ctxTimeout, "yum", args...)
	} else {
		result.ErrorMessage = "no supported package manager found (apt-get/dnf/yum)"
		return result, nil
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.OutputLog = stdout.String()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("install failed: %v, stderr: %s", err, stderr.String())
		result.Status = "failed"
	} else {
		result.Status = "completed"
	}

	return result, nil
}
