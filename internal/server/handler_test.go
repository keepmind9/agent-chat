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

	h := NewHub()
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
