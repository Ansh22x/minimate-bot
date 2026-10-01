-- Migration 005: Automation engine and scheduled tasks
CREATE TABLE IF NOT EXISTS automations (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT NOT NULL,
    name        TEXT NOT NULL,
    trigger     TEXT NOT NULL,
    conditions  JSONB DEFAULT '[]'::jsonb,
    actions     JSONB DEFAULT '[]'::jsonb,
    enabled     BOOLEAN DEFAULT true,
    created_by  BIGINT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_automations_chat ON automations(chat_id);

-- Scheduled tasks: persistent job queue (survives restart)
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT NOT NULL,
    task_type   TEXT NOT NULL,
    payload     JSONB DEFAULT '{}'::jsonb,
    run_at      TIMESTAMPTZ NOT NULL,
    recurrence  TEXT,           -- null=one-time, 'daily', 'weekly', etc.
    last_run    TIMESTAMPTZ,
    next_run    TIMESTAMPTZ,
    done        BOOLEAN DEFAULT false,
    error       TEXT,
    created_by  BIGINT,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_scheduled_run   ON scheduled_tasks(run_at) WHERE done = false;
CREATE INDEX IF NOT EXISTS idx_scheduled_chat  ON scheduled_tasks(chat_id);

-- Automation execution log
CREATE TABLE IF NOT EXISTS automation_logs (
    id             BIGSERIAL PRIMARY KEY,
    automation_id  BIGINT REFERENCES automations(id),
    chat_id        BIGINT NOT NULL,
    trigger_data   JSONB DEFAULT '{}'::jsonb,
    actions_taken  JSONB DEFAULT '[]'::jsonb,
    success        BOOLEAN DEFAULT true,
    error          TEXT,
    executed_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_autolog_automation ON automation_logs(automation_id);
CREATE INDEX IF NOT EXISTS idx_autolog_chat        ON automation_logs(chat_id);
