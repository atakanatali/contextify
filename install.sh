#!/usr/bin/env bash
set -euo pipefail

# ─── Contextify Installer ───
# Sets up Contextify and configures all detected AI tools.
# Usage: ./install.sh          (install)
#        ./install.sh --uninstall  (remove)

CONTEXTIFY_URL="${CONTEXTIFY_URL:-http://localhost:8420}"
CONTEXTIFY_MCP_URL="${CONTEXTIFY_URL}/mcp"
CONTEXTIFY_IMAGE="${CONTEXTIFY_IMAGE:-ghcr.io/atakanatali/contextify:latest}"
HOOKS_DIR="${HOME}/.contextify/hooks"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTEXTIFY_MARKER="<!-- contextify-memory-system -->"

# ─── Colors ───
info()  { printf '\033[0;34m[INFO]\033[0m  %s\n' "$1"; }
ok()    { printf '\033[0;32m[OK]\033[0m    %s\n' "$1"; }
warn()  { printf '\033[0;33m[WARN]\033[0m  %s\n' "$1"; }
fail()  { printf '\033[0;31m[FAIL]\033[0m  %s\n' "$1"; }

# ─── Source JSON merge utilities ───
if [ -f "${SCRIPT_DIR}/scripts/lib/json-merge.sh" ]; then
    # shellcheck source=scripts/lib/json-merge.sh
    source "${SCRIPT_DIR}/scripts/lib/json-merge.sh"
fi

# ─── Phase 1: Prerequisites ───
check_prerequisites() {
    info "Checking prerequisites..."

    if ! command -v docker &>/dev/null; then
        fail "Docker not found. Install: https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! docker info &>/dev/null 2>&1; then
        fail "Docker daemon is not running. Start Docker and retry."
        exit 1
    fi

    if [ -z "${JSON_TOOL:-}" ]; then
        if command -v jq &>/dev/null; then
            JSON_TOOL="jq"
        elif command -v python3 &>/dev/null; then
            JSON_TOOL="python3"
        else
            fail "Neither jq nor python3 found. Install jq: brew install jq"
            exit 1
        fi
    fi

    if ! command -v curl &>/dev/null; then
        fail "curl not found."
        exit 1
    fi

    ok "Prerequisites met (docker, ${JSON_TOOL}, curl)"
}

# ─── Phase 2: Start Contextify ───
start_contextify() {
    if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
        ok "Contextify already running at ${CONTEXTIFY_URL}"
        return 0
    fi

    info "Starting Contextify..."

    # Check if container exists but is stopped
    if docker ps -a --format '{{.Names}}' | grep -q '^contextify$'; then
        docker start contextify &>/dev/null
    else
        docker run -d \
            --name contextify \
            -p 8420:8420 \
            -v contextify-data:/var/lib/postgresql/data \
            --restart unless-stopped \
            "${CONTEXTIFY_IMAGE}" &>/dev/null
    fi

    ok "Contextify container started"
}

# ─── Phase 3: Health Check ───
wait_for_health() {
    info "Waiting for Contextify to be healthy..."
    local max=60
    for i in $(seq 1 $max); do
        if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
            ok "Contextify is healthy"
            return 0
        fi
        if [ "$i" -eq "$max" ]; then
            fail "Contextify did not become healthy within ${max}s"
            fail "Check logs: docker logs contextify"
            exit 1
        fi
        sleep 1
    done
}

# ─── Phase 4: Detect Tools ───
DETECTED_TOOLS=()

