//go:build !windows && !linux && !darwin

package patch

import "context"

func scanOS(ctx context.Context) ([]PatchItem, error) {
	return []PatchItem{}, nil
}
