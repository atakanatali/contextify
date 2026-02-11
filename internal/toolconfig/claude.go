package toolconfig

import (
	"os"
	"path/filepath"
	"strings"
)

const claudeCodePrompt = `<!-- contextify-memory-system -->
## Contextify Memory System

You have access to Contextify, a shared memory system across all your AI tools. Use it proactively.

### Session Start
At the START of every session, IMMEDIATELY call ` + "`get_context`" + ` with ` + "`project_id`" + ` set to the current working directory.
This loads all relevant memories (solutions, patterns, decisions) for the current project.

### When to Store Memories
Call ` + "`store_memory`" + ` automatically when ANY of these occur:
- **Bug fix completed** -> type: "fix", importance: 0.7+
- **Architecture decision** -> type: "decision", importance: 0.8
- **Error resolved** -> type: "error" + follow-up type: "solution"
- **Pattern discovered** -> type: "code_pattern", importance: 0.6+
- **Workflow established** -> type: "workflow", importance: 0.5+

### When to Recall Memories
Call ` + "`recall_memories`" + ` BEFORE starting work when you encounter:
- An error message or stack trace
- A design question about the project
- A technology or library question
- Anything that might have been solved before

### Required Fields
- **title**: Specific and searchable (e.g., "Fix: PostgreSQL connection timeout in Docker")
- **content**: Detailed description including context and reasoning
- **type**: solution | problem | code_pattern | fix | error | workflow | decision | general
- **importance**: 0.8+ critical/permanent, 0.5-0.7 standard, 0.3-0.4 minor/temporary
- **tags**: Always include project name, technology, and category
- **agent_source**: "claude-code"
- **project_id**: Current working directory path
- **scope**: "project" for project-specific, "global" for cross-project knowledge

### Relationship Linking
When storing a fix or solution, link it to the original problem:
` + "```" + `
create_relationship(from_memory_id: solution_id, to_memory_id: problem_id, relationship: "SOLVES")
` + "```" + `

### Importance Guide
| Importance | When to Use | TTL |
|-----------|-------------|-----|
| 0.9-1.0 | Critical architecture decisions, security fixes | Permanent |
| 0.8 | Important patterns, recurring fixes | Permanent (auto) |
| 0.5-0.7 | Standard solutions, workflows | 24h, extended on access |
| 0.3-0.4 | Temporary notes, minor observations | 24h, expires if unused |

Do NOT wait to be asked. Memory operations are automatic and proactive.
<!-- /contextify-memory-system -->`

const sessionStartHook = `#!/usr/bin/env bash
# Contextify SessionStart hook for Claude Code
# Checks if Contextify is running and reminds the agent to load context.

CONTEXTIFY_URL="${CONTEXTIFY_URL:-http://localhost:8420}"

# Read session info from stdin (Claude Code provides JSON)
SESSION_INFO=$(cat 2>/dev/null || echo '{}')

# Extract cwd
CWD=""
if command -v jq &>/dev/null; then
    CWD=$(echo "$SESSION_INFO" | jq -r '.cwd // empty' 2>/dev/null)
elif command -v python3 &>/dev/null; then
    CWD=$(echo "$SESSION_INFO" | python3 -c "import json,sys; print(json.load(sys.stdin).get('cwd',''))" 2>/dev/null)
fi

# Check if Contextify is healthy
if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
    echo "[Contextify] Memory system is online."
    if [ -n "$CWD" ]; then
        echo "[Contextify] IMPORTANT: Call get_context with project_id=\"${CWD}\" to load project memories."
    else
        echo "[Contextify] IMPORTANT: Call get_context with the current project path to load project memories."
    fi
    echo "[Contextify] Store important findings with store_memory (agent_source: \"claude-code\")."
else
    echo "[Contextify] Memory system is not running. Start with: contextify start"
fi

# Always exit 0 — never block Claude Code
exit 0
`

