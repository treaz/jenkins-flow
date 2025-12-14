-- Migration: 000001_initial_schema (down)
-- Description: Rollback initial schema

DROP INDEX IF EXISTS idx_workflow_runs_start_time;
DROP INDEX IF EXISTS idx_workflow_runs_status;
DROP INDEX IF EXISTS idx_workflow_runs_workflow_path;
DROP TABLE IF EXISTS workflow_runs;
