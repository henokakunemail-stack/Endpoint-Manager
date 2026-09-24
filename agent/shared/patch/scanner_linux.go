//go:build linux

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

	// Check if apt-get is available
	if _, err := exec.LookPath("apt-get"); err == nil {
		return scanApt(ctxTimeout)
	}

	// Check if dnf is available
	if _, err := exec.LookPath("dnf"); err == nil {
		return scanDnf(ctxTimeout, "dnf")
	}

	// Check if yum is available
	if _, err := exec.LookPath("yum"); err == nil {
		return scanDnf(ctxTimeout, "yum")
	}

	return []PatchItem{}, nil
}

func scanApt(ctx context.Context) ([]PatchItem, error) {
	cmd := exec.CommandContext(ctx, "apt-get", "-s", "dist-upgrade")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var items []PatchItem
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Inst ") {
			// Format: Inst <pkg> [<old_version>] (<new_version> <suite> [<arch>])
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				pkg := parts[1]
				items = append(items, PatchItem{
					PatchID:        pkg,
					Title:          "Update for " + pkg,
					Description:    line,
					Severity:       SeverityImportant,
					Category:       CategorySecurity,
					InstalledState: StateMissing,
				})
			}
		}
	}
	return items, nil
}

func scanDnf(ctx context.Context, bin string) ([]PatchItem, error) {
	cmd := exec.CommandContext(ctx, bin, "check-update", "-q")
	out, _ := cmd.Output() // dnf check-update returns exit code 100 when updates are available

	var items []PatchItem
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Security:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			pkg := parts[0]
			items = append(items, PatchItem{
				PatchID:        pkg,
				Title:          "Update for " + pkg,
				Description:    "Available version: " + parts[1],
				Severity:       SeverityImportant,
				Category:       CategorySecurity,
				InstalledState: StateMissing,
			})
		}
	}
	return items, nil
}
