package notify

import (
	"testing"

	"github.com/keepmind9/agent-chat/pkg/protocol"
	"github.com/stretchr/testify/assert"
)

func TestNopNotifier(t *testing.T) {
	n := NopNotifier{}
	assert.False(t, n.IsEnabled())
	assert.Nil(t, n.Notify(&protocol.Message{}))
}
