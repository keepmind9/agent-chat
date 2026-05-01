package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background daemon",
		RunE:  runStop,
	}
}

func runStop(_ *cobra.Command, _ []string) error {
	dir := getDataDir()
	pidPath := filepath.Join(dir, pidFileName)
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("agent-chat is not running (PID file not found at %s)", pidPath)
	}

	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		return fmt.Errorf("invalid PID file content")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	if err := stopProcess(proc); err != nil {
		os.Remove(pidPath)
		return fmt.Errorf("process %d not found, cleaned up stale PID file", pid)
	}

	fmt.Printf("agent-chat stopped (PID %d)\n", pid)
	return nil
}
