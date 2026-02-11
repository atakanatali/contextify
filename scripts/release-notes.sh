#!/usr/bin/env bash
# Generate release notes from CHANGELOG.md and git log.
#
# Usage:
#   ./scripts/release-notes.sh 0.4.0          # Extract section + commit diff
#   ./scripts/release-notes.sh 0.4.0 --notes  # Release notes only (for GitHub Release body)
#
# This script:
#   1. Reads [Unreleased] section from CHANGELOG.md
#   2. Generates commit summary between previous tag and HEAD
#   3. Outputs combined release notes
set -euo pipefail

VERSION="${1:?Usage: release-notes.sh VERSION [--notes]}"
NOTES_ONLY="${2:-}"

CHANGELOG="CHANGELOG.md"
REPO="atakanatali/contextify"

# --- Find previous tag ---
PREV_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

# --- Extract [Unreleased] section from CHANGELOG ---
extract_unreleased() {
    if [ ! -f "$CHANGELOG" ]; then
        echo ""
        return
    fi

    awk '
        /^## \[Unreleased\]/ { found=1; next }
        /^## \[/             { if (found) exit }
        found                { print }
    ' "$CHANGELOG" | sed '/^$/N;/^\n$/d'  # collapse multiple blank lines
}

# --- Generate commit summary ---
generate_commit_summary() {
    local range=""
    if [ -n "$PREV_TAG" ]; then
        range="${PREV_TAG}..HEAD"
    fi

    # Group by conventional commit type
    local feats fixes ci refactors docs others
    feats=""
    fixes=""
    ci=""
    refactors=""
    docs=""
    others=""

    while IFS= read -r line; do
        # Extract hash and message
        hash="${line%% *}"
        msg="${line#* }"
        short_hash="${hash:0:7}"

        case "$msg" in
            feat:*|feat\(*) feats="${feats}\n- ${msg} (\`${short_hash}\`)" ;;
            fix:*|fix\(*)   fixes="${fixes}\n- ${msg} (\`${short_hash}\`)" ;;
            ci:*|ci\(*)     ci="${ci}\n- ${msg} (\`${short_hash}\`)" ;;
            refactor:*)     refactors="${refactors}\n- ${msg} (\`${short_hash}\`)" ;;
            docs:*)         docs="${docs}\n- ${msg} (\`${short_hash}\`)" ;;
            Merge*)         ;; # skip merge commits
            *)              others="${others}\n- ${msg} (\`${short_hash}\`)" ;;
        esac
    done < <(git log ${range} --oneline --no-merges 2>/dev/null)

    local has_content=false

    if [ -n "$feats" ]; then
        echo "#### Features"
        echo -e "$feats"
        echo ""
        has_content=true
    fi
    if [ -n "$fixes" ]; then
        echo "#### Fixes"
        echo -e "$fixes"
        echo ""
        has_content=true
    fi
    if [ -n "$refactors" ]; then
        echo "#### Refactoring"
        echo -e "$refactors"
        echo ""
        has_content=true
    fi
    if [ -n "$ci" ]; then
        echo "#### CI/CD"
        echo -e "$ci"
        echo ""
        has_content=true
    fi
    if [ -n "$docs" ]; then
        echo "#### Documentation"
        echo -e "$docs"
        echo ""
        has_content=true
    fi
    if [ -n "$others" ]; then
        echo "#### Other"
        echo -e "$others"
        echo ""
        has_content=true
    fi

    if [ "$has_content" = false ]; then
        echo "No changes since last release."
    fi
}

# --- Build release notes ---
UNRELEASED=$(extract_unreleased)
COMMITS=$(generate_commit_summary)

if [ "$NOTES_ONLY" = "--notes" ]; then
    # GitHub Release body format
    cat <<EOF
## Contextify v${VERSION}

All-in-one Docker image — everything included, zero setup.

### Quick Start (CLI)
\`\`\`bash
curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install-cli.sh | sh
contextify install
\`\`\`

### Quick Start (Docker)
\`\`\`bash
docker run -d --name contextify -p 8420:8420 \\
  -v contextify-data:/var/lib/postgresql/data \\
  ghcr.io/${REPO}:${VERSION}
\`\`\`
EOF

    if [ -n "$UNRELEASED" ]; then
        echo ""
        echo "### What's Changed"
        echo "$UNRELEASED"
    fi

    echo ""
    echo "### Commits"
    echo "$COMMITS"

    if [ -n "$PREV_TAG" ]; then
        echo ""
        echo "**Full changelog**: https://github.com/${REPO}/compare/${PREV_TAG}...v${VERSION}"
    fi
else
    # Full output (for debugging / preview)
    echo "=== Release Notes for v${VERSION} ==="
    echo ""
    if [ -n "$UNRELEASED" ]; then
        echo "--- From CHANGELOG.md [Unreleased] ---"
        echo "$UNRELEASED"
        echo ""
    fi
    echo "--- Commits since ${PREV_TAG:-beginning} ---"
    echo "$COMMITS"
fi
