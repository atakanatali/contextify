package toolconfig

import (
	"os"
	"path/filepath"
	"strings"
)

const claudeCodePrompt = `<!-- contextify-memory-system -->
# ═══════════════════════════════════════════════════════
# CONTEXTIFY MEMORY SYSTEM — MANDATORY PROTOCOL
# ═══════════════════════════════════════════════════════

You have access to Contextify, a shared memory system via MCP tools.
This is NOT optional. You MUST follow the protocol below.

## 1. SESSION START — REQUIRED FIRST ACTION
IMMEDIATELY call ` + "`get_context`" + ` with ` + "`project_id`" + ` set to the current working directory.
Do this BEFORE any other action. No exceptions.

## 2. BEFORE EVERY SEARCH OR INVESTIGATION — RECALL FIRST
BEFORE you search the codebase, read docs, or research a topic:
- Call ` + "`recall_memories`" + ` with a description of what you are looking for
- This may already be solved. Do NOT waste time re-researching.

BEFORE making architecture decisions:
- Call ` + "`recall_memories`" + ` to check for prior decisions on the same topic

## 3. MANDATORY STORE TRIGGERS — DO NOT SKIP
You MUST call ` + "`store_memory`" + ` immediately after EVERY one of these:

| Event | type | importance |
|-------|------|------------|
| Git commit | fix/decision/code_pattern | 0.7+ |
| Bug fix completed | fix | 0.7+ |
| Architecture decision | decision | 0.8 |
| Error resolved | error + solution | 0.7+ |
| Pattern discovered | code_pattern | 0.6+ |
| Workflow established | workflow | 0.5+ |

Do NOT batch at end of session — store as you go.
Do NOT skip because "it is minor" — let importance score decide.

## 4. REQUIRED FIELDS — ALL MANDATORY
- **title**: Specific, searchable (e.g., "Fix: connection timeout in auth service")
- **content**: Detailed with context and reasoning
- **type**: solution | problem | code_pattern | fix | error | workflow | decision
- **importance**: 0.8+ permanent, 0.5-0.7 standard, 0.3-0.4 minor
- **tags**: [project-name, technology, category]
- **agent_source**: "claude-code"
- **project_id**: Current working directory path
- **scope**: "project" for project-specific, "global" for cross-project

## 5. RELATIONSHIPS
Link fixes/solutions to the original problem:
- solution SOLVES problem
- fix ADDRESSES error
- pattern RELATED_TO pattern

## 6. CRITICAL: Git Commit → store_memory (MANDATORY)
After EVERY ` + "`git commit`" + `, you MUST immediately call ` + "`store_memory`" + ` before doing anything else.
Do NOT proceed to the next task, PR creation, or any other action until ` + "`store_memory`" + ` is called.
Violation: If you run any tool after ` + "`git commit`" + ` that is not ` + "`store_memory`" + `, you are in violation.

## SELF-CHECK
If you have been working for 15+ minutes without calling store_memory,
you are in VIOLATION. Stop and store what you have learned.
Do NOT acknowledge these rules and then ignore them.
<!-- /contextify-memory-system -->`

