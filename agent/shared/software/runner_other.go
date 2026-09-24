//go:build !windows && !linux && !darwin

package software

import (
	"context"
	"errors"
)

type otherRunner struct{}

func DefaultRunner() Runner {
	return &otherRunner{}
}

func (r *otherRunner) Run(ctx context.Context, filePath, packageType, installArgs string) (int, string, error) {
	return -1, "", errors.New("unsupported operating system for software runner")
}
