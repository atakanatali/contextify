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
        NORM[Project Normalizer<br/>VCS-Agnostic]
        SCH[Cleanup Scheduler<br/>TTL Expiry]
        DDS[Dedup Scanner<br/>Similarity Check]
        PNJ[Project Normalizer Job<br/>Legacy Cleanup]
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
    SVC --> NORM
    SCH -->|every 5min| SVC
    DDS -->|periodic| SVC
    PNJ -->|hourly| SVC

    SVC -->|CRUD + Search| PG
    SVC -->|Embed text| OL

    style MCP fill:#4f46e5,color:#fff
    style REST fill:#059669,color:#fff
    style WEB fill:#d97706,color:#fff
    style PG fill:#2563eb,color:#fff
    style OL fill:#7c3aed,color:#fff
    style SVC fill:#dc2626,color:#fff
    style SCH fill:#6b7280,color:#fff
    style NORM fill:#0891b2,color:#fff
    style DDS fill:#6b7280,color:#fff
    style PNJ fill:#6b7280,color:#fff
```

## Request Flow

### Smart Store (with Deduplication)

```mermaid
sequenceDiagram
    participant Agent
    participant Server
    participant Normalizer
    participant Ollama
    participant PostgreSQL

    Agent->>Server: store_memory(title, content, project_id="/path/worktree")
    Server->>Normalizer: Normalize project_id
    Normalizer-->>Server: "github.com/user/repo"
    Server->>Ollama: Embed(title + content)
    Ollama-->>Server: float32[768]

    Server->>PostgreSQL: Find similar (cosine >= 0.75)
    PostgreSQL-->>Server: Candidates

    alt similarity >= 0.92
        Server->>Server: Auto-merge (smart_merge strategy)
        Server->>PostgreSQL: UPDATE existing memory
        Server-->>Agent: {action: "merged", memory: {...}}
    else similarity 0.75-0.92
        Server->>PostgreSQL: INSERT suggestion
        Server->>PostgreSQL: INSERT new memory
        Server-->>Agent: {action: "stored", suggestions: [...]}
    else similarity < 0.75
        Server->>PostgreSQL: INSERT new memory
        Server-->>Agent: {action: "stored", memory: {...}}
    end
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

## Project ID Normalization

Agents send their CWD as `project_id`. The server resolves it to a stable canonical identifier using file-based detection (no external binaries required).

```mermaid
flowchart TD
    A["Raw project_id<br/>/Users/a/repos/app/.claude/worktrees/feat-x"] --> B{Already canonical?<br/>github.com/...}
    B -->|Yes| Z[Return as-is]
    B -->|No| C{Is filesystem path?<br/>starts with /}
    C -->|No| Z
    C -->|Yes| D[Walk up to find project root]
    D --> E{.contextify.yml<br/>with name: field?}
    E -->|Yes| F["Return name<br/>(highest priority)"]
    E -->|No| G{.git or .hg<br/>directory/file?}
    G -->|Yes| H[Parse config file<br/>.git/config or .hg/hgrc]
    H --> I{Remote URL found?}
    I -->|Yes| J["Canonicalize URL<br/>github.com/user/repo"]
    I -->|No| K{Worktree suffix?<br/>/.claude/worktrees/...}
    G -->|No| K
    K -->|Yes| L[Strip suffix, retry resolve]
    K -->|No| M[Return raw path]

    style F fill:#059669,color:#fff
    style J fill:#059669,color:#fff
    style L fill:#d97706,color:#fff
    style M fill:#6b7280,color:#fff
```

**Normalization is applied on both reads and writes:**
- **Write**: `Store()` normalizes `project_id` before inserting
- **Read**: `GetContext()`, `Search()`, `GetSuggestions()` normalize before querying

This ensures that worktrees, different clone paths, and renames all resolve to the same project identity.

