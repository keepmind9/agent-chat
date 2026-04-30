package injector

import (
	"testing"

	"github.com/keepmind9/agent-chat/pkg/protocol"
	"github.com/stretchr/testify/assert"
)

func TestFormatDirectMessage(t *testing.T) {
	msg := &protocol.Message{
		FromAgent: "agent-alice",
		Content:   "Hello, are you available?",
	}
	result := FormatDirectMessage(msg)
	assert.Contains(t, result, "[agent-chat] Call check_messages for details, then reply with send_message.")
	assert.Contains(t, result, `from=agent-alice`)
	assert.Contains(t, result, `content="Hello, are you available?"`)
}

func TestFormatDirectMessageReply(t *testing.T) {
	msg := &protocol.Message{
		FromAgent: "agent-alice",
		Content:   "Yes, I'm here!",
		InReplyTo: "msg-123",
	}
	result := FormatDirectMessage(msg)
	assert.Contains(t, result, "Wait for your instruction before responding.")
	assert.Contains(t, result, `from=agent-alice`)
	assert.Contains(t, result, `reply_to=msg-123`)
}

func TestFormatGroupMessage(t *testing.T) {
	msg := &protocol.Message{
		FromAgent: "agent-bob",
		Group:     "dev-team",
		Content:   "Deploying v2 now",
	}
	result := FormatGroupMessage(msg)
	assert.Contains(t, result, "[agent-chat] Call check_messages for details, then reply with send_group_message or send_message.")
	assert.Contains(t, result, `group=dev-team`)
	assert.Contains(t, result, `from=agent-bob`)
}

func TestGetTmuxPane(t *testing.T) {
	// GetTmuxPane reads from the environment; in a test environment it may be
	// empty. We only verify that it returns a string without panicking.
	_ = GetTmuxPane()
}

func TestInjectDisabledWithoutTmux(t *testing.T) {
	inj := New("")
	assert.False(t, inj.IsEnabled())
}
