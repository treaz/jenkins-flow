package database

import (
	"path/filepath"
	"testing"
)

func TestMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-migrations.db")

	// Create database - should apply migrations
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	// Verify workflow_runs table exists
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM workflow_runs").Scan(&count)
	if err != nil {
		t.Fatalf("workflow_runs table doesn't exist: %v", err)
	}

	// Verify schema_migrations table exists (created by golang-migrate)
	err = db.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("schema_migrations table doesn't exist: %v", err)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-idempotent.db")

	// Create database first time
	db1, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("First NewDB failed: %v", err)
	}

	// Insert a test record
	inputs := map[string]string{"test": "value"}
	runID, err := db1.CreateRun("Test", "test.yaml", "config", inputs, false)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	db1.Close()

	// Re-open database - migrations should not run again
	db2, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("Second NewDB failed: %v", err)
	}
	defer db2.Close()

	// Verify the record still exists
	run, err := db2.GetRun(runID)
	if err != nil {
		t.Fatalf("GetRun failed after reopening: %v", err)
	}

	if run.WorkflowName != "Test" {
		t.Errorf("expected WorkflowName 'Test', got %q", run.WorkflowName)
	}
}

func TestDatabaseSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-schema.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	// Test that we can query the schema
	var tableName string
	err = db.conn.QueryRow(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name='workflow_runs'
	`).Scan(&tableName)

	if err != nil {
		t.Fatalf("workflow_runs table not found: %v", err)
	}

	if tableName != "workflow_runs" {
		t.Errorf("expected table name 'workflow_runs', got %q", tableName)
	}

	// Verify indexes exist
	rows, err := db.conn.Query(`
		SELECT name FROM sqlite_master 
		WHERE type='index' AND tbl_name='workflow_runs'
	`)
	if err != nil {
		t.Fatalf("failed to query indexes: %v", err)
	}
	defer rows.Close()

	indexCount := 0
	for rows.Next() {
		var name string
		rows.Scan(&name)
		indexCount++
	}

	// We expect at least 3 indexes (plus possibly sqlite's internal indexes)
	if indexCount < 3 {
		t.Errorf("expected at least 3 indexes, found %d", indexCount)
	}
}
