// Package notify defines the Notifier interface for delivering agent notifications.
// Implementations can range from tmux injection to webhook calls or MCP notifications.
package notify

import "github.com/keepmind9/agent-chat/pkg/protocol"

// Notifier delivers notifications about incoming messages to an agent.
type Notifier interface {
	// Notify delivers a notification for the given message.
	// Implementations handle formatting and delivery internally.
	Notify(msg *protocol.Message) error

	// IsEnabled reports whether this notifier is active and can deliver notifications.
	IsEnabled() bool
}

// NopNotifier is a no-op notifier that discards all notifications.
// Useful for testing or when no notification backend is available.
type NopNotifier struct{}

func (NopNotifier) Notify(_ *protocol.Message) error { return nil }
func (NopNotifier) IsEnabled() bool                  { return false }
