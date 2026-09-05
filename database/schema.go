package database

import (
	"context"
	"log"
)

// CreateTables builds the required database architecture for all modules
func CreateTables() {
	query := `
	-- Greetings Module
	CREATE TABLE IF NOT EXISTS chat_greetings (
		chat_id BIGINT PRIMARY KEY,
		welcome_enabled BOOLEAN DEFAULT false,
		welcome_text TEXT,
		goodbye_enabled BOOLEAN DEFAULT false,
		goodbye_text TEXT
	);

	-- Filters & Notes Module
	CREATE TABLE IF NOT EXISTS filters (
		chat_id BIGINT,
		keyword TEXT,
		reply_text TEXT,
		PRIMARY KEY (chat_id, keyword)
	);

	-- Rules Module
	CREATE TABLE IF NOT EXISTS chat_rules (
		chat_id BIGINT PRIMARY KEY,
		rules_text TEXT
	);

	-- Warnings Module
	CREATE TABLE IF NOT EXISTS user_warns (
		chat_id BIGINT,
		user_id BIGINT,
		warn_count INT DEFAULT 0,
		PRIMARY KEY (chat_id, user_id)
	);

	-- Premium VIP Subscriptions Module
	CREATE TABLE IF NOT EXISTS chat_subscriptions (
		chat_id BIGINT PRIMARY KEY,
		is_vip BOOLEAN DEFAULT false,
		expires_at TIMESTAMPTZ,
		plan_name TEXT DEFAULT 'VIP'
	);

	-- Security Locks Module (Anti-Link, Anti-Forward, Media Locks)
	CREATE TABLE IF NOT EXISTS chat_locks (
		chat_id BIGINT PRIMARY KEY,
		lock_links BOOLEAN DEFAULT false,
		lock_forwards BOOLEAN DEFAULT false,
		lock_stickers BOOLEAN DEFAULT false,
		lock_bots BOOLEAN DEFAULT false,
		lock_media BOOLEAN DEFAULT false,
		lock_invites BOOLEAN DEFAULT false
	);

	-- Automated Human Verification Captcha Module
	CREATE TABLE IF NOT EXISTS chat_captcha (
		chat_id BIGINT PRIMARY KEY,
		enabled BOOLEAN DEFAULT false,
		timeout_seconds INT DEFAULT 120,
		mode TEXT DEFAULT 'button'
	);

	-- Bot Active Chats & Groups Directory
	CREATE TABLE IF NOT EXISTS bot_chats (
		chat_id BIGINT PRIMARY KEY,
		title TEXT,
		chat_type TEXT,
		is_active BOOLEAN DEFAULT true,
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);
	`

	_, err := Pool.Exec(context.Background(), query)
	if err != nil {
		log.Fatalf("❌ Failed to create tables: %v", err)
	}

	log.Println("✅ Database tables successfully verified with Owner & Group Directory modules!")
}