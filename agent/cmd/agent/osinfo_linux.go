//go:build linux

package main

import (
	"github.com/henokakunemail-stack/Endpoint-Manager/agent/shared/osinfo"
	linagent "github.com/henokakunemail-stack/Endpoint-Manager/agent/linux"
)

// newOSInfoProvider returns the Linux implementation.
func newOSInfoProvider() osinfo.Provider { return linagent.New() }
