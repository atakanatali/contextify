# Contextify - Implementasyon Planı

## Özet
Tüm local AI agent'lar (Claude Code, Cursor, Gemini, Antigravity, vb.) için merkezi bir memory sistemi. Docker üzerinde çalışan Go MCP Server + REST API, PostgreSQL + pgvector DB, Ollama embedding, ve Web UI.

---

## Faz 1: Proje İskeleti ve Docker Altyapısı

### 1.1 Go Proje Yapısı
```
contextify/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Env/YAML config loader
│   ├── db/
│   │   ├── postgres.go          # PostgreSQL connection + migrations
│   │   └── migrations/
│   │       └── 001_init.sql     # Initial schema (pgvector, tables, indexes)
│   ├── embedding/
│   │   └── ollama.go            # Ollama embedding client
│   ├── memory/
│   │   ├── model.go             # Memory struct, types, enums
│   │   ├── repository.go        # DB CRUD operations
│   │   ├── service.go           # Business logic (TTL, promotion, search)
│   │   └── service_test.go
│   ├── mcp/
│   │   ├── server.go            # MCP server setup (tools registration)
│   │   ├── tools.go             # MCP tool handlers
│   │   └── transport.go         # Streamable HTTP + stdio transport
│   ├── api/
│   │   ├── router.go            # REST API router (chi/echo)
│   │   ├── handlers.go          # REST endpoint handlers
│   │   └── middleware.go        # Logging, CORS, request ID
│   └── scheduler/
│       └── cleanup.go           # TTL expiry background job
├── web/                          # Web UI (ayrı SPA)
│   ├── package.json
│   ├── src/
│   └── ...
├── docker-compose.yml
├── Dockerfile                    # Multi-stage Go build
├── Dockerfile.web                # Web UI build
├── go.mod
├── go.sum
├── config.yaml                   # Default config
└── README.md
```

### 1.2 Docker Compose
```yaml
services:
  postgres:
    image: pgvector/pgvector:pg16
    ports: ["5432:5432"]
    volumes: [pgdata:/var/lib/postgresql/data]
    environment:
      POSTGRES_DB: contextify
      POSTGRES_USER: contextify
      POSTGRES_PASSWORD: contextify_local
    healthcheck: pg_isready

  ollama:
    image: ollama/ollama:latest
    ports: ["11434:11434"]
    volumes: [ollama_models:/root/.ollama]
    deploy:
      resources:
        reservations:
          devices:
            - capabilities: [gpu]  # GPU varsa kullan

  server:
    build: .
    ports: ["8420:8420"]
    depends_on: [postgres, ollama]
    environment:
      DATABASE_URL: postgres://contextify:contextify_local@postgres:5432/contextify
      OLLAMA_URL: http://ollama:11434
      EMBEDDING_MODEL: nomic-embed-text

  web:
    build:
      dockerfile: Dockerfile.web
    ports: ["3000:3000"]
    depends_on: [server]
```

### 1.3 Veritabanı Schema (001_init.sql)
```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE memory_type AS ENUM (
  'solution', 'problem', 'code_pattern', 'fix',
  'error', 'workflow', 'decision', 'general'
);

CREATE TYPE memory_scope AS ENUM ('global', 'project');

CREATE TABLE memories (
  id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  content       TEXT NOT NULL,
  summary       TEXT,
  embedding     vector(768),
  type          memory_type NOT NULL DEFAULT 'general',
  scope         memory_scope NOT NULL DEFAULT 'project',
  project_id    TEXT,
  agent_source  TEXT,                    -- claude-code, cursor, gemini, etc.
  title         TEXT NOT NULL,
  tags          TEXT[] DEFAULT '{}',
  importance    REAL NOT NULL DEFAULT 0.5 CHECK (importance >= 0 AND importance <= 1),
  ttl_seconds   INTEGER,                 -- NULL = permanent (long-term)
  access_count  INTEGER NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at    TIMESTAMPTZ              -- computed from created_at + ttl
);

-- Relationships table
CREATE TABLE memory_relationships (
  id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  from_memory_id  UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
  to_memory_id    UUID NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
  relationship    TEXT NOT NULL,          -- SOLVES, CAUSES, RELATED_TO, etc.
  strength        REAL DEFAULT 0.5,
  context         TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(from_memory_id, to_memory_id, relationship)
);

-- HNSW index for vector similarity search (better for dynamic data)
CREATE INDEX idx_memories_embedding ON memories
  USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);

-- B-tree indexes for filtering
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_scope ON memories(scope);
CREATE INDEX idx_memories_project ON memories(project_id);
CREATE INDEX idx_memories_importance ON memories(importance);
CREATE INDEX idx_memories_expires ON memories(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_memories_tags ON memories USING gin(tags);
CREATE INDEX idx_memories_created ON memories(created_at DESC);
```