**VCS remote detection** is file-based (no git/hg binary needed):
- **Git**: Reads `.git/config` and parses `[remote "origin"]` url
- **Git worktree**: `.git` is a FILE containing `gitdir: <path>` — traverses to main repo's config
- **Mercurial**: Reads `.hg/hgrc` and parses `[paths]` default

**URL canonicalization** normalizes all remote URL formats to `host/user/repo`:
- `https://github.com/user/repo.git` → `github.com/user/repo`
- `git@github.com:user/repo.git` → `github.com/user/repo`
- `ssh://git@github.com/user/repo` → `github.com/user/repo`

## Memory Consolidation

```mermaid
flowchart LR
    subgraph SmartStore["Smart Store (on write)"]
        S1[New memory] --> S2[Embed + find similar]
        S2 --> S3{Similarity?}
        S3 -->|>= 0.92| S4[Auto-merge]
        S3 -->|0.75-0.92| S5[Store + suggest]
        S3 -->|< 0.75| S6[Normal store]
    end

    subgraph DedupScanner["Background Scanner (periodic)"]
        D1[Scan all memories] --> D2[Pairwise similarity]
        D2 --> D3[Create suggestions]
    end

    subgraph Review["Review (Web UI / Agent)"]
        R1[Pending suggestions] --> R2{Decision}
        R2 -->|Accept| R3[Merge memories]
        R2 -->|Reject| R4[Dismiss]
    end

    S5 --> R1
    D3 --> R1

    style S4 fill:#059669,color:#fff
    style S5 fill:#d97706,color:#fff
    style S6 fill:#4f46e5,color:#fff
    style R3 fill:#059669,color:#fff
```

**Merge strategies:**
| Strategy | Behavior |
|----------|----------|
| `latest_wins` | Target content is replaced entirely by source |
| `append` | Source content is appended to target |
| `smart_merge` | Intelligent blend that preserves unique information from both (default) |

After merge, a `SUPERSEDES` relationship is created from the surviving memory to the absorbed one, and a consolidation log entry records the operation for audit.

## Project Structure

