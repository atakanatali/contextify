-- Contextify: Steward runtime queue, runs, events, derivation lineage, and policy history

CREATE TABLE IF NOT EXISTS steward_jobs (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_type         TEXT NOT NULL,
    project_id       TEXT,
    source_memory_ids UUID[] NOT NULL DEFAULT '{}',
    trigger_reason   TEXT,
    payload          JSONB NOT NULL DEFAULT '{}'::jsonb,
    status           TEXT NOT NULL DEFAULT 'queued',
    priority         INTEGER NOT NULL DEFAULT 0,
    attempt_count    INTEGER NOT NULL DEFAULT 0,
    max_attempts     INTEGER NOT NULL DEFAULT 3,
    run_after        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_by        TEXT,
    locked_at        TIMESTAMPTZ,
    lease_expires_at TIMESTAMPTZ,
    last_error       TEXT,
    idempotency_key  TEXT,
    cancelled_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (max_attempts >= 1),
    CHECK (attempt_count >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_steward_jobs_idempotency_key
    ON steward_jobs (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_steward_jobs_status_run_after_priority
    ON steward_jobs (status, run_after, priority DESC);

CREATE INDEX IF NOT EXISTS idx_steward_jobs_project_status
    ON steward_jobs (project_id, status);

CREATE INDEX IF NOT EXISTS idx_steward_jobs_created
    ON steward_jobs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_steward_jobs_locked
    ON steward_jobs (status, lease_expires_at)
    WHERE status = 'running';

CREATE TRIGGER steward_jobs_updated_at
    BEFORE UPDATE ON steward_jobs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TABLE IF NOT EXISTS steward_runs (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id            UUID REFERENCES steward_jobs(id),
    provider          TEXT,
    model             TEXT,
    input_snapshot    JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_snapshot   JSONB,
    input_hash        TEXT,
    prompt_tokens     INTEGER,
    completion_tokens INTEGER,
    total_tokens      INTEGER,
    latency_ms        INTEGER,
    status            TEXT NOT NULL DEFAULT 'running',
    error_class       TEXT,
    error_message     TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_steward_runs_job_created
    ON steward_runs (job_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_steward_runs_status_created
    ON steward_runs (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_steward_runs_model_created
    ON steward_runs (model, created_at DESC);

CREATE TABLE IF NOT EXISTS steward_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id       UUID REFERENCES steward_jobs(id),
    run_id       UUID REFERENCES steward_runs(id),
    event_type   TEXT NOT NULL,
    data         JSONB NOT NULL DEFAULT '{}'::jsonb,
    schema_version INTEGER NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_steward_events_job_created
    ON steward_events (job_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_steward_events_run_created
    ON steward_events (run_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_steward_events_type_created
    ON steward_events (event_type, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_derivations (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_memory_ids UUID[] NOT NULL DEFAULT '{}',
    derived_memory_id UUID,
    derivation_type  TEXT NOT NULL,
    confidence       REAL,
    novelty          REAL,
    status           TEXT NOT NULL DEFAULT 'candidate',
    model            TEXT,
    payload          JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_memory_derivations_derived_memory
    ON memory_derivations (derived_memory_id);

CREATE INDEX IF NOT EXISTS idx_memory_derivations_created
    ON memory_derivations (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_memory_derivations_status_created
    ON memory_derivations (status, created_at DESC);

CREATE TRIGGER memory_derivations_updated_at
    BEFORE UPDATE ON memory_derivations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TABLE IF NOT EXISTS steward_policy_history (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    policy_key     TEXT NOT NULL,
    prior_value    DOUBLE PRECISION,
    new_value      DOUBLE PRECISION,
    reason         TEXT,
    sample_size    INTEGER,
    evidence       JSONB NOT NULL DEFAULT '{}'::jsonb,
    changed_by     TEXT NOT NULL DEFAULT 'steward',
    rollback_of_id UUID,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (sample_size IS NULL OR sample_size >= 0)
);

CREATE INDEX IF NOT EXISTS idx_steward_policy_history_key_created
    ON steward_policy_history (policy_key, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_steward_policy_history_created
    ON steward_policy_history (created_at DESC);
