-- Dedicated bot_users directory table to avoid colliding with pre-existing Supabase 'users' table
CREATE TABLE IF NOT EXISTS bot_users (
    user_id     BIGINT PRIMARY KEY,
    username    TEXT DEFAULT '',
    first_name  TEXT DEFAULT '',
    last_name   TEXT DEFAULT '',
    is_bot      BOOLEAN DEFAULT false,
    is_premium  BOOLEAN DEFAULT false,
    first_seen  TIMESTAMPTZ DEFAULT NOW(),
    last_seen   TIMESTAMPTZ DEFAULT NOW()
);
