# Architecture

Technical deep-dive into how Contextify works under the hood.

## Core Architecture

```mermaid
graph TB
    subgraph Agents["AI Agents"]
        CC[Claude Code<br/>MCP]
        CU[Cursor<br/>MCP]
        GE[Gemini<br/>REST]
        AG[Antigravity<br/>REST]
        OT[Other Tools<br/>REST]
    end

    subgraph Contextify["Contextify Server :8420"]
        direction TB
        MCP[MCP Handler<br/>/mcp]
        REST[REST API<br/>/api/v1/]
        WEB[Web UI<br/>SPA Static Files]
        SVC[Memory Service<br/>Business Logic]
        SCH[Cleanup Scheduler<br/>TTL Expiry]
    end

    subgraph Storage["Data Layer"]
        PG[(PostgreSQL 16<br/>+ pgvector)]
        OL[Ollama<br/>nomic-embed-text]
    end

    CC -->|Streamable HTTP| MCP
    CU -->|Streamable HTTP| MCP
    GE -->|HTTP JSON| REST
    AG -->|HTTP JSON| REST
    OT -->|HTTP JSON| REST

    MCP --> SVC
    REST --> SVC
    WEB -.->|fetch /api/v1/| REST
    SCH -->|every 5min| SVC

    SVC -->|CRUD + Search| PG
    SVC -->|Embed text| OL

    style MCP fill:#4f46e5,color:#fff
    style REST fill:#059669,color:#fff
    style WEB fill:#d97706,color:#fff
    style PG fill:#2563eb,color:#fff
    style OL fill:#7c3aed,color:#fff
    style SVC fill:#dc2626,color:#fff
    style SCH fill:#6b7280,color:#fff
```

## Request Flow

### Store Memory

```mermaid
sequenceDiagram
    participant Agent
    participant Server
    participant Ollama
    participant PostgreSQL

    Agent->>Server: store_memory(title, content, tags, importance)
    Server->>Ollama: Embed(title + content)
    Ollama-->>Server: float32[768]
    Server->>Server: Set TTL / auto-permanent (importance >= 0.8)
    Server->>PostgreSQL: INSERT INTO memories (... embedding ...)
    PostgreSQL-->>Server: OK
    Server-->>Agent: {id, title, type, scope, ...}
```

### Hybrid Search

```mermaid
sequenceDiagram
    participant Agent
    participant Server
    participant Ollama
    participant PostgreSQL

    Agent->>Server: recall_memories("how did we fix the auth bug?")
    Server->>Ollama: Embed(query)
    Ollama-->>Server: float32[768]
    Server->>PostgreSQL: Hybrid Search (CTE)
    Note over PostgreSQL: vector_score = 1 - (embedding <=> query)<br/>keyword_score = ts_rank(tsvector, tsquery)<br/>combined = 0.7 * vector + 0.3 * keyword
    PostgreSQL-->>Server: Ranked results
    Server->>Server: Async increment access_count for results
    Server-->>Agent: [{memory, score, match_type}, ...]
```

### TTL Lifecycle

```mermaid
stateDiagram-v2
    [*] --> ShortTerm: store (importance < 0.8)
    [*] --> LongTerm: store (importance >= 0.8)

    ShortTerm --> ShortTerm: access (TTL += 50%)
    ShortTerm --> LongTerm: access_count >= 5
    ShortTerm --> LongTerm: promote_memory()
    ShortTerm --> Expired: TTL reached

    Expired --> [*]: cleanup scheduler (every 5min)
    LongTerm --> [*]: manual delete
```

## Project Structure

