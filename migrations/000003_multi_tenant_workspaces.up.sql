ALTER TABLE users
    ADD COLUMN platform_role TEXT NOT NULL DEFAULT 'user'
        CHECK (platform_role IN ('user', 'super_admin')),
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active'
        CHECK (status IN ('active', 'suspended'));

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    type TEXT NOT NULL CHECK (type IN ('personal', 'company')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_workspaces_personal_owner
    ON workspaces(owner_user_id) WHERE type = 'personal';
CREATE INDEX idx_workspaces_type_status ON workspaces(type, status);

CREATE TABLE workspace_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'supervisor', 'manager')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('invited', 'active', 'disabled')),
    supervisor_membership_id UUID REFERENCES workspace_memberships(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, user_id)
);

CREATE INDEX idx_workspace_memberships_workspace_user
    ON workspace_memberships(workspace_id, user_id);
CREATE INDEX idx_workspace_memberships_workspace_role_status
    ON workspace_memberships(workspace_id, role, status);
CREATE INDEX idx_workspace_memberships_supervisor
    ON workspace_memberships(supervisor_membership_id)
    WHERE supervisor_membership_id IS NOT NULL;

CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    action TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID NOT NULL,
    workspace_id UUID REFERENCES workspaces(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_events_actor_created ON audit_events(actor_user_id, created_at DESC);
CREATE INDEX idx_audit_events_workspace_created ON audit_events(workspace_id, created_at DESC);

-- Every existing account receives its own personal tenant.
INSERT INTO workspaces (name, type, owner_user_id)
SELECT split_part(u.email, '@', 1) || '''s workspace', 'personal', u.id
FROM users u
ON CONFLICT (owner_user_id) WHERE type = 'personal' DO NOTHING;

INSERT INTO workspace_memberships (workspace_id, user_id, role, status)
SELECT w.id, w.owner_user_id, 'owner', 'active'
FROM workspaces w
WHERE w.type = 'personal'
ON CONFLICT (workspace_id, user_id) DO NOTHING;

-- Preserve the original single-company model in one explicit legacy tenant.
INSERT INTO workspaces (name, type, owner_user_id)
SELECT 'Legacy Company', 'company', chosen.id
FROM LATERAL (
    SELECT id
    FROM users
    ORDER BY CASE WHEN role = 'admin' THEN 0 ELSE 1 END, created_at, id
    LIMIT 1
) chosen
WHERE EXISTS (SELECT 1 FROM users)
  AND NOT EXISTS (SELECT 1 FROM workspaces WHERE type = 'company' AND name = 'Legacy Company');

INSERT INTO workspace_memberships (workspace_id, user_id, role, status)
SELECT legacy.id,
       u.id,
       CASE
           WHEN u.id = legacy.owner_user_id THEN 'owner'
           WHEN u.role = 'admin' THEN 'admin'
           WHEN u.role = 'supervisor' THEN 'supervisor'
           ELSE 'manager'
       END,
       'active'
FROM users u
CROSS JOIN LATERAL (
    SELECT id, owner_user_id
    FROM workspaces
    WHERE type = 'company' AND name = 'Legacy Company'
    ORDER BY created_at, id
    LIMIT 1
) legacy
ON CONFLICT (workspace_id, user_id) DO NOTHING;

UPDATE workspace_memberships manager_membership
SET supervisor_membership_id = supervisor_membership.id,
    updated_at = NOW()
FROM users manager_user
JOIN workspace_memberships supervisor_membership
  ON supervisor_membership.user_id = manager_user.supervisor_id
 AND supervisor_membership.role = 'supervisor'
 AND supervisor_membership.status = 'active'
WHERE manager_membership.user_id = manager_user.id
  AND manager_membership.role = 'manager'
  AND supervisor_membership.workspace_id = manager_membership.workspace_id;

ALTER TABLE calls
    ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE RESTRICT,
    ADD COLUMN owner_user_id UUID REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN uploaded_by_user_id UUID REFERENCES users(id) ON DELETE RESTRICT;

UPDATE calls c
SET workspace_id = legacy.id,
    owner_user_id = c.manager_id,
    uploaded_by_user_id = c.manager_id
FROM LATERAL (
    SELECT id
    FROM workspaces
    WHERE type = 'company' AND name = 'Legacy Company'
    ORDER BY created_at, id
    LIMIT 1
) legacy
WHERE c.workspace_id IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM calls
        WHERE workspace_id IS NULL OR owner_user_id IS NULL OR uploaded_by_user_id IS NULL
    ) THEN
        RAISE EXCEPTION 'multi-tenant call backfill left NULL ownership columns';
    END IF;
END $$;

ALTER TABLE calls
    ALTER COLUMN workspace_id SET NOT NULL,
    ALTER COLUMN owner_user_id SET NOT NULL,
    ALTER COLUMN uploaded_by_user_id SET NOT NULL;

CREATE INDEX idx_calls_workspace_owner_created
    ON calls(workspace_id, owner_user_id, created_at DESC);
CREATE INDEX idx_calls_workspace_status_created
    ON calls(workspace_id, status, created_at DESC);

CREATE OR REPLACE FUNCTION validate_membership_supervisor() RETURNS trigger AS $$
BEGIN
	IF TG_OP = 'UPDATE' AND OLD.role = 'supervisor' AND OLD.status = 'active'
	   AND (NEW.role <> 'supervisor' OR NEW.status <> 'active') THEN
		UPDATE workspace_memberships
		SET supervisor_membership_id = NULL, updated_at = NOW()
		WHERE workspace_id = OLD.workspace_id AND supervisor_membership_id = OLD.id;
	END IF;
    IF NEW.role <> 'manager' AND NEW.supervisor_membership_id IS NOT NULL THEN
        RAISE EXCEPTION 'only managers may have a supervisor';
    END IF;
    IF NEW.supervisor_membership_id IS NOT NULL AND NOT EXISTS (
        SELECT 1
        FROM workspace_memberships supervisor
        WHERE supervisor.id = NEW.supervisor_membership_id
          AND supervisor.workspace_id = NEW.workspace_id
          AND supervisor.role = 'supervisor'
          AND supervisor.status = 'active'
    ) THEN
        RAISE EXCEPTION 'supervisor must be active and belong to the same workspace';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER workspace_membership_supervisor_guard
BEFORE INSERT OR UPDATE OF workspace_id, role, status, supervisor_membership_id
ON workspace_memberships
FOR EACH ROW EXECUTE FUNCTION validate_membership_supervisor();

CREATE OR REPLACE FUNCTION protect_last_workspace_owner() RETURNS trigger AS $$
BEGIN
    IF OLD.role = 'owner' AND OLD.status = 'active'
       AND (TG_OP = 'DELETE' OR NEW.role <> 'owner' OR NEW.status <> 'active')
       AND NOT EXISTS (
           SELECT 1 FROM workspace_memberships other_owner
           WHERE other_owner.workspace_id = OLD.workspace_id
             AND other_owner.id <> OLD.id
             AND other_owner.role = 'owner'
             AND other_owner.status = 'active'
       ) THEN
        RAISE EXCEPTION 'workspace must retain an active owner';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER workspace_last_owner_guard
BEFORE DELETE OR UPDATE OF role, status ON workspace_memberships
FOR EACH ROW EXECUTE FUNCTION protect_last_workspace_owner();

COMMENT ON COLUMN users.role IS 'Deprecated legacy role retained temporarily for rollback compatibility; authorization uses platform_role and workspace_memberships.';
COMMENT ON COLUMN users.supervisor_id IS 'Deprecated legacy relationship retained temporarily; authorization uses workspace_memberships.supervisor_membership_id.';
COMMENT ON COLUMN calls.manager_id IS 'Deprecated ownership retained temporarily; authorization uses workspace_id and owner_user_id.';
