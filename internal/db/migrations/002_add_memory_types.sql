-- no-transaction
-- Add new memory types for cross-agent compatibility
-- Agents like memorygraph, Cursor, Windsurf may send types not in the original enum
ALTER TYPE memory_type ADD VALUE IF NOT EXISTS 'task';
ALTER TYPE memory_type ADD VALUE IF NOT EXISTS 'technology';
ALTER TYPE memory_type ADD VALUE IF NOT EXISTS 'command';
ALTER TYPE memory_type ADD VALUE IF NOT EXISTS 'file_context';
ALTER TYPE memory_type ADD VALUE IF NOT EXISTS 'conversation';
ALTER TYPE memory_type ADD VALUE IF NOT EXISTS 'project';
