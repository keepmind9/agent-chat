package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	mcplib "github.com/mark3labs/mcp-go/server"

	"github.com/keepmind9/agent-chat/internal/injector"
	mcpmod "github.com/keepmind9/agent-chat/internal/mcp"
)

func main() {
	serverURL := os.Getenv("AGENT_CHAT_SERVER")
	if serverURL == "" {
		fmt.Fprintf(os.Stderr, "AGENT_CHAT_SERVER env is required\n")
		os.Exit(1)
	}

	// Auto-derive agent name from project directory + agent type, or use AGENT_NAME override
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

	// tmux injector
	pane := injector.GetTmuxPane()
	inj := injector.New(pane)

	// API client
	client := mcpmod.NewAPIClient(serverURL, agentName)

	// MCP server
	s := mcplib.NewMCPServer("agent-chat", "1.0.0")

	// Register all tools
	s.AddTool(mcpmod.BuildRegisterTool(), mcpmod.MakeRegisterHandler(client))
	s.AddTool(mcpmod.BuildSendMessageTool(), mcpmod.MakeSendMessageHandler(client))
	s.AddTool(mcpmod.BuildSendGroupMessageTool(), mcpmod.MakeSendGroupMessageHandler(client))
	s.AddTool(mcpmod.BuildCheckMessagesTool(), mcpmod.MakeCheckMessagesHandler(client))
	s.AddTool(mcpmod.BuildReadMessagesTool(), mcpmod.MakeReadMessagesHandler(client))
	s.AddTool(mcpmod.BuildListAgentsTool(), mcpmod.MakeListAgentsHandler(client))
	s.AddTool(mcpmod.BuildListGroupsTool(), mcpmod.MakeListGroupsHandler(client))

	// Auto-register agent
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
			log.Printf("auto register: %v", err)
		}
	}()

	// WebSocket client (background push receiver)
	wsClient := mcpmod.NewWSClient(serverURL, agentName, inj)
	go wsClient.Run()

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[mcp] shutting down...")
		wsClient.Stop()
		cancel()
	}()

	// Start MCP stdio server (blocking)
	if err := mcplib.ServeStdio(s); err != nil && ctx.Err() == nil {
		log.Fatalf("mcp server: %v", err)
	}
}
