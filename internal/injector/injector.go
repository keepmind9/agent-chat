// Package injector formats messages and injects them into tmux panes
// via send-keys, enabling agents running in tmux to receive notifications.
package injector

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/keepmind9/agent-chat/pkg/protocol"
)

// FormatDirectMessage formats a direct (1-to-1) message for tmux injection.
// Instruction first, variable content last — so agents always see the action
// even if the message body is long.
func FormatDirectMessage(msg *protocol.Message) string {
	return fmt.Sprintf(
		`[agent-chat] Call check_messages for details, then reply with send_message. New message from %s: "%s"`,
		msg.FromAgent, msg.Content,
	)
}

// FormatGroupMessage formats a group message for tmux injection.
func FormatGroupMessage(msg *protocol.Message) string {
	return fmt.Sprintf(
		`[agent-chat] Call check_messages for details, then reply with send_group_message or send_message. Group %s message from %s: "%s"`,
		msg.Group, msg.FromAgent, msg.Content,
	)
}

// GetTmuxPane returns the value of the TMUX_PANE environment variable.
func GetTmuxPane() string {
	return os.Getenv("TMUX_PANE")
}

// Injector sends formatted messages into a tmux pane via send-keys.
type Injector struct {
	pane    string
	enabled bool
}

// New creates an Injector targeting the given tmux pane.
// The injector is disabled when pane is empty.
func New(pane string) *Injector {
	return &Injector{
		pane:    pane,
		enabled: pane != "",
	}
}

// IsEnabled reports whether the injector can send to tmux.
func (inj *Injector) IsEnabled() bool {
	return inj.enabled
}

// Inject sends arbitrary text into the configured tmux pane.
// Returns nil immediately when the injector is disabled.
func (inj *Injector) Inject(text string) error {
	if !inj.enabled {
		return nil
	}
	cmd := exec.Command("tmux", "send-keys", "-t", inj.pane, text, "Enter")
	return cmd.Run()
}

// InjectMessage formats msg according to its type (group or direct)
// and injects the result into the tmux pane.
func (inj *Injector) InjectMessage(msg *protocol.Message) error {
	var formatted string
	if msg.Group != "" {
		formatted = FormatGroupMessage(msg)
	} else {
		formatted = FormatDirectMessage(msg)
	}
	return inj.Inject(formatted)
}
