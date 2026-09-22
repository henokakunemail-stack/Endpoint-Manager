//go:build linux

package main

import (
	"github.com/endpoint-mgmt/agent/shared/osinfo"
	linagent "github.com/endpoint-mgmt/agent/linux"
)

// newOSInfoProvider returns the Linux implementation.
func newOSInfoProvider() osinfo.Provider { return linagent.New() }
