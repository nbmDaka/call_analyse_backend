-- Migration 000004: Playbooks, Golden Standards, and Speech Analytics

CREATE TABLE IF NOT EXISTS playbooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    criteria JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(criteria) = 'array'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_playbooks_workspace_id ON playbooks(workspace_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_playbooks_workspace_default
    ON playbooks(workspace_id) WHERE is_default = TRUE;

CREATE TABLE IF NOT EXISTS golden_standards (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    call_id UUID REFERENCES calls(id) ON DELETE SET NULL,
    category TEXT NOT NULL CHECK (btrim(category) <> ''),
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    transcript_snippet TEXT NOT NULL CHECK (btrim(transcript_snippet) <> ''),
    audio_start_seconds DOUBLE PRECISION CHECK (audio_start_seconds >= 0),
    audio_end_seconds DOUBLE PRECISION CHECK (audio_end_seconds >= 0),
    why_golden TEXT NOT NULL CHECK (btrim(why_golden) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_golden_standards_workspace_category
    ON golden_standards(workspace_id, category);

ALTER TABLE calls
    ADD COLUMN IF NOT EXISTS playbook_id UUID REFERENCES playbooks(id) ON DELETE SET NULL;

ALTER TABLE call_analyses
    ADD COLUMN IF NOT EXISTS speech_analytics JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(speech_analytics) = 'object'),
    ADD COLUMN IF NOT EXISTS violations JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(violations) = 'array'),
    ADD COLUMN IF NOT EXISTS actionable_coaching JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(actionable_coaching) = 'array'),
    ADD COLUMN IF NOT EXISTS role_mapping JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(role_mapping) = 'object');

-- Seed default sales playbook for every workspace
INSERT INTO playbooks (workspace_id, name, description, is_default, criteria)
SELECT
    w.id,
    'Стандартный регламент продаж (100 баллов)',
    'Базовый чек-лист контроля качества: приветствие, установление контакта, выявление потребностей, презентация, отработка возражений, закрытие и следующий шаг.',
    TRUE,
    '[
        {"key": "greeting", "title": "Корпоративное приветствие", "max_score": 5, "description": "Менеджер должен четко поздороваться, назвать свое имя и компанию."},
        {"key": "rapport", "title": "Установление контакта", "max_score": 10, "description": "Вежливый и доброжелательный тон, обращение к клиенту по имени."},
        {"key": "needs_discovery", "title": "Выявление потребностей", "max_score": 20, "description": "Задать минимум 2-3 открытых вопроса для понимания задачи и бюджета клиента."},
        {"key": "presentation", "title": "Презентация решения", "max_score": 15, "description": "Презентовать продукт через выгоды для клиента, а не просто перечислять функции."},
        {"key": "objection_handling", "title": "Отработка возражений", "max_score": 20, "description": "Выслушать сомнения клиента, согласиться с важностью вопроса и аргументированно отработать."},
        {"key": "next_action", "title": "Фиксация следующего шага", "max_score": 15, "description": "Назначить конкретную дату, время и цель следующего контакта."},
        {"key": "communication", "title": "Грамотность и этика речи", "max_score": 10, "description": "Отсутствие слов-паразитов, перебиваний и пассивной агрессии."},
        {"key": "closing", "title": "Завершение разговора", "max_score": 5, "description": "Позитивное прощание и подтверждение договоренностей."}
    ]'::jsonb
FROM workspaces w
ON CONFLICT (workspace_id) WHERE is_default = TRUE DO NOTHING;
