package service

import (
	"errors"
	"fmt"
	"os"
)

var (
	ErrNotSupported     = errors.New("service management not supported on this OS")
	ErrAlreadyInstalled = errors.New("service is already installed")
	ErrNotInstalled     = errors.New("service is not installed")
)

// Config defines the metadata and parameters for registering the agent as an OS service.
type Config struct {
	Name        string
	DisplayName string
	Description string
	Arguments   []string
}

// Manager controls the OS service lifecycle.
type Manager interface {
	Install() error
	Uninstall() error
	Start() error
	Stop() error
	Status() (string, error)
}

// DefaultConfig returns the standard enterprise service configuration for the agent.
func DefaultConfig(extraArgs []string) Config {
	return Config{
		Name:        "endpoint-agent",
		DisplayName: "Enterprise Endpoint Management Agent",
		Description: "Central fleet management, hardware inventory, patch compliance, and security monitoring daemon.",
		Arguments:   extraArgs,
	}
}

// GetExecutablePath returns the absolute path of the currently executing agent binary.
func GetExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("lookup executable path: %w", err)
	}
	return exe, nil
}
