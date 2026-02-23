# ADR-001: Memory Steward for 24/7 Lifecycle Management

- Status: Accepted (architecture baseline for STW02+)
- Date: 2026-02-23
- Owners: Contextify maintainers
- Related Issues: #39 (STW01), STW02-STW16

## Context

Contextify already performs memory lifecycle actions in multiple places:
- `Store()` handles immediate dedup checks, auto-merge, and suggestion generation.
- Background schedulers handle TTL cleanup, dedup scans, and project ID normalization.
- Telemetry exists for recall/store funnels, but there is no steward-specific execution trace or job lifecycle.

This split makes it difficult to safely introduce continuous autonomous memory maintenance (dedup rechecks, model-assisted merge planning, derivations, policy tuning) with clear runtime ownership, retries, auditability, and operator controls.

## Decision

Introduce a local-first **Memory Steward** subsystem that runs inside the Contextify server process and is disabled by default. The Steward provides a job-oriented control plane for autonomous memory lifecycle actions with:
- PostgreSQL-backed queue and run/event audit trail
- single active leader per deployment using PostgreSQL advisory lock
- claim-loop execution using `FOR UPDATE SKIP LOCKED`
- deterministic guardrails around model-assisted decisions
- complete observability records suitable for later UI inspection

When disabled, existing store/search/consolidation behavior remains unchanged.

## Goals

- Centralize autonomous memory lifecycle execution behind one runtime contract.
- Make every steward write action auditable and replayable.
- Support phased rollout (dry-run -> selective apply -> guarded auto-apply).
- Enable future UI/API controls (pause/resume/run-once/retry/cancel) without redesigning execution semantics.

## Non-Goals

- Replacing the existing `Store()` smart-dedup path in STW01.
- Adding external cloud dependencies or hosted queues.
- Building a distributed multi-region scheduler.
- Full policy self-tuning in this ADR (only contract and safety envelope are defined here).
- UI implementation details beyond telemetry shape needed to support UI later.

## Constraints and Assumptions

- Local-first deployment: PostgreSQL + Ollama are available locally/in-cluster.
- Backward compatibility: steward disabled means current behavior is preserved.
- Autonomous actions may use model output, but final write behavior must remain deterministic and validated.
- PostgreSQL is the source of truth for queue, run state, and audit events.

## Module Boundaries

### 1. Orchestrator

Responsibilities:
- process lifecycle (start/stop)
- feature flag and config gating
- acquire/renew leader lock
- supervise worker loops and backoff
- expose in-memory health and counters to metrics layer

### 2. Queue / Claimer

Responsibilities:
- create jobs from triggers (scanner, API, policy, periodic recheck)
- claim jobs with `FOR UPDATE SKIP LOCKED`
- lease/heartbeat tracking
- retry scheduling and dead-letter transitions

### 3. Executors

Executors are job-type specific workers behind a shared interface.

Required job types:
- `auto_merge`: auto-apply high-confidence duplicate merges with guardrails
- `derive`: derive candidate memories from source material or consolidations (dry-run first)
- `recheck`: re-evaluate previously skipped or pending candidates
- `policy_tune`: propose threshold changes (initially dry-run and suggestion-only)

Executor contract:
- validate input snapshot
- optionally call model(s)
- produce structured decision payload
- validate decision against deterministic rules
- either apply DB writes or emit `write_skipped`
- emit completion/failure events

### 4. Audit Logger

Responsibilities:
- persist run timeline and event stream
- persist input/output snapshots (with redaction)
- persist token/latency/model metadata
- ensure append-only semantics for event rows

### 5. Metrics Emitter

Responsibilities:
- counters/histograms for queue depth, claim rate, success/failure, latency, retries
- leader state gauge
- per-job-type outcomes
- export dimensions compatible with future API/UI summaries

## Job Model and State Machine

### Canonical Job States

- `queued`
- `running`
- `succeeded`
- `failed`
- `dead_letter`
- `cancelled`

### Transition Rules

- `queued -> running`: successful claim by active leader worker
- `running -> succeeded`: executor completed and all required writes/events committed
- `running -> failed`: executor failed; retry may be scheduled depending on policy
- `failed -> queued`: retry scheduled with incremented attempt count and backoff
- `failed -> dead_letter`: retry limit exceeded or non-retryable validation failure
- `queued -> cancelled`: operator/admin cancel before claim
- `running -> cancelled`: cooperative cancel only (best-effort; terminal event must explain partial work status)

### Retry / Idempotency Strategy

- Every job has an idempotency key derived from job type + target resource + decision context version.
- Writes must use one of:
  - unique constraints + upsert semantics
  - compare-and-swap version checks
  - explicit “already applied” detection before mutation
- Retried jobs must emit a new run attempt but reuse the same logical job id.
- Non-retryable failures (invalid payload, invariant violation, unsupported state) go directly to `dead_letter`.

## Concurrency and Leadership

### Single Active Leader

Use PostgreSQL advisory lock to ensure exactly one active leader per deployment:
- Orchestrator attempts lock on startup and periodically re-verifies lock ownership.
- If lock is lost, worker claim loops stop and in-flight work is allowed to finish or fail safely.
- Non-leader replicas remain passive but may expose read-only status.

### Worker Claiming

