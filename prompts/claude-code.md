## Contextify Memory System

You have access to Contextify, a shared memory system across all your AI tools. Use it proactively.

### Session Start
At the START of every session, IMMEDIATELY call `get_context` with `project_id` set to the current working directory.
This loads all relevant memories (solutions, patterns, decisions) for the current project.

### When to Store Memories
Call `store_memory` automatically when ANY of these occur:
- **Bug fix completed** -> type: "fix", importance: 0.7+
- **Architecture decision** -> type: "decision", importance: 0.8
- **Error resolved** -> type: "error" + follow-up type: "solution"
- **Pattern discovered** -> type: "code_pattern", importance: 0.6+
- **Workflow established** -> type: "workflow", importance: 0.5+

### When to Recall Memories
Call `recall_memories` BEFORE starting work when you encounter:
- An error message or stack trace
- A design question about the project
- A technology or library question
- Anything that might have been solved before

### Required Fields
- **title**: Specific and searchable (e.g., "Fix: PostgreSQL connection timeout in Docker")
- **content**: Detailed description including context and reasoning
- **type**: solution | problem | code_pattern | fix | error | workflow | decision | general
- **importance**: 0.8+ critical/permanent, 0.5-0.7 standard, 0.3-0.4 minor/temporary
- **tags**: Always include project name, technology, and category
- **agent_source**: "claude-code"
- **project_id**: Current working directory path
- **scope**: "project" for project-specific, "global" for cross-project knowledge

### Relationship Linking
When storing a fix or solution, link it to the original problem:
```
create_relationship(from_memory_id: solution_id, to_memory_id: problem_id, relationship: "SOLVES")
```

### Importance Guide
| Importance | When to Use | TTL |
|-----------|-------------|-----|
| 0.9-1.0 | Critical architecture decisions, security fixes | Permanent |
| 0.8 | Important patterns, recurring fixes | Permanent (auto) |
| 0.5-0.7 | Standard solutions, workflows | 24h, extended on access |
| 0.3-0.4 | Temporary notes, minor observations | 24h, expires if unused |

Do NOT wait to be asked. Memory operations are automatic and proactive.
