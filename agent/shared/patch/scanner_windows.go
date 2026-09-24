//go:build windows

package patch

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const winScanScript = `
$ErrorActionPreference = 'SilentlyContinue'
try {
    $session = New-Object -ComObject Microsoft.Update.Session
    $searcher = $session.CreateUpdateSearcher()
    $searcher.ServerSelection = 2
    $result = $searcher.Search("IsInstalled=0 and Type='Software'")
    $list = @()
    if ($result -and $result.Updates) {
        foreach ($u in $result.Updates) {
            $kb = ""
            if ($u.KBArticleIDs -and $u.KBArticleIDs.Count -gt 0) {
                $kb = "KB" + $u.KBArticleIDs[0]
            }
            $sev = "unspecified"
            switch ($u.MsrcSeverity) {
                "Critical"  { $sev = "critical" }
                "Important" { $sev = "important" }
                "Moderate"  { $sev = "moderate" }
                "Low"        { $sev = "low" }
            }
            $cat = "security"
            if ($u.Categories -and $u.Categories.Count -gt 0) {
                $catName = $u.Categories[0].Name
                if ($catName -match "Security") { $cat = "security" }
                elseif ($catName -match "Critical") { $cat = "critical" }
                elseif ($catName -match "Definition") { $cat = "definition" }
                else { $cat = "updates" }
            }
            $pid = if ($kb -ne "") { $kb } else { $u.Identity.UpdateID }
            $list += [PSCustomObject]@{
                patch_id        = [string]$pid
                title           = [string]$u.Title
                description     = [string]$u.Description
                severity        = [string]$sev
                category        = [string]$cat
                kb_id           = [string]$kb
                size_bytes      = 0
                installed_state = "missing"
                reboot_required = [bool]$u.RebootRequired
            }
        }
    }
    if ($list.Count -eq 0) {
        Write-Output "[]"
    } else {
        $list | ConvertTo-Json -Compress
    }
} catch {
    Write-Output "[]"
}
`

func scanOS(ctx context.Context) ([]PatchItem, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctxTimeout, "powershell.exe",
		"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-Command", winScanScript)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("powershell update scan: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" || trimmed == "[]" {
		return []PatchItem{}, nil
	}

	// Output might be single object or array
	if strings.HasPrefix(trimmed, "{") {
		var single PatchItem
		if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
			return nil, fmt.Errorf("unmarshal single patch: %w", err)
		}
		return []PatchItem{single}, nil
	}

	var items []PatchItem
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, fmt.Errorf("unmarshal patches list: %w", err)
	}
	return items, nil
}
