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
	`
	
	_, err := Pool.Exec(context.Background(), query)
	if err != nil {
		log.Fatalf("❌ Failed to create tables: %v", err)
	}
	
	log.Println("✅ Database tables successfully verified!")
}