# Steward Telemetry Contract (STW08)

This document defines the canonical event taxonomy and payload expectations for steward runtime observability.

## Goals

- Provide UI-ready run/event traces (inputs, outputs, decisions, side effects).
- Keep payloads structured and versionable.
- Support redaction hooks without losing operational utility.

## Event Taxonomy (Minimum)

- `run_started`
- `input_prepared`
- `model_called`
- `model_returned`
- `decision_emitted`
- `write_attempted`
- `write_applied`
- `write_skipped`
- `run_failed`
- `run_succeeded`

Additional runtime events (claim/queue lifecycle) may coexist:
- `job_created`
- `job_claimed`
- `job_completed`
- `job_failed`

## Run Record Contract (`steward_runs`)

- `job_id`
- `provider`
- `model`
- `input_snapshot` (redacted JSON)
- `output_snapshot` (structured JSON)
- `prompt_tokens`
- `completion_tokens`
- `total_tokens`
- `latency_ms`
- `status`
- `error_class`
- `error_message`

## Side Effects Contract

Store side effects as machine-readable JSON (in `output_snapshot.side_effects` and/or event payloads).

Examples:
- `{ "type": "merge_applied", "target_id": "<uuid>", "source_id": "<uuid>" }`
- `{ "type": "suggestion_accepted", "suggestion_id": "<uuid>" }`
- `{ "type": "derivation_created", "derived_memory_id": "<uuid>" }`
- `{ "type": "derivation_skipped", "reason": "threshold", "title": "..." }`

## Redaction Notes

- Payloads are currently structured for redaction but not fully sanitized yet (planned in STW14).
- Callers should prefer summaries/excerpts over raw full content where possible.

## Aggregations for UI

Repository methods should support:
- runs/hour
- success rate
- average tokens/run
- p95 latency
- top failure reasons
