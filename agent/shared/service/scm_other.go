//go:build !windows

package service

// RunAsService on non-Windows platforms always returns false.
//
// Linux (systemd) and macOS (launchd) supervise background daemons as standard
// processes launched with standard signals (SIGTERM/SIGINT) without requiring
// a Windows-style Service Control Manager handshake or dispatcher loop.
func RunAsService() bool {
	return false
}

// Serve executes the agent function directly on non-Windows platforms.
func Serve(name string, run func() error) error {
	return run()
}
