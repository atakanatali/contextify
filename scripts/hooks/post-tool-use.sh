#!/usr/bin/env bash
# Contextify PostToolUse hook for Claude Code
# Enforces session readiness and auto-store for high-confidence events.
# Installed by: install.sh → ~/.contextify/hooks/post-tool-use.sh

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

# --- Session readiness enforcement (get_context must run first when required) ---
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

# --- State machine: track commit -> store_memory flow ---

# Check if there's a pending memory from a previous commit
if [ -f "$PENDING_FILE" ]; then
    # The previous tool was a commit. Check if THIS tool is store_memory
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

# --- Auto-store orchestration (high-confidence only) ---

# Detect git commit and auto-store
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
