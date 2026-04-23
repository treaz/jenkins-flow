# ADR-0001: Native macOS App with Wails v2

## Status

Accepted

## Context

Jenkins Flow was originally a CLI tool that starts an HTTP server and requires users to open a browser to interact with the dashboard. This works but has UX friction: the app feels like a web page rather than a native tool, there is no dock icon, and the user must manage the server process manually.

We wanted to offer a native macOS application experience (proper window, dock icon, menu bar) while reusing the existing Go backend and Vue.js frontend with minimal changes.

## Decision

We adopted [Wails v2](https://wails.io/) to wrap the existing Go + Vue.js application into a native macOS `.app` bundle. The approach uses the "hybrid" integration strategy:

- The existing Chi router is passed to Wails as the `AssetServer.Handler`, which handles all API requests.
- The embedded Vue.js static files are passed as `AssetServer.Assets`, served by Wails' built-in WebKit WebView.
- No localhost HTTP server is needed in the macOS app — Wails routes requests internally.
- The Wails entry point lives at `main.go` (project root), while the CLI entry point remains at `cmd/jenkins-flow/main.go`.
- Both entry points share all backend packages unchanged.

The `pkg/server` package was refactored to expose `BuildRouter()` (returns a configured Chi router) and `StartAsync()` (non-blocking server start), allowing both entry points to use the same router setup.

## Alternatives Considered

**Tauri** — Rust-based framework with web frontend support. Would require running the Go backend as a sidecar process with IPC overhead, adding complexity without clear benefit given our Go codebase.

**Electron** — Chromium-based, widely used. Rejected due to ~150 MB binary size (vs ~13 MB with Wails), Chromium overhead, and the need to manage a Go sidecar process.

**Fyne** — Go-native UI toolkit. Would require a complete rewrite of the Vue.js frontend in Go, discarding all existing UI work.

**go-app** — Go to WebAssembly PWA approach. Incompatible with CGO dependencies (SQLite via `go-sqlite3`), and would require significant frontend rewrite.

## Consequences

### Positive

- Native macOS window with dock icon, menu bar, and standard window chrome
- ~13 MB app bundle using system WebKit (no bundled browser engine)
- Zero changes to Vue.js frontend or Go API handlers
- CLI mode preserved as a separate entry point — no regressions
- Single codebase serves both modes
- SQLite and all CGO dependencies work natively

### Negative

- Wails v2 is an additional build dependency (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- macOS app configuration paths differ from CLI mode (`~/.config/jenkins-flow/` vs current directory)
- Wails expects `main.go` at the project root, which is a non-standard layout for a Go project that already uses `cmd/` directories
