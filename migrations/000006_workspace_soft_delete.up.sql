-- Migration 000006: Workspace logical deletion (soft delete)

ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE workspaces
    DROP CONSTRAINT IF EXISTS workspaces_status_check;

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_status_check
        CHECK (status IN ('active', 'suspended', 'deleted'));

CREATE INDEX IF NOT EXISTS idx_workspaces_deleted_at
    ON workspaces(deleted_at)
    WHERE deleted_at IS NOT NULL;
