#!/usr/bin/env bash
set -euo pipefail

# ─── Contextify Setup Wizard ───
# Interactive installer that configures Contextify for your AI tools.
# Usage: ./install.sh                              (interactive wizard)
#        ./install.sh --tools claude-code,cursor    (non-interactive)
#        ./install.sh --all                         (configure all tools)
#        ./install.sh --status                      (show config status)
#        ./install.sh --uninstall                   (remove configs)
#        ./install.sh --help                        (show help)

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

# ─── Status Check Functions ───

check_docker_status() {
    if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
        echo "running"
    elif docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q '^contextify$'; then
        echo "stopped"
    else
        echo "not-exists"
    fi
}

check_claude_code_status() {
    local settings="${HOME}/.claude/settings.json"
    local claude_md="${HOME}/.claude/CLAUDE.md"
    local has_mcp=false has_hooks=false has_md=false

    json_has_key "$settings" "mcpServers.contextify" 2>/dev/null && has_mcp=true
    if [ -f "$settings" ] && grep -q "session-start.sh" "$settings" 2>/dev/null; then
        has_hooks=true
    fi
    if [ -f "$claude_md" ] && grep -qF "$CONTEXTIFY_MARKER" "$claude_md" 2>/dev/null; then
        has_md=true
    fi

    if $has_mcp && $has_hooks && $has_md; then
        echo "configured"
    elif $has_mcp || $has_hooks || $has_md; then
        echo "partial"
    else
        echo "not-configured"
    fi
}

check_cursor_status() {
    local mcp_file="${HOME}/.cursor/mcp.json"
    local rules_file="${HOME}/.cursor/rules/contextify.md"
    local has_mcp=false has_rules=false

    json_has_key "$mcp_file" "mcpServers.contextify" 2>/dev/null && has_mcp=true
    [ -f "$rules_file" ] && has_rules=true

    if $has_mcp && $has_rules; then
        echo "configured"
    elif $has_mcp || $has_rules; then
        echo "partial"
    else
        echo "not-configured"
    fi
}

check_windsurf_status() {
    local mcp_file="${HOME}/.codeium/windsurf/mcp_config.json"
    local rules_file="${HOME}/.codeium/windsurf/memories/contextify.md"
    local has_mcp=false has_rules=false

    json_has_key "$mcp_file" "mcpServers.contextify" 2>/dev/null && has_mcp=true
    [ -f "$rules_file" ] && has_rules=true

    if $has_mcp && $has_rules; then
        echo "configured"
    elif $has_mcp || $has_rules; then
        echo "partial"
    else
        echo "not-configured"
    fi
}

check_gemini_status() {
    if [ -f "${HOME}/.contextify/gemini-instructions.md" ]; then
        echo "configured"
    else
        echo "not-configured"
    fi
}

_status_label() {
    case "$1" in
        configured)     printf '\033[0;32m✓ configured\033[0m' ;;
        partial)        printf '\033[0;33m◐ partial\033[0m' ;;
        not-configured) printf '\033[0;90m○ not configured\033[0m' ;;
    esac
}

# ─── Prerequisites ───
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

