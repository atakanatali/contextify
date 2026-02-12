#!/usr/bin/env bash
# Contextify SessionStart hook for Claude Code
# Checks if Contextify is running and provides the agent with a canonical project_id.
# Installed by: install.sh → ~/.contextify/hooks/session-start.sh

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

# Extract session id (best-effort)
SESSION_ID=""
if command -v jq &>/dev/null; then
    SESSION_ID=$(echo "$SESSION_INFO" | jq -r '.session_id // .sessionId // empty' 2>/dev/null)
elif command -v python3 &>/dev/null; then
    SESSION_ID=$(echo "$SESSION_INFO" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('session_id') or d.get('sessionId') or '')" 2>/dev/null)
fi
if [ -z "$SESSION_ID" ]; then
    SESSION_ID="session-$(date +%s)"
fi

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

# --- Project ID normalization ---
# Resolves the CWD to a canonical project identifier using file-based detection.
# Priority: .contextify.yml name > VCS remote URL > worktree strip > raw path.
# No external binary (git, hg) is required — config files are parsed directly.

# Parse remote URL from a git config file
parse_git_config_remote() {
    local config_file="$1"
    [ -f "$config_file" ] || return
    local in_remote_origin=false
    while IFS= read -r line; do
        line=$(echo "$line" | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//')
        if [[ "$line" == '[remote "origin"]' ]]; then
            in_remote_origin=true
            continue
        elif [[ "$line" == \[* ]]; then
            in_remote_origin=false
            continue
        fi
        if $in_remote_origin && [[ "$line" == url* ]]; then
            echo "$line" | sed 's/^url[[:space:]]*=[[:space:]]*//'
            return
        fi
    done < "$config_file"
}

# Parse default path from Mercurial hgrc
parse_hg_remote() {
    local hgrc="$1"
    [ -f "$hgrc" ] || return
    local in_paths=false
    while IFS= read -r line; do
        line=$(echo "$line" | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//')
        if [[ "$line" == "[paths]" ]]; then
            in_paths=true
            continue
        elif [[ "$line" == \[* ]]; then
            in_paths=false
            continue
        fi
        if $in_paths && [[ "$line" == default* ]]; then
            echo "$line" | sed 's/^default[[:space:]]*=[[:space:]]*//'
            return
        fi
    done < "$hgrc"
}

# Convert a remote URL to canonical format: host/user/repo
canonicalize_url() {
    local url="$1"
    [ -z "$url" ] && return

    # SCP-style: git@github.com:user/repo.git
    if [[ "$url" =~ ^[a-zA-Z0-9._-]+@([^:]+):(.+)$ ]]; then
        local host="${BASH_REMATCH[1]}"
        local path="${BASH_REMATCH[2]}"
        path="${path%.git}"
        path="${path%/}"
        echo "${host}/${path}"
        return
    fi

    # Standard URL: https://github.com/user/repo.git
    # Strip protocol
    local stripped="$url"
    stripped="${stripped#https://}"
    stripped="${stripped#http://}"
    stripped="${stripped#ssh://}"
    stripped="${stripped#git://}"
    # Strip user@ prefix
    stripped="${stripped#*@}"
    # Strip .git suffix and trailing slash
    stripped="${stripped%.git}"
    stripped="${stripped%/}"

    # Validate: must have host/path format
    if [[ "$stripped" == */* ]]; then
        echo "$stripped"
    fi
}

# Resolve VCS remote from a project root directory
parse_vcs_remote() {
    local root="$1"

    # Git: normal repo (.git is a directory)
    if [ -d "$root/.git" ]; then
        parse_git_config_remote "$root/.git/config"
        return
    fi

    # Git: worktree (.git is a file with gitdir pointer)
    if [ -f "$root/.git" ]; then
        local gitdir
        gitdir=$(sed 's/^gitdir: //' "$root/.git" 2>/dev/null)
        [ -z "$gitdir" ] && return

        # Resolve relative path
        if [[ "$gitdir" != /* ]]; then
            gitdir="$root/$gitdir"
        fi

        # Worktree gitdir = .git/worktrees/<name>, main .git = two levels up
        local main_git_dir
        main_git_dir=$(cd "$gitdir/../.." 2>/dev/null && pwd)
        if [ -f "$main_git_dir/config" ]; then
            parse_git_config_remote "$main_git_dir/config"
            return
        fi
        return
    fi

    # Mercurial
    if [ -f "$root/.hg/hgrc" ]; then
        parse_hg_remote "$root/.hg/hgrc"
        return
    fi
}

# Normalize a CWD path to a canonical project identifier
normalize_project_id() {
    local cwd="$1"
    [ -z "$cwd" ] && return

    local dir="$cwd"

    # Walk up to find project root
    while [ "$dir" != "/" ] && [ -n "$dir" ]; do
        # Priority 1: .contextify.yml with name field
        if [ -f "$dir/.contextify.yml" ]; then
            local name
            name=$(grep '^name:' "$dir/.contextify.yml" 2>/dev/null | sed "s/^name:[[:space:]]*//" | sed "s/^[\"']//" | sed "s/[\"']$//")
            if [ -n "$name" ]; then
                echo "$name"
                return
            fi
        fi

        # Priority 2: VCS remote
        if [ -d "$dir/.git" ] || [ -f "$dir/.git" ] || [ -d "$dir/.hg" ]; then
            local remote
            remote=$(parse_vcs_remote "$dir")
            if [ -n "$remote" ]; then
                local canonical
                canonical=$(canonicalize_url "$remote")
                if [ -n "$canonical" ]; then
                    echo "$canonical"
                    return
                fi
            fi
        fi

        dir=$(dirname "$dir")
    done

    # Priority 3: Strip worktree suffix
    local stripped
    stripped=$(echo "$cwd" | sed 's|/\.claude/worktrees/[^/]*$||')
    echo "$stripped"
}

# --- Main ---

# Resolve canonical project ID
PROJECT_ID=""
if [ -n "$CWD" ]; then
    PROJECT_ID=$(normalize_project_id "$CWD")
fi

# Check if Contextify is healthy
if curl -sf "${CONTEXTIFY_URL}/health" &>/dev/null; then
    echo "[Contextify] Memory system is online."

    if [ -z "$PROJECT_ID" ]; then
        rm -f "$READY_FILE"
        touch "$REQUIRED_FILE"
        echo "[Contextify] ⚠ Session NOT READY: project_id could not be resolved."
        echo "[Contextify] FIRST ACTION MUST be get_context with the current project path."
    else
        ENCODED_PROJECT_ID=$(url_encode "$PROJECT_ID")
        CONTEXT_URL="${CONTEXTIFY_URL}/api/v1/context/${ENCODED_PROJECT_ID}"
        CONTEXT_JSON=$(curl -sf -X POST "$CONTEXT_URL" \
            -H "X-Session-ID: ${SESSION_ID}" \
            -H "Content-Type: application/json" 2>/dev/null)
        if [ $? -eq 0 ]; then
            MEM_COUNT="unknown"
            if command -v jq &>/dev/null; then
                MEM_COUNT=$(printf '%s' "$CONTEXT_JSON" | jq 'length' 2>/dev/null || echo "unknown")
            fi
            printf 'session_id=%s\nproject_id=%s\nloaded_at=%s\n' "$SESSION_ID" "$PROJECT_ID" "$(date -u +%FT%TZ)" > "$READY_FILE"
            rm -f "$REQUIRED_FILE"
            echo "[Contextify] Session READY: context preloaded for project_id=\"${PROJECT_ID}\" (${MEM_COUNT} memories)."
        else
            rm -f "$READY_FILE"
            touch "$REQUIRED_FILE"
            echo "[Contextify] ⚠ Session NOT READY: automatic context preload failed."
            echo "[Contextify] FIRST ACTION MUST be get_context with project_id=\"${PROJECT_ID}\"."
        fi
    fi

    echo "[Contextify] Store important findings with store_memory (agent_source: \"claude-code\")."
else
    rm -f "$READY_FILE"
    touch "$REQUIRED_FILE"
    echo "[Contextify] Memory system is not running. Start with: docker start contextify"
    if [ -n "$PROJECT_ID" ]; then
        echo "[Contextify] Session NOT READY until get_context succeeds for project_id=\"${PROJECT_ID}\"."
    fi
fi

# Always exit 0 — never block Claude Code
exit 0
