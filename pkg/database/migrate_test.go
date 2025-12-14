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

	// Verify schema_migrations table exists
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("schema_migrations table doesn't exist: %v", err)
	}

	if count == 0 {
		t.Error("expected at least one migration to be applied")
	}

	// Verify workflow_runs table exists
	err = db.conn.QueryRow("SELECT COUNT(*) FROM workflow_runs").Scan(&count)
	if err != nil {
		t.Fatalf("workflow_runs table doesn't exist: %v", err)
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

	// Verify migration count hasn't changed
	var count int
	err = db2.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count migrations: %v", err)
	}

	// Should have exactly 1 migration applied
	if count != 1 {
		t.Errorf("expected 1 migration, got %d (migrations should not be reapplied)", count)
	}
}

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations failed: %v", err)
	}

	if len(migrations) == 0 {
		t.Error("expected at least one migration to be loaded")
	}

	// Verify migrations are sorted by version
	for i := 1; i < len(migrations); i++ {
		if migrations[i].Version <= migrations[i-1].Version {
			t.Errorf("migrations not sorted: migration %d (version %d) comes after migration %d (version %d)",
				i, migrations[i].Version, i-1, migrations[i-1].Version)
		}
	}

	// Verify first migration
	if migrations[0].Version != 1 {
		t.Errorf("expected first migration version to be 1, got %d", migrations[0].Version)
	}

	if migrations[0].Name != "initial_schema" {
		t.Errorf("expected first migration name 'initial_schema', got %q", migrations[0].Name)
	}

	if len(migrations[0].SQL) == 0 {
		t.Error("expected migration SQL to be non-empty")
	}
}
