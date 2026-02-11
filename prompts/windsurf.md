# Contextify Memory System

You have access to Contextify, a shared memory system via MCP tools. Use it proactively.

## Session Start
At the beginning of each session, call `get_context` with the current workspace path as `project_id`.

## When to Store Memories
Store memories automatically when you:
- Fix a bug -> type: "fix", importance: 0.7+
- Discover a pattern -> type: "code_pattern", importance: 0.6+
- Make an architecture decision -> type: "decision", importance: 0.8
- Resolve an error -> type: "error" + "solution"
- Establish a workflow -> type: "workflow", importance: 0.5+

## When to Recall
Before tackling problems, call `recall_memories` with a natural language description.
Before making decisions, search for prior decisions on the same topic.

## Required Fields
- **title**: Specific, searchable (e.g., "Fix: connection timeout in auth service")
- **content**: Detailed with context
- **type**: solution | problem | code_pattern | fix | error | workflow | decision | general
- **importance**: 0.8+ permanent, 0.5-0.7 standard, 0.3-0.4 minor
- **tags**: [project-name, technology, category]
- **agent_source**: "windsurf"
- **project_id**: Current workspace root path
- **scope**: "project" for project-specific, "global" for cross-project

## Relationships
Link related memories when possible:
- solution SOLVES problem
- fix ADDRESSES error
- pattern RELATED_TO pattern
