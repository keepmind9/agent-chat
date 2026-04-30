// Package protocol defines the shared types used across all agent-chat modules:
// HTTP handlers, WebSocket hub, storage layer, and MCP tools.
package protocol

import "time"

// Message represents a chat message exchanged between agents or within a group.
type Message struct {
	ID        string    `json:"id"`
	FromAgent string    `json:"from_agent"`
	ToAgent   string    `json:"to_agent"`
	Group     string    `json:"group"`
	Content   string    `json:"content"`
	InReplyTo string    `json:"in_reply_to,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Agent represents a registered agent in the system.
type Agent struct {
	Name         string    `json:"name"`
	Groups       []string  `json:"groups"`
	Status       string    `json:"status"`
	RegisteredAt time.Time `json:"registered_at"`
}

// MessageRead tracks which agents have read which messages.
type MessageRead struct {
	MessageID string    `json:"message_id"`
	AgentName string    `json:"agent_name"`
	ReadAt    time.Time `json:"read_at"`
}

// WSPush is the envelope for pushing data over a WebSocket connection.
type WSPush struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// SendRequest is the payload for sending a new message.
type SendRequest struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Group     string `json:"group"`
	Content   string `json:"content"`
	InReplyTo string `json:"in_reply_to,omitempty"`
}

// RegisterRequest is the payload for registering a new agent.
type RegisterRequest struct {
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

// ReadRequest is the payload for marking messages as read.
type ReadRequest struct {
	AgentName  string   `json:"agent_name"`
	MessageIDs []string `json:"message_ids"`
}

// UpdateStatusRequest is the payload for updating an agent's work status.
type UpdateStatusRequest struct {
	AgentName string `json:"agent_name"`
	Status    string `json:"status"` // "idle" or "working"
}
