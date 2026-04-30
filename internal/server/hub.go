package server

import (
	"encoding/json"
	"sync"

	"github.com/keepmind9/agent-chat/pkg/protocol"
)

// Hub manages WebSocket push channels for agents and group membership.
type Hub struct {
	mu           sync.RWMutex
	agents       map[string]chan []byte
	groupMembers map[string][]string
	stopCh       chan struct{}
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		agents:       make(map[string]chan []byte),
		groupMembers: make(map[string][]string),
		stopCh:       make(chan struct{}),
	}
}

// Run blocks until Stop is called.
func (h *Hub) Run() {
	<-h.stopCh
}

// Stop signals the Hub to stop running.
func (h *Hub) Stop() {
	close(h.stopCh)
}

// Register adds an agent's push channel to the hub.
func (h *Hub) Register(name string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.agents[name] = ch
}

// Unregister removes an agent's push channel from the hub.
func (h *Hub) Unregister(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.agents, name)
}

// IsOnline reports whether an agent is currently registered.
func (h *Hub) IsOnline(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agents[name]
	return ok
}

// SetGroupMembers replaces the member list for a group.
func (h *Hub) SetGroupMembers(group string, members []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.groupMembers[group] = members
}

// PushToAgent sends a WSPush envelope to the named agent's channel.
// It is a no-op if the agent is not online.
func (h *Hub) PushToAgent(name string, msg *protocol.Message) {
	h.mu.RLock()
	ch, ok := h.agents[name]
	h.mu.RUnlock()
	if !ok {
		return
	}

	push := protocol.WSPush{Type: "new_message", Data: msg}
	data, err := json.Marshal(push)
	if err != nil {
		return
	}

	select {
	case ch <- data:
	default:
		// Channel full, drop the message.
	}
}

// PushToGroup sends a WSPush envelope to all members of a group,
// excluding the agent identified by excludeAgent.
func (h *Hub) PushToGroup(group string, msg *protocol.Message, excludeAgent string) {
	h.mu.RLock()
	members := h.groupMembers[group]
	h.mu.RUnlock()

	for _, member := range members {
		if member == excludeAgent {
			continue
		}
		h.PushToAgent(member, msg)
	}
}

// PushStatusChange broadcasts an agent's status change to all connected agents.
func (h *Hub) PushStatusChange(agentName, status string) {
	push := protocol.WSPush{
		Type: "agent_status",
		Data: map[string]string{"agent": agentName, "status": status},
	}
	data, err := json.Marshal(push)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.agents {
		select {
		case ch <- data:
		default:
		}
	}
}
