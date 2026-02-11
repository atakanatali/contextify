# Contextify

<p align="center">
  <img width="456" height="456" alt="Contextify Logo" src="https://github.com/user-attachments/assets/bb64c170-d4e0-4f09-9f60-467bd9f043f8" />
</p>

<p align="center">
  <a href="https://github.com/atakanatali/contextify/pkgs/container/contextify"><img src="https://img.shields.io/badge/ghcr.io-contextify-blue?logo=docker" alt="Docker Image"></a>
  <a href="https://github.com/atakanatali/contextify/releases"><img src="https://img.shields.io/github/v/release/atakanatali/contextify" alt="GitHub Release"></a>
  <a href="https://github.com/atakanatali/contextify/blob/main/LICENSE"><img src="https://img.shields.io/github/license/atakanatali/contextify" alt="License"></a>
</p>

Unified memory system for AI agents. Provides shared short-term and long-term memory across Claude Code, Cursor, Gemini, Antigravity, and any other AI tool.

## Architecture

```
┌──────────────────────────────────────────────┐
│                Docker Compose                │
│                                              │
│  PostgreSQL+pgvector  Ollama       Web UI    │
│     :5432              :11434      :3000     │
│         └───────┬───────┘            │       │
│           Contextify Server ◄────────┘       │
│           (Go) :8420                         │
│           MCP + REST API                     │
└──────────────┬───────────────────────────────┘
               │
     AI Agents (MCP or REST)
```

## Quick Start

Download [`docker-compose.prod.yml`](https://github.com/atakanatali/contextify/releases/latest/download/docker-compose.prod.yml) and run:

```bash
curl -fsSL https://github.com/atakanatali/contextify/releases/latest/download/docker-compose.prod.yml -o docker-compose.yml
docker compose up
```

Or for development (build from source):

```bash
git clone https://github.com/atakanatali/contextify.git
cd contextify
docker compose up
```

Services:
- **API**: http://localhost:8420
- **MCP**: http://localhost:8420/mcp
- **Web UI**: http://localhost:3000
- **Health**: http://localhost:8420/health

## Agent Setup

### Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "contextify": {
      "type": "streamableHttp",
      "url": "http://localhost:8420/mcp"
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "contextify": {
      "url": "http://localhost:8420/mcp",
      "transport": "streamable-http"
    }
  }
}
```

### Gemini / Antigravity / Other

Use the REST API. Add to your system prompt:

```
Memory API: http://localhost:8420/api/v1/
- Start each session: POST /api/v1/context/{project}
- Store insights: POST /api/v1/memories
- Search: POST /api/v1/memories/search
- Recall (semantic): POST /api/v1/memories/recall
```

## MCP Tools

| Tool | Description |
|------|-------------|
| `store_memory` | Store a new memory (auto-embeds) |
| `recall_memories` | Semantic search with natural language |
| `search_memories` | Advanced search with filters |
| `get_memory` | Get memory by ID |
| `update_memory` | Update existing memory |
| `delete_memory` | Delete memory and relationships |
| `create_relationship` | Link two memories |
| `get_related_memories` | Find connected memories |
| `get_context` | Load all project memories (session start) |
| `promote_memory` | Promote short-term to permanent |

## REST API

```
POST   /api/v1/memories            Store memory
GET    /api/v1/memories/:id         Get memory
PUT    /api/v1/memories/:id         Update memory
DELETE /api/v1/memories/:id         Delete memory
POST   /api/v1/memories/search      Search
POST   /api/v1/memories/recall      Semantic recall
POST   /api/v1/memories/:id/promote Promote to long-term
GET    /api/v1/memories/:id/related Get related memories
POST   /api/v1/relationships        Create relationship
GET    /api/v1/stats                Stats
POST   /api/v1/context/:project     Get project context
```

## Memory Model

Each memory has:
- **type**: solution, problem, code_pattern, fix, error, workflow, decision, general
- **scope**: global (all projects) or project (scoped)
- **importance**: 0.0-1.0 (>= 0.8 = auto-permanent)
- **TTL**: automatic expiry with access-based extension
- **tags**: array for filtering
- **embedding**: auto-generated via Ollama (nomic-embed-text, 768d)

## TTL + Importance System

- New memories get default TTL of 24h
- Each access extends TTL by 50%
- Importance >= 0.8 -> automatic permanent storage
- Access count >= 5 -> auto-promoted to permanent
- Background job cleans expired memories every 5 minutes

## Tech Stack

- **Server**: Go + official MCP Go SDK
- **Database**: PostgreSQL 16 + pgvector (HNSW index)
- **Embeddings**: Ollama + nomic-embed-text (local, free)
- **Web UI**: React + Vite + Tailwind CSS
- **Transport**: Streamable HTTP (MCP) + REST API

## License

MIT
