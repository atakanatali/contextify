# Steward Reliability Hardening

This document summarizes the runtime guardrails added for continuous steward operation.

## Runtime Safeguards

- Backpressure queue limits
  - Global queued job cap is derived from `claim_batch_size * 20` (minimum `100`).
  - Per-project queued job cap is derived from `claim_batch_size * 5` (minimum `20`).
  - Auto-merge suggestion job enqueue honors both caps.
  - Policy tuning enqueue is skipped when global queue cap is reached.

- Circuit breaker (LLM conflict-guard path)
  - Tracks consecutive failures for `auto_merge_from_suggestion` jobs while LLM conflict guard is enabled.
  - Opens after 3 consecutive failures and enters cooldown (2 minutes).
  - While open, steward falls back to deterministic auto-merge execution by skipping the LLM conflict-guard call.
  - After cooldown, LLM calls are allowed again; a successful auto-merge clears breaker state.

- Recovery
  - Stale `running` jobs are recovered on startup using lease expiry and requeued/dead-lettered according to attempt count.
  - `startup_recovered_stale_jobs` is exposed in steward status for operator visibility.

## Health Signals (Status Endpoint)

`GET /api/v1/steward/status` includes:

- `health.queued_total`
- `health.queued_by_project_top`
- `health.dead_letter_total`
- `health.average_processing_latency_ms`
- `circuit_breaker` state and timestamps
- `backpressure` limits

## Failure Handling

1. If queue depth remains near cap, pause steward and inspect `queued_by_project_top`.
2. If circuit breaker is open, inspect recent failed `auto_merge_from_suggestion` runs/events.
3. Use UI/API controls to cancel, retry, or pause processing while investigating.
4. Resume after queue depth and failure rate normalize.
