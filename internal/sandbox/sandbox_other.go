//go:build !linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

type Config struct {
	Enabled            bool
	NamespaceIsolation bool
	NetworkIsolation   bool
	FilesystemMode     string
	AllowedMounts      []string
}

type Status struct {
	Enabled            bool   `json:"enabled"`
	NamespaceIsolation bool   `json:"namespace_isolation"`
	NetworkIsolation   bool   `json:"network_isolation"`
	FilesystemMode     string `json:"filesystem_mode"`
	Platform           string `json:"platform"`
	ContainerEnv       bool   `json:"container_env"`
}

type Manager struct {
	config Config
}

func NewManager(config Config) *Manager {
	return &Manager{config: config}
}

func (m *Manager) GetStatus() Status {
	return Status{
		Enabled:            m.config.Enabled,
		NamespaceIsolation: m.config.NamespaceIsolation,
		NetworkIsolation:   m.config.NetworkIsolation,
		FilesystemMode:     m.config.FilesystemMode,
		Platform:           runtime.GOOS,
		ContainerEnv:       m.detectContainer(),
	}
}

func (m *Manager) detectContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		return true
	}
	return false
}

func (m *Manager) WrapCommand(cmd *exec.Cmd) error {
	return nil
}

func DefaultConfig() Config {
	return Config{
		Enabled:            false,
		NamespaceIsolation: false,
		NetworkIsolation:   false,
		FilesystemMode:     "off",
		AllowedMounts:      []string{},
	}
}

func RunSandboxed(cmd *exec.Cmd, config Config) error {
	return cmd.Run()
}

func IsLinux() bool {
	return runtime.GOOS == "linux"
}

func SupportsNamespaces() bool {
	return false
}

func (s Status) String() string {
	return fmt.Sprintf(
		"Sandbox Status:\n  Enabled: %v\n  Namespace Isolation: %v\n  Network Isolation: %v\n  Filesystem Mode: %s\n  Platform: %s\n  Container Env: %v",
		s.Enabled, s.NamespaceIsolation, s.NetworkIsolation, s.FilesystemMode, s.Platform, s.ContainerEnv,
	)
}
