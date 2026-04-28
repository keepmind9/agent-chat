# E2E Test Plan

Fully automated end-to-end test using real Claude Code and Codex instances in tmux sessions.

## Test Architecture

```
tmux:agent-demo          agent-chat server         tmux:agent-demo2         tmux:agent-demo3
┌──────────────────┐    ┌─────────────┐    ┌──────────────────┐    ┌──────────────────┐
│ claude (interactive) │◄─►│  HTTP + WS  │◄─►│ claude (interactive) │◄─►│ codex (interactive)  │
│ MCP plugin           │◄─►│  :8080      │◄─►│ MCP plugin           │◄─►│ MCP plugin           │
│ tmux push injection  │    │  SQLite     │    │ tmux push injection  │    │ tmux push injection  │
└──────────────────┘    └─────────────┘    └──────────────────┘    └──────────────────┘
demo/ (Claude)                              demo2/ (Claude)                              demo3/ (Codex)
```

Three agents in two categories:
- **Claude Code** (demo, demo2) — `claude` CLI, tmux `send-keys text Enter` combined works
- **Codex** (demo3) — `codex` CLI, tmux send-keys must be split into two calls (text, then Enter)

## Automation Tools

| Tool | Purpose |
|------|---------|
| `tmux send-keys` | Type prompts into agent sessions |
| `tmux capture-pane` | Read output for verification |
| `curl` | Query HTTP API for cross-validation |
| Polling loop | Wait for agent responses and tool calls |

## Codex-Specific Notes

Codex TUI requires special tmux handling:
- **Split send-keys**: Must send text and Enter as separate `tmux send-keys` commands with ~100ms gap
- **No TMUX_PANE**: Codex doesn't pass `TMUX_PANE` to MCP child processes; the plugin reads `/proc/<ppid>/environ` as fallback (Linux-only)
- **Config**: MCP config is in `~/.codex/config.toml` or `.codex/config.toml` (project-level), not `.mcp.json`

## Prerequisites

- agent-chat binary built (`make build`)
- Three working directories for agents (e.g. `~/demo`, `~/demo2`, `~/demo3`)

## Execution Steps

### Phase 0 — Pre-configuration

1. Update `settings.local.json` in Claude Code demo dirs — add MCP tool permissions to `permissions.allow`:
   ```json
   "mcp__agent-chat__register",
   "mcp__agent-chat__send_message",
   "mcp__agent-chat__send_group_message",
   "mcp__agent-chat__check_messages",
   "mcp__agent-chat__read_messages",
   "mcp__agent-chat__list_agents",
   "mcp__agent-chat__list_groups"
   ```
2. Update `.mcp.json` in both Claude demo dirs — add `AGENT_GROUPS` env var for group testing:
   ```json
   "AGENT_GROUPS": "dev-team"
   ```
3. Configure Codex MCP in `~/.codex/config.toml` (or project-level `.codex/config.toml`):
   ```toml
   [mcp_servers.agent-chat]
   command = "/path/to/agent-chat"
   args = ["mcp"]
   [mcp_servers.agent-chat.env]
   AGENT_CHAT_SERVER = "http://localhost:8080"
   AGENT_NAME = "agent-codex"
   AGENT_GROUPS = "dev-team"
   ```
4. Trust the Codex project directory in `~/.codex/config.toml`:
   ```toml
   [projects."/path/to/demo3"]
   trust_level = "trusted"
   ```
5. Clean up existing tmux sessions (`agent-demo`, `agent-demo2`, `agent-demo3`) and old SQLite database.

### Phase 1 — Start Server

1. Launch `./agent-chat server` in background.
2. `curl /api/agents` — confirm empty initial state.

### Phase 2 — Launch Agents

1. Create tmux session `agent-demo`, cd to your first demo directory, start `claude`.
2. Create tmux session `agent-demo2`, cd to your second demo directory, start `claude`.
3. Create tmux session `agent-demo3`, cd to your Codex demo directory, start `codex`.
4. Poll `curl /api/agents` until all three agents appear (auto-registered by MCP plugin).

### Phase 3 — Test Direct Messaging (Claude → Claude)

1. In `agent-demo` tmux, send prompt:
   ```
   send a message to agent-demo2 saying 'Hello from demo'
   ```
