package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

// RunMigrations applies all pending SQL migrations in order.
// Tracks applied migrations in schema_migrations table.
func RunMigrations() {
	// Create the migrations tracking table if it doesn't exist
	_, err := Pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("❌ Failed to create schema_migrations table: %v", err)
	}

	// Load all migration files from the embedded list
	migrations := getMigrationFiles()
	sort.Strings(migrations)

	applied := 0
	for _, version := range migrations {
		// Check if already applied
		var count int
		err := Pool.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
		if err != nil {
			log.Printf("⚠️ Migration check error for %s: %v", version, err)
			continue
		}
		if count > 0 {
			continue // already applied
		}

		// Read SQL file
		sqlPath := fmt.Sprintf("database/migrations/%s", version)
		sqlBytes, err := os.ReadFile(sqlPath)
		if err != nil {
			log.Printf("⚠️ Cannot read migration %s: %v", version, err)
			continue
		}

		// Execute each statement separately (split on semicolon)
		stmts := splitSQL(string(sqlBytes))
		txErr := false
		for _, stmt := range stmts {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" || strings.HasPrefix(stmt, "--") {
				continue
			}
			if _, err := Pool.Exec(context.Background(), stmt); err != nil {
				log.Printf("❌ Migration %s failed:\n  SQL: %s\n  Error: %v", version, truncate(stmt, 120), err)
				txErr = true
				break
			}
		}

		if txErr {
			log.Fatalf("❌ Halting: migration %s failed. Fix and restart.", version)
		}

		// Mark as applied
		_, err = Pool.Exec(context.Background(),
			"INSERT INTO schema_migrations (version) VALUES ($1) ON CONFLICT DO NOTHING", version)
		if err != nil {
			log.Printf("⚠️ Could not record migration %s: %v", version, err)
		} else {
			log.Printf("✅ Applied migration: %s", version)
			applied++
		}
	}

	if applied == 0 {
		log.Println("✅ All migrations up to date.")
	} else {
		log.Printf("✅ Applied %d new migration(s).", applied)
	}
}

// getMigrationFiles returns the ordered list of migration SQL filenames.
func getMigrationFiles() []string {
	return []string{
		"001_foundation.sql",
		"002_antispam.sql",
		"003_antiraid.sql",
		"004_verification.sql",
		"005_automation.sql",
		"006_analytics.sql",
	}
}

// splitSQL splits a SQL file into individual statements by semicolons,
// handling multi-line statements and comments correctly.
func splitSQL(sql string) []string {
	var stmts []string
	var current strings.Builder
	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue // skip comment lines
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
			current.Reset()
		}
	}
	// Capture any trailing statement without semicolon
	if remaining := strings.TrimSpace(current.String()); remaining != "" {
		stmts = append(stmts, remaining)
	}
	return stmts
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
