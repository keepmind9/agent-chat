package mcp

import (
	"encoding/json"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/keepmind9/agent-chat/internal/notify"
	"github.com/keepmind9/agent-chat/pkg/protocol"
)

const (
	// heartbeatInterval is how often the WSClient sends ping frames to the server.
	heartbeatInterval = 30 * time.Second
)

// WSClient connects to the agent-chat server via WebSocket to receive
// real-time push notifications and deliver them via a Notifier.
type WSClient struct {
	serverURL string
	agentName string
	notifier  notify.Notifier
	logger    *slog.Logger
	conn      *websocket.Conn
	mu        sync.Mutex
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// NewWSClient creates a new WebSocket client.
func NewWSClient(serverURL, agentName string, n notify.Notifier, logger *slog.Logger) *WSClient {
	return &WSClient{
		serverURL: serverURL,
		agentName: agentName,
		notifier:  n,
		logger:    logger.With("component", "wsclient"),
		stopCh:    make(chan struct{}),
	}
}

// Connect establishes a WebSocket connection to the server.
func (c *WSClient) Connect() error {
	url := c.wsURL()
	c.logger.Info("connecting", "url", c.wsURL())

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	c.logger.Info("connected", "url", c.wsURL())
	return nil
}

// Run starts the retry loop: Connect -> readLoop -> sleep -> retry.
// It blocks until Stop is called.
func (c *WSClient) Run() {
	const (
		initInterval = 1 * time.Second
		maxInterval  = 30 * time.Second
		multiplier   = 2.0
	)
	interval := initInterval

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if err := c.Connect(); err != nil {
			c.logger.Warn("connect error, retrying", "error", err, "retry", interval)
			if c.sleepWithStop(jitter(interval)) {
				return
			}
			interval = min(time.Duration(float64(interval)*multiplier), maxInterval)
			continue
		}

		// Reset backoff on successful connection
		interval = initInterval

		// heartbeatDone is closed when the heartbeat goroutine exits.
		heartbeatDone := make(chan struct{})
		go c.heartbeat(heartbeatDone)

		c.readLoop()

		// Signal heartbeat goroutine to stop and wait for it.
		close(heartbeatDone)

		// Connection lost, wait before retry
		c.logger.Warn("connection lost, retrying", "retry", interval)
		if c.sleepWithStop(jitter(interval)) {
			return
		}
		interval = min(time.Duration(float64(interval)*multiplier), maxInterval)
	}
}

// heartbeat sends periodic ping frames to keep the connection alive.
// It runs until heartbeatDone is closed or stopCh is closed.
func (c *WSClient) heartbeat(done chan struct{}) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()

			if conn == nil {
				return
			}
			if err := conn.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(5*time.Second),
			); err != nil {
				c.logger.Warn("heartbeat ping failed, closing connection", "error", err)
				conn.Close()
				return
			}
		}
	}
}

// jitter adds randomization in [d/2, d] (i.e. -50% to 0%) to avoid thundering herd.
func jitter(d time.Duration) time.Duration {
	half := d / 2
	return d - half + time.Duration(rand.Int63n(int64(half)+1))
}

// Stop shuts down the WebSocket client. Safe to call multiple times.
func (c *WSClient) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)

		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	})
}

// readLoop reads messages from the WebSocket connection and dispatches them.
func (c *WSClient) readLoop() {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			c.logger.Warn("read error", "error", err)
			return
		}

		var push protocol.WSPush
		if err := json.Unmarshal(msgBytes, &push); err != nil {
			c.logger.Warn("unmarshal error", "error", err)
			continue
		}

		if push.Type == "new_message" {
			dataBytes, err := json.Marshal(push.Data)
			if err != nil {
				c.logger.Warn("marshal message data", "error", err)
				continue
			}
			var msg protocol.Message
			if err := json.Unmarshal(dataBytes, &msg); err != nil {
				c.logger.Warn("unmarshal message", "error", err)
				continue
			}
			c.handleMessage(&msg)
		}
	}
}

// handleMessage logs and delivers a message notification.
func (c *WSClient) handleMessage(msg *protocol.Message) {
	c.logger.Info("received message", "from", msg.FromAgent)
	if err := c.notifier.Notify(msg); err != nil {
		c.logger.Error("notify error", "error", err)
	}
}

// wsURL converts the server HTTP URL to a WebSocket URL and appends the agent query param.
func (c *WSClient) wsURL() string {
	u := c.serverURL
	u = strings.TrimRight(u, "/")
	if strings.HasPrefix(u, "https://") {
		u = "wss://" + u[8:]
	} else if strings.HasPrefix(u, "http://") {
		u = "ws://" + u[7:]
	}
	return u + "/ws?agent=" + c.agentName
}

// sleepWithStop sleeps for the given duration, returning true if stopCh was closed.
func (c *WSClient) sleepWithStop(d time.Duration) bool {
	select {
	case <-c.stopCh:
		return true
	case <-time.After(d):
		return false
	}
}
