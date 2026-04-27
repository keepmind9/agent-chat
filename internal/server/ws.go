package server

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// upgrader upgrades HTTP connections to WebSocket.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HandleWebSocket upgrades an HTTP connection to WebSocket for real-time push.
// The agent name is expected as the "agent" query parameter.
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	agent := r.URL.Query().Get("agent")
	if agent == "" {
		http.Error(w, "agent query parameter is required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed for agent %q: %v", agent, err)
		return
	}

	// Create a push channel and register with the hub.
	ch := make(chan []byte, 64)
	h.hub.Register(agent, ch)

	// Set agent status to online.
	_ = h.store.SetAgentStatus(agent, "online")

	// Goroutine: read from push channel and write to WebSocket.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ch {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Main loop: read from WebSocket to detect disconnect.
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	// Cleanup on disconnect.
	h.hub.Unregister(agent)
	_ = h.store.SetAgentStatus(agent, "offline")
	close(ch)
	conn.Close()

	// Wait for the writer goroutine to finish.
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}
