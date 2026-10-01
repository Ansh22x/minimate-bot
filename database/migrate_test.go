package database

import (
	"testing"
)

func TestSplitSQL(t *testing.T) {
	raw := `
	-- Comment 1
	CREATE TABLE t1 (id INT PRIMARY KEY);

	-- Comment 2
	CREATE TABLE t2 (
		name TEXT
	);
	`
	stmts := splitSQL(raw)
	if len(stmts) != 2 {
		t.Fatalf("Expected 2 statements, got %d", len(stmts))
	}
}
