# 0001. Project layout and toolchain

- Status: accepted
- Date: 2026-10-30

## Context

The project plan (§11) proposed `cmd/shellvane` for the desktop entry point.
Wails v3 tooling (Taskfiles in `build/`, `wails3 dev`, binding generation)
assumes the GUI `main` package lives at the module root and embeds
`frontend/dist`. Go's `embed` cannot reach parent directories.

## Decision

- The desktop app's `main.go` lives at the repository root; the CLI lives in
  `cmd/shellvane-cli`. Business logic stays in `core/`, the Wails-facing service
  layer in `app/`.
- Toolchain pinned: Go 1.26 (required by Wails v3.0.0), Wails v3.0.0,
  Angular 22, Node.js 24 LTS (Angular 22 requires >= 22.22.3 or >= 24.15).
- On Linux, Wails v3.0.0 targets GTK4 + WebKitGTK 6.0 by default; GTK3 remains
  available through a build tag and is part of the spike's test matrix.
- Generated bindings (`frontend/bindings`) are committed so the frontend builds
  and tests without a Go toolchain; `wails3 build` regenerates them.

## Consequences

The unmodified Wails Taskfiles work for build, dev mode and packaging on all
three platforms. `go vet ./...` at the root requires a built frontend, so CI
vets `./core/... ./app/... ./cmd/...` and builds the root package separately.
