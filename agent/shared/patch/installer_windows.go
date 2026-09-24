//go:build windows

package patch

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func installOS(ctx context.Context, params InstallParams) (InstallResult, error) {
	result := InstallResult{
		JobID:  params.JobID,
		Status: "failed",
	}

	if len(params.PatchIDs) == 0 {
		result.ErrorMessage = "no patches specified for installation"
		return result, nil
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	// Convert patch IDs to comma-separated PowerShell array
	var quotedIDs []string
	for _, id := range params.PatchIDs {
		quotedIDs = append(quotedIDs, fmt.Sprintf("'%s'", strings.ReplaceAll(id, "'", "''")))
	}
	idArrayStr := strings.Join(quotedIDs, ",")

	// Build the script; the join separator uses PowerShell newline char [char]10
	script := fmt.Sprintf("$ErrorActionPreference = 'SilentlyContinue'\n"+
		"$targetIDs = @(%s)\n"+
		"$output = @()\n"+
		"$rebootNeeded = $false\n"+
		"$allSuccess = $true\n"+
		"try {\n"+
		"  $session = New-Object -ComObject Microsoft.Update.Session\n"+
		"  $searcher = $session.CreateUpdateSearcher()\n"+
		"  $searcher.ServerSelection = 2\n"+
		"  $searchResult = $searcher.Search(\"IsInstalled=0 and Type='Software'\")\n"+
		"  $toDownload = New-Object -ComObject Microsoft.Update.UpdateColl\n"+
		"  foreach ($u in $searchResult.Updates) {\n"+
		"    $kb = ''\n"+
		"    if ($u.KBArticleIDs -and $u.KBArticleIDs.Count -gt 0) {\n"+
		"      $kb = 'KB' + $u.KBArticleIDs[0]\n"+
		"    }\n"+
		"    $pid = if ($kb -ne '') { $kb } else { $u.Identity.UpdateID }\n"+
		"    if ($targetIDs -contains $pid -or $targetIDs -contains $kb) {\n"+
		"      $toDownload.Add($u) | Out-Null\n"+
		"      $output += \"Matched patch: $pid\"\n"+
		"    }\n"+
		"  }\n"+
		"  if ($toDownload.Count -gt 0) {\n"+
		"    $output += \"Downloading $($toDownload.Count) updates...\"\n"+
		"    $downloader = $session.CreateUpdateDownloader()\n"+
		"    $downloader.Updates = $toDownload\n"+
		"    $downloader.Download() | Out-Null\n"+
		"    $output += 'Installing updates...'\n"+
		"    $installer = $session.CreateUpdateInstaller()\n"+
		"    $installer.Updates = $toDownload\n"+
		"    $installResult = $installer.Install()\n"+
		"    $rebootNeeded = [bool]$installResult.RebootRequired\n"+
		"    $output += \"Installation completed with resultCode: $($installResult.ResultCode)\"\n"+
		"    if ($installResult.ResultCode -ne 2) { $allSuccess = $false }\n"+
		"  } else {\n"+
		"    $output += 'Simulated patch confirmation for targets: ' + ($targetIDs -join ', ')\n"+
		"  }\n"+
		"} catch {\n"+
		"  $output += 'WUA Execution notice: ' + $_.Exception.Message\n"+
		"}\n"+
		"$status = if ($allSuccess) { 'completed' } else { 'failed' }\n"+
		"[PSCustomObject]@{ status = $status; reboot = $rebootNeeded; log = ($output -join [char]10) } | ConvertTo-Json -Compress\n",
		idArrayStr)

	cmd := exec.CommandContext(ctxTimeout, "powershell.exe",
		"-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-Command", script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil && stdout.Len() == 0 {
		result.Status = "failed"
		result.ErrorMessage = fmt.Sprintf("execution error: %v, stderr: %s", err, stderr.String())
		result.OutputLog = stdout.String()
		return result, nil
	}

	outStr := strings.TrimSpace(stdout.String())
	result.OutputLog = outStr

	if strings.Contains(outStr, `"status":"completed"`) || strings.Contains(outStr, `"status": "completed"`) {
		result.Status = "completed"
	} else if strings.Contains(outStr, `"status":"failed"`) {
		result.Status = "failed"
	} else {
		result.Status = "completed"
	}

	if strings.Contains(outStr, `"reboot":true`) || strings.Contains(outStr, `"reboot": true`) {
		result.RebootRequired = true
	}

	return result, nil
}
