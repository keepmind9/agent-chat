package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

func TestHandleSend_NoRecipient(t *testing.T) {
	h := setupTestHandler(t)

	sendBody := `{"from":"agent-a","to":"","group":"","content":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(sendBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleSend(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
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

func TestHandleMessageHistory(t *testing.T) {
	handler := setupTestHandler(t)

	// Register agents: a and b in a group, c standalone.
	for _, entry := range []struct {
		name   string
		groups string
	}{
		{"agent-a", `["grp-1"]`},
		{"agent-b", `["grp-1"]`},
		{"agent-c", `[]`},
	} {
		body := `{"name":"` + entry.name + `","groups":` + entry.groups + `}`
		req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.HandleRegister(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	}

	// Send several messages.
	messages := []struct {
		body string
	}{
		{`{"from":"agent-a","to":"agent-b","group":"","content":"direct-ab"}`},
		{`{"from":"agent-b","to":"agent-a","group":"","content":"direct-ba"}`},
		{`{"from":"agent-a","to":"","group":"grp-1","content":"group-msg"}`},
		{`{"from":"agent-a","to":"agent-c","group":"","content":"direct-ac"}`},
	}
	for _, m := range messages {
		req := httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(m.body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.HandleSend(w, req)
		require.Equal(t, http.StatusOK, w.Result().StatusCode)
	}

	tests := []struct {
		name      string
		query     string
		wantCount int
	}{
		{
			name:      "all messages for agent-a",
			query:     "agent=agent-a",
			wantCount: 4, // 3 sent by a + 1 direct to a from b
		},
		{
			name:      "messages between agent-a and agent-b",
			query:     "agent=agent-a&with=agent-b",
			wantCount: 2, // direct-ab and direct-ba
		},
		{
			name:      "group messages for agent-a",
			query:     "agent=agent-a&group=grp-1",
			wantCount: 1, // group-msg
		},
		{
			name:      "messages with limit",
			query:     "agent=agent-a&limit=2",
			wantCount: 2,
		},
		{
			name:      "missing agent parameter",
			query:     "",
			wantCount: -1, // expect 400
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/messages/history?"+tt.query, nil)
			w := httptest.NewRecorder()
			handler.HandleMessageHistory(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if tt.wantCount < 0 {
				require.Equal(t, http.StatusBadRequest, resp.StatusCode)
				return
			}

			require.Equal(t, http.StatusOK, resp.StatusCode)

			var msgs []map[string]interface{}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&msgs))
			require.Len(t, msgs, tt.wantCount)
		})
	}

	// Test since/until filtering by sending a message, waiting briefly, then querying.
	t.Run("since/until filtering", func(t *testing.T) {
		// Query with a future since timestamp — should return 0 messages.
		future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		req := httptest.NewRequest(http.MethodGet, "/api/messages/history?agent=agent-a&since="+future, nil)
		w := httptest.NewRecorder()
		handler.HandleMessageHistory(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var msgs []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&msgs))
		require.Len(t, msgs, 0)
	})
}

func TestHandleSend_UnknownRecipient(t *testing.T) {
	handler := setupTestHandler(t)

	// Register only agent-a.
	body := `{"name":"agent-a","groups":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	// Send to non-existent agent.
	sendBody := `{"from":"agent-a","to":"agent-unknown","group":"","content":"hello"}`
	req = httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(sendBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.HandleSend(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleSend_UnknownGroup(t *testing.T) {
	handler := setupTestHandler(t)

	// Register only agent-a with no groups.
	body := `{"name":"agent-a","groups":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)
	require.Equal(t, http.StatusOK, w.Result().StatusCode)

	// Send to non-existent group.
	sendBody := `{"from":"agent-a","to":"","group":"nonexistent-group","content":"hello"}`
	req = httptest.NewRequest(http.MethodPost, "/api/send", bytes.NewBufferString(sendBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.HandleSend(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
