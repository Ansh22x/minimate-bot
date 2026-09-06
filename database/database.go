package database

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is the global database connection pool
var Pool *pgxpool.Pool

// InitDB connects to PostgreSQL and pings the database to verify the connection
func InitDB() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL must be set in the .env file")
	}

	ctx := context.Background()

	// Parse database URL into pgxpool configuration
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Unable to parse DATABASE_URL: %v\n", err)
	}

	// Disable automatic prepared statement caching for Supabase / PgBouncer pooler compatibility
	// This fixes the "prepared statement already exists (SQLSTATE 42P05)" error
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Connect to the database pool
	Pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Unable to create database connection pool: %v\n", err)
	}

	// Ping the database to ensure the credentials are correct
	err = Pool.Ping(ctx)
	if err != nil {
		log.Fatalf("Unable to ping the database: %v\n", err)
	}

	log.Println("✅ Successfully connected to Supabase PostgreSQL (Simple Protocol Enabled)!")
}

// CloseDB gracefully terminates all connections in the pool
func CloseDB() {
	if Pool != nil {
		Pool.Close()
		log.Println("🔒 Database connection pool closed.")
	}
}
