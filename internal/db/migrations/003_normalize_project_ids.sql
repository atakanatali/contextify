-- Migration 003: Normalize project_ids by stripping worktree suffixes.
-- Full canonical normalization (path → github.com/user/repo) is done at runtime
-- by the ProjectNormalizer in the service layer and the background worker.

-- Strip .claude/worktrees/<name> suffixes from memories
UPDATE memories
SET project_id = regexp_replace(project_id, '/\.claude/worktrees/[^/]+$', '')
WHERE project_id LIKE '%/.claude/worktrees/%';

-- Strip .claude/worktrees/<name> suffixes from consolidation_suggestions
UPDATE consolidation_suggestions
SET project_id = regexp_replace(project_id, '/\.claude/worktrees/[^/]+$', '')
WHERE project_id LIKE '%/.claude/worktrees/%';
