//go:build windows

package main

import (
	"github.com/endpoint-mgmt/agent/shared/osinfo"
	winagent "github.com/endpoint-mgmt/agent/windows"
)

// newOSInfoProvider returns the Windows implementation.
func newOSInfoProvider() osinfo.Provider { return winagent.New() }
