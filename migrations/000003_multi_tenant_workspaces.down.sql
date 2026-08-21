DROP TRIGGER IF EXISTS workspace_last_owner_guard ON workspace_memberships;
DROP FUNCTION IF EXISTS protect_last_workspace_owner();
DROP TRIGGER IF EXISTS workspace_membership_supervisor_guard ON workspace_memberships;
DROP FUNCTION IF EXISTS validate_membership_supervisor();

DROP INDEX IF EXISTS idx_calls_workspace_status_created;
DROP INDEX IF EXISTS idx_calls_workspace_owner_created;
ALTER TABLE calls
    DROP COLUMN IF EXISTS uploaded_by_user_id,
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS workspace_id;

DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS workspace_memberships;
DROP TABLE IF EXISTS workspaces;

ALTER TABLE users
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS platform_role;
