package database

import (
	"context"
	"log"
	"os"

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

	// Create a background context
	ctx := context.Background()

	// Connect to the database pool
	var err error
	Pool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to create database connection pool: %v\n", err)
	}

	// Ping the database to ensure the credentials are correct
	err = Pool.Ping(ctx)
	if err != nil {
		log.Fatalf("Unable to ping the database: %v\n", err)
	}

	log.Println("✅ Successfully connected to Supabase PostgreSQL!")
}

// CloseDB gracefully terminates all connections in the pool
func CloseDB() {
	if Pool != nil {
		Pool.Close()
		log.Println("🔒 Database connection pool closed.")
	}
}