const postToolUseHook = `#!/usr/bin/env bash
# Contextify PostToolUse hook for Claude Code
# Reminds the agent to store memories after git commits.

# Read tool use info from stdin
TOOL_INFO=$(cat 2>/dev/null || echo '{}')

# Extract tool name and input
TOOL_NAME=""
TOOL_INPUT=""
if command -v jq &>/dev/null; then
    TOOL_NAME=$(echo "$TOOL_INFO" | jq -r '.tool_name // empty' 2>/dev/null)
    TOOL_INPUT=$(echo "$TOOL_INFO" | jq -r '.tool_input.command // empty' 2>/dev/null)
elif command -v python3 &>/dev/null; then
    TOOL_NAME=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null)
    TOOL_INPUT=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('command',''))" 2>/dev/null)
fi

# Only remind after git commits
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qE 'git commit'; then
    echo "[Contextify] Commit detected. Consider storing what was fixed/added with store_memory."
fi

# Always exit 0
exit 0
`

func ConfigureClaudeCode(mcpURL string) error {
	settingsPath := expandPath("~/.claude/settings.json")
	claudeMDPath := expandPath("~/.claude/CLAUDE.md")
	hooksDir := expandPath("~/.contextify/hooks")

	// 1. Add MCP server to settings.json
	mcpConfig := map[string]any{
		"type": "streamableHttp",
		"url":  mcpURL,
	}
	if err := jsonSetNested(settingsPath, "mcpServers.contextify", mcpConfig); err != nil {
		return err
	}

	// 2. Install hooks
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return err
	}
	if err := writeExecutable(filepath.Join(hooksDir, "session-start.sh"), sessionStartHook); err != nil {
		return err
	}
	if err := writeExecutable(filepath.Join(hooksDir, "post-tool-use.sh"), postToolUseHook); err != nil {
		return err
	}

	// Add hooks to settings.json
	sessionStartCmd := expandPath("~/.contextify/hooks/session-start.sh")
	postToolUseCmd := expandPath("~/.contextify/hooks/post-tool-use.sh")
	if err := jsonAddHook(settingsPath, "SessionStart", sessionStartCmd); err != nil {
		return err
	}
	if err := jsonAddHook(settingsPath, "PostToolUse", postToolUseCmd); err != nil {
		return err
	}

	// 3. Append memory instructions to CLAUDE.md
	if err := appendClaudeMD(claudeMDPath, claudeCodePrompt); err != nil {
		return err
	}

	return nil
}

func UninstallClaudeCode() error {
	settingsPath := expandPath("~/.claude/settings.json")
	claudeMDPath := expandPath("~/.claude/CLAUDE.md")
	hooksDir := expandPath("~/.contextify/hooks")

	// Remove MCP server
	_ = jsonRemoveKey(settingsPath, "mcpServers.contextify")

	// Remove hooks
	sessionStartCmd := expandPath("~/.contextify/hooks/session-start.sh")
	postToolUseCmd := expandPath("~/.contextify/hooks/post-tool-use.sh")
	_ = jsonRemoveHook(settingsPath, "SessionStart", sessionStartCmd)
	_ = jsonRemoveHook(settingsPath, "PostToolUse", postToolUseCmd)
	_ = os.RemoveAll(hooksDir)

	// Remove CLAUDE.md marker block
	_ = removeClaudeMDBlock(claudeMDPath)

	return nil
}

func appendClaudeMD(path, content string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if already present
	if strings.Contains(string(existing), "<!-- contextify-memory-system -->") {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("\n" + content + "\n")
	return err
}

func removeClaudeMDBlock(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // file doesn't exist, nothing to do
	}

	content := string(data)
	startMarker := "<!-- contextify-memory-system -->"
	endMarker := "<!-- /contextify-memory-system -->"

	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)
	if startIdx == -1 || endIdx == -1 {
		return nil
	}

	// Remove everything from start marker to end marker (inclusive) plus surrounding newlines
	before := strings.TrimRight(content[:startIdx], "\n")
	after := strings.TrimLeft(content[endIdx+len(endMarker):], "\n")

	newContent := before
	if after != "" {
		newContent += "\n" + after
	}

	return os.WriteFile(path, []byte(newContent), 0644)
}

func writeExecutable(path, content string) error {
	return os.WriteFile(path, []byte(content), 0755)
}
