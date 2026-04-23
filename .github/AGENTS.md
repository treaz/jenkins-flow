# AGENTS.md

## Build & Test

### Setup

```bash
make deps              # Install Go dependencies
cd web && npm install   # Install frontend dependencies
```

### Build

```bash
make build       # Full build (frontend + backend)
make build-web   # Build frontend only
make clean       # Remove binary and static assets
```

### Run

```bash
make serve                 # Build and start the server
./jenkins-flow             # Run on default port
./jenkins-flow -port 8080  # Specify custom port
make stop-server           # Stop running server process
```

### Test

```bash
make test                  # Run all tests
go test -v ./...           # Run all tests directly
go test -v ./pkg/config    # Test a specific package
```

### Formatting & Linting

- `make fmt` — run `go fmt`
- `make vet` — run `go vet`
- `make lint` — both fmt and vet

### Code Generation

API spec: `api/openapi.yaml`
Generated code: `pkg/api/server.gen.go` — do not edit directly

```bash
make generate-api  # Regenerate server code from OpenAPI spec
```

After editing `api/openapi.yaml`:
1. Run `make generate-api`
2. Update `pkg/server/server.go` to implement the generated interface

### Frontend Development

```bash
cd web
npm run dev    # Start Vite dev server
npm run build  # Build for production
```

Production build copies `web/dist/*` to `pkg/server/static/` (embedded in Go binary).

### Troubleshooting

- **Build fails after API changes**: Run `make generate-api`
- **Frontend not updating**: Run `make build-web`
- **Server won't start**: Check port with `lsof -i :32567`
- **Stop hanging server**: `make stop-server`

## Architecture

See `docs/architecture.md` for system design, component descriptions, and data flow.
See `docs/adr/` for architecture decision records.

Read these before:
- Adding a new service or package
- Changing data flow between services
- Modifying the API spec or domain model
- Altering core business logic

## Key Conventions

- **Package structure**: `cmd/` for entry points, `pkg/` for library code
- **Test files**: `*_test.go` alongside the code they test; test data in `testdata/` subdirectories
- **Import groups**: stdlib, external packages, then internal packages
- **Config files**: `instances.yaml` (gitignored, auth tokens), `workflows/*.yaml` (workflow definitions)
- **Security**: Never commit `instances.yaml`; use `auth_env` for environment-based auth
- **External deps**: Go 1.25.4+, Node.js, terminal-notifier (`brew install terminal-notifier`)

## When in doubt

- Read `docs/architecture.md` before modifying service boundaries or data flow
- Read the relevant ADR in `docs/adr/` before making architectural changes
- Run `make test` before committing
- If build fails after API changes, run `make generate-api`
- Ask rather than guess about logic that is not deducible from code

## Self-maintenance

When you discover something non-obvious about this codebase — a hidden dependency, a surprising behavior, a constraint that isn't documented — update the relevant file:
- Build commands, conventions, pointers → this file (`AGENTS.md`)
- System design, component descriptions, data flow → `docs/architecture.md`
- Significant architectural decisions → new ADR in `docs/adr/`

Keep entries concise.
