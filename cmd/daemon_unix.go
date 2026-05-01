//go:build !windows

package cmd

import (
	"os"
	"os/exec"
	"syscall"
)

func setDaemonSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}
