CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin', 'supervisor', 'manager')),
    supervisor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

CREATE TABLE IF NOT EXISTS calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    manager_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status TEXT NOT NULL DEFAULT 'uploaded' CHECK (
        status IN ('uploaded', 'queued', 'transcribing', 'transcribed', 'analyzing', 'completed', 'failed')
    ),
    original_filename TEXT NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    duration_seconds INTEGER CHECK (duration_seconds >= 0),
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_calls_manager_id ON calls(manager_id);
CREATE INDEX IF NOT EXISTS idx_calls_status ON calls(status);
CREATE INDEX IF NOT EXISTS idx_calls_created_at ON calls(created_at DESC);

CREATE TABLE IF NOT EXISTS call_transcripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    call_id UUID NOT NULL UNIQUE REFERENCES calls(id) ON DELETE CASCADE,
    full_text TEXT NOT NULL,
    segments JSONB NOT NULL DEFAULT '[]'::jsonb,
    raw_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(segments) = 'array')
);

CREATE TABLE IF NOT EXISTS call_analyses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    call_id UUID NOT NULL UNIQUE REFERENCES calls(id) ON DELETE CASCADE,
    summary TEXT NOT NULL,
    needs JSONB NOT NULL DEFAULT '[]'::jsonb,
    objections JSONB NOT NULL DEFAULT '[]'::jsonb,
    refusal_reason TEXT,
    mistakes JSONB NOT NULL DEFAULT '[]'::jsonb,
    strengths JSONB NOT NULL DEFAULT '[]'::jsonb,
    next_action TEXT NOT NULL,
    criterion_results JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (jsonb_typeof(needs) = 'array'),
    CHECK (jsonb_typeof(objections) = 'array'),
    CHECK (jsonb_typeof(mistakes) = 'array'),
    CHECK (jsonb_typeof(strengths) = 'array'),
    CHECK (jsonb_typeof(criterion_results) = 'object')
);

CREATE TABLE IF NOT EXISTS call_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    call_id UUID NOT NULL UNIQUE REFERENCES calls(id) ON DELETE CASCADE,
    greeting_score INTEGER NOT NULL CHECK (greeting_score BETWEEN 0 AND 5),
    rapport_score INTEGER NOT NULL CHECK (rapport_score BETWEEN 0 AND 10),
    needs_discovery_score INTEGER NOT NULL CHECK (needs_discovery_score BETWEEN 0 AND 20),
    presentation_score INTEGER NOT NULL CHECK (presentation_score BETWEEN 0 AND 15),
    objection_handling_score INTEGER NOT NULL CHECK (objection_handling_score BETWEEN 0 AND 20),
    next_action_score INTEGER NOT NULL CHECK (next_action_score BETWEEN 0 AND 15),
    communication_score INTEGER NOT NULL CHECK (communication_score BETWEEN 0 AND 10),
    closing_score INTEGER NOT NULL CHECK (closing_score BETWEEN 0 AND 5),
    total_score INTEGER NOT NULL CHECK (total_score BETWEEN 0 AND 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
