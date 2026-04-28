package mcp

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/keepmind9/agent-chat/internal/notify"
)

func TestNewWSClient(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test-agent", notify.NopNotifier{}, slog.Default())

	assert.Equal(t, "http://localhost:8080", client.serverURL)
	assert.Equal(t, "test-agent", client.agentName)
	assert.NotNil(t, client.notifier)
	assert.NotNil(t, client.stopCh)
}

func TestWSURL(t *testing.T) {
	tests := []struct {
		name      string
		serverURL string
		agentName string
		expected  string
	}{
		{
			name:      "http to ws",
			serverURL: "http://localhost:8080",
			agentName: "agent1",
			expected:  "ws://localhost:8080/ws?agent=agent1",
		},
		{
			name:      "https to wss",
			serverURL: "https://example.com",
			agentName: "agent2",
			expected:  "wss://example.com/ws?agent=agent2",
		},
		{
			name:      "trailing slash stripped",
			serverURL: "http://localhost:8080/",
			agentName: "agent3",
			expected:  "ws://localhost:8080/ws?agent=agent3",
		},
		{
			name:      "bare URL passed through",
			serverURL: "ws://already-ws.com",
			agentName: "agent4",
			expected:  "ws://already-ws.com/ws?agent=agent4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewWSClient(tt.serverURL, tt.agentName, nil, slog.Default())
			assert.Equal(t, tt.expected, client.wsURL())
		})
	}
}

func TestStop(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test", notify.NopNotifier{}, slog.Default())
	client.Stop()

	// Calling Stop twice should not panic
	client.Stop()
}

func TestSleepWithStop(t *testing.T) {
	client := NewWSClient("http://localhost:8080", "test", notify.NopNotifier{}, slog.Default())
	client.Stop()

	// With stopCh closed, sleepWithStop should return true quickly
	result := client.sleepWithStop(5 * time.Second)
	assert.True(t, result)
}