detect_tools() {
    info "Detecting AI tools..."

    if [ -d "${HOME}/.claude" ] || command -v claude &>/dev/null; then
        DETECTED_TOOLS+=("claude-code")
        ok "Detected: Claude Code"
    fi

    if [ -d "${HOME}/.cursor" ]; then
        DETECTED_TOOLS+=("cursor")
        ok "Detected: Cursor"
    fi

    if [ ${#DETECTED_TOOLS[@]} -eq 0 ]; then
        warn "No AI tools detected automatically."
        warn "Manual setup: see README.md for Claude Code, Cursor, and Gemini instructions."
    fi
}

# ─── Phase 5: Configure Tools ───

configure_claude_code() {
    info "Configuring Claude Code..."

    local settings="${HOME}/.claude/settings.json"
    mkdir -p "${HOME}/.claude"

    # 1. MCP server
    if json_has_key "$settings" "mcpServers.contextify" 2>/dev/null; then
        ok "MCP server already configured (skipping)"
    else
        json_set_nested "$settings" "mcpServers.contextify" '{"type":"streamableHttp","url":"'"${CONTEXTIFY_MCP_URL}"'"}'
        ok "Added MCP server to ${settings}"
    fi

    # 2. Install hooks
    mkdir -p "${HOOKS_DIR}"
    if [ -f "${SCRIPT_DIR}/scripts/hooks/session-start.sh" ]; then
        cp "${SCRIPT_DIR}/scripts/hooks/session-start.sh" "${HOOKS_DIR}/"
        cp "${SCRIPT_DIR}/scripts/hooks/post-tool-use.sh" "${HOOKS_DIR}/"
    else
        # Embedded fallback — create minimal hooks inline
        cat > "${HOOKS_DIR}/session-start.sh" << 'HOOKEOF'
#!/usr/bin/env bash
CONTEXTIFY_URL="${CONTEXTIFY_URL:-http://localhost:8420}"
SESSION_INFO=$(cat 2>/dev/null || echo '{}')
CWD=""
if command -v jq &>/dev/null; then CWD=$(echo "$SESSION_INFO" | jq -r '.cwd // empty' 2>/dev/null); fi
if [ -z "$CWD" ] && command -v python3 &>/dev/null; then CWD=$(echo "$SESSION_INFO" | python3 -c "import json,sys; print(json.load(sys.stdin).get('cwd',''))" 2>/dev/null); fi
if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
    echo "[Contextify] Memory system is online."
    [ -n "$CWD" ] && echo "[Contextify] IMPORTANT: Call get_context with project_id=\"${CWD}\" to load project memories."
    echo "[Contextify] Store important findings with store_memory (agent_source: \"claude-code\")."
else
    echo "[Contextify] Memory system is not running. Start with: docker start contextify"
fi
exit 0
HOOKEOF
        cat > "${HOOKS_DIR}/post-tool-use.sh" << 'HOOKEOF'
#!/usr/bin/env bash
TOOL_INFO=$(cat 2>/dev/null || echo '{}')
TOOL_NAME=""; TOOL_INPUT=""
if command -v jq &>/dev/null; then TOOL_NAME=$(echo "$TOOL_INFO" | jq -r '.tool_name // empty' 2>/dev/null); TOOL_INPUT=$(echo "$TOOL_INFO" | jq -r '.tool_input.command // empty' 2>/dev/null); fi
if [ "$TOOL_NAME" = "Bash" ] && echo "$TOOL_INPUT" | grep -qE 'git commit'; then
    echo "[Contextify] Commit detected. Consider storing what was fixed/added with store_memory."
fi
exit 0
HOOKEOF
    fi
    chmod +x "${HOOKS_DIR}/session-start.sh" "${HOOKS_DIR}/post-tool-use.sh"

    # Register hooks in settings.json
    json_add_hook "$settings" "SessionStart" "${HOOKS_DIR}/session-start.sh"
    json_add_hook "$settings" "PostToolUse" "${HOOKS_DIR}/post-tool-use.sh"
    ok "Installed Claude Code hooks"

    # 3. CLAUDE.md — append memory instructions
    local claude_md="${HOME}/.claude/CLAUDE.md"
    if [ -f "$claude_md" ] && grep -qF "$CONTEXTIFY_MARKER" "$claude_md"; then
        ok "CLAUDE.md already has Contextify instructions (skipping)"
    else
        local prompt_content=""
        if [ -f "${SCRIPT_DIR}/prompts/claude-code.md" ]; then
            prompt_content=$(cat "${SCRIPT_DIR}/prompts/claude-code.md")
        else
            # Embedded fallback
            prompt_content='## Contextify Memory System

At the START of every session, call `get_context` with `project_id` set to the current working directory.
Store memories with `store_memory` when you fix bugs, discover patterns, or make decisions.
Recall memories with `recall_memories` before tackling problems.
Always set `agent_source` to "claude-code" and `project_id` to the current directory.'
        fi

        {
            echo ""
            echo "$CONTEXTIFY_MARKER"
            echo "$prompt_content"
            echo "<!-- /contextify-memory-system -->"
        } >> "$claude_md"
        ok "Appended Contextify instructions to ${claude_md}"
    fi
}

configure_cursor() {
    info "Configuring Cursor..."

    # 1. MCP server
    local mcp_file="${HOME}/.cursor/mcp.json"
    mkdir -p "${HOME}/.cursor"

    if json_has_key "$mcp_file" "mcpServers.contextify" 2>/dev/null; then
        ok "MCP server already configured (skipping)"
    else
        json_set_nested "$mcp_file" "mcpServers.contextify" '{"url":"'"${CONTEXTIFY_MCP_URL}"'","transport":"streamable-http"}'
        ok "Added MCP server to ${mcp_file}"
    fi

    # 2. Rules
    local rules_dir="${HOME}/.cursor/rules"
    mkdir -p "$rules_dir"
    if [ -f "${SCRIPT_DIR}/prompts/cursorrules.md" ]; then
        cp "${SCRIPT_DIR}/prompts/cursorrules.md" "${rules_dir}/contextify.md"
    else
        cat > "${rules_dir}/contextify.md" << 'RULESEOF'
# Contextify Memory System
At session start, call `get_context` with the current workspace path.
Store memories when you fix bugs, discover patterns, or make decisions.
Recall memories before tackling problems. Set `agent_source` to "cursor".
RULESEOF
    fi
    ok "Installed Cursor rules to ${rules_dir}/contextify.md"
}

configure_tools() {
    for tool in "${DETECTED_TOOLS[@]}"; do
        case "$tool" in
            claude-code) configure_claude_code ;;
            cursor) configure_cursor ;;
        esac
    done

    # Always copy Gemini template if available
    if [ -f "${SCRIPT_DIR}/prompts/gemini.md" ]; then
        mkdir -p "${HOME}/.contextify"
        cp "${SCRIPT_DIR}/prompts/gemini.md" "${HOME}/.contextify/gemini-instructions.md"
        ok "Gemini prompt template saved to ~/.contextify/gemini-instructions.md"
    fi
}

