-- Migration 008: Clean up NULL or duplicate user_ids on pre-existing users table and add unique index
DELETE FROM users WHERE user_id IS NULL;
DELETE FROM users a USING users b WHERE a.ctid < b.ctid AND a.user_id = b.user_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_user_id_unique ON users(user_id);
