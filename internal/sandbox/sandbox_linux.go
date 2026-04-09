//go:build linux

package sandbox

import (
	"os"
	"os/exec"
	"syscall"
)

func (m *Manager) WrapCommand(cmd *exec.Cmd) error {
	if !m.config.Enabled {
		return nil
	}

	if m.config.NamespaceIsolation {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
		}
	}

	return nil
}

func runLinuxSandbox(cmd *exec.Cmd, config Config) error {
	if config.NamespaceIsolation || config.NetworkIsolation {
		args := []string{}

		if config.NamespaceIsolation {
			args = append(args, "--user", "--pid", "--fork", "--mount-proc")
		}

		if config.NetworkIsolation {
			args = append(args, "--net")
		}

		args = append(args, "--", cmd.Path)
		args = append(args, cmd.Args[1:]...)

		unshare := exec.Command("unshare", args...)
		unshare.Dir = cmd.Dir
		unshare.Stdout = cmd.Stdout
		unshare.Stderr = cmd.Stderr
		unshare.Stdin = cmd.Stdin

		return unshare.Run()
	}

	return cmd.Run()
}

func SupportsNamespaces() bool {
	_, err := exec.LookPath("unshare")
	return err == nil
}
