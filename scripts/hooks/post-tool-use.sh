#!/usr/bin/env bash
# Contextify PostToolUse hook for Claude Code
# Enforces store_memory after git commits.
# Installed by: install.sh → ~/.contextify/hooks/post-tool-use.sh

READY_FILE="/tmp/contextify-session-ready"
REQUIRED_FILE="/tmp/contextify-context-required"
STATE_FILE="/tmp/contextify-pending-memory.$$"

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

# --- State machine: track commit → store_memory flow ---

# Check if there's a pending memory from a previous commit
if [ -f /tmp/contextify-pending-memory ]; then
    # The previous tool was a commit. Check if THIS tool is store_memory
    if [ "$TOOL_NAME" = "mcp__contextify__store_memory" ]; then
        # Good — memory stored after commit
        rm -f /tmp/contextify-pending-memory
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

# Detect git commit and set pending flag
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qE 'git commit'; then
    touch /tmp/contextify-pending-memory
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo "🔴 [Contextify] COMMIT DETECTED — store_memory is REQUIRED"
    echo ""
    echo "   Your NEXT action MUST be store_memory."
    echo "   Do NOT proceed to any other task until memory is stored."
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
fi

# Always exit 0
exit 0
