#!/usr/bin/env bash
# Contextify SessionStart hook for Claude Code
# Checks if Contextify is running and reminds the agent to load context.
# Installed by: install.sh → ~/.contextify/hooks/session-start.sh

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
    echo "[Contextify] Memory system is not running. Start with: docker start contextify"
fi

# Always exit 0 — never block Claude Code
exit 0
