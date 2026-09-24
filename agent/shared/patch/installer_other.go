//go:build !windows && !linux && !darwin

package patch

import "context"

func installOS(ctx context.Context, params InstallParams) (InstallResult, error) {
	return InstallResult{
		JobID:        params.JobID,
		Status:       "failed",
		ErrorMessage: "unsupported operating system for patch installation",
	}, nil
}
