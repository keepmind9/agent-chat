package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper opens an in-memory store for testing.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRegisterAndGetAgent(t *testing.T) {
	s := openTestStore(t)

	err := s.RegisterAgent("agent-a", []string{"dev", "ops"})
	require.NoError(t, err)

	agent, err := s.GetAgent("agent-a")
	require.NoError(t, err)

	assert.Equal(t, "agent-a", agent.Name)
	assert.Equal(t, []string{"dev", "ops"}, agent.Groups)
	assert.Equal(t, "online", agent.Status)
	assert.False(t, agent.RegisteredAt.IsZero())
}

func TestRegisterAgentDuplicate(t *testing.T) {
	s := openTestStore(t)

	err := s.RegisterAgent("agent-a", []string{"dev"})
	require.NoError(t, err)

	err = s.RegisterAgent("agent-a", []string{"dev"})
	assert.Error(t, err)
}

func TestSaveAndGetDirectMessage(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("agent-a", nil))
	require.NoError(t, s.RegisterAgent("agent-b", nil))

	msgID, err := s.SaveMessage("agent-a", "agent-b", "", "hello b", "")
	require.NoError(t, err)
	assert.NotEmpty(t, msgID)

	msg, err := s.GetMessage(msgID)
	require.NoError(t, err)
	assert.Equal(t, "agent-a", msg.FromAgent)
	assert.Equal(t, "agent-b", msg.ToAgent)
	assert.Equal(t, "", msg.Group)
	assert.Equal(t, "hello b", msg.Content)

	unread, err := s.GetUnreadMessages("agent-b", 10)
	require.NoError(t, err)
	assert.Len(t, unread, 1)
	assert.Equal(t, "hello b", unread[0].Content)
}

func TestSaveAndGetGroupMessage(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("sender", []string{"team"}))
	require.NoError(t, s.RegisterAgent("member1", []string{"team"}))
	require.NoError(t, s.RegisterAgent("member2", []string{"team"}))
	require.NoError(t, s.RegisterAgent("outsider", []string{"other"}))

	msgID, err := s.SaveMessage("sender", "", "team", "hello team", "")
	require.NoError(t, err)
	assert.NotEmpty(t, msgID)

	// member1 should see the group message as unread
	unread1, err := s.GetUnreadMessages("member1", 10)
	require.NoError(t, err)
	assert.Len(t, unread1, 1)
	assert.Equal(t, "hello team", unread1[0].Content)

	// member2 should also see it
	unread2, err := s.GetUnreadMessages("member2", 10)
	require.NoError(t, err)
	assert.Len(t, unread2, 1)

	// sender should NOT see own message as unread
	unreadSender, err := s.GetUnreadMessages("sender", 10)
	require.NoError(t, err)
	assert.Len(t, unreadSender, 0)

	// outsider should NOT see it (not in group)
	unreadOutsider, err := s.GetUnreadMessages("outsider", 10)
	require.NoError(t, err)
	assert.Len(t, unreadOutsider, 0)
}

func TestMarkRead(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("agent-a", nil))
	require.NoError(t, s.RegisterAgent("agent-b", nil))

	msgID, err := s.SaveMessage("agent-a", "agent-b", "", "hello", "")
	require.NoError(t, err)

	unread, err := s.GetUnreadMessages("agent-b", 10)
	require.NoError(t, err)
	assert.Len(t, unread, 1)

	err = s.MarkRead("agent-b", []string{msgID})
	require.NoError(t, err)

	unread, err = s.GetUnreadMessages("agent-b", 10)
	require.NoError(t, err)
	assert.Len(t, unread, 0)
}

func TestListAgents(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("agent-a", []string{"dev"}))
	require.NoError(t, s.RegisterAgent("agent-b", []string{"ops"}))

	agents, err := s.ListAgents()
	require.NoError(t, err)
	assert.Len(t, agents, 2)

	names := map[string]bool{}
	for _, a := range agents {
		names[a.Name] = true
	}
	assert.True(t, names["agent-a"])
	assert.True(t, names["agent-b"])
}

func TestSetAgentOffline(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("agent-a", nil))

	agent, err := s.GetAgent("agent-a")
	require.NoError(t, err)
	assert.Equal(t, "online", agent.Status)

	err = s.SetAgentStatus("agent-a", "offline")
	require.NoError(t, err)

	agent, err = s.GetAgent("agent-a")
	require.NoError(t, err)
	assert.Equal(t, "offline", agent.Status)
}

func TestGetGroupMembers(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("agent-a", []string{"dev", "ops"}))
	require.NoError(t, s.RegisterAgent("agent-b", []string{"dev"}))
	require.NoError(t, s.RegisterAgent("agent-c", []string{"ops"}))

	members, err := s.GetGroupMembers("dev")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"agent-a", "agent-b"}, members)
}

func TestListGroups(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("agent-a", []string{"dev", "ops"}))
	require.NoError(t, s.RegisterAgent("agent-b", []string{"dev"}))

	groups, err := s.ListGroups()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"dev", "ops"}, groups)
}

func TestGetRecentMessages(t *testing.T) {
	s := openTestStore(t)

	require.NoError(t, s.RegisterAgent("agent-a", nil))
	require.NoError(t, s.RegisterAgent("agent-b", nil))

	// Save 5 messages
	for i := 0; i < 5; i++ {
		_, err := s.SaveMessage("agent-a", "agent-b", "", "msg", "")
		require.NoError(t, err)
	}

	// Get 3 most recent
	msgs, err := s.GetRecentMessages(3)
	require.NoError(t, err)
	assert.Len(t, msgs, 3)

	// They should be ordered by created_at DESC (newest first)
	for i := 1; i < len(msgs); i++ {
		assert.False(t, msgs[i].CreatedAt.After(msgs[i-1].CreatedAt))
	}
}
