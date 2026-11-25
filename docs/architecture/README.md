# Architecture — desktop.locafy

Initial draft, Phase 0. This document describes the desktop application's architecture and the Wails/Go/React communication patterns. It evolves with the product; changes that alter these patterns require a decision record in `docs/decisions/`.

## Overview

Locafy desktop is a single Wails v3 process. The Go backend is the local engine — it owns all state, side effects, and platform interaction. The React frontend renders views and calls the engine; it holds no authoritative state.

```
┌──────────────────────────────────────────────┐
│ React + TypeScript (frontend/)               │
│ views · TanStack Query (async state) ·       │
│ Zustand (UI state) · xterm.js                │
└──────────────┬───────────────────────────────┘
               │ generated TypeScript bindings (frontend/bindings/)
               │ typed events (app.Event.Emit)
┌──────────────▼───────────────────────────────┐
│ Go engine (internal/)                        │
│ services registered in main.go               │
│ SQLite persistence · process manager ·       │
│ reverse proxy · runtimes · logs              │
└──────────────────────────────────────────────┘
```

## Communication patterns

**Request/response** — the only mechanism for the frontend to call the engine. Go service structs with exported methods are registered in `main.go` via `application.NewService(...)`; Wails generates type-safe TypeScript classes into `frontend/bindings/`. The frontend imports only generated bindings and calls methods as Promises. Generated files are never edited.

**Push events** — engine-to-frontend notifications (state changes, log lines, health updates, progress) use registered typed events (`application.RegisterEvent[T]`) emitted with `app.Event.Emit`. The frontend subscribes; no polling for state that the engine already knows.

**Assets** — the built frontend is embedded into the binary (`go:embed all:frontend/dist`) and served by Wails' asset handler. No external web server.

Rules:

- Domain logic never lives in `main.go`; it wires services and windows only.
- Services expose small, typed methods and return errors as values; the frontend handles and displays them.
- Engine state changes are communicated by events, never by the frontend guessing.

## Engine structure

One Go package per capability area, mirroring the product model:

```
internal/
  db/         SQLite open/migrate (embedded migrations)
  settings/   application settings
  projects/   project model, import, lifecycle orchestration
  applications/  workloads, process lifecycle, ports, health, project orchestration
  runtimes/   runtime detection/selection per platform
  services/   native service engines and their instances (PostgreSQL today)
  network/    local domains, reverse proxy, certificates
  env/        environment configuration and secrets
  logs/       log collection and retrieval
  terminal/   PTY sessions
  monitor/    process and resource monitoring
```

Planned areas appear as their features are built; packages are added when a feature starts, not speculatively.

**Persistence** — one SQLite database for desktop state — the control-plane database described in ADR-0004 — stored in the platform data directory via xdg (`%APPDATA%/Locafy/data/locafy.db` on Windows). Schema changes go through embedded, ordered, repeatable migrations (`internal/db/migrations/`); each feature owns its tables. Project source files and managed service data live outside this database and are referenced, never embedded.

**Logging** — Go `slog` structured logging to rotating files under the user data directory, with secrets masked at the logging boundary. Log scope (machine/project/app/service) is part of the record.

**Process management** — managed applications and services are started and tracked by the engine (PID, lifecycle state, owner project). Only engine-started processes are ever stopped. Platform differences (signals, process groups, PTY) are isolated in per-platform files using build tags (`windows.go`, `darwin.go`, `linux.go`), selected at compile time — consistent with ADR-0001 (cross-platform from day one). CI is configured to build and test on all three platforms; whether those jobs are green is read on the CI service, not from this document.

## Conventions

- Go: standard layout, `internal/` for engine packages, errors wrapped with context, no global mutable state.
- TypeScript: strict mode; bindings are the only API surface; async state via TanStack Query.
- Build orchestration lives in `Taskfile.yml` (see `docs/development/`).
- Comments follow `AGENTS.md`: explain intent, not mechanics; no document references in code.

## Status

- Phase 0: architecture defined (this document); toolchain verified (ADR-0002); platform policy decided (ADR-0001).
- Built on Windows: the SQLite control plane, projects, applications with supervision, ports and health, project lifecycle orchestration, native service instances (PostgreSQL delivered; the native packaging rule it follows is proposed in ADR-0020, with that instance as the evidence), framework scaffolds, the runtime registry, managed installs and the activity feed.
- macOS and Linux compile and pass their tests, and CI is configured for all three platforms; neither has had a manual run.
- Still planned: local domains and the reverse proxy, environment configuration, tunnels and analytics.
