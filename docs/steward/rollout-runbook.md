# Steward Rollout, SLOs, and Operator Runbook

This document defines a phased rollout plan for the in-process steward runtime, measurable success criteria, alert thresholds, and operational response steps.

## Local-First Assumptions

- Steward remains **disabled by default**
- Rollout is per deployment (single local node / self-hosted instance)
- Operators can pause steward at runtime via UI or API
- Dry-run mode is the default first activation mode

## Model Sizing Expectations (Low-Resource Machines)

- Embeddings: `nomic-embed-text` (existing default)
- Steward conflict-guard LLM (optional):
  - Minimum: 2-4 GB RAM free (small quantized local model)
  - Recommended: 8 GB RAM free for smoother concurrent use
- If machine is memory constrained:
  - keep `llm_conflict_guard_enabled=false`
  - run steward in `dry_run=true`
  - lower `claim_batch_size`

## Phased Rollout Plan

## Phase A — Enabled + Dry-Run Only

Configuration:

- `steward.enabled=true`
- `steward.dry_run=true`
- `steward.auto_merge_from_suggestions=true`
- `steward.derivation.enabled=false`
- `steward.self_learn.enabled=false`

Entry criteria:

- Baseline `go test ./...` green
- Steward UI `/steward` loads successfully
- Observability endpoints respond (`status`, `runs`, `events`, `metrics`)

Success gate (minimum 24h):

- `dead_letter_total <= 5`
- `circuit_breaker.open = false` for >95% of samples
- no user-visible store path regression >50ms p95 compared to baseline

Rollback trigger:

- repeated breaker-open state >10 minutes
- dead-letter growth >20/day
- runaway queue depth near cap for >15 minutes

Rollback action:

1. Pause steward
2. Keep dry-run on
3. Inspect `/steward` failed runs and event timeline

## Phase B — Auto-Merge Only (High Confidence Subset)

Configuration delta:

- `steward.dry_run=false`
- `steward.derivation.enabled=false`
- `steward.auto_merge_threshold >= 0.92`
- optionally `steward.llm_conflict_guard_enabled=true`

Success gate (minimum 48h):

- wrong auto-merge rate < `1%`
- steward run success rate >= `95%`
- queue depth remains below `70%` of configured cap

Rollback trigger:

- wrong auto-merge rate >= `1%`
- breaker open continuously >15 minutes (if LLM guard enabled)

Rollback action:

1. Switch to `dry_run=true`
2. Pause steward if queue is unstable
3. Retry/cancel dead-lettered jobs only after root-cause review

## Phase C — Derivation Enabled (Strict Thresholds)

Configuration delta:

- `steward.derivation.enabled=true`
- `steward.derivation.min_confidence >= 0.80`
- `steward.derivation.min_novelty >= 0.20`

Success gate (minimum 72h):

- derivation acceptance rate > `60%`
- derivation dead-letter contribution < `20%` of all dead letters
- no sustained queue saturation

Rollback trigger:

- derivation acceptance < `40%` for 2 consecutive review windows
- queue depth > `85%` cap for >15 minutes

Rollback action:

1. Disable derivation
2. Leave auto-merge in current mode
3. Review derivation payloads and thresholds

## Phase D — Self-Learn Enabled (Conservative Cadence)

Configuration delta:

- `steward.self_learn.enabled=true`
- `steward.self_learn.eval_interval >= 24h`
- `steward.self_learn.min_sample_size >= 100`

Success gate (minimum 7 days):

- policy changes are infrequent and bounded
- no SLO regression after policy updates
- rollback endpoint tested successfully

Rollback trigger:

- quality metrics worsen for 2 consecutive evaluation cycles
- unsafe threshold drift observed (outside approved bounds)

Rollback action:

1. Disable self-learn
2. Use policy rollback endpoint for affected key
3. Revert to last known-good thresholds

## SLOs and Alert Thresholds

## Primary SLOs

- Wrong auto-merge rate: `< 1%` (Phase B+)
- Derivation acceptance rate: `> 60%` (Phase C+)
- Store path latency regression: `< 50ms p95` increase versus baseline
- Dead-letter rate: `< 5%` of steward runs per 24h

## Alert Conditions

- Queue depth alert:
  - warning: `queued_total >= 70%` of `max_queued_total`
  - critical: `queued_total >= 90%` of `max_queued_total` for 10 minutes
- Failure spike alert:
  - failed runs > `20` in 15 minutes OR success rate < `80%` in 1 hour
- Breaker-open alert:
  - `circuit_breaker.open=true` for >10 minutes
- Token anomaly alert:
  - avg tokens/run > `2x` 7-day rolling baseline (when tracked)

## Rollback Drill (Operational Validation)

Run at least once before broad rollout:

1. Enable steward in dry-run.
2. Trigger run-once.
3. Toggle to write-enabled, then immediately back to dry-run.
4. Roll back one policy key (if policy history exists).
5. Confirm status endpoint reflects expected mode and breaker/health fields.

## Operator Runbook

## Pause Steward

UI:

- Open `/steward`
- Click `Pause`

API:

```bash
curl -X PUT http://localhost:8420/api/v1/steward/mode \
  -H 'Content-Type: application/json' \
  -d '{"paused":true,"dry_run":true}'
```

If `STEWARD_ADMIN_TOKEN` is set, also add:

```bash
-H "X-Steward-Admin-Token: $STEWARD_ADMIN_TOKEN"
```

## Inspect Failed Runs

1. Open `/steward`
2. Filter `status=failed` or `dead_letter`
3. Inspect:
   - input snapshot
   - model output JSON
   - event timeline
   - redaction markers/reasons

## Replay (Retry) Dead-Letter Job

UI:

- Select failed/dead-letter row
- Click `Retry` in row actions

API:

```bash
curl -X POST http://localhost:8420/api/v1/steward/jobs/<job-id>/retry
```

## Cancel Queued/Running Job

UI:

- Click `Cancel` on queued/running row (confirmation required)

API:

```bash
curl -X POST http://localhost:8420/api/v1/steward/jobs/<job-id>/cancel
```

## Adjust Thresholds Safely

1. Start in `dry_run=true`
2. Change one threshold at a time
3. Observe 24h metrics (`success_rate`, queue depth, dead-letter count)
4. Proceed only if SLOs remain green

## Rollback Policy Version

```bash
curl -X POST http://localhost:8420/api/v1/steward/policies/rollback \
  -H 'Content-Type: application/json' \
  -d '{"policy_key":"auto_merge_threshold"}'
```

Supported keys:

- `auto_merge_threshold`
- `derivation.min_confidence`
- `derivation.min_novelty`

## Documentation Checklist for Rollout

- ADR and architecture docs reviewed
- telemetry contract documented
- reliability/log-security/verification docs reviewed
- rollout owner acknowledges rollback triggers and actions
