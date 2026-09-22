//go:build darwin

package main

import (
	"github.com/endpoint-mgmt/agent/shared/osinfo"
	macagent "github.com/endpoint-mgmt/agent/macos"
)

// newOSInfoProvider returns the macOS implementation.
func newOSInfoProvider() osinfo.Provider { return macagent.New() }
