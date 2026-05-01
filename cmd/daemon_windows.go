//go:build windows

package cmd

import (
	"os"
	"os/exec"
)

func setDaemonSysProcAttr(cmd *exec.Cmd) {
	// No-op on Windows
}

func stopProcess(proc *os.Process) error {
	return proc.Kill()
}
