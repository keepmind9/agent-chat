package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/keepmind9/agent-chat/internal/store"
)

// setupTestHandler creates an in-memory store and hub for testing.
func setupTestHandler(t *testing.T) *Handler {
	t.Helper()
	s, err := store.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	h := NewHub(slog.Default())
	go h.Run()
	t.Cleanup(func() { h.Stop() })

	return NewHandler(s, h, slog.Default())
}

func TestHandleRegister(t *testing.T) {
	h := setupTestHandler(t)

	body := `{"name":"agent-a","groups":["grp-1","grp-2"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleRegister(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, "ok", result["status"])
}

func TestHandleSend(t *testing.T) {
	h := setupTestHandler(t)

	// Register two agents first.
	for _, name := range []string{"agent-a", "agent-b"} {
		body := `{"name":"` + name + `","groups":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleRegister(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	}

	// Send a message from agent-a to agent-b.
	sendBody := `{"from":"agent-a","to":"agent-b","group":"","content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(sendBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleSend(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, "ok", result["status"])
	require.NotEmpty(t, result["id"])
}

func TestHandleGetMessages(t *testing.T) {
	h := setupTestHandler(t)

	// Register two agents.
	for _, name := range []string{"agent-a", "agent-b"} {
		body := `{"name":"` + name + `","groups":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleRegister(w, req)
	}

	// Send a message.
	sendBody := `{"from":"agent-a","to":"agent-b","group":"","content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(sendBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleSend(w, req)

	// Get unread messages for agent-b.
	req = httptest.NewRequest(http.MethodGet, "/api/messages?agent=agent-b&limit=10", nil)
	w = httptest.NewRecorder()
	h.HandleGetMessages(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var messages []map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages))
	require.Len(t, messages, 1)
	require.Equal(t, "hello", messages[0]["content"])
}

func TestHandleListAgents(t *testing.T) {
	h := setupTestHandler(t)

	// Register two agents.
	for _, name := range []string{"agent-a", "agent-b"} {
		body := `{"name":"` + name + `","groups":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleRegister(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	w := httptest.NewRecorder()
	h.HandleListAgents(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var agents []map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&agents))
	require.Len(t, agents, 2)
}

func TestHandleListGroups(t *testing.T) {
	h := setupTestHandler(t)

	body := `{"name":"agent-a","groups":["grp-1","grp-2"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleRegister(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	req = httptest.NewRequest(http.MethodGet, "/api/groups", nil)
	w = httptest.NewRecorder()
	h.HandleListGroups(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var groups []string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&groups))
	require.ElementsMatch(t, []string{"grp-1", "grp-2"}, groups)
}

func TestHandleUpdateStatus(t *testing.T) {
	h := setupTestHandler(t)

	// Register an agent.
	body := `{"name":"agent-a","groups":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleRegister(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"set working", `{"agent_name":"agent-a","status":"working"}`, http.StatusOK},
		{"set idle", `{"agent_name":"agent-a","status":"idle"}`, http.StatusOK},
		{"invalid status", `{"agent_name":"agent-a","status":"busy"}`, http.StatusBadRequest},
		{"missing name", `{"agent_name":"","status":"idle"}`, http.StatusBadRequest},
		{"unknown agent", `{"agent_name":"unknown","status":"idle"}`, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/agents/status", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.HandleUpdateStatus(w, req)
			require.Equal(t, tt.wantStatus, w.Result().StatusCode)
		})
	}
}

func TestHandleMarkRead(t *testing.T) {
	h := setupTestHandler(t)

	for _, name := range []string{"agent-a", "agent-b"} {
		body := `{"name":"` + name + `","groups":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleRegister(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	}

	sendBody := `{"from":"agent-a","to":"agent-b","group":"","content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(sendBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.HandleSend(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&result))
	msgID, ok := result["id"].(string)
	require.True(t, ok)

	req = httptest.NewRequest(http.MethodPost, "/api/messages/read", bytes.NewBufferString(`{"agent_name":"agent-b","message_ids":["`+msgID+`"]}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.HandleMarkRead(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	req = httptest.NewRequest(http.MethodGet, "/api/messages?agent=agent-b&limit=10", nil)
	w = httptest.NewRecorder()
	h.HandleGetMessages(w, req)
	var msgs []map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&msgs))
	require.Len(t, msgs, 0)
}

func TestHandleRecentMessages(t *testing.T) {
	h := setupTestHandler(t)

	for _, name := range []string{"agent-a", "agent-b"} {
		body := `{"name":"` + name + `","groups":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleRegister(w, req)
	}

	for _, content := range []string{"msg1", "msg2"} {
		body := `{"from":"agent-a","to":"agent-b","group":"","content":"` + content + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.HandleSend(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/messages/recent?limit=5", nil)
	w := httptest.NewRecorder()
	h.HandleRecentMessages(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var messages []map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&messages))
	require.Len(t, messages, 2)
}