```
contextify/
├── cmd/server/main.go           # Entry point, wires all components
├── internal/
│   ├── api/
│   │   ├── router.go            # Chi router, CORS, SPA static files
│   │   ├── handlers.go          # REST endpoint handlers
│   │   └── middleware.go         # Request ID + logging
│   ├── config/
│   │   └── config.go            # YAML + env config loader
│   ├── db/
│   │   ├── postgres.go          # Connection pool + embedded migrations
│   │   └── migrations/
│   │       └── 001_init.sql     # Schema: memories + relationships + indexes
│   ├── embedding/
│   │   └── ollama.go            # Ollama HTTP client, auto-pull model
│   ├── mcp/
│   │   ├── server.go            # MCP server setup (Streamable HTTP)
│   │   └── tools.go             # 10 MCP tool handlers
│   ├── memory/
│   │   ├── model.go             # Memory, Relationship, Stats structs
│   │   ├── repository.go        # PostgreSQL CRUD + hybrid search
│   │   └── service.go           # Business logic, auto-promote, TTL
│   └── scheduler/
│       └── cleanup.go           # Background TTL cleanup goroutine
├── web/                         # React + Vite + Tailwind CSS
│   ├── src/
│   │   ├── api.js               # API client (fetch /api/v1/)
│   │   ├── pages/               # Dashboard, MemoryBrowser, Search
│   │   └── components/          # Layout, MemoryCard
│   └── ...
├── Dockerfile                   # Dev image: Go server + Web UI (3-stage)
├── Dockerfile.allinone          # Prod image: PG + Ollama + model + server + UI
├── docker/
│   └── entrypoint.sh            # All-in-one process manager (PG → Ollama → server)
├── docker-compose.yml           # Development (separate services)
└── docker-compose.prod.yml      # Production (single all-in-one container)
```

## Database Schema

### memories table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key (auto-generated) |
| `title` | TEXT | Short descriptive title |
| `content` | TEXT | Detailed memory content |
| `summary` | TEXT | Optional brief summary |
| `embedding` | vector(768) | nomic-embed-text embedding |
| `type` | ENUM | solution, problem, code_pattern, fix, error, workflow, decision, general |
| `scope` | ENUM | global (all projects) or project (scoped) |
| `project_id` | TEXT | Project identifier for scoped memories |
| `agent_source` | TEXT | Source agent (claude-code, cursor, gemini, etc.) |
| `tags` | TEXT[] | Array of tags for filtering |
| `importance` | REAL | 0.0-1.0 score (>= 0.8 = auto-permanent) |
| `ttl_seconds` | INTEGER | Time-to-live (NULL = permanent) |
| `access_count` | INTEGER | Number of reads (>= 5 = auto-promoted) |
| `expires_at` | TIMESTAMPTZ | Computed expiry time |
| `created_at` | TIMESTAMPTZ | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | Last update (auto-trigger) |

### memory_relationships table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `from_memory_id` | UUID | Source memory (FK, CASCADE) |
| `to_memory_id` | UUID | Target memory (FK, CASCADE) |
| `relationship` | TEXT | Type: SOLVES, CAUSES, REQUIRES, RELATED_TO, etc. |
| `strength` | REAL | 0.0-1.0 relationship strength |
| `context` | TEXT | Optional description |

### Indexes

| Index | Type | Purpose |
|-------|------|---------|
| `idx_memories_embedding` | HNSW (cosine) | Vector similarity search (m=16, ef_construction=64) |
| `idx_memories_content_fts` | GIN (tsvector) | Full-text keyword search |
| `idx_memories_tags` | GIN | Tag array containment queries |
| `idx_memories_type` | B-tree | Filter by memory type |
| `idx_memories_scope` | B-tree | Filter by scope |
| `idx_memories_project` | B-tree | Filter by project |
| `idx_memories_importance` | B-tree | Filter by importance |
| `idx_memories_expires` | B-tree (partial) | TTL cleanup (only non-null) |
| `idx_memories_created` | B-tree (DESC) | Recent memories first |
| `idx_memories_agent` | B-tree | Filter by agent source |

## Hybrid Search Algorithm

The search combines vector similarity and keyword matching using PostgreSQL CTEs:

```sql
WITH vector_scores AS (
    SELECT id, 1 - (embedding <=> query_embedding) AS vector_score
    FROM memories
    WHERE ...filters...
),
keyword_scores AS (
    SELECT id, ts_rank(to_tsvector('english', content), plainto_tsquery('english', query)) AS keyword_score
    FROM memories
    WHERE ...filters...
)
SELECT m.*,
       (v.vector_score * 0.7 + k.keyword_score * 0.3) AS combined_score
FROM memories m
LEFT JOIN vector_scores v ON m.id = v.id
LEFT JOIN keyword_scores k ON m.id = k.id
ORDER BY combined_score DESC
```

**Weight distribution**: 70% vector (semantic meaning) + 30% keyword (exact term matching). This ensures that semantically similar results rank high while still boosting exact keyword matches.

**Why HNSW over IVFFlat**: HNSW supports incremental inserts without rebuilding the index. Since memories are continuously added and deleted (TTL), IVFFlat would require periodic reindexing. HNSW maintains consistent recall as data changes.

## Memory Lifecycle