2. Wait for Claude to call `send_message` MCP tool (poll tmux pane for tool call output).
3. `curl /api/messages` — verify message stored on server.
4. Capture `agent-demo2` tmux pane — verify WebSocket push notification injected:
   ```
   [agent-chat] Call check_messages for details, then reply with send_message. New message from agent-demo: "Hello from demo"
   ```
5. In `agent-demo2` tmux, send prompt:
   ```
   check your unread messages
   ```
6. Verify response contains the message from demo.

**Pass criteria:** message appears in API response AND agent-demo2 pane shows the notification.

### Phase 4 — Test Read + Reply (Claude → Claude)

1. In `agent-demo2` tmux, send prompt:
   ```
   mark your messages as read and reply 'Got it, thanks!'
   ```
2. `curl /api/messages` — verify message status changed to `read=true`.
3. Verify reply message created on server with sender `agent-demo2`, recipient `agent-demo`, and `in_reply_to` set to the original message ID.
4. Capture `agent-demo` tmux pane — verify anti-loop reply notification:
   ```
   [agent-chat] REPLY received from agent-demo2: "Got it, thanks!" (reply to msg-xxx). Do NOT auto-reply...
   ```

**Pass criteria:** original message marked read, reply message exists with `in_reply_to`, anti-loop notification shown.

### Phase 5 — Test Direct Messaging (Claude → Codex)

1. In `agent-demo` tmux, send prompt:
   ```
   send a message to agent-demo3 saying 'Hey Codex, can you help with the API?'
   ```
2. `curl /api/messages` — verify message stored with `to_agent=agent-demo3`.
3. Capture `agent-demo3` tmux pane — verify push notification injected into Codex session.
4. In `agent-demo3` tmux, send prompt (use split send-keys for Codex):
   ```
   check your unread messages
   ```
5. Verify Codex calls `check_messages` and sees the message.

**Pass criteria:** message stored, Codex receives notification and reads it.

### Phase 6 — Test Codex Sending Messages (Codex → Claude)

1. In `agent-demo3` tmux, send prompt:
   ```
   send a message to agent-demo saying 'Sure, I can help with the API'
   ```
2. `curl /api/messages` — verify message stored with `from_agent=agent-demo3`.
3. Capture `agent-demo` tmux pane — verify notification from Codex.

**Pass criteria:** Codex successfully sends a message, Claude agent receives notification.

### Phase 7 — Test Group Messaging (3 agents)

1. In `agent-demo` tmux, send prompt:
   ```
   send a group message to 'dev-team' saying 'Standup in 5 minutes'
   ```
2. `curl /api/messages` — verify group message stored.
3. Capture `agent-demo2` tmux pane — verify group message push notification injected.
4. Capture `agent-demo3` tmux pane — verify group message push notification injected into Codex.

**Pass criteria:** group message visible in API and delivered to both other group members via WebSocket.

### Phase 8 — Test Query Operations

1. In `agent-demo` tmux, send prompt:
   ```
   list all registered agents
   ```
2. Verify response includes all three agents: `agent-demo`, `agent-demo2`, `agent-demo3`.
3. Send prompt:
   ```
   list all available groups
   ```
4. Verify response includes `dev-team`.

**Pass criteria:** all three agents listed, group listed.

### Phase 9 — Cleanup + Report

1. Kill tmux sessions (`agent-demo`, `agent-demo2`, `agent-demo3`).
2. Stop agent-chat server process.
3. Remove test SQLite database file.
4. Print test summary:
   ```
   [PASS] Phase 3: Direct messaging (Claude → Claude)
   [PASS] Phase 4: Read + reply with anti-loop (Claude → Claude)
   [PASS] Phase 5: Direct messaging (Claude → Codex)
   [PASS] Phase 6: Codex sending messages (Codex → Claude)
   [PASS] Phase 7: Group messaging (3 agents)
   [PASS] Phase 8: Query operations
   ```

## Failure Handling

- If any phase times out (60s per prompt), capture current tmux pane output and server logs for debugging.
- If MCP tool permissions block execution, verify `settings.local.json` allow list (Claude) or `config.toml` trust level (Codex).
- If Codex doesn't receive tmux injection, verify:
  - `TMUX_PANE` env or `/proc/<ppid>/environ` fallback is working
  - Split send-keys is used (text first, then Enter after 100ms)
- If server returns errors, check server stderr output.
