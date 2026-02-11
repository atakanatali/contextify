#!/usr/bin/env bash
# Update CHANGELOG.md for a new release.
#
# Usage:
#   ./scripts/update-changelog.sh 0.4.0
#
# This script:
#   1. Moves [Unreleased] content to a new [VERSION] section with today's date
#   2. Resets [Unreleased] to empty
#   3. Updates comparison links at the bottom
set -euo pipefail

VERSION="${1:?Usage: update-changelog.sh VERSION}"
DATE=$(date +%Y-%m-%d)
CHANGELOG="CHANGELOG.md"
REPO="atakanatali/contextify"

if [ ! -f "$CHANGELOG" ]; then
    echo "Error: $CHANGELOG not found"
    exit 1
fi

# Find previous version from the first ## [x.y.z] line (portable, works on macOS + Linux)
PREV_VERSION=$(sed -n 's/^## \[\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)\].*/\1/p' "$CHANGELOG" | head -1)

if [ -z "$PREV_VERSION" ]; then
    echo "Error: Could not find previous version in $CHANGELOG"
    exit 1
fi

# Build the new content using awk
awk -v version="$VERSION" -v date="$DATE" -v prev="$PREV_VERSION" -v repo="$REPO" '
BEGIN { done_unreleased = 0 }

# Replace [Unreleased] header: add empty Unreleased + new version header
/^## \[Unreleased\]/ {
    print "## [Unreleased]"
    print ""
    print "## [" version "] - " date
    done_unreleased = 1
    next
}

# Update bottom links
/^\[Unreleased\]:/ {
    print "[Unreleased]: https://github.com/" repo "/compare/v" version "...HEAD"
    print "[" version "]: https://github.com/" repo "/compare/v" prev "...v" version
    next
}

# Print everything else as-is
{ print }
' "$CHANGELOG" > "${CHANGELOG}.tmp"

mv "${CHANGELOG}.tmp" "$CHANGELOG"

echo "Updated $CHANGELOG: [Unreleased] → [${VERSION}] - ${DATE}"
echo "Previous version: ${PREV_VERSION}"
