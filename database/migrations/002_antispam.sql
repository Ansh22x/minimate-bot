-- Migration 002: Anti-spam support tables
-- Antispam incidents log
CREATE TABLE IF NOT EXISTS antispam_incidents (
    id         BIGSERIAL PRIMARY KEY,
    chat_id    BIGINT NOT NULL,
    user_id    BIGINT NOT NULL,
    spam_type  TEXT NOT NULL,
    detail     TEXT,
    action     TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_antispam_chat    ON antispam_incidents(chat_id);
CREATE INDEX IF NOT EXISTS idx_antispam_user    ON antispam_incidents(user_id);
CREATE INDEX IF NOT EXISTS idx_antispam_created ON antispam_incidents(created_at DESC);

-- Flood tracking is handled in-memory, but incidents are logged here for analytics
