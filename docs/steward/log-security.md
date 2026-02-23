# Steward Log Security

This document describes the steward log safety controls for run snapshots, event payloads, and steward admin operations.

## Redaction

Steward redacts persisted run/event payloads before writing to `steward_runs` and `steward_events`.

Redaction targets include:

- bearer tokens
- API-key-like token patterns
- PEM/private key blocks
- sensitive field names (for example `password`, `secret`, `token`, `authorization`, `private_key`, `env`)

Redacted payloads include markers:

- `_redacted: true`
- `_redaction_reasons: [...]`

UI surfaces these as explicit `REDACTED` indicators.

## Retention

Retention cleanup runs from the steward runtime loop (hourly cadence) using configured values:

- `steward.retention.run_log_days`
- `steward.retention.event_log_days`

Rows older than the configured window are hard-deleted.

## API Guardrails

Steward write/admin endpoints support an optional token guard:

- Set env `STEWARD_ADMIN_TOKEN`
- Send header `X-Steward-Admin-Token: <token>`

When the env var is set, steward admin/write endpoints fail closed with `403` unless the header matches.

Protected endpoints include:

- run-once
- mode updates
- retry/cancel job
- policy rollback

Read-only steward endpoints remain accessible for observability.
