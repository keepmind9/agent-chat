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
	expected := `[agent-chat] Call check_messages for details, then reply with send_message. New message from agent-alice: "Hello, are you available?"`
	assert.Equal(t, expected, FormatDirectMessage(msg))
}

func TestFormatGroupMessage(t *testing.T) {
	msg := &protocol.Message{
		FromAgent: "agent-bob",
		Group:     "dev-team",
		Content:   "Deploying v2 now",
	}
	expected := `[agent-chat] Call check_messages for details, then reply with send_group_message or send_message. Group dev-team message from agent-bob: "Deploying v2 now"`
	assert.Equal(t, expected, FormatGroupMessage(msg))
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
