#!/usr/bin/env bash
# Contextify PostToolUse hook for Claude Code
# Reminds the agent to store memories after git commits.
# Installed by: install.sh → ~/.contextify/hooks/post-tool-use.sh

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
