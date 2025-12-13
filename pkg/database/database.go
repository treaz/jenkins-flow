package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// WorkflowRun represents a historical workflow execution record.
type WorkflowRun struct {
	ID             int64             `json:"id"`
	WorkflowName   string            `json:"workflow_name"`
	WorkflowPath   string            `json:"workflow_path"`
	StartTime      time.Time         `json:"start_time"`
	EndTime        *time.Time        `json:"end_time,omitempty"`
	Status         string            `json:"status"`
	InputsJSON     string            `json:"inputs_json"`
	Inputs         map[string]string `json:"inputs,omitempty"`
	ConfigSnapshot string            `json:"config_snapshot"`
	SkipPRCheck    bool              `json:"skip_pr_check"`
}

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
	path string
}

// NewDB initializes a new database connection and creates tables if needed.
func NewDB(dbPath string) (*DB, error) {
	// Expand home directory if needed
	if dbPath[:2] == "~/" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, dbPath[2:])
	}

	// Create directory structure if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{
		conn: conn,
		path: dbPath,
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// initSchema creates the workflow_runs table if it doesn't exist.
func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS workflow_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workflow_name TEXT NOT NULL,
		workflow_path TEXT NOT NULL,
		start_time TIMESTAMP NOT NULL,
		end_time TIMESTAMP,
		status TEXT NOT NULL,
		inputs_json TEXT NOT NULL,
		config_snapshot TEXT NOT NULL,
		skip_pr_check BOOLEAN NOT NULL DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow_path ON workflow_runs(workflow_path);
	CREATE INDEX IF NOT EXISTS idx_workflow_runs_status ON workflow_runs(status);
	CREATE INDEX IF NOT EXISTS idx_workflow_runs_start_time ON workflow_runs(start_time DESC);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// CreateRun creates a new workflow run record with status "running".
func (db *DB) CreateRun(workflowName, workflowPath, configSnapshot string, inputs map[string]string, skipPRCheck bool) (int64, error) {
	if db.conn == nil {
		return 0, fmt.Errorf("database connection is nil")
	}

	// Serialize inputs to JSON
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal inputs: %w", err)
	}

	query := `
		INSERT INTO workflow_runs (workflow_name, workflow_path, start_time, status, inputs_json, config_snapshot, skip_pr_check)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.conn.Exec(query, workflowName, workflowPath, time.Now().UTC(), "running", string(inputsJSON), configSnapshot, skipPRCheck)
	if err != nil {
		return 0, fmt.Errorf("failed to insert workflow run: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

// UpdateRunComplete updates a workflow run with final status and end time.
func (db *DB) UpdateRunComplete(runID int64, status string, endTime time.Time) error {
	if db.conn == nil {
		return fmt.Errorf("database connection is nil")
	}

	query := `
		UPDATE workflow_runs
		SET status = ?, end_time = ?
		WHERE id = ?
	`

	result, err := db.conn.Exec(query, status, endTime.UTC(), runID)
	if err != nil {
		return fmt.Errorf("failed to update workflow run: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("workflow run with id %d not found", runID)
	}

	return nil
}

// GetRuns retrieves workflow runs with pagination and optional filters.
func (db *DB) GetRuns(limit, offset int, workflowPath, status string) ([]WorkflowRun, error) {
	if db.conn == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `
		SELECT id, workflow_name, workflow_path, start_time, end_time, status, inputs_json, config_snapshot, skip_pr_check
		FROM workflow_runs
		WHERE 1=1
	`
	args := []interface{}{}

	if workflowPath != "" {
		query += " AND workflow_path = ?"
		args = append(args, workflowPath)
	}

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}

	query += " ORDER BY start_time DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow runs: %w", err)
	}
	defer rows.Close()

	var runs []WorkflowRun
	for rows.Next() {
		var run WorkflowRun
		var endTime sql.NullTime

		err := rows.Scan(&run.ID, &run.WorkflowName, &run.WorkflowPath, &run.StartTime, &endTime, &run.Status, &run.InputsJSON, &run.ConfigSnapshot, &run.SkipPRCheck)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow run: %w", err)
		}

		if endTime.Valid {
			run.EndTime = &endTime.Time
		}

		// Unmarshal inputs for convenience
		if run.InputsJSON != "" {
			if err := json.Unmarshal([]byte(run.InputsJSON), &run.Inputs); err != nil {
				// Log error but don't fail the entire query
				run.Inputs = make(map[string]string)
			}
		}

		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating workflow runs: %w", err)
	}

	return runs, nil
}

// GetRun retrieves a specific workflow run by ID.
func (db *DB) GetRun(runID int64) (*WorkflowRun, error) {
	if db.conn == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `
		SELECT id, workflow_name, workflow_path, start_time, end_time, status, inputs_json, config_snapshot, skip_pr_check
		FROM workflow_runs
		WHERE id = ?
	`

	var run WorkflowRun
	var endTime sql.NullTime

	err := db.conn.QueryRow(query, runID).Scan(&run.ID, &run.WorkflowName, &run.WorkflowPath, &run.StartTime, &endTime, &run.Status, &run.InputsJSON, &run.ConfigSnapshot, &run.SkipPRCheck)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow run with id %d not found", runID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query workflow run: %w", err)
	}

	if endTime.Valid {
		run.EndTime = &endTime.Time
	}

	// Unmarshal inputs for convenience
	if run.InputsJSON != "" {
		if err := json.Unmarshal([]byte(run.InputsJSON), &run.Inputs); err != nil {
			// Set empty map if unmarshal fails
			run.Inputs = make(map[string]string)
		}
	}

	return &run, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Path returns the database file path.
func (db *DB) Path() string {
	return db.path
}
