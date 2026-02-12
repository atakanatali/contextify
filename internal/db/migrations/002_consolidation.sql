-- Contextify: Memory Consolidation & Deduplication
-- Adds merge tracking, consolidation log, and duplicate suggestions

-- Add consolidation fields to memories table
ALTER TABLE memories ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
ALTER TABLE memories ADD COLUMN merged_from UUID[] DEFAULT '{}';
ALTER TABLE memories ADD COLUMN replaced_by UUID;

-- Index for finding replaced (soft-deleted) memories
CREATE INDEX idx_memories_replaced_by ON memories(replaced_by) WHERE replaced_by IS NOT NULL;

-- Consolidation audit log
CREATE TABLE consolidation_log (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    target_id       UUID NOT NULL,
    source_ids      UUID[] NOT NULL,
    merge_strategy  TEXT NOT NULL,
    similarity_score REAL,
    content_before  TEXT,
    content_after   TEXT,
    performed_by    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_consolidation_log_target ON consolidation_log(target_id);
CREATE INDEX idx_consolidation_log_created ON consolidation_log(created_at DESC);

-- Duplicate detection suggestions (populated by background scanner)
CREATE TABLE consolidation_suggestions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    memory_a_id     UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    memory_b_id     UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    similarity      REAL NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    project_id      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    UNIQUE(memory_a_id, memory_b_id)
);

CREATE INDEX idx_suggestions_status ON consolidation_suggestions(status) WHERE status = 'pending';
CREATE INDEX idx_suggestions_project ON consolidation_suggestions(project_id);
CREATE INDEX idx_suggestions_similarity ON consolidation_suggestions(similarity DESC);
