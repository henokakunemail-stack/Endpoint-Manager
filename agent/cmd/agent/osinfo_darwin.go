//go:build darwin

package main

import (
	"github.com/henokakunemail-stack/Endpoint-Manager/agent/shared/osinfo"
	macagent "github.com/henokakunemail-stack/Endpoint-Manager/agent/macos"
)

// newOSInfoProvider returns the macOS implementation.
func newOSInfoProvider() osinfo.Provider { return macagent.New() }
