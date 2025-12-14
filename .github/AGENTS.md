# AGENTS.md

## Project Overview

Jenkins Flow CLI is a Go-based application that orchestrates Jenkins jobs across multiple instances. It provides a web dashboard (Vue 3 + Vite) for managing workflows, with support for sequential and parallel job execution, PR waiting, and notifications.

**Tech Stack:**
- Backend: Go 1.25.4
- Frontend: Vue 3 + Vite
- API: OpenAPI 3.0 specification with code generation
- Architecture: CLI tool with embedded web server and static assets

## Setup Commands

```bash
# Install Go dependencies
make deps

# Install frontend dependencies (if working on web UI)
cd web && npm install

# Build everything (includes frontend build + Go binary)
make build
```

## Development Workflow

### Starting the Development Server

```bash
# Build and start the server (includes web build)
make serve

# Or run directly after building
./jenkins-flow -port 32567
```

The server will be available at `http://localhost:32567`.

### Frontend Development

For faster frontend iteration without full rebuilds:

```bash
cd web
npm run dev    # Start Vite dev server
npm run build  # Build for production
```

Note: The production build copies `web/dist/*` to `pkg/server/static/` which is embedded in the Go binary.

### API-First Development

This project uses OpenAPI for API design:

1. **Edit API specification**: Modify `api/openapi.yaml`
2. **Regenerate server code**: Run `make generate-api`
3. **Implement handlers**: Update `pkg/server/server.go` to implement the generated interface in `pkg/api/server.gen.go`

The Swagger UI is available at `http://localhost:{PORT}/swagger` when the server is running.

### Configuration Files

- `instances.yaml` - Jenkins instance configurations (gitignored, contains auth tokens)
- `instances.yaml.template` - Template for instances configuration
- `workflows/*.yaml` - Workflow definitions
- `examples/*.yaml` - Example workflow files

## Testing Instructions

```bash
# Run all tests
make test

# Or directly with Go
go test -v ./...

# Run tests for a specific package
go test -v ./pkg/config
go test -v ./pkg/server
go test -v ./pkg/workflow
```

**Test file locations:**
- `pkg/config/config_test.go` - Configuration loading and validation tests
- `pkg/github/client_test.go` - GitHub client tests
- `pkg/server/server_test.go` - HTTP server tests
- `pkg/server/state_test.go` - State management tests
- `pkg/workflow/engine_test.go` - Workflow engine tests

**Test data:** Located in `pkg/config/testdata/` with various workflow and instance YAML files.

## Code Style Guidelines

### Go Conventions

- **Formatting**: Use `go fmt` (run via `make fmt`)
- **Linting**: Use `go vet` (run via `make vet`)
- **Combined check**: Run `make lint` for both fmt and vet
- **Package structure**: Follow the standard Go project layout
  - `cmd/` - Application entry points
  - `pkg/` - Library code meant to be reusable
  - `internal/` would be for private application code (if needed)

### File Organization

- Each package should have its own directory under `pkg/`
- Test files should be named `*_test.go` and placed alongside the code they test
- Test data goes in `testdata/` subdirectories
- Static web assets are embedded from `pkg/server/static/`

### Import Patterns

- Group imports: stdlib, external packages, then internal packages
- Use standard library when possible
- Key dependencies:
  - `gopkg.in/yaml.v3` - YAML parsing
  - `github.com/go-chi/chi/v5` - HTTP routing
  - `github.com/oapi-codegen/runtime` - OpenAPI code generation

## Build and Deployment

### Build Process

```bash
# Full build (frontend + backend)
make build

# Frontend only
make build-web

# Backend only (without frontend rebuild)
go build -o jenkins-flow cmd/jenkins-flow/main.go
```

**Build outputs:**
- Binary: `jenkins-flow` (in root directory)
- Frontend assets: `pkg/server/static/` (embedded in binary)

### Clean Build

```bash
make clean  # Removes binary and static assets
```

### Running the Binary

```bash
./jenkins-flow              # Runs on default port
./jenkins-flow -port 8080   # Specify custom port
```

## Development Tips

### Quick Package Navigation

- Main entry point: `cmd/jenkins-flow/main.go`
- Server implementation: `pkg/server/server.go`
- Workflow engine: `pkg/workflow/engine.go`
- Config parsing: `pkg/config/config.go`
- Jenkins client: `pkg/jenkins/client.go`
- GitHub client: `pkg/github/client.go`
- Vue components: `web/src/components/`

### Workflow File Format

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

### Common Commands Summary

```bash
make help        # Show all available make targets
make deps        # Download dependencies
make build       # Full build
make serve       # Build and run server
make test        # Run tests
make lint        # Format and vet code
make clean       # Clean build artifacts
make build-web   # Build frontend only
make generate-api # Regenerate API code from OpenAPI spec
make stop-server # Stop running server process
```

## Debugging and Troubleshooting

### Common Issues

1. **Build fails after API changes**: Run `make generate-api` to regenerate server stubs
2. **Frontend not updating**: Run `make build-web` to rebuild static assets
3. **Server won't start**: Check if port is already in use with `lsof -i :32567`
4. **Stop hanging server**: Use `make stop-server` to kill the process

### Logging

- The project uses a custom logger in `pkg/logger/logger.go`
- HTTP transport logging available in `pkg/logger/transport.go`

### Environment Variables

When using `auth_env` in `instances.yaml`:
```bash
export JENKINS_AUTH_US="username:token"
export GITHUB_TOKEN="ghp_xxxxxxxxxxxx"
```

## Project Structure Notes

- **Monolithic binary**: The web UI is embedded in the Go binary
- **Static assets**: Automatically copied from `web/dist/` to `pkg/server/static/` during build
- **API generation**: Server code in `pkg/api/server.gen.go` is generated, don't edit directly
- **State management**: In-memory state for workflow execution in `pkg/server/state.go`

## External Dependencies

- **terminal-notifier**: macOS notifications (install via `brew install terminal-notifier`)
- **Go 1.25.4+**: Required for building (as specified in go.mod)
- **Node.js**: Required for frontend development
- **oapi-codegen**: Auto-installed by `make generate-api`

## Security Notes

- Never commit `instances.yaml` (contains auth tokens)
- Use `auth_env` for environment-based authentication
- Jenkins API tokens are persistent and tied to user accounts
- Keep Slack webhook URLs secure