**Neden HNSW (IVFFlat değil):**
- Dinamik data: Memory sürekli ekleniyor/siliniyor, HNSW incremental index destekliyor
- IVFFlat centroid'leri statik, data değişince rebuild gerekiyor
- HNSW recall'u daha yüksek (~0.998 vs IVFFlat ~0.95)
- Build süresi uzun ama memory sistemi için kabul edilebilir

---

## Faz 2: Go Server Core

### 2.1 Memory Model (internal/memory/model.go)
- `Memory` struct: DB tablosuyla 1:1 map
- `MemoryRelationship` struct
- `StoreRequest`, `SearchRequest`, `UpdateRequest` DTO'lar
- `SearchResult` struct (memory + similarity score)

### 2.2 Embedding Client (internal/embedding/ollama.go)
- Ollama `/api/embed` endpoint'ine HTTP call
- Model: `nomic-embed-text` (768 dim)
- Batch embedding desteği (birden fazla text'i tek request'te)
- Connection pooling + retry logic
- Startup'ta model pull kontrolü (`ollama pull nomic-embed-text`)

### 2.3 Repository Layer (internal/memory/repository.go)
- `Store(ctx, memory)` → INSERT + embedding oluştur
- `Get(ctx, id)` → tek memory getir
- `Update(ctx, id, updates)` → UPDATE + re-embed if content changed
- `Delete(ctx, id)` → CASCADE delete (relationships dahil)
- `Search(ctx, query, filters)` → Hybrid search:
  1. Vector similarity: `embedding <=> query_embedding` (cosine distance)
  2. Keyword match: `to_tsvector(content) @@ plainto_tsquery(query)`
  3. Tag filter: `tags @> ARRAY[...]`
  4. Combined score: `0.7 * vector_score + 0.3 * keyword_score`
- `IncrementAccess(ctx, id)` → access_count++ ve expires_at uzat
- `FindExpired(ctx)` → TTL'i dolmuş memory'ler
- `PromoteToLongTerm(ctx, id)` → ttl_seconds = NULL, expires_at = NULL

### 2.4 Service Layer (internal/memory/service.go)
- Store: embedding oluştur → DB'ye kaydet → TTL hesapla
- Search: query embed → hybrid search → access_count++ → sonuçları döndür
- **Auto-promote logic:**
  - `importance >= 0.8` → direkt long-term
  - `access_count >= 5` → auto-promote to long-term
  - Her erişimde `expires_at += original_ttl * 0.5` (decay extend)
- Recall: semantic search + tag filter + type filter

### 2.5 Scheduler (internal/scheduler/cleanup.go)
- Her 5 dakikada bir çalışan goroutine
- `DELETE FROM memories WHERE expires_at IS NOT NULL AND expires_at < NOW()`
- Log: kaç memory temizlendi

---

## Faz 3: MCP Server + REST API

### 3.1 MCP Server (internal/mcp/)
**SDK:** `github.com/modelcontextprotocol/go-sdk` (resmi SDK)

**Transport:**
- **Streamable HTTP** (yeni standart, SSE deprecated): `/mcp` endpoint'i
- **Stdio** da desteklenecek (Claude Code stdio transport kullanabilir)

**MCP Tools (memorygraph-uyumlu API + ek özellikler):**

| Tool | Açıklama |
|------|----------|
| `store_memory` | Yeni memory kaydet (auto-embed) |
| `recall_memories` | Semantic search + filters |
| `search_memories` | Advanced search (tags, type, importance filter) |
| `get_memory` | ID ile memory getir |
| `update_memory` | Memory güncelle |
| `delete_memory` | Memory sil |
| `create_relationship` | İki memory arasında ilişki kur |
| `get_related_memories` | İlişkili memory'leri bul |
| `get_context` | Proje bazlı tüm önemli memory'leri getir (session başlangıcı için) |
| `promote_memory` | Manual olarak long-term'e promote et |

### 3.2 REST API (internal/api/)
MCP desteklemeyen agent'lar için (Gemini, Antigravity, vb.):

```
POST   /api/v1/memories           → Store
GET    /api/v1/memories/:id       → Get
PUT    /api/v1/memories/:id       → Update
DELETE /api/v1/memories/:id       → Delete
POST   /api/v1/memories/search    → Search (body: query, filters)
POST   /api/v1/memories/recall    → Semantic recall
POST   /api/v1/relationships      → Create relationship
GET    /api/v1/memories/:id/related → Get related
GET    /api/v1/stats              → Memory stats (count by type, scope, etc.)
POST   /api/v1/context/:project   → Get project context
```

