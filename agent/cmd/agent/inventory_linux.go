//go:build linux

package main

import (
	"github.com/endpoint-mgmt/agent/shared/inventory"
	linuxagent "github.com/endpoint-mgmt/agent/linux"
)

// newInventoryCollector returns the Linux inventory collector.
func newInventoryCollector() inventory.Collector { return linuxagent.NewCollector() }

// inventoryCapabilities are the command types this agent build can serve.
func inventoryCapabilities() []string {
	return []string{"ping", "inventory.collect"}
}