# ─── Phase 6: Self-test ───
run_self_test() {
    info "Running self-test..."

    # Store
    local store_resp
    store_resp=$(curl -sf -X POST "${CONTEXTIFY_URL}/api/v1/memories" \
        -H "Content-Type: application/json" \
        -d '{
            "title":"install.sh self-test","content":"Automated test memory.","type":"general",
            "scope":"global","tags":["self-test"],"importance":0.3,"agent_source":"install-script","ttl_seconds":300
        }' 2>/dev/null) || true

    local test_id=""
    if [ "$JSON_TOOL" = "jq" ]; then
        test_id=$(echo "$store_resp" | jq -r '.id // empty' 2>/dev/null)
    else
        test_id=$(echo "$store_resp" | python3 -c "import json,sys; print(json.load(sys.stdin).get('id',''))" 2>/dev/null || echo "")
    fi

    if [ -z "$test_id" ]; then
        fail "Self-test: Failed to store test memory"
        return 1
    fi
    ok "Self-test: Stored memory (${test_id:0:8}...)"

    # Get
    if curl -sf "${CONTEXTIFY_URL}/api/v1/memories/${test_id}" &>/dev/null; then
        ok "Self-test: Retrieved memory"
    else
        fail "Self-test: Failed to retrieve memory"
        return 1
    fi

    # Recall
    local recall_resp
    recall_resp=$(curl -sf -X POST "${CONTEXTIFY_URL}/api/v1/memories/recall" \
        -H "Content-Type: application/json" \
        -d '{"query":"install self-test","limit":5}' 2>/dev/null) || true
    if [ -n "$recall_resp" ]; then
        ok "Self-test: Semantic search works"
    else
        warn "Self-test: Semantic search returned empty (model may still be loading)"
    fi

    # Cleanup
    curl -sf -X DELETE "${CONTEXTIFY_URL}/api/v1/memories/${test_id}" &>/dev/null || true
    ok "Self-test: Cleaned up"

    ok "Self-test passed!"
}

