package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewDB(t *testing.T) {
	// Create a temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	if db.Path() != dbPath {
		t.Errorf("expected path %q, got %q", dbPath, db.Path())
	}

	// Verify table was created
	var tableName string
	err = db.conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='workflow_runs'").Scan(&tableName)
	if err != nil {
		t.Fatalf("table 'workflow_runs' was not created: %v", err)
	}
}

func TestCreateRun(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	inputs := map[string]string{
		"env":     "production",
		"version": "1.2.3",
	}

	runID, err := db.CreateRun("Test Workflow", "workflows/test.yaml", "name: Test Workflow\nworkflow: []", inputs)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	if runID <= 0 {
		t.Errorf("expected positive run ID, got %d", runID)
	}

	// Verify the run was created
	run, err := db.GetRun(runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if run.WorkflowName != "Test Workflow" {
		t.Errorf("expected workflow name 'Test Workflow', got %q", run.WorkflowName)
	}

	if run.Status != "running" {
		t.Errorf("expected status 'running', got %q", run.Status)
	}

	if run.Inputs["env"] != "production" {
		t.Errorf("expected input env='production', got %q", run.Inputs["env"])
	}
}

func TestUpdateRunComplete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	inputs := map[string]string{"key": "value"}
	runID, err := db.CreateRun("Test Workflow", "workflows/test.yaml", "config", inputs)
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Update to success
	endTime := time.Now()
	err = db.UpdateRunComplete(runID, "success", endTime)
	if err != nil {
		t.Fatalf("UpdateRunComplete failed: %v", err)
	}

	// Verify update
	run, err := db.GetRun(runID)
	if err != nil {
		t.Fatalf("GetRun failed: %v", err)
	}

	if run.Status != "success" {
		t.Errorf("expected status 'success', got %q", run.Status)
	}

	if run.EndTime == nil {
		t.Error("expected end_time to be set")
	}
}

func TestGetRuns(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	// Create multiple runs
	inputs := map[string]string{"key": "value"}
	for i := 0; i < 5; i++ {
		_, err := db.CreateRun("Test Workflow", "workflows/test.yaml", "config", inputs)
		if err != nil {
			t.Fatalf("CreateRun failed: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Test pagination
	runs, err := db.GetRuns(2, 0, "", "")
	if err != nil {
		t.Fatalf("GetRuns failed: %v", err)
	}

	if len(runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(runs))
	}

	// Test offset
	runs, err = db.GetRuns(2, 2, "", "")
	if err != nil {
		t.Fatalf("GetRuns with offset failed: %v", err)
	}

	if len(runs) != 2 {
		t.Errorf("expected 2 runs with offset, got %d", len(runs))
	}

	// Test status filter
	runs, err = db.GetRuns(10, 0, "", "running")
	if err != nil {
		t.Fatalf("GetRuns with status filter failed: %v", err)
	}

	if len(runs) != 5 {
		t.Errorf("expected 5 running workflows, got %d", len(runs))
	}

	// Test workflow path filter
	runs, err = db.GetRuns(10, 0, "workflows/test.yaml", "")
	if err != nil {
		t.Fatalf("GetRuns with workflow_path filter failed: %v", err)
	}

	if len(runs) != 5 {
		t.Errorf("expected 5 workflows with path filter, got %d", len(runs))
	}
}

func TestGetRun_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	_, err = db.GetRun(999)
	if err == nil {
		t.Error("expected error for non-existent run, got nil")
	}
}

func TestNewDB_DirectoryCreation(t *testing.T) {
	// Test that directory creation works
	tmpSubdir := filepath.Join(os.TempDir(), "jenkins-flow-test")
	os.RemoveAll(tmpSubdir) // Clean up before test
	defer os.RemoveAll(tmpSubdir)

	// Create a path with nested directories
	dbPath := filepath.Join(tmpSubdir, "data", "test.db")

	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	// Verify the directory was created
	if _, err := os.Stat(filepath.Dir(dbPath)); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}
}