| Event | Action |
|-------|--------|
| **Store** (importance < 0.8) | Short-term memory with default TTL (24h) |
| **Store** (importance >= 0.8) | Permanent long-term memory (no TTL) |
| **Access** (read/search hit) | TTL extended by 50% of original |
| **Access count >= 5** | Auto-promoted to permanent (TTL removed) |
| **Manual promote** | TTL removed, becomes permanent |
| **TTL expiry** | Cleaned up by scheduler (every 5 minutes) |

## Transport Protocols

### MCP (Model Context Protocol)

- **Endpoint**: `/mcp`
- **Transport**: Streamable HTTP (SSE deprecated since March 2025)
- **SDK**: `github.com/modelcontextprotocol/go-sdk v1.3.0`
- **Compatible agents**: Claude Code, Cursor, any MCP-capable client

### REST API

- **Base path**: `/api/v1/`
- **Format**: JSON request/response
- **Compatible agents**: Gemini, Antigravity, custom integrations

### Web UI

- **Path**: `/` (SPA with client-side routing)
- **Stack**: React + Vite + Tailwind CSS
- **Served by**: Go server's static file handler (no separate nginx)

## Embedding Pipeline

1. Agent stores a memory with `title` and `content`
2. Server concatenates `title + " " + content`
3. Ollama generates a 768-dimensional embedding using `nomic-embed-text`
4. Embedding is stored as `vector(768)` in PostgreSQL
5. On search, the query text is embedded the same way
6. PostgreSQL HNSW index finds nearest neighbors using cosine distance

**Model auto-pull**: On startup, the server calls Ollama's `/api/show` endpoint. If `nomic-embed-text` is not found, it automatically pulls the model via `/api/pull`.

## Configuration

Configuration is loaded from `config.yaml` with environment variable overrides:

| Env Var | Config Path | Default | Description |
|---------|-------------|---------|-------------|
| `DATABASE_URL` | `database.url` | `postgres://contextify:contextify_local@localhost:5432/contextify?sslmode=disable` | PostgreSQL connection string |
| `OLLAMA_URL` | `embedding.ollama_url` | `http://localhost:11434` | Ollama server URL |
| `EMBEDDING_MODEL` | `embedding.model` | `nomic-embed-text` | Embedding model name |
| `SERVER_PORT` | `server.port` | `8420` | HTTP server port |

### Tunable Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `memory.default_ttl` | 86400 (24h) | Default TTL for new short-term memories |
| `memory.promote_access_count` | 5 | Access count threshold for auto-promotion |
| `memory.promote_importance` | 0.8 | Importance threshold for auto-permanent |
| `memory.ttl_extend_factor` | 0.5 | TTL extension on each access (50%) |
| `memory.cleanup_interval` | 5m | Background cleanup frequency |
| `search.vector_weight` | 0.7 | Vector similarity weight in hybrid search |
| `search.keyword_weight` | 0.3 | Keyword matching weight in hybrid search |
| `search.default_limit` | 20 | Default search result limit |
| `search.max_limit` | 100 | Maximum search result limit |

## Deployment

### All-in-One Image (Production)

The production Docker image (`Dockerfile.allinone`) bundles everything into a single container:

| Component | Details |
|-----------|---------|
| **PostgreSQL 16 + pgvector** | Listens on `127.0.0.1:5432` (internal only) |
| **Ollama** | Listens on `127.0.0.1:11434` (internal only) |
| **nomic-embed-text** | Pre-pulled at build time (~274MB, zero first-run downloads) |
| **Contextify Server** | Go binary + Web UI static files |

**Base image**: `pgvector/pgvector:pg16` (Debian Bookworm)

**Process management**: Bash entrypoint script (`docker/entrypoint.sh`) with PID tracking and `trap SIGTERM` for graceful shutdown. Startup order: PostgreSQL → Ollama → Contextify.

**Volume**: Only PostgreSQL data (`/var/lib/postgresql/data`) needs persistence. Everything else is baked into the image.

**Estimated image size**: ~750MB (pgvector base ~400MB + Ollama ~50MB + model ~274MB + Go+web ~30MB)

### Development Setup

Development uses separate containers (`docker-compose.yml`) for fast iteration. The Go server is built from source using the lightweight `Dockerfile` (Alpine-based, no PG/Ollama).

### Connection Retry

The Go server includes connection retry logic (10 attempts, linear backoff) to handle the startup delay when PostgreSQL and Ollama initialize inside the same container.
