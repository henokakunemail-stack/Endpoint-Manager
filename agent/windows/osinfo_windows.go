//go:build windows

package windows

import (
	"fmt"
	"os"

	"github.com/endpoint-mgmt/agent/shared/osinfo"
	"golang.org/x/sys/windows"
)

// agentVersion is bumped with each agent release.
const agentVersion = "0.1.0"

// Provider implements osinfo.Provider for Windows.
type Provider struct{}

func New() Provider { return Provider{} }

func (Provider) Collect() (osinfo.Info, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	// RtlGetVersion ignores manifest-based version lying, unlike the deprecated
	// GetVersionEx, so the reported build number is the real OS build.
	v := windows.RtlGetVersion()
	if v == nil {
		return osinfo.Info{
			Name: "windows", Version: "unknown", Hostname: hostname, AgentVersion: agentVersion,
		}, nil
	}
	return osinfo.Info{
		Name:         "windows",
		Version:      fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber),
		Hostname:     hostname,
		AgentVersion: agentVersion,
	}, nil
}
