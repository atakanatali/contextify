package toolconfig

import (
	"os"
	"path/filepath"
)

const claudeChatInstructions = `# Contextify Memory System — Claude Chat (claude.ai)

Contextify is configured as a connector in your Claude.ai account.
The MCP tools are available when the Contextify connector is enabled in a conversation.

## How to enable Contextify in a conversation
1. Click the "+" button at the bottom of the chat
2. Select "Connectors"
3. Toggle ON the "Contextify" connector

## Memory Protocol
Once enabled, follow these rules:

### SESSION START
Call ` + "`get_context`" + ` with your project path as ` + "`project_id`" + ` at the start of every conversation.

### STORE MEMORIES
Call ` + "`store_memory`" + ` after:
- Completing a task or fixing a bug
- Making an architecture decision
- Discovering a reusable pattern
- Resolving an error

### RECALL MEMORIES
Call ` + "`recall_memories`" + ` before:
- Starting research on a new topic
- Making architecture decisions
- Troubleshooting an error

### Required fields
- **agent_source**: "claude-chat"
- **project_id**: Your project path or identifier
- **type**: solution | problem | code_pattern | fix | error | workflow | decision
- **importance**: 0.8+ permanent, 0.5-0.7 standard, 0.3-0.4 minor
`

// ConfigureClaudeChat saves instructions and prints guidance for manual setup.
// Claude Chat (claude.ai) requires manual configuration via Settings > Connectors.
// There is no programmatic way to add MCP servers to claude.ai.
func ConfigureClaudeChat(mcpURL string) error {
	instrPath := expandPath("~/.contextify/claude-chat-instructions.md")

	if err := os.MkdirAll(filepath.Dir(instrPath), 0755); err != nil {
		return err
	}

	content := claudeChatInstructions + "\n## MCP Server URL\n" +
		"Use this URL when adding the custom connector:\n" +
		"  " + mcpURL + "\n"

	return os.WriteFile(instrPath, []byte(content), 0644)
}

// UpdateClaudeChat is an alias for ConfigureClaudeChat (always overwrites).
func UpdateClaudeChat(mcpURL string) error {
	return ConfigureClaudeChat(mcpURL)
}

// UninstallClaudeChat removes the instructions file.
func UninstallClaudeChat() error {
	_ = os.Remove(expandPath("~/.contextify/claude-chat-instructions.md"))
	return nil
}

func checkClaudeChatStatus() ToolStatus {
	instrPath := expandPath("~/.contextify/claude-chat-instructions.md")
	if fileExists(instrPath) {
		return StatusConfigured
	}
	return StatusNotConfigured
}