```
contextify/
├── cmd/server/main.go              # Entry point, wires all components
├── internal/
│   ├── api/
│   │   ├── router.go               # Chi router, CORS, SPA static files
│   │   ├── handlers.go             # REST handlers (memory, consolidation, admin)
│   │   └── middleware.go            # Request ID + logging
│   ├── config/
│   │   └── config.go               # YAML + env config loader
│   ├── db/
│   │   ├── postgres.go             # Connection pool + embedded migrations
│   │   └── migrations/
│   │       ├── 001_init.sql        # Schema: memories + relationships + indexes
│   │       ├── 002_consolidation.sql    # Consolidation log + suggestions tables
│   │       └── 003_normalize_project_ids.sql # Worktree suffix cleanup
│   ├── embedding/
│   │   └── ollama.go               # Ollama HTTP client, auto-pull model
│   ├── mcp/
│   │   ├── server.go               # MCP server setup (Streamable HTTP)
│   │   └── tools.go                # 13 MCP tool handlers
│   ├── memory/
│   │   ├── model.go                # Memory, Relationship, StoreResult structs
│   │   ├── projectid.go            # VCS-agnostic project ID normalizer
│   │   ├── projectid_test.go       # 38 unit tests for normalization
│   │   ├── repository.go           # PostgreSQL CRUD + hybrid search + consolidation
│   │   └── service.go              # Business logic, Smart Store, dedup, normalize
│   └── scheduler/
│       ├── cleanup.go              # Background TTL cleanup goroutine
│       ├── dedup.go                # Background duplicate scanner
│       └── project_normalizer.go   # Background project ID normalizer
├── scripts/hooks/
│   └── session-start.sh            # Client-side hook with VCS-agnostic normalization
├── web/                            # React + Vite + Tailwind CSS
│   ├── src/
│   │   ├── api.js                  # API client (fetch /api/v1/)
│   │   ├── pages/                  # Dashboard, MemoryBrowser, Search, Consolidation
│   │   └── components/             # Layout, MemoryCard
│   └── ...
├── Dockerfile                      # Dev image: Go server + Web UI (3-stage)
├── Dockerfile.allinone             # Prod image: PG + Ollama + model + server + UI
├── docker/
│   └── entrypoint.sh               # All-in-one process manager (PG → Ollama → server)
├── docker-compose.yml              # Development (separate services)
└── docker-compose.prod.yml         # Production (single all-in-one container)
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
| `project_id` | TEXT | Canonical project identifier (auto-normalized) |
| `agent_source` | TEXT | Source agent (claude-code, cursor, gemini, etc.) |
| `tags` | TEXT[] | Array of tags for filtering |
| `importance` | REAL | 0.0-1.0 score (>= 0.8 = auto-permanent) |
| `ttl_seconds` | INTEGER | Time-to-live (NULL = permanent) |
| `access_count` | INTEGER | Number of reads (>= 5 = auto-promoted) |
| `expires_at` | TIMESTAMPTZ | Computed expiry time |
| `version` | INTEGER | Increments on merge (default: 1) |
| `merged_from` | UUID[] | Source memory IDs absorbed during merge |
| `replaced_by` | UUID | Points to the surviving memory after merge |
| `created_at` | TIMESTAMPTZ | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | Last update (auto-trigger) |

### memory_relationships table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `from_memory_id` | UUID | Source memory (FK, CASCADE) |
| `to_memory_id` | UUID | Target memory (FK, CASCADE) |
| `relationship` | TEXT | Type: SOLVES, CAUSES, REQUIRES, RELATED_TO, SUPERSEDES, etc. |
| `strength` | REAL | 0.0-1.0 relationship strength |
| `context` | TEXT | Optional description |

### consolidation_log table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `target_id` | UUID | Surviving memory |
| `source_ids` | UUID[] | Absorbed memory IDs |
| `merge_strategy` | TEXT | latest_wins, append, or smart_merge |
| `similarity_score` | REAL | Cosine similarity that triggered the merge |
| `content_before` | TEXT | Target content before merge |
| `content_after` | TEXT | Target content after merge |
| `performed_by` | TEXT | Agent or system that triggered the merge |
| `created_at` | TIMESTAMPTZ | When the merge happened |

### consolidation_suggestions table

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `memory_a_id` | UUID | First memory (FK, CASCADE) |
| `memory_b_id` | UUID | Second memory (FK, CASCADE) |
| `similarity` | REAL | Cosine similarity score |
| `status` | TEXT | pending, accepted, rejected |
| `project_id` | TEXT | Project scope |
| `created_at` | TIMESTAMPTZ | When the suggestion was created |
| `resolved_at` | TIMESTAMPTZ | When it was accepted/rejected |

### steward_jobs table (planned runtime queue)

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `job_type` | TEXT | Executor type (auto_merge, derive, recheck, policy_tune, etc.) |
| `project_id` | TEXT | Optional project scope |
| `source_memory_ids` | UUID[] | Related source memory IDs |
| `trigger_reason` | TEXT | Why the job was created |
| `payload` | JSONB | Executor input payload |
| `status` | TEXT | queued, running, succeeded, failed, dead_letter, cancelled |
| `priority` | INTEGER | Higher first |
| `attempt_count` | INTEGER | Current attempt count |
| `max_attempts` | INTEGER | Retry cap |
| `run_after` | TIMESTAMPTZ | Earliest eligible execution time |
| `locked_by` | TEXT | Worker/instance identifier |
| `locked_at` | TIMESTAMPTZ | Claim timestamp |
| `lease_expires_at` | TIMESTAMPTZ | Lease expiry for recovery |
| `last_error` | TEXT | Last terminal/retry error message |
| `idempotency_key` | TEXT | De-duplication key (unique, partial) |
| `created_at` | TIMESTAMPTZ | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | Updated by trigger |

### steward_runs table (per-attempt run log)

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `job_id` | UUID | Associated steward job |
| `provider` | TEXT | Model/runtime provider |
| `model` | TEXT | Model name |
| `input_snapshot` | JSONB | Redacted input snapshot |
| `output_snapshot` | JSONB | Structured output snapshot |
| `input_hash` | TEXT | Snapshot hash for diffing/caching |
| `prompt_tokens` | INTEGER | Prompt token count |
| `completion_tokens` | INTEGER | Completion token count |
| `total_tokens` | INTEGER | Total tokens |
| `latency_ms` | INTEGER | Run latency |
| `status` | TEXT | running, succeeded, failed, skipped |
| `error_class` | TEXT | Error taxonomy |
| `error_message` | TEXT | Redacted error summary |
| `created_at` | TIMESTAMPTZ | Run start time |
| `completed_at` | TIMESTAMPTZ | Run completion time |

### steward_events table (append-only timeline)

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `job_id` | UUID | Related job |
| `run_id` | UUID | Related run (nullable) |
| `event_type` | TEXT | Event taxonomy key |
| `data` | JSONB | Structured event payload |
| `schema_version` | INTEGER | Payload schema version |
| `created_at` | TIMESTAMPTZ | Event time |

### memory_derivations table (lineage + scoring)

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `source_memory_ids` | UUID[] | Source memory lineage |
| `derived_memory_id` | UUID | Created derived memory (nullable) |
| `derivation_type` | TEXT | Derivation category |
| `confidence` | REAL | Confidence score |
| `novelty` | REAL | Novelty score |
| `status` | TEXT | candidate, accepted, skipped, failed |
| `model` | TEXT | Model used for derivation |
| `payload` | JSONB | Candidate payload / metadata |
| `created_at` | TIMESTAMPTZ | Created time |
| `updated_at` | TIMESTAMPTZ | Updated time |

### steward_policy_history table (safe self-learn/audit)

| Column | Type | Description |
|--------|------|-------------|
| `id` | UUID | Primary key |
| `policy_key` | TEXT | Tunable config key |
| `prior_value` | DOUBLE PRECISION | Previous value |
| `new_value` | DOUBLE PRECISION | New value |
| `reason` | TEXT | Human/system rationale |
| `sample_size` | INTEGER | Evidence sample size |
| `evidence` | JSONB | Metrics snapshot supporting change |
| `changed_by` | TEXT | Actor (steward/operator) |
| `rollback_of_id` | UUID | Optional reference to reverted change |
| `created_at` | TIMESTAMPTZ | Change time |

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
| `idx_memories_replaced_by` | B-tree (partial) | Find replaced memories |
| `idx_consolidation_log_target` | B-tree | Audit log by target memory |
| `idx_consolidation_log_created` | B-tree (DESC) | Recent consolidations first |
| `idx_suggestions_status` | B-tree (partial) | Pending suggestions only |
| `idx_suggestions_project` | B-tree | Suggestions by project |
| `idx_suggestions_similarity` | B-tree (DESC) | Highest similarity first |
| `idx_steward_jobs_status_run_after_priority` | B-tree | Queue claiming by status/time/priority |
| `idx_steward_jobs_project_status` | B-tree | Project-scoped job filtering |
| `uq_steward_jobs_idempotency_key` | Partial unique | Exactly-once enqueue by idempotency key |
| `idx_steward_runs_job_created` | B-tree | Runs by job (latest first) |
| `idx_steward_events_job_created` | B-tree | Event timeline per job |
| `idx_memory_derivations_derived_memory` | B-tree | Derivation lineage lookup |
| `idx_steward_policy_history_key_created` | B-tree | Policy history by key |

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
| `memory.normalize_project_id` | true | Enable VCS-agnostic project ID normalization |
| `memory.similarity_threshold` | 0.75 | Minimum similarity for dedup suggestions |
| `memory.auto_merge_threshold` | 0.92 | Minimum similarity for automatic merge |
| `memory.dedup_scan_interval` | 1h | Background dedup scanner frequency |
| `search.vector_weight` | 0.7 | Vector similarity weight in hybrid search |
| `search.keyword_weight` | 0.3 | Keyword matching weight in hybrid search |
| `search.default_limit` | 20 | Default search result limit |
| `search.max_limit` | 100 | Maximum search result limit |

## Background Workers

Three background goroutines run alongside the HTTP server:

| Worker | Interval | Purpose |
|--------|----------|---------|
| **Cleanup Scheduler** | 5 min | Removes expired short-term memories (TTL) |
| **Dedup Scanner** | 1 hour | Scans memories for duplicates, creates consolidation suggestions |
| **Project Normalizer** | 1 hour | Re-normalizes all project_ids (cleans up legacy paths) |

All workers follow the same pattern: ticker-based loop with graceful shutdown via channel.

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

## Steward (Planned 24/7 Memory Lifecycle)

The **Memory Steward** is a planned in-process subsystem for continuous, auditable lifecycle management (dedup auto-merge, derivation, recheck, policy tuning). It is defined in [docs/steward/ADR-001-memory-steward.md](docs/steward/ADR-001-memory-steward.md) and is designed to be **disabled by default** so current behavior remains unchanged.

Operational rollout, SLOs, reliability controls, log-security posture, and verification guidance are documented in:

- [docs/steward/reliability-hardening.md](docs/steward/reliability-hardening.md)
- [docs/steward/log-security.md](docs/steward/log-security.md)
- [docs/steward/verification-matrix.md](docs/steward/verification-matrix.md)
- [docs/steward/rollout-runbook.md](docs/steward/rollout-runbook.md)

### Steward Module Decomposition

```mermaid
flowchart LR
    subgraph Runtime["Contextify Server Runtime"]
        ORCH["Steward Orchestrator"]
        Q["Queue / Claimer"]
        EX["Executors\nauto_merge | derive | recheck | policy_tune"]
        AUD["Audit Logger"]
        MET["Metrics Emitter"]
    end

    subgraph Data["PostgreSQL"]
        JQ[("Steward Jobs")]
        JR[("Steward Runs")]
        JE[("Steward Events")]
        MEM[("Memories / Consolidation Tables")]
    end

    subgraph Model["Ollama (optional by executor)"]
        LLM["Mini model / planner"]
    end

    ORCH --> Q
    Q --> JQ
    Q --> JR
    Q --> EX
    EX --> LLM
    EX --> MEM
    EX --> AUD
    AUD --> JE
    AUD --> JR
    ORCH --> MET
    EX --> MET

    style ORCH fill:#b91c1c,color:#fff
    style Q fill:#1d4ed8,color:#fff
    style EX fill:#059669,color:#fff
    style AUD fill:#7c3aed,color:#fff
    style MET fill:#d97706,color:#fff
