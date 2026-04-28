// Package injector formats messages and injects them into tmux panes
// via send-keys, enabling agents running in tmux to receive notifications.
package injector

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/keepmind9/agent-chat/internal/notify"
	"github.com/keepmind9/agent-chat/pkg/protocol"
)

// Compile-time check that Injector implements notify.Notifier.
var _ notify.Notifier = (*Injector)(nil)

// FormatDirectMessage formats a direct (1-to-1) message for tmux injection.
// Instruction first, variable content last — so agents always see the action
// even if the message body is long.
// For replies (InReplyTo set), adds a no-auto-reply directive to prevent loops.
func FormatDirectMessage(msg *protocol.Message) string {
	if msg.InReplyTo != "" {
		return fmt.Sprintf(
			`[agent-chat] REPLY received. Do NOT auto-reply — wait for human user to explicitly ask you to respond. From %s: "%s" (reply to %s)`,
			msg.FromAgent, msg.Content, msg.InReplyTo,
		)
	}
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

// GetTmuxPane returns the tmux pane ID for the current process.
// Checks TMUX_PANE env first, then falls back to the parent process's
// environment (needed for AI CLIs like Codex that don't forward TMUX_PANE
// to MCP child processes).
func GetTmuxPane() string {
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		return pane
	}
	return parentTmuxPane()
}

// parentTmuxPane reads TMUX_PANE from the parent process's environment.
// Works on Linux via /proc/<ppid>/environ; returns "" on other platforms.
func parentTmuxPane() string {
	ppid := os.Getppid()
	path := "/proc/" + strconv.Itoa(ppid) + "/environ"
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, entry := range splitNull(data) {
		if len(entry) >= len("TMUX_PANE=") && string(entry[:len("TMUX_PANE=")]) == "TMUX_PANE=" {
			return string(entry[len("TMUX_PANE="):])
		}
	}
	return ""
}

func splitNull(data []byte) [][]byte {
	var entries [][]byte
	for len(data) > 0 {
		i := 0
		for i < len(data) && data[i] != 0 {
			i++
		}
		if i > 0 {
			entries = append(entries, data[:i])
		}
		if i < len(data) {
			i++
		}
		data = data[i:]
	}
	return entries
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
// Text and Enter are sent as separate send-keys calls with a short delay
// to ensure compatibility with different AI CLI TUIs (Claude Code, Codex, etc.).
// Returns nil immediately when the injector is disabled.
func (inj *Injector) Inject(text string) error {
	if !inj.enabled {
		return nil
	}
	if err := exec.Command("tmux", "send-keys", "-t", inj.pane, text).Run(); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return exec.Command("tmux", "send-keys", "-t", inj.pane, "Enter").Run()
}

// Notify formats msg according to its type (group or direct)
// and injects the result into the tmux pane.
func (inj *Injector) Notify(msg *protocol.Message) error {
	var formatted string
	if msg.Group != "" {
		formatted = FormatGroupMessage(msg)
	} else {
		formatted = FormatDirectMessage(msg)
	}
	return inj.Inject(formatted)
}