Workers claim jobs using `FOR UPDATE SKIP LOCKED` in short transactions:
- select eligible `queued` jobs ordered by priority + created_at
- mark as `running`, set lease owner and lease expiry, increment attempt if applicable
- commit claim before executing long-running work

### Leases and Heartbeats

- Running jobs carry a lease expiry timestamp.
- Executor heartbeats extend lease.
- Expired running jobs may be re-queued by a sweeper after a safety delay.
- Reclaim logic must preserve idempotency and append a recovery event.

## Integration with Existing Contextify Components

### Memory Service Integration

- Steward executors call existing `memory.Service` methods where possible to preserve business rules (normalization, cache invalidation, audit consistency).
- New steward-specific write paths must remain compatible with existing repository constraints and consolidation semantics.
- `Store()` smart dedup remains active and independent while steward is disabled or in dry-run mode.

### Consolidation Integration

- `auto_merge` executor reuses consolidation merge logic and audit logging patterns.
- Steward decisions must record why a merge was applied/skipped, including thresholds and confidence.
- Suggestions created by scanner/UI remain valid triggers for steward jobs.

### Scheduler Integration

- Existing periodic goroutines remain in place initially.
- STW02+ may route scanner outputs into steward queue instead of direct actions.
- Steward orchestrator should be started alongside existing scheduler stack and honor the same shutdown lifecycle.

### Telemetry Integration

- Existing recall/store telemetry remains unchanged.
- Steward emits separate event streams and aggregate metrics.
- Future analytics endpoints may join recall/store telemetry with steward events for full lifecycle insight.

## Steward Event Model (Audit Timeline)

Required event types:
- `job_created`
- `job_claimed`
- `model_invoked`
- `decision_made`
- `write_applied`
- `write_skipped`
- `job_completed`
- `job_failed`

Event requirements:
- append-only
- timestamped with server time (`TIMESTAMPTZ`)
- include `job_id`, `run_id`, `attempt`, `job_type`
- include actor (`system`, `steward`, `operator`, future API client)
- include structured payload and redaction metadata

## UI-Oriented Telemetry Contract (Future STW08/STW10)

Each steward run/event must support later UI inspection with the following fields (schema may be split across run + event tables):

- `input_snapshot_redacted` (JSON)
- `output_json` (JSON; structured model output or deterministic result)
- `model_name` (nullable for non-model steps)
- `prompt_tokens` (nullable integer)
- `completion_tokens` (nullable integer)
- `total_tokens` (nullable integer)
- `latency_ms` (nullable integer)
- `error_class` (nullable string)
- `error_message_redacted` (nullable string)
- `decision_code` (string; deterministic enum-like result)
- `side_effects` (JSON array of applied/skipped writes)

Redaction expectations:
- Raw memory content snapshots may be partially stored, but secrets/credentials must be redacted before persistence.
- Redaction metadata should describe which fields were redacted and why.

## Rollout Strategy

### Phase 1: Dry-Run Only

- Jobs can be queued/claimed/executed.
- Model calls and decisions are recorded.
- No memory writes are applied by steward executors.
- `write_skipped` events explain what would have happened.

### Phase 2: Selective Auto-Apply

- Enable auto-apply only for explicitly whitelisted job types (for example `auto_merge`) and confidence thresholds.
- Require deterministic validation before write.
- Keep operator-visible audit trail and rollback guidance.

### Phase 3: Full Auto with Guardrails

- Multiple job types may auto-apply.
- Policy tuning remains suggestion-driven unless separately approved.
- Operational controls (pause/resume/cancel/retry) and UI visibility are mandatory.

## Alternatives Considered

### A. Keep Logic in Existing Store + Background Jobs (Rejected)

Pros:
- Minimal short-term code changes

Cons:
- No unified lifecycle
- weak auditability for autonomous actions
- hard to add retries/cancellation/controls
- fragmented observability

### B. External Queue / Workflow Engine (Rejected for now)

Pros:
- Mature scheduling/retry semantics

Cons:
- Violates local-first simplicity goal
- adds operational complexity
- creates integration overhead before the contract is mature

## Tradeoffs

- PostgreSQL queue is simpler to deploy but requires careful indexing/locking discipline at scale.
- Single-leader design simplifies correctness but constrains horizontal execution until sharding/partitioning is introduced.
- Rich audit events increase storage cost, but this is required for trust and operator debugging.

## Risk Register

- Operational risk: Leader lock flapping can pause or duplicate work during DB instability.
  - Mitigation: lease-based claims, heartbeat grace periods, lock-loss detection, recovery events.
- Operational risk: Model latency spikes can exhaust worker concurrency and queue throughput.
  - Mitigation: bounded worker pools, per-job timeouts, backpressure, circuit breakers (STW13).
- Integration risk: Steward writes may bypass existing `memory.Service` invariants if executors write directly.
  - Mitigation: route through service methods by default and document explicit exceptions.
- Integration risk: Event schema drift can break future UI/API expectations.
  - Mitigation: define telemetry contract early (this ADR) and version payloads.
- Security/privacy risk: Audit snapshots may store sensitive content.
  - Mitigation: mandatory redaction pipeline, retention policy, access guardrails (STW14).

## Acceptance / Review Checklist

- State machine and retries are explicit and testable.
- Leadership and claim semantics are deterministic.
- Every autonomous write has an audit event path.
- Steward disabled path preserves existing Contextify behavior.
- UI-oriented telemetry fields are defined for future implementation.
