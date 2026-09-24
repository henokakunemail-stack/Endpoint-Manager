#!/usr/bin/env bash
# ==============================================================================
# Linux Desktop GUI Uninstaller for Endpoint Management Agent
# ==============================================================================
set -euo pipefail

TITLE="Endpoint Management Agent Uninstaller"

# 1. Elevate to root via pkexec/sudo
if [ "$EUID" -ne 0 ]; then
    if command -v pkexec >/dev/null 2>&1; then
        exec pkexec "$0" "$@"
    elif command -v gksudo >/dev/null 2>&1; then
        exec gksudo "$0" "$@"
    else
        echo "[-] Uninstaller requires root privileges. Please run with sudo: sudo $0"
        exit 1
    fi
fi

USE_TOOL=""
if command -v zenity >/dev/null 2>&1; then
    USE_TOOL="zenity"
elif command -v kdialog >/dev/null 2>&1; then
    USE_TOOL="kdialog"
fi

# 2. Confirmation Dialog
CONFIRM_MSG="Are you sure you want to completely uninstall Endpoint Management Agent?\n\nThis will stop the daemon service, remove the agent executable, and delete all local credentials and logs."
if [ "$USE_TOOL" = "zenity" ]; then
    zenity --question --title="$TITLE" --width=450 --text="$CONFIRM_MSG" || exit 0
elif [ "$USE_TOOL" = "kdialog" ]; then
    kdialog --title "$TITLE" --yesno "$CONFIRM_MSG" || exit 0
else
    read -rp "Are you sure you want to uninstall Endpoint Agent? [y/N]: " ANS
    if [[ ! "$ANS" =~ ^[Yy]$ ]]; then
        echo "Uninstallation canceled."
        exit 0
    fi
fi

# 3. Stop and Uninstall Service
if [ -x /usr/local/bin/endpoint-agent ]; then
    /usr/local/bin/endpoint-agent -service stop || true
    /usr/local/bin/endpoint-agent -service uninstall || true
fi

# Additional systemd cleanup
if [ -d /run/systemd/system ]; then
    systemctl stop endpoint-agent.service 2>/dev/null || true
    systemctl disable endpoint-agent.service 2>/dev/null || true
    rm -f /etc/systemd/system/endpoint-agent.service
    systemctl daemon-reload 2>/dev/null || true
fi

# 4. Remove Files, Credentials, and Desktop Launcher
rm -f /usr/local/bin/endpoint-agent
rm -rf /etc/endpoint-agent
rm -f /usr/share/applications/endpoint-agent-uninstall.desktop
rm -f /usr/local/bin/endpoint-agent-uninstall-gui

# 5. Success Dialog
DONE_MSG="Endpoint Management Agent has been completely removed from this system."
if [ "$USE_TOOL" = "zenity" ]; then
    zenity --info --title="$TITLE" --width=400 --text="$DONE_MSG"
elif [ "$USE_TOOL" = "kdialog" ]; then
    kdialog --title "$TITLE" --msgbox "$DONE_MSG"
else
    echo "[+] $DONE_MSG"
fi
