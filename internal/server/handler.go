package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/keepmind9/agent-chat/internal/store"
	"github.com/keepmind9/agent-chat/pkg/protocol"
)

// Handler provides HTTP endpoints for the agent-chat system.
type Handler struct {
	store *store.Store
	hub   *Hub
}

// NewHandler creates a new Handler with the given store and hub.
func NewHandler(s *store.Store, h *Hub) *Handler {
	return &Handler{store: s, hub: h}
}

// HandleRegister registers a new agent and updates hub group membership.
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req protocol.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "agent name is required", http.StatusBadRequest)
		return
	}

	if err := h.store.RegisterAgent(req.Name, req.Groups); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update hub group membership for each group.
	for _, group := range req.Groups {
		members, err := h.store.GetGroupMembers(group)
		if err != nil {
			continue
		}
		h.hub.SetGroupMembers(group, members)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleSend sends a message from one agent to another or to a group.
func (h *Handler) HandleSend(w http.ResponseWriter, r *http.Request) {
	var req protocol.SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.From == "" {
		http.Error(w, "from field is required", http.StatusBadRequest)
		return
	}

	msgID, err := h.store.SaveMessage(req.From, req.To, req.Group, req.Content, req.InReplyTo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	msg, err := h.store.GetMessage(msgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Push the message via hub.
	if req.Group != "" {
		h.hub.PushToGroup(req.Group, msg, req.From)
	} else if req.To != "" {
		h.hub.PushToAgent(req.To, msg)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "id": msgID})
}

// HandleGetMessages returns unread messages for the given agent.
func (h *Handler) HandleGetMessages(w http.ResponseWriter, r *http.Request) {
	agent := r.URL.Query().Get("agent")
	if agent == "" {
		http.Error(w, "agent query parameter is required", http.StatusBadRequest)
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	msgs, err := h.store.GetUnreadMessages(agent, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if msgs == nil {
		msgs = []*protocol.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// HandleMarkRead marks messages as read by an agent.
func (h *Handler) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	var req protocol.ReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.AgentName == "" {
		http.Error(w, "agent_name is required", http.StatusBadRequest)
		return
	}

	if err := h.store.MarkRead(req.AgentName, req.MessageIDs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HandleListAgents returns all registered agents.
func (h *Handler) HandleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.store.ListAgents()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if agents == nil {
		agents = []*protocol.Agent{}
	}
	writeJSON(w, http.StatusOK, agents)
}

// HandleListGroups returns all distinct groups.
func (h *Handler) HandleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.store.ListGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if groups == nil {
		groups = []string{}
	}
	writeJSON(w, http.StatusOK, groups)
}

// HandleRecentMessages returns the most recent 50 messages.
func (h *Handler) HandleRecentMessages(w http.ResponseWriter, r *http.Request) {
	msgs, err := h.store.GetRecentMessages(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if msgs == nil {
		msgs = []*protocol.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