### 3.3 Dual Server Setup
```go
// main.go
func main() {
    // Config, DB, Ollama bağlantıları
    // ...

    // MCP Server (Streamable HTTP on /mcp)
    mcpHandler := mcp.NewStreamableHTTPHandler(...)

    // REST API (/api/v1/...)
    apiRouter := api.NewRouter(memoryService)

    // Combined HTTP server
    mux := http.NewServeMux()
    mux.Handle("/mcp", mcpHandler)
    mux.Handle("/api/", apiRouter)

    // Tek port: 8420
    http.ListenAndServe(":8420", mux)
}
```

---

## Faz 4: Web UI

### 4.1 Teknoloji
- **Svelte** (veya React) + Tailwind CSS
- REST API'yi kullanır (`http://localhost:8420/api/v1/`)
- Single-page app, Docker'da nginx ile serve edilir

### 4.2 Sayfalar
1. **Dashboard**: Memory stats, son eklenenler, scope/type breakdown chart
2. **Memory Browser**: Liste + filtreleme (type, scope, project, tags, importance)
3. **Memory Detail**: İçerik, ilişkiler, erişim geçmişi
4. **Search**: Semantic search arayüzü
5. **Graph View**: Memory ilişkilerini görselleştir (d3.js force graph)
6. **Settings**: Cleanup politikası, embedding model seçimi

---

## Faz 5: Agent Entegrasyonu

### 5.1 Claude Code
```json
// ~/.claude/settings.json → mcpServers
{
  "contextify": {
    "type": "streamableHttp",
    "url": "http://localhost:8420/mcp"
  }
}
```
Alternatif: stdio transport ile doğrudan binary çalıştırma.

### 5.2 Cursor
```json
// .cursor/mcp.json
{
  "mcpServers": {
    "contextify": {
      "url": "http://localhost:8420/mcp",
      "transport": "streamable-http"
    }
  }
}
```

### 5.3 Gemini / Antigravity / Diğerleri
REST API kullanımı. Sistem prompt'a eklenir:
```
Memory API: http://localhost:8420/api/v1/
- Her session başında GET /api/v1/context/{project} çağır
- Önemli bilgileri POST /api/v1/memories ile kaydet
```

### 5.4 Memorygraph Migration
Mevcut memorygraph tablosu boş (Table Memory does not exist hatası alındı), dolayısıyla veri migration'a gerek yok. Ancak migration tool'u yine de yazılacak (gelecekte lazım olabilir):
- DuckDB dosyasını okuyup PostgreSQL'e aktaran CLI komutu
- `contextify migrate --from duckdb --file /path/to/memory.duckdb`

---

## Implementasyon Sırası

| Sıra | Faz | Tahmini Süre | Detay |
|------|-----|-------------|-------|
| 1 | Docker Compose + DB Schema | İlk | PostgreSQL, Ollama, migration |
| 2 | Go Project Skeleton | İlk | go mod, config, DB connection |
| 3 | Embedding Client | İlk | Ollama integration |
| 4 | Memory Repository + Service | Core | CRUD, hybrid search, TTL logic |
| 5 | MCP Server | Core | Tools, Streamable HTTP transport |
| 6 | REST API | Core | Chi router, handlers |
| 7 | Scheduler | Core | TTL cleanup goroutine |
| 8 | Web UI | Son | Svelte/React dashboard |
| 9 | Agent Config + Docs | Son | Entegrasyon kılavuzları |

---

## Teknik Kararlar Özeti

| Karar | Seçim | Gerekçe |
|-------|-------|---------|
| Dil | Go | Düşük resource, hızlı binary, resmi MCP SDK |
| DB | PostgreSQL 16 + pgvector | Production-grade, native vector search, TTL desteği |
| Vector Index | HNSW | Dinamik data için ideal, yüksek recall |
| Embedding | Ollama + nomic-embed-text (768 dim) | Local, ücretsiz, gizlilik dostu |
| MCP Transport | Streamable HTTP | Yeni standart (SSE deprecated March 2025) |
| MCP SDK | modelcontextprotocol/go-sdk (resmi) | Official, Google co-maintained |
| REST Framework | chi veya stdlib | Hafif, Go idiomatik |
| Web UI | Svelte + Tailwind | Hafif, hızlı, tek dosya build |
| Auth | Yok (localhost only) | Local development, network dışına kapalı |
| Port | 8420 | MCP + REST aynı port |