# ─── Start Contextify ───
start_contextify() {
    if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
        ok "Contextify already running at ${CONTEXTIFY_URL}"
        return 0
    fi

    info "Starting Contextify..."

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

# ─── Update Contextify ───
update_contextify() {
    info "Updating Contextify..."

    # Pull latest image
    info "Pulling latest image..."
    docker pull "${CONTEXTIFY_IMAGE}"

    # Stop + remove old container (volume is preserved)
    if docker ps --format '{{.Names}}' | grep -q '^contextify$'; then
        info "Stopping current container..."
        docker stop contextify
    fi
    if docker ps -a --format '{{.Names}}' | grep -q '^contextify$'; then
        docker rm contextify
    fi

    # Start with new image, same volume
    docker run -d \
        --name contextify \
        -p 8420:8420 \
        -v contextify-data:/var/lib/postgresql/data \
        --restart unless-stopped \
        "${CONTEXTIFY_IMAGE}"

    ok "Container recreated with latest image"
    # Migrations run automatically on Go server startup
}

# ─── Health Check ───
wait_for_health() {
    if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
        ok "Contextify is healthy"
        return 0
    fi

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

# ─── Interactive Tool Selection ───
SELECTED_TOOLS=()

interactive_tool_selection() {
    if [ ! -t 0 ]; then
        fail "Interactive mode requires a terminal."
        fail "Use --tools claude-code,cursor,windsurf,gemini or --all for non-interactive mode."
        exit 1
    fi

    local claude_status cursor_status windsurf_status gemini_status
    claude_status=$(check_claude_code_status)
    cursor_status=$(check_cursor_status)
    windsurf_status=$(check_windsurf_status)
    gemini_status=$(check_gemini_status)

    echo ""
    echo "  Which AI tools would you like to configure?"
    echo ""
    printf "    [1] Claude Code   "; _status_label "$claude_status"; echo ""
    printf "    [2] Cursor        "; _status_label "$cursor_status"; echo ""
    printf "    [3] Windsurf      "; _status_label "$windsurf_status"; echo ""
    printf "    [4] Gemini        "; _status_label "$gemini_status"; echo ""
    echo ""
    echo "    [a] All of the above"
    echo "    [q] Quit"
    echo ""
    printf "  Select tools (e.g. 1,3 or a): "
    read -r selection

    SELECTED_TOOLS=()
    selection=$(echo "$selection" | tr ',' ' ')
    for item in $selection; do
        case "$item" in
            1) SELECTED_TOOLS+=("claude-code") ;;
            2) SELECTED_TOOLS+=("cursor") ;;
            3) SELECTED_TOOLS+=("windsurf") ;;
            4) SELECTED_TOOLS+=("gemini") ;;
            a|A|all) SELECTED_TOOLS=("claude-code" "cursor" "windsurf" "gemini"); break ;;
            q|Q|quit) echo ""; info "Setup cancelled."; exit 0 ;;
            *) warn "Unknown selection: $item (ignoring)" ;;
        esac
    done

    if [ ${#SELECTED_TOOLS[@]} -eq 0 ]; then
        warn "No tools selected. Nothing to configure."
        exit 0
    fi

    echo ""
}

# ─── Configure Tools ───

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
    if [ -f "${rules_dir}/contextify.md" ]; then
        ok "Cursor rules already installed (skipping)"
    else
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
    fi
}

configure_windsurf() {
    info "Configuring Windsurf..."

    # 1. MCP server
    local mcp_file="${HOME}/.codeium/windsurf/mcp_config.json"
    mkdir -p "${HOME}/.codeium/windsurf"

    if json_has_key "$mcp_file" "mcpServers.contextify" 2>/dev/null; then
        ok "MCP server already configured (skipping)"
    else
        json_set_nested "$mcp_file" "mcpServers.contextify" '{"serverUrl":"'"${CONTEXTIFY_MCP_URL}"'"}'
        ok "Added MCP server to ${mcp_file}"
    fi

    # 2. Rules
    local rules_dir="${HOME}/.codeium/windsurf/memories"
    mkdir -p "$rules_dir"
    if [ -f "${rules_dir}/contextify.md" ]; then
        ok "Windsurf rules already installed (skipping)"
    else
        if [ -f "${SCRIPT_DIR}/prompts/windsurf.md" ]; then
            cp "${SCRIPT_DIR}/prompts/windsurf.md" "${rules_dir}/contextify.md"
        else
            cat > "${rules_dir}/contextify.md" << 'RULESEOF'
# Contextify Memory System
At session start, call `get_context` with the current workspace path.
Store memories when you fix bugs, discover patterns, or make decisions.
Recall memories before tackling problems. Set `agent_source` to "windsurf".
RULESEOF
        fi
        ok "Installed Windsurf rules to ${rules_dir}/contextify.md"
    fi
}

configure_gemini() {
    info "Configuring Gemini..."

    local dest="${HOME}/.contextify/gemini-instructions.md"

    if [ -f "$dest" ]; then
        ok "Gemini instructions already installed (skipping)"
    else
        mkdir -p "${HOME}/.contextify"
        if [ -f "${SCRIPT_DIR}/prompts/gemini.md" ]; then
            cp "${SCRIPT_DIR}/prompts/gemini.md" "$dest"
        else
            cat > "$dest" << 'GEMINIEOF'
# Contextify Memory System
Use the REST API at http://localhost:8420/api/v1/ for memory operations.
Set agent_source to "gemini".
GEMINIEOF
        fi
        ok "Gemini instructions saved to ${dest}"
    fi
}

