//go:build windows

package main

import (
	"github.com/henokakunemail-stack/Endpoint-Manager/agent/shared/osinfo"
	winagent "github.com/henokakunemail-stack/Endpoint-Manager/agent/windows"
)

// newOSInfoProvider returns the Windows implementation.
func newOSInfoProvider() osinfo.Provider { return winagent.New() }
