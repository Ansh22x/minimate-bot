-- Migration 006: Analytics tables
CREATE TABLE IF NOT EXISTS analytics_daily (
    id      BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    date    DATE NOT NULL,
    metric  TEXT NOT NULL,
    value   BIGINT DEFAULT 0,
    UNIQUE (chat_id, date, metric)
);
CREATE INDEX IF NOT EXISTS idx_analytics_chat_date ON analytics_daily(chat_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_metric     ON analytics_daily(metric);

-- Real-time counters (in-memory, flushed to analytics_daily periodically)
-- This table holds the pending increments before flush
CREATE TABLE IF NOT EXISTS analytics_buffer (
    chat_id    BIGINT NOT NULL,
    metric     TEXT NOT NULL,
    value      BIGINT DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (chat_id, metric)
);