configure_tools() {
    for tool in "${SELECTED_TOOLS[@]}"; do
        case "$tool" in
            claude-code) configure_claude_code ;;
            cursor)      configure_cursor ;;
            windsurf)    configure_windsurf ;;
            gemini)      configure_gemini ;;
        esac
    done
}

# ─── Self-test ───
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

# ─── Restart Tools ───
restart_tools() {
    local needs_restart=false

    for tool in "${SELECTED_TOOLS[@]}"; do
        case "$tool" in
            cursor|windsurf) needs_restart=true; break ;;
        esac
    done

    if ! $needs_restart; then
        return 0
    fi

    echo ""
    info "Restarting configured tools to load MCP servers..."

    for tool in "${SELECTED_TOOLS[@]}"; do
        case "$tool" in
            cursor)
                if pgrep -f "Cursor" &>/dev/null; then
                    pkill -f "Cursor" 2>/dev/null || true
                    sleep 2
                    open -a "Cursor" 2>/dev/null && ok "Cursor restarted" || warn "Could not relaunch Cursor (open it manually)"
                fi
                ;;
            windsurf)
                if pgrep -f "Windsurf" &>/dev/null; then
                    pkill -f "Windsurf" 2>/dev/null || true
                    sleep 2
                    open -a "Windsurf" 2>/dev/null && ok "Windsurf restarted" || warn "Could not relaunch Windsurf (open it manually)"
                fi
                ;;
        esac
    done

    # Claude Code is a CLI — can't restart from here, inform user
    for tool in "${SELECTED_TOOLS[@]}"; do
        if [ "$tool" = "claude-code" ]; then
            warn "Claude Code: start a new session to load Contextify MCP tools"
            break
        fi
    done
}

# ─── Summary ───
print_summary() {
    echo ""
    echo "  ╔═══════════════════════════════════════╗"
    echo "  ║      Contextify — Setup Complete      ║"
    echo "  ╚═══════════════════════════════════════╝"
    echo ""
    echo "  API:    ${CONTEXTIFY_URL}/api/v1/"
    echo "  MCP:    ${CONTEXTIFY_MCP_URL}"
    echo "  Web UI: ${CONTEXTIFY_URL}"
    echo ""
    if [ ${#SELECTED_TOOLS[@]} -gt 0 ]; then
        echo "  Configured tools:"
        for tool in "${SELECTED_TOOLS[@]}"; do
            local status
            case "$tool" in
                claude-code) status=$(check_claude_code_status) ;;
                cursor)      status=$(check_cursor_status) ;;
                windsurf)    status=$(check_windsurf_status) ;;
                gemini)      status=$(check_gemini_status) ;;
            esac
            printf "    %-14s " "${tool}"
            _status_label "$status"
            echo ""
        done
    fi
    echo ""
    echo "  What happens next:"
    echo "    1. Open any configured AI tool"
    echo "    2. Contextify MCP tools are automatically available"
    echo "    3. The agent loads project context at session start"
    echo "    4. Memories are shared across all your AI tools"
    echo ""
    echo "  Re-run this wizard anytime: $0"
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

    # Windsurf
    local windsurf_mcp="${HOME}/.codeium/windsurf/mcp_config.json"
    if [ -f "$windsurf_mcp" ]; then
        json_remove_key "$windsurf_mcp" "mcpServers.contextify" 2>/dev/null || true
        ok "Removed Windsurf MCP config"
    fi
    rm -f "${HOME}/.codeium/windsurf/memories/contextify.md" 2>/dev/null || true

    # Hooks directory & Gemini
    rm -rf "${HOOKS_DIR}" 2>/dev/null || true
    rm -f "${HOME}/.contextify/gemini-instructions.md" 2>/dev/null || true
    rmdir "${HOME}/.contextify" 2>/dev/null || true

    ok "Uninstall complete. Docker container left running (stop with: docker stop contextify)"
}

