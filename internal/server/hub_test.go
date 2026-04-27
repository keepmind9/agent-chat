package server

import (
	"testing"
	"time"

	"github.com/keepmind9/agent-chat/pkg/protocol"
	"github.com/stretchr/testify/assert"
)

func TestHubRegisterUnregister(t *testing.T) {
	h := NewHub()

	ch := make(chan []byte, 8)
	h.Register("agent-a", ch)

	assert.True(t, h.IsOnline("agent-a"), "agent-a should be online after register")
	assert.False(t, h.IsOnline("agent-b"), "agent-b should not be online")

	h.Unregister("agent-a")

	// Allow goroutine scheduling to settle
	time.Sleep(10 * time.Millisecond)

	assert.False(t, h.IsOnline("agent-a"), "agent-a should be offline after unregister")
}

func TestHubPushToAgent(t *testing.T) {
	h := NewHub()

	ch := make(chan []byte, 8)
	h.Register("agent-a", ch)

	msg := &protocol.Message{
		ID:        "msg-1",
		FromAgent: "agent-b",
		ToAgent:   "agent-a",
		Content:   "hello",
	}

	h.PushToAgent("agent-a", msg)

	select {
	case data := <-ch:
		assert.NotEmpty(t, data, "channel should receive non-empty data")
	default:
		t.Fatal("expected data on channel but got none")
	}

	// Pushing to an offline agent should not panic or block.
	h.PushToAgent("agent-offline", msg)
}

func TestHubPushToGroup(t *testing.T) {
	h := NewHub()

	chA := make(chan []byte, 8)
	chB := make(chan []byte, 8)
	chC := make(chan []byte, 8)

	h.Register("agent-a", chA)
	h.Register("agent-b", chB)
	h.Register("agent-c", chC)

	h.SetGroupMembers("room-1", []string{"agent-a", "agent-b", "agent-c"})

	msg := &protocol.Message{
		ID:        "msg-2",
		FromAgent: "agent-a",
		Group:     "room-1",
		Content:   "group hello",
	}

	h.PushToGroup("room-1", msg, "agent-a")

	// agent-a should NOT receive the message (excluded as sender)
	select {
	case <-chA:
		t.Fatal("agent-a should not receive the message")
	default:
		// expected
	}

	// agent-b should receive the message
	select {
	case data := <-chB:
		assert.NotEmpty(t, data, "agent-b should receive non-empty data")
	default:
		t.Fatal("agent-b expected data on channel but got none")
	}

	// agent-c should receive the message
	select {
	case data := <-chC:
		assert.NotEmpty(t, data, "agent-c should receive non-empty data")
	default:
		t.Fatal("agent-c expected data on channel but got none")
	}
}
