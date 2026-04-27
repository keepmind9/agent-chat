package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageJSONMarshal(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	cases := []struct {
		name    string
		message Message
	}{
		{
			name: "direct message",
			message: Message{
				ID:         "msg-001",
				FromAgent:  "agent-a",
				ToAgent:    "agent-b",
				Group:      "",
				Content:    "hello",
				CreatedAt:  now,
			},
		},
		{
			name: "group message",
			message: Message{
				ID:         "msg-002",
				FromAgent:  "agent-a",
				ToAgent:    "",
				Group:      "team-1",
				Content:    "broadcast",
				CreatedAt:  now,
			},
		},
		{
			name: "empty content",
			message: Message{
				ID:         "msg-003",
				FromAgent:  "agent-c",
				Content:    "",
				CreatedAt:  now,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.message)
			require.NoError(t, err, "marshal should succeed")

			var got Message
			err = json.Unmarshal(data, &got)
			require.NoError(t, err, "unmarshal should succeed")

			assert.Equal(t, tc.message.ID, got.ID, "ID should roundtrip")
			assert.Equal(t, tc.message.FromAgent, got.FromAgent, "FromAgent should roundtrip")
			assert.Equal(t, tc.message.ToAgent, got.ToAgent, "ToAgent should roundtrip")
			assert.Equal(t, tc.message.Group, got.Group, "Group should roundtrip")
			assert.Equal(t, tc.message.Content, got.Content, "Content should roundtrip")
			assert.True(t, tc.message.CreatedAt.Equal(got.CreatedAt), "CreatedAt should roundtrip")
		})
	}
}

func TestWSPushMessage(t *testing.T) {
	cases := []struct {
		name     string
		push     WSPush
		wantType string
	}{
		{
			name:     "new message push",
			push:     WSPush{Type: "new_message", Data: map[string]string{"foo": "bar"}},
			wantType: "new_message",
		},
		{
			name:     "status update push",
			push:     WSPush{Type: "status_update", Data: "agent-online"},
			wantType: "status_update",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.push)
			require.NoError(t, err)
			assert.NotEmpty(t, data, "marshaled output should not be empty")

			var got WSPush
			err = json.Unmarshal(data, &got)
			require.NoError(t, err)
			assert.Equal(t, tc.wantType, got.Type)
		})
	}
}