# ─── Help ───
show_help() {
    echo ""
    echo "  Contextify Setup Wizard"
    echo "  ─────────────────────────"
    echo ""
    echo "  Usage:"
    echo "    ./install.sh                              Interactive setup wizard"
    echo "    ./install.sh --tools claude-code,cursor    Configure specific tools"
    echo "    ./install.sh --all                         Configure all tools"
    echo "    ./install.sh --status                      Show configuration status"
    echo "    ./install.sh --update                      Update to latest version"
    echo "    ./install.sh --uninstall                   Remove all configurations"
    echo "    ./install.sh --help                        Show this help"
    echo ""
    echo "  Supported tools: claude-code, cursor, windsurf, gemini"
    echo ""
    echo "  Environment variables:"
    echo "    CONTEXTIFY_URL     Server URL (default: http://localhost:8420)"
    echo "    CONTEXTIFY_IMAGE   Docker image (default: ghcr.io/atakanatali/contextify:latest)"
    echo ""
}

# ─── Status ───
show_status() {
    echo ""
    echo "  Contextify Status"
    echo "  ─────────────────"
    echo ""

    local docker_status
    docker_status=$(check_docker_status)
    case "$docker_status" in
        running)    ok "Server: running at ${CONTEXTIFY_URL}" ;;
        stopped)    warn "Server: container stopped (run: docker start contextify)" ;;
        not-exists) warn "Server: not installed (run: ./install.sh)" ;;
    esac

    echo ""
    echo "  Tool configurations:"
    local tools=("claude-code" "cursor" "windsurf" "gemini")
    local names=("Claude Code" "Cursor" "Windsurf" "Gemini")
    for i in "${!tools[@]}"; do
        local status
        case "${tools[$i]}" in
            claude-code) status=$(check_claude_code_status) ;;
            cursor)      status=$(check_cursor_status) ;;
            windsurf)    status=$(check_windsurf_status) ;;
            gemini)      status=$(check_gemini_status) ;;
        esac
        printf "    %-14s " "${names[$i]}"
        _status_label "$status"
        echo ""
    done
    echo ""
}

# ─── Welcome Banner ───
show_welcome_banner() {
    echo ""
    echo "  ╔═══════════════════════════════════════╗"
    echo "  ║      Contextify Setup Wizard          ║"
    echo "  ╠═══════════════════════════════════════╣"
    echo "  ║  Shared memory for your AI agents     ║"
    echo "  ╚═══════════════════════════════════════╝"
    echo ""
}

# ─── Main ───
main() {
    local mode="interactive"
    SELECTED_TOOLS=()

    while [ $# -gt 0 ]; do
        case "$1" in
            --uninstall)
                mode="uninstall" ;;
            --tools)
                mode="non-interactive"
                shift
                IFS=',' read -ra SELECTED_TOOLS <<< "$1"
                # Validate tool names
                for tool in "${SELECTED_TOOLS[@]}"; do
                    case "$tool" in
                        claude-code|cursor|windsurf|gemini) ;;
                        *) fail "Unknown tool: $tool. Valid: claude-code, cursor, windsurf, gemini"; exit 1 ;;
                    esac
                done
                ;;
            --all)
                mode="non-interactive"
                SELECTED_TOOLS=("claude-code" "cursor" "windsurf" "gemini")
                ;;
            --update)
                mode="update" ;;
            --status)
                mode="status" ;;
            --help|-h)
                mode="help" ;;
            *)
                fail "Unknown option: $1. Run with --help for usage."
                exit 1 ;;
        esac
        shift
    done

    case "$mode" in
        help)
            show_help
            exit 0
            ;;
        status)
            check_prerequisites
            show_status
            exit 0
            ;;
        update)
            check_prerequisites
            update_contextify
            wait_for_health
            run_self_test
            ok "Contextify updated successfully!"
            exit 0
            ;;
        uninstall)
            check_prerequisites
            uninstall
            exit 0
            ;;
        interactive)
            show_welcome_banner
            check_prerequisites
            start_contextify
            wait_for_health
            interactive_tool_selection
            configure_tools
            run_self_test
            restart_tools
            print_summary
            ;;
        non-interactive)
            show_welcome_banner
            check_prerequisites
            start_contextify
            wait_for_health
            configure_tools
            run_self_test
            restart_tools
            print_summary
            ;;
    esac
}

main "$@"