# ─── Phase 7: Summary ───
print_summary() {
    echo ""
    echo "========================================="
    echo "  Contextify — Setup Complete"
    echo "========================================="
    echo ""
    echo "  API:    ${CONTEXTIFY_URL}/api/v1/"
    echo "  MCP:    ${CONTEXTIFY_MCP_URL}"
    echo "  Web UI: ${CONTEXTIFY_URL}"
    echo "  Health: ${CONTEXTIFY_URL}/health"
    echo ""
    if [ ${#DETECTED_TOOLS[@]} -gt 0 ]; then
        echo "  Configured tools:"
        for tool in "${DETECTED_TOOLS[@]}"; do
            echo "    - ${tool}"
        done
    fi
    echo ""
    echo "  What happens next:"
    echo "    1. Open any configured AI tool"
    echo "    2. Contextify MCP tools are automatically available"
    echo "    3. The agent loads project context at session start"
    echo "    4. Memories are shared across all your AI tools"
    echo ""
    echo "  To uninstall: $0 --uninstall"
    echo ""
}

# ─── Uninstall ───
uninstall() {
    info "Uninstalling Contextify configurations..."

    # Claude Code
    local claude_settings="${HOME}/.claude/settings.json"
    if [ -f "$claude_settings" ]; then
        json_remove_key "$claude_settings" "mcpServers.contextify" 2>/dev/null || true
        json_remove_hook "$claude_settings" "SessionStart" "${HOOKS_DIR}/session-start.sh" 2>/dev/null || true
        json_remove_hook "$claude_settings" "PostToolUse" "${HOOKS_DIR}/post-tool-use.sh" 2>/dev/null || true
        ok "Removed Claude Code MCP + hooks config"
    fi

    # CLAUDE.md — remove marker block
    local claude_md="${HOME}/.claude/CLAUDE.md"
    if [ -f "$claude_md" ] && grep -qF "$CONTEXTIFY_MARKER" "$claude_md"; then
        if [ "$JSON_TOOL" = "python3" ] || command -v python3 &>/dev/null; then
            python3 -c "
import re
with open('$claude_md', 'r') as f:
    content = f.read()
content = re.sub(r'\n?$CONTEXTIFY_MARKER.*?<!-- /contextify-memory-system -->\n?', '', content, flags=re.DOTALL)
with open('$claude_md', 'w') as f:
    f.write(content)
"
        fi
        ok "Removed Contextify section from CLAUDE.md"
    fi

    # Cursor
    local cursor_mcp="${HOME}/.cursor/mcp.json"
    if [ -f "$cursor_mcp" ]; then
        json_remove_key "$cursor_mcp" "mcpServers.contextify" 2>/dev/null || true
        ok "Removed Cursor MCP config"
    fi
    rm -f "${HOME}/.cursor/rules/contextify.md" 2>/dev/null || true

    # Hooks directory
    rm -rf "${HOOKS_DIR}" 2>/dev/null || true
    rm -f "${HOME}/.contextify/gemini-instructions.md" 2>/dev/null || true
    rmdir "${HOME}/.contextify" 2>/dev/null || true

    ok "Uninstall complete. Docker container left running (stop with: docker stop contextify)"
}

# ─── Main ───
main() {
    echo ""
    echo "  Contextify Installer"
    echo "  ─────────────────────"
    echo ""

    if [ "${1:-}" = "--uninstall" ]; then
        check_prerequisites
        uninstall
        exit 0
    fi

    check_prerequisites
    start_contextify
    wait_for_health
    detect_tools
    configure_tools
    run_self_test
    print_summary
}

main "$@"
