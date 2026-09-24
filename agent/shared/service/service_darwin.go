//go:build darwin

package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type darwinManager struct {
	cfg Config
}

// NewManager returns the macOS launchd LaunchDaemon implementation.
func NewManager(cfg Config) (Manager, error) {
	if cfg.Name == "" {
		cfg.Name = "com.endpoint-mgmt.agent"
	}
	if !strings.HasPrefix(cfg.Name, "com.") {
		cfg.Name = "com.endpoint-mgmt." + cfg.Name
	}
	return &darwinManager{cfg: cfg}, nil
}

func (m *darwinManager) plistPath() string {
	return filepath.Join("/Library/LaunchDaemons", m.cfg.Name+".plist")
}

func (m *darwinManager) Install() error {
	exePath, err := GetExecutablePath()
	if err != nil {
		return err
	}

	argsXML := fmt.Sprintf("\t\t<string>%s</string>\n", exePath)
	for _, arg := range m.cfg.Arguments {
		argsXML += fmt.Sprintf("\t\t<string>%s</string>\n", arg)
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>/var/log/endpoint-agent.log</string>
	<key>StandardErrorPath</key>
	<string>/var/log/endpoint-agent.err</string>
</dict>
</plist>
`, m.cfg.Name, argsXML)

	if err := os.WriteFile(m.plistPath(), []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("write launchd plist: %w (ensure root/sudo)", err)
	}

	// Load daemon into launchd
	if out, err := exec.Command("launchctl", "load", "-w", m.plistPath()).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (m *darwinManager) Uninstall() error {
	_ = m.Stop()
	_ = exec.Command("launchctl", "unload", "-w", m.plistPath()).Run()

	if err := os.Remove(m.plistPath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove launchd plist: %w", err)
	}
	return nil
}

func (m *darwinManager) Start() error {
	out, err := exec.Command("launchctl", "start", m.cfg.Name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl start: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (m *darwinManager) Stop() error {
	out, err := exec.Command("launchctl", "stop", m.cfg.Name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl stop: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (m *darwinManager) Status() (string, error) {
	out, err := exec.Command("launchctl", "list", m.cfg.Name).CombinedOutput()
	if err != nil {
		return "STOPPED", nil
	}
	if strings.Contains(string(out), "PID") {
		return "RUNNING", nil
	}
	return "UNKNOWN", nil
}
