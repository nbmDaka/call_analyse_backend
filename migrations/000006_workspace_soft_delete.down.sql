DROP INDEX IF EXISTS idx_workspaces_deleted_at;
ALTER TABLE workspaces DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_status_check;
ALTER TABLE workspaces ADD CONSTRAINT workspaces_status_check CHECK (status IN ('active', 'suspended'));
