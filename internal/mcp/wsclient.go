package mcp

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/keepmind9/agent-chat/internal/injector"
	"github.com/keepmind9/agent-chat/pkg/protocol"
)

// WSClient connects to the agent-chat server via WebSocket to receive
// real-time push notifications and inject them into the agent's tmux pane.
type WSClient struct {
	serverURL string
	agentName string
	injector  *injector.Injector
	conn      *websocket.Conn
	mu        sync.Mutex
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// NewWSClient creates a new WebSocket client.
func NewWSClient(serverURL, agentName string, inj *injector.Injector) *WSClient {
	return &WSClient{
		serverURL: serverURL,
		agentName: agentName,
		injector:  inj,
		stopCh:    make(chan struct{}),
	}
}

// Connect establishes a WebSocket connection to the server.
func (c *WSClient) Connect() error {
	url := c.wsURL()
	log.Printf("[wsclient] connecting to %s", url)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	log.Printf("[wsclient] connected to %s", url)
	return nil
}

// Run starts the retry loop: Connect -> readLoop -> sleep -> retry.
// It blocks until Stop is called.
func (c *WSClient) Run() {
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if err := c.Connect(); err != nil {
			log.Printf("[wsclient] connect error: %v, retrying in 2s", err)
			if c.sleepWithStop(2 * time.Second) {
				return
			}
			continue
		}

		c.readLoop()

		// Connection lost, wait before retry
		log.Printf("[wsclient] connection lost, retrying in 2s")
		if c.sleepWithStop(2 * time.Second) {
			return
		}
	}
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
			log.Printf("[wsclient] read error: %v", err)
			return
		}

		var push protocol.WSPush
		if err := json.Unmarshal(msgBytes, &push); err != nil {
			log.Printf("[wsclient] unmarshal error: %v", err)
			continue
		}

		if push.Type == "new_message" {
			dataBytes, err := json.Marshal(push.Data)
			if err != nil {
				log.Printf("[wsclient] marshal message data: %v", err)
				continue
			}
			var msg protocol.Message
			if err := json.Unmarshal(dataBytes, &msg); err != nil {
				log.Printf("[wsclient] unmarshal message: %v", err)
				continue
			}
			c.handleMessage(&msg)
		}
	}
}

// handleMessage logs and injects a message into the tmux pane.
func (c *WSClient) handleMessage(msg *protocol.Message) {
	log.Printf("[wsclient] received message from %s", msg.FromAgent)
	if err := c.injector.InjectMessage(msg); err != nil {
		log.Printf("[wsclient] inject error: %v", err)
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
