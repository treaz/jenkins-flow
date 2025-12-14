-- Migration: 001_initial_schema
-- Description: Create workflow_runs table and indexes

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
