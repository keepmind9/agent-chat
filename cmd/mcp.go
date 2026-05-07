package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	mcplib "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/keepmind9/agent-chat/internal/injector"
	mcpmod "github.com/keepmind9/agent-chat/internal/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the agent-chat MCP plugin (launched by Claude Code / Codex)",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL := os.Getenv("AGENT_CHAT_SERVER")
		if serverURL == "" {
			return fmt.Errorf("AGENT_CHAT_SERVER env is required")
		}

		agentName := os.Getenv("AGENT_NAME")
		if agentName == "" {
			cwd, _ := os.Getwd()
			dirName := filepath.Base(cwd)
			agentType := "agent"
			if at := os.Getenv("AGENT_TYPE"); at != "" {
				agentType = at
			}
			agentName = agentType + "-" + dirName
		}

		groups := os.Getenv("AGENT_GROUPS")

		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

		pane := injector.GetTmuxPane()
		inj := injector.New(pane)

		client := mcpmod.NewAPIClient(serverURL, agentName)
		if apiKey := os.Getenv("AGENT_CHAT_API_KEY"); apiKey != "" {
			client.SetAPIKey(apiKey)
		}

		s := mcplib.NewMCPServer("agent-chat", version)

		s.AddTool(mcpmod.BuildRegisterTool(), mcpmod.MakeRegisterHandler(client))
		s.AddTool(mcpmod.BuildSendMessageTool(), mcpmod.MakeSendMessageHandler(client))
		s.AddTool(mcpmod.BuildSendGroupMessageTool(), mcpmod.MakeSendGroupMessageHandler(client))
		s.AddTool(mcpmod.BuildCheckMessagesTool(), mcpmod.MakeCheckMessagesHandler(client))
		s.AddTool(mcpmod.BuildReadMessagesTool(), mcpmod.MakeReadMessagesHandler(client))
		s.AddTool(mcpmod.BuildListAgentsTool(), mcpmod.MakeListAgentsHandler(client))
		s.AddTool(mcpmod.BuildListGroupsTool(), mcpmod.MakeListGroupsHandler(client))
		s.AddTool(mcpmod.BuildUpdateStatusTool(), mcpmod.MakeUpdateStatusHandler(client))
		s.AddTool(mcpmod.BuildDeregisterTool(), mcpmod.MakeDeregisterHandler(client))

		go func() {
			groupList := []string{}
			if groups != "" {
				for _, g := range strings.Split(groups, ",") {
					g = strings.TrimSpace(g)
					if g != "" {
						groupList = append(groupList, g)
					}
				}
			}
			_, err := client.DoRequest("POST", "/api/register", map[string]interface{}{
				"name":   agentName,
				"groups": groupList,
			})
			if err != nil {
				logger.Warn("auto register failed", "error", err)
			}
		}()

		wsClient := mcpmod.NewWSClient(serverURL, agentName, inj, logger)
		go wsClient.Run()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			logger.Info("shutting down")
			wsClient.Stop()
			cancel()
		}()

		if err := mcplib.ServeStdio(s); err != nil && ctx.Err() == nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
