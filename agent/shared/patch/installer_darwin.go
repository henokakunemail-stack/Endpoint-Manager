//go:build darwin

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

	ctxTimeout, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	args := []string{"-i", "--no-scan"}
	args = append(args, params.PatchIDs...)

	cmd := exec.CommandContext(ctxTimeout, "softwareupdate", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result.OutputLog = stdout.String()
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("softwareupdate failed: %v, stderr: %s", err, stderr.String())
		result.Status = "failed"
	} else {
		result.Status = "completed"
	}

	return result, nil
}
