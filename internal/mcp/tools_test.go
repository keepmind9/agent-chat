package mcp

import (
	"testing"

	mcptool "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestBuildRegisterTool(t *testing.T) {
	tool := BuildRegisterTool()
	assert.Equal(t, "register", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "name")
	assert.Contains(t, tool.InputSchema.Properties, "groups")
}

func TestBuildSendMessageTool(t *testing.T) {
	tool := BuildSendMessageTool()
	assert.Equal(t, "send_message", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "to")
	assert.Contains(t, tool.InputSchema.Properties, "content")
}

func TestBuildSendGroupMessageTool(t *testing.T) {
	tool := BuildSendGroupMessageTool()
	assert.Equal(t, "send_group_message", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "group")
	assert.Contains(t, tool.InputSchema.Properties, "content")
}

func TestBuildCheckMessagesTool(t *testing.T) {
	tool := BuildCheckMessagesTool()
	assert.Equal(t, "check_messages", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "limit")
}

func TestBuildReadMessagesTool(t *testing.T) {
	tool := BuildReadMessagesTool()
	assert.Equal(t, "read_messages", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "message_ids")
}

func TestBuildListAgentsTool(t *testing.T) {
	tool := BuildListAgentsTool()
	assert.Equal(t, "list_agents", tool.Name)
	assert.NotEmpty(t, tool.Description)
}

func TestBuildListGroupsTool(t *testing.T) {
	tool := BuildListGroupsTool()
	assert.Equal(t, "list_groups", tool.Name)
	assert.NotEmpty(t, tool.Description)
}

func TestRequiredParams(t *testing.T) {
	tests := []struct {
		name            string
		tool            mcptool.Tool
		expectedRequire []string
	}{
		{
			name:            "register requires name",
			tool:            BuildRegisterTool(),
			expectedRequire: []string{"name"},
		},
		{
			name:            "send_message requires to and content",
			tool:            BuildSendMessageTool(),
			expectedRequire: []string{"to", "content"},
		},
		{
			name:            "send_group_message requires group and content",
			tool:            BuildSendGroupMessageTool(),
			expectedRequire: []string{"group", "content"},
		},
		{
			name:            "check_messages has no required params",
			tool:            BuildCheckMessagesTool(),
			expectedRequire: nil,
		},
		{
			name:            "read_messages requires message_ids",
			tool:            BuildReadMessagesTool(),
			expectedRequire: []string{"message_ids"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedRequire, tt.tool.InputSchema.Required)
		})
	}
}
