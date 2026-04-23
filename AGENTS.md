# AGENTS.md

## Environment

- **Go**: 1.25.4 (per `go.mod`)
- **Node.js**: required for the Vue 3 / Vite frontend in `web/`
- **Wails CLI**: v2.12.0 — `make wails-install` installs it into `$(go env GOPATH)/bin`
- **terminal-notifier** (macOS notifications): `brew install terminal-notifier`

Module: `github.com/treaz/jenkins-flow`. Binary entry point: `cmd/jenkins-flow/main.go`.

## Build & Test

Use `make` targets — they encode the cross-cutting steps (frontend build, static asset copy, code generation).

### Setup

```bash
make deps               # go mod tidy + download
cd web && npm install   # frontend deps
```

### Build

```bash
make build       # full build: web bundle + Go binary -> ./jenkins-flow
make build-web   # frontend only; copies web/dist/* into pkg/server/static/
make clean       # remove binary and embedded static assets
```

### Run

```bash
make serve                 # build then start the server
./jenkins-flow             # default port
./jenkins-flow -port 8080  # custom port
make stop-server           # kill running jenkins-flow process
make mock-jenkins          # local mock Jenkins server on :9090 for smoke tests
```

### Wails desktop app (macOS)

```bash
make wails-dev    # hot-reload dev mode
make wails-build  # build .app bundle to build/bin/
```

### Test

```bash
make test                          # go test -v ./...
go test -v ./pkg/config            # single package
go test -run TestName ./pkg/...    # single test
```

### Format & Lint

```bash
make fmt    # go fmt ./...
make vet    # go vet ./...
make lint   # both
```

### Code Generation (OpenAPI)

API spec: `api/openapi.yaml`. Generated code: `pkg/api/server.gen.go` — **do not edit directly**.

```bash
make generate-api
```

After editing `api/openapi.yaml`:
1. `make generate-api`
2. Update `pkg/server/server.go` to implement the regenerated interface

### Wails frontend bindings

Wails generates Go↔JS bindings into `web/src/wailsjs/` — do not hand-edit these files.

### Troubleshooting

- **Build fails after API changes** → `make generate-api`
- **Frontend not updating** → `make build-web`
- **Server won't start** → check port: `lsof -i :32567`
- **Stop hanging server** → `make stop-server`

## Workflow YAML

### Variable substitution

Two flavors of `${...}` references resolve in step `params`:

- **Top-level inputs** — `${git_branch_to_deploy}` reads from the workflow's `inputs:` map.
- **Upstream step outputs** — `${steps.<id>.<field>}` reads outputs captured from earlier steps.

Each step has an ID derived from its `name:` (lowercased, non-alphanumeric runs collapsed to `_`), or set explicitly via `id:`. Available fields after a Jenkins step succeeds:

| Field          | Value                                              |
| -------------- | -------------------------------------------------- |
| `build_number` | The Jenkins build number (e.g. `7777`)             |
| `build_url`    | The full Jenkins build URL                         |

Outputs from a parallel group's siblings are not visible to each other — siblings only see outputs from steps that completed before the group started. Duplicate resolved IDs cause a validation error at load time; resolve by adding an explicit `id:` to one of the colliding steps.

Example:

```yaml
- name: "Build NOS Docker Image"
  id: build_nos                   # optional; defaults to slug(name)
  instance: qa-global
  job: "/job/nos-php-docker-image"
- parallel:
    steps:
      - name: Deploy NOS US
        instance: qa-ore
        job: /job/NOS_PHP_ALL_Deploy/
        params:
          tag: ${steps.build_nos.build_number}
```

## Architecture

See `docs/architecture.md` for system design, component descriptions, and data flow.
See `docs/adr/` for architecture decision records.

Read these before:
- Adding a new package under `pkg/`
- Changing data flow between server, workflow engine, and Jenkins client
- Modifying the OpenAPI spec or domain model
- Altering core business logic in `pkg/workflow/` or `pkg/server/`

## Key Conventions

- **Package layout**: `cmd/` for entry points (`jenkins-flow`, `mock-jenkins`); `pkg/` for library code (`api`, `config`, `database`, `github`, `jenkins`, `logger`, `notifier`, `server`, `settings`, `workflow`); `api/` for OpenAPI spec; `web/` for Vue frontend; `workflows/` and `examples/` for workflow YAML definitions.
- **Test files**: `*_test.go` alongside the code they test; test fixtures in `testdata/` subdirectories.
- **Import groups**: stdlib, external packages, internal (`github.com/treaz/jenkins-flow/...`) — separated by blank lines.
- **Generated code**: `pkg/api/server.gen.go` (oapi-codegen) and `web/src/wailsjs/**` (Wails) are generated — never edit by hand.
- **Embedded assets**: the Go binary embeds `pkg/server/static/`, populated by `make build-web` from `web/dist/`.
- **Config files**: `instances.yaml` is gitignored and holds Jenkins auth tokens — use `instances.yaml.template` as a starting point. Workflow definitions live in `workflows/*.yaml`.
- **Modern Go**: target the version in `go.mod` (1.25.4); use generics, `any`, `errors.Is/As`, `slog`, range-over-func where appropriate.

## Boundaries

### Always
- Run `make lint` (fmt + vet) before committing
- Run `make test` before pushing
- Read `docs/architecture.md` and relevant ADRs before changing service boundaries or data flow
- Regenerate API code via `make generate-api` after editing `api/openapi.yaml`
- Ask rather than guess about logic that is not deducible from code

### Ask first
- Adding or upgrading Go modules or npm packages
- Creating new packages under `pkg/`
- Modifying `api/openapi.yaml` (changes the public API surface and forces regeneration)
- Altering CI/CD workflows in `.github/`

### Never
- Edit generated files: `pkg/api/server.gen.go`, `web/src/wailsjs/**`
- Commit `instances.yaml`, secrets, or auth tokens
- Commit the built binaries (`jenkins-flow`, `jenkins-flow-app`) or `web/node_modules/`
- Invoke `wails` directly when a `make` target exists — use `make wails-dev` / `make wails-build`

## Security

- Never commit `instances.yaml` — it contains Jenkins API tokens. Use `auth_env` to pull tokens from environment variables instead of inlining them.
- Never commit secrets, API keys, tokens, or `.env` files.
- Never log sensitive data (auth tokens, full request bodies that may contain credentials).
- Treat anything matching `*.env`, `*credentials*`, `*secret*`, `instances.yaml` as sensitive — do not stage or commit.

## Tech Stack

- **Language**: Go 1.25.4
- **HTTP router**: chi v5
- **Persistence**: SQLite (`mattn/go-sqlite3`) with `golang-migrate/migrate` for schema migrations
- **API**: OpenAPI 3 (`api/openapi.yaml`) with `oapi-codegen` for server generation; `kin-openapi` for spec parsing
- **Desktop shell**: Wails v2.12.0 (macOS app bundle)
- **Frontend**: Vue 3 + Vite 8, plain JavaScript (no TypeScript)
- **Concurrency**: `golang.org/x/sync`
- **Config**: YAML (`gopkg.in/yaml.v3`)
- **Notifications**: `terminal-notifier` (macOS) via the `notifier` package

## Self-maintenance

When you discover something non-obvious about this codebase — a hidden dependency, a surprising behavior, a constraint that isn't documented — update the relevant file:
- Build commands, conventions, pointers → this file (`AGENTS.md`)
- System design, component descriptions, data flow → `docs/architecture.md`
- Significant architectural decisions → new ADR in `docs/adr/`
- After any change that affects setup, commands, file paths, or developer workflow → verify `README.md` is still accurate and update it if needed

Keep entries concise.