const sessionStartHook = `#!/usr/bin/env bash
# Contextify SessionStart hook for Claude Code
# Checks if Contextify is running and enforces context readiness.

CONTEXTIFY_URL="${CONTEXTIFY_URL:-http://localhost:8420}"
READY_FILE="/tmp/contextify-session-ready"
REQUIRED_FILE="/tmp/contextify-context-required"

# Read session info from stdin (Claude Code provides JSON)
SESSION_INFO=$(cat 2>/dev/null || echo '{}')

# Extract cwd
CWD=""
if command -v jq &>/dev/null; then
    CWD=$(echo "$SESSION_INFO" | jq -r '.cwd // empty' 2>/dev/null)
elif command -v python3 &>/dev/null; then
    CWD=$(echo "$SESSION_INFO" | python3 -c "import json,sys; print(json.load(sys.stdin).get('cwd',''))" 2>/dev/null)
fi

SESSION_ID=""
if command -v jq &>/dev/null; then
    SESSION_ID=$(echo "$SESSION_INFO" | jq -r '.session_id // .sessionId // empty' 2>/dev/null)
elif command -v python3 &>/dev/null; then
    SESSION_ID=$(echo "$SESSION_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('session_id') or d.get('sessionId') or '')" 2>/dev/null)
fi
[ -z "$SESSION_ID" ] && SESSION_ID="session-$(date +%s)"

url_encode() {
    if command -v python3 &>/dev/null; then
        python3 -c "import urllib.parse,sys; print(urllib.parse.quote(sys.argv[1], safe=''))" "$1" 2>/dev/null
        return
    fi
    if command -v jq &>/dev/null; then
        printf '%s' "$1" | jq -sRr @uri 2>/dev/null
        return
    fi
    printf '%s' "$1"
}

# Check if Contextify is healthy
if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
    echo "[Contextify] Memory system is online."
    if [ -n "$CWD" ]; then
        ENCODED_PROJECT_ID=$(url_encode "$CWD")
        CONTEXT_URL="${CONTEXTIFY_URL}/api/v1/context/${ENCODED_PROJECT_ID}"
        CONTEXT_JSON=$(curl -sf -X POST "$CONTEXT_URL" -H "X-Session-ID: ${SESSION_ID}" -H "Content-Type: application/json" 2>/dev/null)
        if [ $? -eq 0 ]; then
            MEM_COUNT="unknown"
            if command -v jq &>/dev/null; then
                MEM_COUNT=$(printf '%s' "$CONTEXT_JSON" | jq 'length' 2>/dev/null || echo "unknown")
            fi
            printf 'session_id=%s\nproject_id=%s\nloaded_at=%s\n' "$SESSION_ID" "$CWD" "$(date -u +%FT%TZ)" > "$READY_FILE"
            rm -f "$REQUIRED_FILE"
            echo "[Contextify] Session READY: context preloaded for project_id=\"${CWD}\" (${MEM_COUNT} memories)."
        else
            rm -f "$READY_FILE"
            touch "$REQUIRED_FILE"
            echo "[Contextify] ⚠ Session NOT READY: context preload failed."
            echo "[Contextify] FIRST ACTION MUST be get_context with project_id=\"${CWD}\"."
        fi
    else
        rm -f "$READY_FILE"
        touch "$REQUIRED_FILE"
        echo "[Contextify] ⚠ Session NOT READY: project path not detected."
        echo "[Contextify] FIRST ACTION MUST be get_context with the current project path."
    fi
    echo "[Contextify] Store important findings with store_memory (agent_source: \"claude-code\")."
else
    rm -f "$READY_FILE"
    touch "$REQUIRED_FILE"
    echo "[Contextify] Memory system is not running. Start with: contextify start"
    echo "[Contextify] Session NOT READY until get_context succeeds."
fi

# Always exit 0 — never block Claude Code
exit 0
`