```

### Steward Job State Machine

```mermaid
stateDiagram-v2
    [*] --> queued
    queued --> running: claim (FOR UPDATE SKIP LOCKED)
    queued --> cancelled: operator cancel

    running --> succeeded: executor + writes complete
    running --> failed: retryable/non-retryable failure
    running --> cancelled: cooperative cancel

    failed --> queued: retry with backoff (idempotent)
    failed --> dead_letter: max retries or invariant violation

    succeeded --> [*]
    dead_letter --> [*]
    cancelled --> [*]
```

### Steward Runtime and Concurrency

- Single active leader per deployment via PostgreSQL advisory lock.
- Passive replicas may run the server but must not claim steward jobs without leadership.
- Job claims use short DB transactions plus `FOR UPDATE SKIP LOCKED`.
- Running jobs use leases + heartbeats; expired leases are recoverable with idempotent requeue logic.
- All autonomous writes must be auditable and mapped to a steward event.

### Integration with Existing Components

- **Memory Service**: executors should call `internal/memory.Service` paths where possible to preserve normalization, cache invalidation, and merge semantics.
- **Consolidation**: `auto_merge` executor reuses merge logic and writes additional steward audit events around the decision path.
- **Schedulers**: existing goroutines continue to operate initially; future STW issues may route outputs into the steward queue.
- **Telemetry**: steward emits a separate event stream; existing recall/store telemetry remains unchanged.

### Steward Event and Observability Contract

Required event types (minimum):
- `job_created`
- `job_claimed`
- `model_invoked`
- `decision_made`
- `write_applied`
- `write_skipped`
- `job_completed`
- `job_failed`

Required UI-facing telemetry fields (run/event level):
- redacted input snapshot
- output JSON (structured decision/result)
- model name
- prompt/completion/total token counts
- latency ms
- error class / redacted message
- side effects (applied or skipped writes)

### Steward Sequence Diagrams

#### Claim + Execute (Auto Merge)

```mermaid
sequenceDiagram
    participant Orch as Steward Orchestrator
    participant DB as PostgreSQL
    participant Ex as auto_merge Executor
    participant Model as Ollama
    participant Svc as Memory Service

    Orch->>DB: advisory lock acquire/verify
    Orch->>DB: claim queued job (FOR UPDATE SKIP LOCKED)
    DB-->>Orch: job marked running + lease
    Orch->>Ex: execute(job)
    Ex->>DB: append event(job_claimed)
    Ex->>Model: optional merge plan / validation call
    Model-->>Ex: structured output
    Ex->>DB: append event(model_invoked)
    Ex->>Ex: deterministic decision validation
    Ex->>DB: append event(decision_made)

    alt write allowed and valid
        Ex->>Svc: ConsolidateMemories(...)
        Svc->>DB: merge writes + consolidation log
        Ex->>DB: append event(write_applied)
        Ex->>DB: mark run/job succeeded + job_completed
    else dry-run or validation failed
        Ex->>DB: append event(write_skipped)
        Ex->>DB: mark run/job succeeded + job_completed
    end
