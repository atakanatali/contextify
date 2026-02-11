#!/usr/bin/env bash
# json-merge.sh — JSON merge utilities for install.sh
# Uses jq (preferred) or python3 (fallback)

# Detect JSON tool
_detect_json_tool() {
    if command -v jq &>/dev/null; then
        echo "jq"
    elif command -v python3 &>/dev/null; then
        echo "python3"
    else
        echo ""
    fi
}

JSON_TOOL="${JSON_TOOL:-$(_detect_json_tool)}"

# json_set_nested FILE KEY VALUE
# Sets a nested key in a JSON file. Creates file if it doesn't exist.
# Example: json_set_nested settings.json "mcpServers.contextify" '{"type":"streamableHttp"}'
json_set_nested() {
    local file="$1"
    local key="$2"
    local value="$3"

    # Create file with empty object if it doesn't exist
    if [ ! -f "$file" ]; then
        mkdir -p "$(dirname "$file")"
        echo '{}' > "$file"
    fi

    if [ "$JSON_TOOL" = "jq" ]; then
        local tmp
        tmp=$(mktemp)
        local parts
        IFS='.' read -ra parts <<< "$key"
        if [ ${#parts[@]} -eq 2 ]; then
            jq --argjson v "$value" ".\"${parts[0]}\" //= {} | .\"${parts[0]}\".\"${parts[1]}\" = \$v" "$file" > "$tmp"
        elif [ ${#parts[@]} -eq 1 ]; then
            jq --argjson v "$value" ".\"${parts[0]}\" = \$v" "$file" > "$tmp"
        elif [ ${#parts[@]} -eq 3 ]; then
            jq --argjson v "$value" ".\"${parts[0]}\" //= {} | .\"${parts[0]}\".\"${parts[1]}\" //= {} | .\"${parts[0]}\".\"${parts[1]}\".\"${parts[2]}\" = \$v" "$file" > "$tmp"
        fi
        mv "$tmp" "$file"
    else
        python3 -c "
import json, os

file_path = '$file'
key_path = '$key'.split('.')
value = json.loads('''$value''')

with open(file_path, 'r') as f:
    data = json.load(f)

# Navigate/create nested path
current = data
for part in key_path[:-1]:
    if part not in current or not isinstance(current[part], dict):
        current[part] = {}
    current = current[part]
current[key_path[-1]] = value

with open(file_path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"
    fi
}

# json_has_key FILE KEY
# Returns 0 if key exists, 1 if not. Supports dotted paths.
json_has_key() {
    local file="$1"
    local key="$2"

    if [ ! -f "$file" ]; then
        return 1
    fi

    if [ "$JSON_TOOL" = "jq" ]; then
        local parts
        IFS='.' read -ra parts <<< "$key"
        local jq_path=""
        for part in "${parts[@]}"; do
            jq_path="${jq_path}.\"${part}\""
        done
        jq -e "${jq_path} // empty" "$file" &>/dev/null
        return $?
    else
        python3 -c "
import json, sys

with open('$file', 'r') as f:
    data = json.load(f)

parts = '$key'.split('.')
current = data
for part in parts:
    if not isinstance(current, dict) or part not in current:
        sys.exit(1)
    current = current[part]
sys.exit(0)
" 2>/dev/null
        return $?
    fi
}

# json_remove_key FILE KEY
# Removes a nested key from a JSON file. Supports dotted paths.
json_remove_key() {
    local file="$1"
    local key="$2"

    if [ ! -f "$file" ]; then
        return 0
    fi

    if [ "$JSON_TOOL" = "jq" ]; then
        local tmp
        tmp=$(mktemp)
        local parts
        IFS='.' read -ra parts <<< "$key"
        if [ ${#parts[@]} -eq 2 ]; then
            jq "del(.\"${parts[0]}\".\"${parts[1]}\")" "$file" > "$tmp"
        elif [ ${#parts[@]} -eq 1 ]; then
            jq "del(.\"${parts[0]}\")" "$file" > "$tmp"
        fi
        mv "$tmp" "$file"
    else
        python3 -c "
import json

file_path = '$file'
parts = '$key'.split('.')

with open(file_path, 'r') as f:
    data = json.load(f)

current = data
for part in parts[:-1]:
    if part not in current:
        # Key path doesn't exist, nothing to remove
        with open(file_path, 'w') as f:
            json.dump(data, f, indent=2)
            f.write('\n')
        exit()
    current = current[part]

current.pop(parts[-1], None)

with open(file_path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"
    fi
}

# json_add_hook FILE EVENT COMMAND
# Adds a Claude Code hook entry if the command isn't already registered.
json_add_hook() {
    local file="$1"
    local event="$2"
    local command="$3"

    if [ ! -f "$file" ]; then
        mkdir -p "$(dirname "$file")"
        echo '{}' > "$file"
    fi

    if [ "$JSON_TOOL" = "jq" ]; then
        # Check if already present
        if jq -e ".hooks.\"${event}\"[]?.hooks[]? | select(.command == \"${command}\")" "$file" &>/dev/null; then
            return 0
        fi
        local tmp
        tmp=$(mktemp)
        jq --arg event "$event" --arg cmd "$command" '
            .hooks //= {} |
            .hooks[$event] //= [] |
            .hooks[$event] += [{"matcher": "", "hooks": [{"type": "command", "command": $cmd}]}]
        ' "$file" > "$tmp"
        mv "$tmp" "$file"
    else
        python3 -c "
import json

file_path = '$file'
event = '$event'
cmd = '$command'

with open(file_path, 'r') as f:
    data = json.load(f)

data.setdefault('hooks', {})
data['hooks'].setdefault(event, [])

# Check if already present
already = any(
    any(h.get('command') == cmd for h in entry.get('hooks', []))
    for entry in data['hooks'][event]
)

if not already:
    data['hooks'][event].append({
        'matcher': '',
        'hooks': [{'type': 'command', 'command': cmd}]
    })

with open(file_path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"
    fi
}

# json_remove_hook FILE EVENT COMMAND
# Removes a Claude Code hook entry by command path.
json_remove_hook() {
    local file="$1"
    local event="$2"
    local command="$3"

    if [ ! -f "$file" ]; then
        return 0
    fi

    if [ "$JSON_TOOL" = "jq" ]; then
        local tmp
        tmp=$(mktemp)
        jq --arg event "$event" --arg cmd "$command" '
            if .hooks[$event] then
                .hooks[$event] |= map(select(.hooks | all(.command != $cmd)))
            else . end
        ' "$file" > "$tmp"
        mv "$tmp" "$file"
    else
        python3 -c "
import json

file_path = '$file'
event = '$event'
cmd = '$command'

with open(file_path, 'r') as f:
    data = json.load(f)

if 'hooks' in data and event in data['hooks']:
    data['hooks'][event] = [
        entry for entry in data['hooks'][event]
        if not any(h.get('command') == cmd for h in entry.get('hooks', []))
    ]

with open(file_path, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
"
    fi
}
