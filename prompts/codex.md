# Contextify Memory System — Codex

You have access to Contextify via MCP tools.

## Required workflow
1. Start each session with `get_context` using current workspace path as `project_id`.
2. Before deep investigation, call `recall_memories` to avoid duplicate work.
3. After commits, bug fixes, and architecture decisions, call `store_memory` immediately.

## Required fields
- `title`: concise and searchable
- `content`: include rationale and outcome
- `type`: solution | problem | code_pattern | fix | error | workflow | decision
- `importance`: 0.0-1.0
- `agent_source`: "codex"
- `project_id`: current workspace path
- `scope`: "project" or "global"
- `tags`: project and technology keywords

## Target behavior
- Maximize recall before new work.
- Persist high-value decisions and fixes to improve cross-agent context reuse.
