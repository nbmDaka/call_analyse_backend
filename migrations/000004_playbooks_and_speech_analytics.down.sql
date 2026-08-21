-- Migration 000004 Down: Revert playbooks, golden standards, and speech analytics

ALTER TABLE call_analyses
    DROP COLUMN IF EXISTS role_mapping,
    DROP COLUMN IF EXISTS actionable_coaching,
    DROP COLUMN IF EXISTS violations,
    DROP COLUMN IF EXISTS speech_analytics;

ALTER TABLE calls
    DROP COLUMN IF EXISTS playbook_id;

DROP TABLE IF EXISTS golden_standards;
DROP TABLE IF EXISTS playbooks;
