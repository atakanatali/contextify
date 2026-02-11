-- Contextify: Initial Schema
-- PostgreSQL 16 + pgvector

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Memory type enum
CREATE TYPE memory_type AS ENUM (
    'solution',
    'problem',
    'code_pattern',
    'fix',
    'error',
    'workflow',
    'decision',
    'general'
);

-- Memory scope enum
CREATE TYPE memory_scope AS ENUM ('global', 'project');

-- Main memories table
CREATE TABLE memories (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         TEXT NOT NULL,
    content       TEXT NOT NULL,
    summary       TEXT,
    embedding     vector(768),
    type          memory_type NOT NULL DEFAULT 'general',
    scope         memory_scope NOT NULL DEFAULT 'project',
    project_id    TEXT,
    agent_source  TEXT,
    tags          TEXT[] DEFAULT '{}',
    importance    REAL NOT NULL DEFAULT 0.5 CHECK (importance >= 0 AND importance <= 1),
    ttl_seconds   INTEGER,
    access_count  INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ
);

-- Memory relationships table
CREATE TABLE memory_relationships (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_memory_id  UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    to_memory_id    UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    relationship    TEXT NOT NULL,
    strength        REAL DEFAULT 0.5 CHECK (strength >= 0 AND strength <= 1),
    context         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(from_memory_id, to_memory_id, relationship)
);

-- HNSW index for vector similarity search
-- m=16: max connections per node (higher = better recall, more memory)
-- ef_construction=64: build-time search width (higher = better index quality, slower build)
CREATE INDEX idx_memories_embedding ON memories
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- Full-text search index
CREATE INDEX idx_memories_content_fts ON memories
    USING gin (to_tsvector('english', content));

-- B-tree indexes for filtering
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_scope ON memories(scope);
CREATE INDEX idx_memories_project ON memories(project_id);
CREATE INDEX idx_memories_importance ON memories(importance);
CREATE INDEX idx_memories_expires ON memories(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_memories_created ON memories(created_at DESC);
CREATE INDEX idx_memories_agent ON memories(agent_source);

-- GIN index for tag array queries
CREATE INDEX idx_memories_tags ON memories USING gin(tags);

-- Relationship indexes
CREATE INDEX idx_relationships_from ON memory_relationships(from_memory_id);
CREATE INDEX idx_relationships_to ON memory_relationships(to_memory_id);

-- Updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER memories_updated_at
    BEFORE UPDATE ON memories
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
