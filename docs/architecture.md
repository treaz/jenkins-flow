# Architecture: jenkins-flow

## Overview

Jenkins Flow orchestrates Jenkins jobs across multiple instances. It provides a Vue 3 web dashboard for managing workflows, supporting sequential and parallel job execution, PR waiting, and Slack notifications.

The application runs in two modes from the same codebase:
- **macOS App** — Native `.app` bundle using [Wails v2](https://wails.io/), which renders the Vue frontend in a system WebKit window. No browser required.
- **CLI Mode** — Standalone HTTP server serving the same Vue frontend to a browser.

Both modes share all backend packages and the same embedded frontend assets.

## Key Components

- **main.go** — Wails v2 entry point for the macOS app; creates a native window with the Vue frontend rendered in WebKit, API requests handled via the `AssetServer.Handler`
- **cmd/jenkins-flow/** — CLI entry point; parses flags and starts a standalone HTTP server
- **pkg/server/** — HTTP server (Chi router) implementing the OpenAPI-generated interface; serves the Vue SPA and REST API; manages in-memory execution state (`state.go`); exposes `BuildRouter()` for both entry points and `StartAsync()` for non-blocking startup
- **pkg/api/** — Auto-generated OpenAPI server stubs (`server.gen.go`) — do not edit directly
- **pkg/workflow/** — Workflow engine that executes sequential and parallel steps, handles variable substitution, and coordinates job triggers
- **pkg/config/** — YAML parsing for `instances.yaml` and workflow files
- **pkg/jenkins/** — Jenkins REST API client for triggering and polling jobs
- **pkg/github/** — GitHub API client for PR status checks
- **pkg/logger/** — Custom logging and HTTP transport logging
- **web/** — Vue 3 + Vite frontend application

## Data Flow

1. User opens the app (native macOS window or browser)
2. Vue frontend is served from embedded static assets (`pkg/server/static/`)
3. Frontend calls the REST API to list workflows, start executions, and poll status
4. Server loads workflow YAML definitions from the configured workflow directories
5. Workflow engine executes steps: triggers Jenkins jobs via `pkg/jenkins/`, optionally waits for PR merges via `pkg/github/`
6. Execution state is tracked in-memory in `pkg/server/state.go`
7. Optional Slack webhook notifications are sent on workflow completion

### macOS App Mode

In the macOS app, Wails embeds the frontend via `AssetServer.Assets` (static files) and routes API requests through `AssetServer.Handler` (the Chi router). There is no localhost HTTP server — Wails handles request routing internally via WebKit.

Configuration files are resolved from `~/.config/jenkins-flow/` when running as a `.app` bundle, with fallback to the current directory for development.

### CLI Mode

The CLI starts a standard HTTP server on the configured port (default 32567) and serves the same embedded static assets and API routes.

## Infrastructure

- **macOS app** — `make wails-build` produces `build/bin/Jenkins Flow.app` (~13 MB, uses system WebKit)
- **CLI binary** — `make build` produces `jenkins-flow` with embedded frontend
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
- `wails.json` — Wails v2 project configuration (app name, frontend build commands)
- `build/darwin/Info.plist` — macOS app metadata (bundle ID, minimum OS version)
- `build/appicon.png` — Application icon for the macOS app bundle

## Key ADRs

See [Architecture Decision Records](adr/README.md) for detailed decision documentation.
