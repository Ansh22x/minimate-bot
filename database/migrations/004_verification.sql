-- Migration 004: Verification sessions
CREATE TABLE IF NOT EXISTS verification_sessions (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    session_key TEXT,
    method      TEXT DEFAULT 'button',
    attempts    INT DEFAULT 0,
    passed      BOOLEAN DEFAULT false,
    kicked      BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    expires_at  TIMESTAMPTZ,
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_verify_chat_user  ON verification_sessions(chat_id, user_id);
CREATE INDEX IF NOT EXISTS idx_verify_expires    ON verification_sessions(expires_at) WHERE passed = false AND kicked = false;
