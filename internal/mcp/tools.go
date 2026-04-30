package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	mcptool "github.com/mark3labs/mcp-go/mcp"
)

// APIClient communicates with the agent-chat server API.
type APIClient struct {
	baseURL    string
	agentName  string
	httpClient *http.Client
}

// NewAPIClient creates a new API client for the given server base URL and agent name.
func NewAPIClient(baseURL, agentName string) *APIClient {
	return &APIClient{
		baseURL:    baseURL,
		agentName:  agentName,
		httpClient: &http.Client{},
	}
}

// DoRequest sends an HTTP request to the server and returns the parsed response body.
func (c *APIClient) DoRequest(method, path string, body interface{}) (interface{}, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = newByteReader(data)
	}

	req, err := http.NewRequestWithContext(context.Background(), method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(respData))
	}

	var result interface{}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

type byteReader struct {
	data []byte
	pos  int
}

func newByteReader(data []byte) *byteReader {
	return &byteReader{data: data}
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// --- Tool Definitions ---

// BuildRegisterTool builds the MCP tool definition for agent registration.
func BuildRegisterTool() mcptool.Tool {
	return mcptool.NewTool("register",
		mcptool.WithDescription("Register the current agent to the communication platform. Called automatically at startup."),
		mcptool.WithString("name",
			mcptool.Required(),
			mcptool.Description("The name of the agent to register"),
		),
		mcptool.WithArray("groups",
			mcptool.Description("List of group names to join"),
			mcptool.WithStringItems(),
		),
	)
}

// BuildSendMessageTool builds the MCP tool definition for sending direct messages.
func BuildSendMessageTool() mcptool.Tool {
	return mcptool.NewTool("send_message",
		mcptool.WithDescription("Send a direct message to another agent. Include reply_to with the original message ID when replying to prevent notification loops."),
		mcptool.WithString("to",
			mcptool.Required(),
			mcptool.Description("The name of the target agent"),
		),
		mcptool.WithString("content",
			mcptool.Required(),
			mcptool.Description("The message content to send"),
		),
		mcptool.WithString("reply_to",
			mcptool.Description("The message ID being replied to. Include when replying to prevent notification loops."),
		),
	)
}

// BuildSendGroupMessageTool builds the MCP tool definition for sending group messages.
func BuildSendGroupMessageTool() mcptool.Tool {
	return mcptool.NewTool("send_group_message",
		mcptool.WithDescription("Send a message to all members of a group."),
		mcptool.WithString("group",
			mcptool.Required(),
			mcptool.Description("The name of the target group"),
		),
		mcptool.WithString("content",
			mcptool.Required(),
			mcptool.Description("The message content to send"),
		),
	)
}

// BuildCheckMessagesTool builds the MCP tool definition for checking unread messages.
func BuildCheckMessagesTool() mcptool.Tool {
	return mcptool.NewTool("check_messages",
		mcptool.WithDescription("Check unread messages from other agents. Call this when you see an [agent-chat] notification."),
		mcptool.WithNumber("limit",
			mcptool.Description("Maximum number of messages to return"),
			mcptool.DefaultNumber(20),
		),
	)
}

// BuildReadMessagesTool builds the MCP tool definition for marking messages as read.
func BuildReadMessagesTool() mcptool.Tool {
	return mcptool.NewTool("read_messages",
		mcptool.WithDescription("Mark messages as read after processing them."),
		mcptool.WithArray("message_ids",
			mcptool.Required(),
			mcptool.Description("List of message IDs to mark as read"),
			mcptool.WithStringItems(),
		),
	)
}

// BuildListAgentsTool builds the MCP tool definition for listing registered agents.
func BuildListAgentsTool() mcptool.Tool {
	return mcptool.NewTool("list_agents",
		mcptool.WithDescription("List all currently registered agents."),
	)
}

// BuildListGroupsTool builds the MCP tool definition for listing available groups.
func BuildListGroupsTool() mcptool.Tool {
	return mcptool.NewTool("list_groups",
		mcptool.WithDescription("List all available groups."),
	)
}

// BuildUpdateStatusTool builds the MCP tool definition for updating agent work status.
func BuildUpdateStatusTool() mcptool.Tool {
	return mcptool.NewTool("update_status",
		mcptool.WithDescription("Update your work status so other agents know if you are busy. Set to 'working' when you start a task and 'idle' when done."),
		mcptool.WithString("status",
			mcptool.Required(),
			mcptool.Description("Your current status: 'idle' or 'working'"),
			mcptool.Enum("idle", "working"),
		),
	)
}

// --- Tool Handlers ---

// MakeRegisterHandler creates a handler for the register tool.
func MakeRegisterHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return mcptool.NewToolResultError(err.Error()), nil
		}
		groups := req.GetStringSlice("groups", nil)

		client.agentName = name
		body := map[string]interface{}{
			"name":   name,
			"groups": groups,
		}
		result, err := client.DoRequest(http.MethodPost, "/api/register", body)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("registration failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}

// MakeSendMessageHandler creates a handler for the send_message tool.
func MakeSendMessageHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		to, err := req.RequireString("to")
		if err != nil {
			return mcptool.NewToolResultError(err.Error()), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcptool.NewToolResultError(err.Error()), nil
		}

		body := map[string]interface{}{
			"from":        client.agentName,
			"to":          to,
			"content":     content,
			"in_reply_to": req.GetString("reply_to", ""),
		}
		result, err := client.DoRequest(http.MethodPost, "/api/send", body)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("send message failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}

// MakeSendGroupMessageHandler creates a handler for the send_group_message tool.
func MakeSendGroupMessageHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		group, err := req.RequireString("group")
		if err != nil {
			return mcptool.NewToolResultError(err.Error()), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcptool.NewToolResultError(err.Error()), nil
		}

		body := map[string]interface{}{
			"from":    client.agentName,
			"group":   group,
			"content": content,
		}
		result, err := client.DoRequest(http.MethodPost, "/api/send", body)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("send group message failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}

// MakeCheckMessagesHandler creates a handler for the check_messages tool.
func MakeCheckMessagesHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		limit := req.GetInt("limit", 20)
		path := fmt.Sprintf("/api/messages?agent=%s&limit=%d", client.agentName, limit)

		result, err := client.DoRequest(http.MethodGet, path, nil)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("check messages failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}

// MakeReadMessagesHandler creates a handler for the read_messages tool.
func MakeReadMessagesHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		messageIDs, err := req.RequireStringSlice("message_ids")
		if err != nil {
			return mcptool.NewToolResultError(err.Error()), nil
		}

		body := map[string]interface{}{
			"agent_name":  client.agentName,
			"message_ids": messageIDs,
		}
		result, err := client.DoRequest(http.MethodPost, "/api/messages/read", body)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("mark read failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}

// MakeListAgentsHandler creates a handler for the list_agents tool.
func MakeListAgentsHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		result, err := client.DoRequest(http.MethodGet, "/api/agents", nil)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("list agents failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}

// MakeListGroupsHandler creates a handler for the list_groups tool.
func MakeListGroupsHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		result, err := client.DoRequest(http.MethodGet, "/api/groups", nil)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("list groups failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}

// MakeUpdateStatusHandler creates a handler for the update_status tool.
func MakeUpdateStatusHandler(client *APIClient) func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
	return func(ctx context.Context, req mcptool.CallToolRequest) (*mcptool.CallToolResult, error) {
		status, err := req.RequireString("status")
		if err != nil {
			return mcptool.NewToolResultError(err.Error()), nil
		}

		body := map[string]interface{}{
			"agent_name": client.agentName,
			"status":     status,
		}
		result, err := client.DoRequest(http.MethodPost, "/api/agents/status", body)
		if err != nil {
			return mcptool.NewToolResultError(fmt.Sprintf("update status failed: %v", err)), nil
		}
		data, _ := json.Marshal(result)
		return mcptool.NewToolResultText(string(data)), nil
	}
}
