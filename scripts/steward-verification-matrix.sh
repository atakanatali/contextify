#!/usr/bin/env bash
set -euo pipefail

OUT_PATH="${STEWARD_VERIFY_REPORT_PATH:-artifacts/steward-verification-matrix.json}"
mkdir -p "$(dirname "$OUT_PATH")"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

run_step() {
  local name="$1"
  shift
  local logfile="$tmpdir/${name}.log"
  local status="pass"
  local started ended
  started="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  if ! "$@" >"$logfile" 2>&1; then
    status="fail"
  fi
  ended="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  printf '{"name":"%s","status":"%s","started_at":"%s","ended_at":"%s","log_path":"%s"}' \
    "$name" "$status" "$started" "$ended" "$logfile"
  return 0
}

unit="$(run_step unit_steward go test ./internal/steward -count=1)"
api="$(run_step api_handlers go test ./internal/api -count=1)"
full="$(run_step full_repo go test ./... -count=1)"

cat > "$OUT_PATH" <<EOF
{
  "generated_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "suite": "steward_verification_matrix",
  "steps": [
    $unit,
    $api,
    $full
  ],
  "notes": [
    "E2E steward UI/control tests require a running local stack and are executed separately.",
    "Recall benchmark artifact is produced by the existing bench-recall workflow step."
  ]
}
EOF

echo "Wrote $OUT_PATH"
