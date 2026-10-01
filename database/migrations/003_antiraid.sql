-- Migration 003: Anti-raid events
CREATE TABLE IF NOT EXISTS raid_events (
    id           BIGSERIAL PRIMARY KEY,
    chat_id      BIGINT NOT NULL,
    join_count   INT NOT NULL,
    sensitivity  TEXT DEFAULT 'MEDIUM',
    triggered_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at  TIMESTAMPTZ,
    action       TEXT,
    resolved_by  BIGINT
);
CREATE INDEX IF NOT EXISTS idx_raid_chat ON raid_events(chat_id);
CREATE INDEX IF NOT EXISTS idx_raid_time ON raid_events(triggered_at DESC);

-- Raid join log: record each suspicious join during raid window
CREATE TABLE IF NOT EXISTS raid_joins (
    id          BIGSERIAL PRIMARY KEY,
    raid_id     BIGINT REFERENCES raid_events(id),
    chat_id     BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    joined_at   TIMESTAMPTZ DEFAULT NOW()
);