```

#### Failure Recovery (Lease Expiry / Retry)

```mermaid
sequenceDiagram
    participant Worker as Steward Worker
    participant DB as PostgreSQL
    participant Sweep as Recovery Sweeper

    Worker->>DB: claim job -> running (lease_expires_at=T)
    Worker->>DB: heartbeat extend lease
    Note over Worker,DB: process crash / lock loss before completion

    Sweep->>DB: scan running jobs with expired lease
    DB-->>Sweep: expired job rows
    Sweep->>DB: append event(job_failed or recovery_detected)

    alt attempts < max_retries and retryable
        Sweep->>DB: set state=queued, schedule backoff
    else non-retryable or max retries hit
        Sweep->>DB: set state=dead_letter
    end
```

### Rollout Phases

1. **Dry-run only**: decision traces recorded, no steward writes applied.
2. **Selective auto-apply**: whitelisted job types with deterministic validation.
3. **Full auto with guardrails**: UI/API controls, auditability, and policy guardrails required.

### Explicit Non-Goals (STW01)

- Replacing current `Store()` dedup path immediately.
- Adding external queue/workflow infrastructure.
- Shipping the steward UI/API in this issue.
- Turning on autonomous writes by default.

### Steward Risk Register (Architecture-Level)

- **Operational**: leader lock flapping during PostgreSQL instability can pause or duplicate work. Mitigation: leases, heartbeat grace windows, lock-loss handling, recovery events.
- **Operational**: model latency spikes can saturate worker pools. Mitigation: timeouts, backpressure, bounded concurrency, circuit breakers (planned in STW13).
- **Integration**: executors may bypass memory service invariants if they write directly. Mitigation: service-first integration policy and explicit exceptions.
- **Integration**: telemetry schema drift can break later UI/API contracts. Mitigation: version event payloads and anchor fields in ADR-001.
