-- Contextify: Telemetry Events
-- Captures recall/store funnel events for analytics and quality tuning.

CREATE TABLE IF NOT EXISTS memory_telemetry_events (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type    TEXT NOT NULL,
    session_id    TEXT,
    request_id    TEXT,
    agent_source  TEXT,
    project_id    TEXT,
    memory_id     UUID,
    query_text    TEXT,
    action        TEXT,
    hit_count     INTEGER,
    latency_ms    INTEGER,
    metadata      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_telemetry_event_type_created
    ON memory_telemetry_events (event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_created
    ON memory_telemetry_events (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_telemetry_project
    ON memory_telemetry_events (project_id);

CREATE INDEX IF NOT EXISTS idx_telemetry_agent
    ON memory_telemetry_events (agent_source);

CREATE INDEX IF NOT EXISTS idx_telemetry_session
    ON memory_telemetry_events (session_id);
