-- Migration 000007: Playbook logical deletion (soft delete)

ALTER TABLE playbooks
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

DROP INDEX IF EXISTS idx_playbooks_workspace_default;
CREATE UNIQUE INDEX IF NOT EXISTS idx_playbooks_workspace_default
    ON playbooks(workspace_id)
    WHERE is_default = TRUE AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_playbooks_deleted_at
    ON playbooks(workspace_id, deleted_at)
    WHERE deleted_at IS NOT NULL;
