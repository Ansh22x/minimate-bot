-- Migration 001: Foundation tables
-- Groups: tracks all chats the bot is in
CREATE TABLE IF NOT EXISTS groups (
    chat_id      BIGINT PRIMARY KEY,
    title        TEXT,
    username     TEXT,
    chat_type    TEXT,
    member_count INT DEFAULT 0,
    is_active    BOOLEAN DEFAULT true,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Users: global user registry
CREATE TABLE IF NOT EXISTS users (
    user_id     BIGINT PRIMARY KEY,
    username    TEXT,
    first_name  TEXT,
    last_name   TEXT,
    is_bot      BOOLEAN DEFAULT false,
    is_premium  BOOLEAN DEFAULT false,
    first_seen  TIMESTAMPTZ DEFAULT NOW(),
    last_seen   TIMESTAMPTZ DEFAULT NOW()
);

-- Group members: per-group membership tracking
CREATE TABLE IF NOT EXISTS group_members (
    chat_id    BIGINT NOT NULL,
    user_id    BIGINT NOT NULL,
    joined_at  TIMESTAMPTZ DEFAULT NOW(),
    left_at    TIMESTAMPTZ,
    is_active  BOOLEAN DEFAULT true,
    warn_count INT DEFAULT 0,
    PRIMARY KEY (chat_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_group_members_chat ON group_members(chat_id);
CREATE INDEX IF NOT EXISTS idx_group_members_user ON group_members(user_id);

-- Moderation logs: complete audit trail
CREATE TABLE IF NOT EXISTS moderation_logs (
    id         BIGSERIAL PRIMARY KEY,
    chat_id    BIGINT NOT NULL,
    user_id    BIGINT NOT NULL,
    mod_id     BIGINT NOT NULL,
    action     TEXT NOT NULL,
    reason     TEXT,
    extra      JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    undone     BOOLEAN DEFAULT false,
    undone_by  BIGINT,
    undone_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_modlogs_chat    ON moderation_logs(chat_id);
CREATE INDEX IF NOT EXISTS idx_modlogs_user    ON moderation_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_modlogs_action  ON moderation_logs(action);
CREATE INDEX IF NOT EXISTS idx_modlogs_created ON moderation_logs(created_at DESC);

-- Temp actions: background worker picks these up and restores permissions
CREATE TABLE IF NOT EXISTS temp_actions (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    action_type TEXT NOT NULL,  -- 'mute', 'ban', 'restrict'
    log_id      BIGINT,
    expires_at  TIMESTAMPTZ NOT NULL,
    done        BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_tempactions_expires ON temp_actions(expires_at) WHERE done = false;
CREATE INDEX IF NOT EXISTS idx_tempactions_chat    ON temp_actions(chat_id);

-- Group settings: consolidated JSONB settings per group
CREATE TABLE IF NOT EXISTS group_settings (
    chat_id    BIGINT PRIMARY KEY,
    settings   JSONB DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Admin roles: per-group bot permission roles
CREATE TABLE IF NOT EXISTS admin_roles (
    chat_id     BIGINT NOT NULL,
    user_id     BIGINT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'moderator',
    permissions JSONB DEFAULT '{}'::jsonb,
    set_by      BIGINT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (chat_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_admin_roles_chat ON admin_roles(chat_id);

-- Warn entries: individual warn records (separate from warn count)
CREATE TABLE IF NOT EXISTS warn_entries (
    id         BIGSERIAL PRIMARY KEY,
    chat_id    BIGINT NOT NULL,
    user_id    BIGINT NOT NULL,
    mod_id     BIGINT NOT NULL,
    reason     TEXT,
    active     BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_warns_chat_user ON warn_entries(chat_id, user_id) WHERE active = true;
CREATE INDEX IF NOT EXISTS idx_warns_expires   ON warn_entries(expires_at) WHERE active = true;
