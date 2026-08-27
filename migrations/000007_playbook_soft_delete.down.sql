-- Migration 000007 Down: Revert Playbook logical deletion

DROP INDEX IF EXISTS idx_playbooks_deleted_at;

DROP INDEX IF EXISTS idx_playbooks_workspace_default;
CREATE UNIQUE INDEX IF NOT EXISTS idx_playbooks_workspace_default
    ON playbooks(workspace_id)
    WHERE is_default = TRUE;

ALTER TABLE playbooks
    DROP COLUMN IF EXISTS deleted_at;
