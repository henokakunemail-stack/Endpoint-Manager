#!/usr/bin/env bash
# ==============================================================================
# Native macOS GUI Uninstaller for Endpoint Management Agent
# Uses AppleScript (osascript) dialogs and Administrator elevation
# ==============================================================================

osascript <<'APPLESCRIPT'
try
    set dialogResult to display dialog "Are you sure you want to completely uninstall Endpoint Management Agent?" & return & return & "This will stop the background launchd daemon, remove the agent binary, and delete all local credentials and logs." with title "Endpoint Management Agent Uninstaller" buttons {"Cancel", "Uninstall"} default button "Uninstall" with icon caution

    if button returned of dialogResult is "Uninstall" then
        set uninstScript to "launchctl unload -w /Library/LaunchDaemons/com.endpoint-mgmt.agent.plist 2>/dev/null || true; " & ¬
            "rm -f /Library/LaunchDaemons/com.endpoint-mgmt.agent.plist; " & ¬
            "rm -f /usr/local/bin/endpoint-agent; " & ¬
            "rm -rf /etc/endpoint-agent; " & ¬
            "rm -f /var/log/endpoint-agent.log /var/log/endpoint-agent.err;"

        do shell script uninstScript with administrator privileges

        display dialog "Endpoint Management Agent has been successfully uninstalled from your Mac." with title "Uninstallation Complete" buttons {"OK"} default button "OK" with icon note
    end if
on error number -128
    -- User canceled, do nothing
end try
APPLESCRIPT
