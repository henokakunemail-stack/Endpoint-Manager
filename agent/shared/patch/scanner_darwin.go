//go:build darwin

package patch

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"time"
)

func scanOS(ctx context.Context) ([]PatchItem, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "softwareupdate", "-l")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var items []PatchItem
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// macOS softwareupdate format:
		// * Label: macOS Ventura 13.5-22G74
		//   Title: macOS Ventura 13.5, Version: 13.5, Size: 123456K, Recommended: YES, Action: restart
		if strings.HasPrefix(line, "* Label: ") || strings.HasPrefix(line, "* ") {
			label := strings.TrimPrefix(line, "* Label: ")
			label = strings.TrimPrefix(label, "* ")
			items = append(items, PatchItem{
				PatchID:        label,
				Title:          label,
				Severity:       SeverityImportant,
				Category:       CategorySecurity,
				InstalledState: StateMissing,
			})
		}
	}
	return items, nil
}