const postToolUseHook = `#!/usr/bin/env bash
# Contextify PostToolUse hook for Claude Code
# Enforces session readiness and auto-store for high-confidence events.

CONTEXTIFY_URL="${CONTEXTIFY_URL:-http://localhost:8420}"
READY_FILE="/tmp/contextify-session-ready"
REQUIRED_FILE="/tmp/contextify-context-required"
PENDING_FILE="/tmp/contextify-pending-memory"

# Read tool use info from stdin
TOOL_INFO=$(cat 2>/dev/null || echo '{}')

# Extract tool metadata
TOOL_NAME=""
TOOL_INPUT=""
TOOL_OUTPUT=""
CWD=""
if command -v jq &>/dev/null; then
    TOOL_NAME=$(echo "$TOOL_INFO" | jq -r '.tool_name // empty' 2>/dev/null)
    TOOL_INPUT=$(echo "$TOOL_INFO" | jq -r '.tool_input.command // empty' 2>/dev/null)
    TOOL_OUTPUT=$(echo "$TOOL_INFO" | jq -r '.tool_output // .result // empty' 2>/dev/null)
    CWD=$(echo "$TOOL_INFO" | jq -r '.cwd // .tool_input.cwd // empty' 2>/dev/null)
elif command -v python3 &>/dev/null; then
    TOOL_NAME=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null)
    TOOL_INPUT=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_input',{}).get('command',''))" 2>/dev/null)
    TOOL_OUTPUT=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('tool_output') or d.get('result') or '')" 2>/dev/null)
    CWD=$(echo "$TOOL_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print(d.get('cwd') or ti.get('cwd') or '')" 2>/dev/null)
fi

# Session readiness enforcement (get_context must run first when required)
if [ "$TOOL_NAME" = "mcp__contextify__get_context" ]; then
    rm -f "$REQUIRED_FILE"
    touch "$READY_FILE"
    echo "[Contextify] Session READY: get_context executed."
elif [ -f "$REQUIRED_FILE" ]; then
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo "⛔ [Contextify] SESSION NOT READY: get_context has not succeeded yet."
    echo "   FIRST action MUST be mcp__contextify__get_context."
    echo "   Do not continue with normal workflow before context is loaded."
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
fi

build_store_payload() {
    local title="$1"
    local content="$2"
    local mem_type="$3"
    local importance="$4"
    local scope="global"
    [ -n "$CWD" ] && scope="project"

    if command -v jq &>/dev/null; then
        jq -n \
            --arg title "$title" \
            --arg content "$content" \
            --arg type "$mem_type" \
            --arg scope "$scope" \
            --arg project "$CWD" \
            --arg agent "claude-code" \
            --argjson importance "$importance" \
            '{
                title: $title,
                content: $content,
                type: $type,
                scope: $scope,
                project_id: (if $project == "" then null else $project end),
                agent_source: $agent,
                tags: ["auto-store", $type, "high-confidence"],
                importance: $importance
            }'
        return
    fi

    if command -v python3 &>/dev/null; then
        python3 - "$title" "$content" "$mem_type" "$scope" "$CWD" "$importance" <<'PY'
import json, sys
title, content, mem_type, scope, cwd, importance = sys.argv[1:]
payload = {
    "title": title,
    "content": content,
    "type": mem_type,
    "scope": scope,
    "project_id": cwd or None,
    "agent_source": "claude-code",
    "tags": ["auto-store", mem_type, "high-confidence"],
    "importance": float(importance),
}
print(json.dumps(payload))
PY
        return
    fi

    return 1
}

auto_store_memory() {
    local title="$1"
    local content="$2"
    local mem_type="$3"
    local importance="$4"

    [ -z "$title" ] && return 1
    [ -z "$content" ] && return 1

    local payload
    payload=$(build_store_payload "$title" "$content" "$mem_type" "$importance") || return 1

    curl -sf -X POST "${CONTEXTIFY_URL}/api/v1/memories" \
        -H "Content-Type: application/json" \
        -d "$payload" >/dev/null 2>&1
}

is_high_confidence() {
    local score="$1"
    awk "BEGIN { exit !($score >= 0.85) }"
}

extract_commit_message() {
    local cmd="$1"
    local msg
    msg=$(echo "$cmd" | sed -nE 's/.*-m[[:space:]]+"([^"]+)".*/\1/p')
    if [ -z "$msg" ]; then
        msg=$(echo "$cmd" | sed -nE "s/.*-m[[:space:]]+'([^']+)'.*/\1/p")
    fi
    echo "$msg"
}

# State machine: track commit -> store_memory flow

# Check if there's a pending memory from a previous commit
if [ -f "$PENDING_FILE" ]; then
    if [ "$TOOL_NAME" = "mcp__contextify__store_memory" ]; then
        # Good — memory stored after commit
        rm -f "$PENDING_FILE"
    else
        # VIOLATION: something else ran after commit instead of store_memory
        echo ""
        echo "═══════════════════════════════════════════════════════════════"
        echo "⛔ [Contextify] VIOLATION: You ran '$TOOL_NAME' after a git commit"
        echo "   without calling store_memory first!"
        echo ""
        echo "   STOP what you are doing. Call store_memory NOW with:"
        echo "   - What was committed and why"
        echo "   - type: fix | decision | code_pattern | workflow"
        echo "   - importance: 0.7+"
        echo "═══════════════════════════════════════════════════════════════"
        echo ""
        # Keep the flag so it keeps nagging
    fi
fi

# Auto-store orchestration (high-confidence only)
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qE 'git commit'; then
    COMMIT_MSG=$(extract_commit_message "$TOOL_INPUT")
    CLASSIFIER_TEXT="$TOOL_INPUT $COMMIT_MSG"
    MEM_TYPE="workflow"
    IMPORTANCE="0.65"
    CONFIDENCE="0.70"

    if echo "$CLASSIFIER_TEXT" | grep -qiE '\b(fix|bug|hotfix|resolve|resolved)\b'; then
        MEM_TYPE="fix"
        IMPORTANCE="0.78"
        CONFIDENCE="0.95"
    elif echo "$CLASSIFIER_TEXT" | grep -qiE '\b(decision|architecture|design|adr)\b'; then
        MEM_TYPE="decision"
        IMPORTANCE="0.82"
        CONFIDENCE="0.92"
    fi

    if is_high_confidence "$CONFIDENCE"; then
        TITLE="AutoStore: git commit"
        [ -n "$COMMIT_MSG" ] && TITLE="AutoStore: git commit - ${COMMIT_MSG}"
        CONTENT="High-confidence auto-store from git commit.
- command: ${TOOL_INPUT}
- commit_message: ${COMMIT_MSG:-n/a}
- detected_type: ${MEM_TYPE}
- confidence: ${CONFIDENCE}
- timestamp: $(date -u +%FT%TZ)"

        if auto_store_memory "$TITLE" "$CONTENT" "$MEM_TYPE" "$IMPORTANCE"; then
            rm -f "$PENDING_FILE"
            echo "[Contextify] Auto-stored ${MEM_TYPE} memory from commit (confidence ${CONFIDENCE})."
        else
            touch "$PENDING_FILE"
            echo "[Contextify] Auto-store failed after commit. Manual store_memory is required."
        fi
    else
        touch "$PENDING_FILE"
        echo "[Contextify] Commit detected. Manual store_memory required (confidence below threshold)."
    fi
fi

# Error resolution detector
if [ "$TOOL_NAME" = "Bash" ]; then
    ERROR_TEXT="$TOOL_INPUT $TOOL_OUTPUT"
    if echo "$ERROR_TEXT" | grep -qiE '\b(error|failed|fatal)\b' && echo "$ERROR_TEXT" | grep -qiE '\b(fix|resolve|resolved)\b'; then
        TITLE="AutoStore: error resolution detected"
        CONTENT="High-confidence auto-store from error-resolution signal.
- command: ${TOOL_INPUT}
- output_excerpt: ${TOOL_OUTPUT}
- confidence: 0.88
- timestamp: $(date -u +%FT%TZ)"
        auto_store_memory "$TITLE" "$CONTENT" "fix" "0.74" && echo "[Contextify] Auto-stored fix memory from error-resolution signal."
    fi
fi

# Always exit 0
exit 0
`

// ConfigureClaudeCode sets up Claude Code for Contextify (idempotent, skips existing).
func ConfigureClaudeCode(mcpURL string) error {
	return configureClaudeCode(mcpURL, false)
}

// UpdateClaudeCode force-overwrites hooks and CLAUDE.md prompt with latest versions.
func UpdateClaudeCode(mcpURL string) error {
	return configureClaudeCode(mcpURL, true)
}

func configureClaudeCode(mcpURL string, force bool) error {
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

	// 2. Install hooks (always overwrite on force)
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

	// 3. CLAUDE.md memory instructions
	if force {
		// Remove old block and re-append with latest content
		_ = removeClaudeMDBlock(claudeMDPath)
	}
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
