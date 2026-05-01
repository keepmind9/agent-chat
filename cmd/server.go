package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/keepmind9/agent-chat/internal/server"
	"github.com/keepmind9/agent-chat/internal/store"
)

// WebFS is set by main before Execute to serve embedded web assets.
var WebFS fs.FS

var (
	serverPort    string
	dbPath        string
	retentionDays int
	pidFileName   = "agent-chat.pid"
)

var serverCmd = &cobra.Command{
	Use:     "serve",
	Aliases: []string{"start"},
	Short:   "Start the agent-chat central server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if asDaemon {
			return startDaemon()
		}
		return runServe()
	},
}

var asDaemon bool

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(newStopCmd())
	serverCmd.Flags().BoolVarP(&asDaemon, "daemon", "d", false, "run as background daemon")
	serverCmd.Flags().StringVar(&serverPort, "port", "8080", "server port")

	homeDir, _ := os.UserHomeDir()
	defaultDBPath := filepath.Join(homeDir, ".agent-chat", "agent-chat.db")
	serverCmd.Flags().StringVar(&dbPath, "db", defaultDBPath, "SQLite database path")
	serverCmd.Flags().IntVar(&retentionDays, "retention", 30, "message retention period in days (0 to disable)")
}

func getDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".agent-chat")
	}
	return ""
}

func runServe() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	s, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer s.Close()

	hub := server.NewHub()
	go hub.Run()

	if retentionDays > 0 {
		go func() {
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				n, err := s.DeleteOldMessages(context.Background(), retentionDays)
				if err != nil {
					logger.Error("message cleanup failed", "error", err)
				} else if n > 0 {
					logger.Info("cleaned up old messages", "deleted", n)
				}
			}
		}()
	}

	h := server.NewHandler(s, hub, logger)

	r := gin.Default()
	r.SetTrustedProxies(nil)

	r.POST("/api/register", gin.WrapF(h.HandleRegister))
	r.POST("/api/send", gin.WrapF(h.HandleSend))
	r.POST("/api/send-group", gin.WrapF(h.HandleSend))
	r.GET("/api/messages", gin.WrapF(h.HandleGetMessages))
	r.GET("/api/messages/recent", gin.WrapF(h.HandleRecentMessages))
	r.POST("/api/messages/read", gin.WrapF(h.HandleMarkRead))
	r.GET("/api/agents", gin.WrapF(h.HandleListAgents))
	r.GET("/api/groups", gin.WrapF(h.HandleListGroups))
	r.POST("/api/agents/status", gin.WrapF(h.HandleUpdateStatus))

	r.GET("/ws", gin.WrapF(h.HandleWebSocket))

	webContent, _ := fs.Sub(WebFS, "web")
	r.StaticFS("/web", http.FS(webContent))
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/web/") })

	srv := &http.Server{
		Addr:    ":" + serverPort,
		Handler: r,
	}

	writePIDFile()
	defer removePIDFile()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("agent-chat server starting", "port", serverPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case sig := <-sigCh:
		logger.Info("shutting down", "signal", sig)
	case err := <-errCh:
		return err
	}

	hub.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	return nil
}

func startDaemon() error {
	dataDir := getDataDir()
	if dataDir == "" {
		return fmt.Errorf("cannot determine data directory")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	pidPath := filepath.Join(dataDir, pidFileName)
	if pidData, err := os.ReadFile(pidPath); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(pidData))); err == nil {
			if proc, err := os.FindProcess(pid); err == nil {
				if proc.Signal(syscall.Signal(0)) == nil {
					return fmt.Errorf("agent-chat is already running (PID %d)", pid)
				}
			}
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cmd := exec.Command(execPath, "serve")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	setDaemonSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	fmt.Printf("agent-chat started (PID %d)\n", cmd.Process.Pid)
	fmt.Printf("Data dir: %s\n", dataDir)
	fmt.Printf("Use 'agent-chat stop' to stop the daemon.\n")
	return nil
}

func writePIDFile() {
	dataDir := getDataDir()
	if dataDir == "" {
		return
	}
	pidPath := filepath.Join(dataDir, pidFileName)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func removePIDFile() {
	dataDir := getDataDir()
	if dataDir == "" {
		return
	}
	pidPath := filepath.Join(dataDir, pidFileName)
	_ = os.Remove(pidPath)
}
