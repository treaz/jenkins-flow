# Architecture: jenkins-flow

## Overview

Jenkins Flow is a CLI tool with an embedded web server that orchestrates Jenkins jobs across multiple instances. It provides a Vue 3 web dashboard for managing workflows, supporting sequential and parallel job execution, PR waiting, and Slack notifications.

The Go binary embeds the frontend static assets, producing a single deployable artifact.

## Key Components

- **cmd/jenkins-flow/** — Application entry point; parses flags and starts the server
- **pkg/server/** — HTTP server (Chi router) implementing the OpenAPI-generated interface; serves the Vue SPA and REST API; manages in-memory execution state (`state.go`)
- **pkg/api/** — Auto-generated OpenAPI server stubs (`server.gen.go`) — do not edit directly
- **pkg/workflow/** — Workflow engine that executes sequential and parallel steps, handles variable substitution, and coordinates job triggers
- **pkg/config/** — YAML parsing for `instances.yaml` and workflow files
- **pkg/jenkins/** — Jenkins REST API client for triggering and polling jobs
- **pkg/github/** — GitHub API client for PR status checks
- **pkg/logger/** — Custom logging and HTTP transport logging
- **web/** — Vue 3 + Vite frontend application

## Data Flow

1. User loads the web dashboard (served as embedded static assets from `pkg/server/static/`)
2. Frontend calls the REST API to list workflows, start executions, and poll status
3. Server loads workflow YAML definitions from `workflows/` directory
4. Workflow engine executes steps: triggers Jenkins jobs via `pkg/jenkins/`, optionally waits for PR merges via `pkg/github/`
5. Execution state is tracked in-memory in `pkg/server/state.go`
6. Optional Slack webhook notifications are sent on workflow completion

## Infrastructure

- **Single binary deployment** — `make build` produces `jenkins-flow` with embedded frontend
- **OpenAPI code generation** — `make generate-api` regenerates `pkg/api/server.gen.go` from `api/openapi.yaml`
- **CI** — GitHub Actions with Dependabot for dependency updates

## Workflow File Format

Workflows are defined in YAML with support for:
- Sequential steps
- Parallel execution blocks
- Variable substitution with `${variable_name}`
- Input definitions for UI-configurable values
- PR waiting based on branch names
- Slack webhook notifications

Example workflow structure:
```yaml
name: "Workflow Name"
inputs:
  variable_name: default_value
slack_webhook: "optional-webhook-url"
workflow:
  - name: "Step Name"
    instance: instance_id
    job: "/job/path"
    params:
      KEY: "${variable_name}"
  - parallel:
      name: "Group Name"
      steps:
        - name: "Parallel Step 1"
          instance: instance_id
          job: "/job/path"
```

## Configuration

- `instances.yaml` — Jenkins instance configurations (gitignored, contains auth tokens)
- `instances.yaml.template` — Template for instances configuration
- `workflows/*.yaml` — Workflow definitions
- `examples/*.yaml` — Example workflow files
- `api/openapi.yaml` — OpenAPI specification

## Key ADRs

See [Architecture Decision Records](adr/README.md) for detailed decision documentation.
