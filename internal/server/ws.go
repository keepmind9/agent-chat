package server

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// wsChannelBuf is the buffer size for per-agent push channels.
	wsChannelBuf = 64
	// wsGoroutineWait is how long to wait for the writer goroutine on cleanup.
	wsGoroutineWait = 1 * time.Second
	// writeWait is the time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// pongWait is the maximum time to wait for a pong from the peer.
	pongWait = 60 * time.Second
	// pingPeriod is how often to send pings. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
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
		h.logger.Error("websocket upgrade failed", "agent", agent, "error", err)
		return
	}

	// Create a push channel and register with the hub.
	ch := make(chan []byte, wsChannelBuf)
	h.hub.Register(agent, ch)

	// Set agent status to online.
	_ = h.store.SetAgentStatus(agent, "online")

	// Set initial read deadline for pong detection.
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Goroutine: read from push channel and write to WebSocket.
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
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
	case <-time.After(wsGoroutineWait):
	}
}
