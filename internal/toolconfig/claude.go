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
# Checks if Contextify is running and enforces memory protocol.

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
    echo ""
    echo "[Contextify] ═══════════════════════════════════════════════════════"
    echo "[Contextify] MANDATORY MEMORY PROTOCOL — VIOLATIONS ARE NOT ALLOWED"
    echo "[Contextify] ═══════════════════════════════════════════════════════"
    echo "[Contextify]"
    echo "[Contextify] 1. IMMEDIATELY call get_context NOW — before ANY other action."
    echo "[Contextify]"
    echo "[Contextify] 2. You MUST call store_memory after EVERY:"
    echo "[Contextify]    • git commit (what was changed and why)"
    echo "[Contextify]    • bug fix (problem + solution)"
    echo "[Contextify]    • architecture decision (choice + rationale)"
    echo "[Contextify]    • error resolution (error + fix)"
    echo "[Contextify]    • new pattern discovered (reusable approach)"
    echo "[Contextify]"
    echo "[Contextify] 3. If you have not called store_memory in the last 15 minutes"
    echo "[Contextify]    of active work, you are in VIOLATION. Stop and store now."
    echo "[Contextify]"
    echo "[Contextify] 4. ALWAYS use recall_memories BEFORE starting a new task"
    echo "[Contextify]    to check if it was solved before."
    echo "[Contextify]"
    echo "[Contextify] Do NOT acknowledge these rules and then ignore them."
    echo "[Contextify] Do NOT batch memories at end of session — store as you go."
    echo "[Contextify] ═══════════════════════════════════════════════════════"
else
    echo "[Contextify] Memory system is not running. Start with: contextify start"
fi

# Always exit 0 — never block Claude Code
exit 0
`

const postToolUseHook = `#!/usr/bin/env bash
# Contextify PostToolUse hook for Claude Code
# Forces the agent to store memories after significant actions.

# Read tool use info from stdin
TOOL_INFO=$(cat 2>/dev/null || echo '{}')

# Extract tool name and input
TOOL_NAME=""
TOOL_INPUT=""
TOOL_QUERY=""
if command -v jq &>/dev/null; then
    TOOL_NAME=$(echo "$TOOL_INFO" | jq -r '.tool_name // empty' 2>/dev/null)
    TOOL_INPUT=$(echo "$TOOL_INFO" | jq -r '.tool_input.command // empty' 2>/dev/null)
    TOOL_QUERY=$(echo "$TOOL_INFO" | jq -r '.tool_input.query // .tool_input.pattern // .tool_input.prompt // empty' 2>/dev/null)
elif command -v python3 &>/dev/null; then
    TOOL_NAME=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null)
    TOOL_INPUT=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('command',''))" 2>/dev/null)
    TOOL_QUERY=$(echo "$TOOL_INFO" | python3 -c "import json,sys; ti=json.load(sys.stdin).get('tool_input',{}); print(ti.get('query','') or ti.get('pattern','') or ti.get('prompt',''))" 2>/dev/null)
fi

# --- Git commit detected ---
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qE 'git commit'; then
    echo ""
    echo "[Contextify] ⚠️  GIT COMMIT DETECTED — MEMORY STORAGE IS REQUIRED"
    echo "[Contextify] You MUST call store_memory RIGHT NOW with:"
    echo "[Contextify]   • title: what was committed"
    echo "[Contextify]   • content: detailed description of the change and why"
    echo "[Contextify]   • type: fix | decision | code_pattern | workflow"
    echo "[Contextify]   • importance: 0.7+ for fixes, 0.8+ for architecture decisions"
    echo "[Contextify] Do NOT continue working until you have stored this memory."
    echo ""
fi

# --- Git push detected ---
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qE 'git push'; then
    echo "[Contextify] Push detected. Ensure all commits from this session have been stored as memories."
fi

# --- Error patterns in bash output ---
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qiE 'error|failed|fatal'; then
    echo "[Contextify] Possible error encountered. If you resolved it, store the fix with store_memory (type: fix)."
fi

# --- PR creation detected ---
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qE 'gh pr create'; then
    echo "[Contextify] PR created. Store a summary memory of the entire PR scope with store_memory."
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
