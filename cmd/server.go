package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/keepmind9/agent-chat/internal/server"
	"github.com/keepmind9/agent-chat/internal/store"
)

var (
	serverPort string
	dbPath     string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the agent-chat central server",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		h := server.NewHandler(s, hub, logger)

		r := gin.Default()

		r.POST("/api/register", gin.WrapF(h.HandleRegister))
		r.POST("/api/send", gin.WrapF(h.HandleSend))
		r.POST("/api/send-group", gin.WrapF(h.HandleSend))
		r.GET("/api/messages", gin.WrapF(h.HandleGetMessages))
		r.GET("/api/messages/recent", gin.WrapF(h.HandleRecentMessages))
		r.POST("/api/messages/read", gin.WrapF(h.HandleMarkRead))
		r.GET("/api/agents", gin.WrapF(h.HandleListAgents))
		r.GET("/api/groups", gin.WrapF(h.HandleListGroups))

		r.GET("/ws", gin.WrapF(h.HandleWebSocket))

		r.StaticFS("/web", http.Dir("./web"))
		r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/web/") })

		logger.Info("agent-chat server starting", "port", serverPort)
		return r.Run(":" + serverPort)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVar(&serverPort, "port", "8080", "server port")

	homeDir, _ := os.UserHomeDir()
	defaultDBPath := filepath.Join(homeDir, ".agent-chat", "agent-chat.db")
	serverCmd.Flags().StringVar(&dbPath, "db", defaultDBPath, "SQLite database path")
}
