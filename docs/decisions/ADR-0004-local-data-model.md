# ADR-0004: Local data model — SQLite as the control-plane database

- **Date:** 2026-11-13
- **Status:** Accepted
- **Depends on:** ADR-0001 (cross-platform), ADR-0003 (SQLite driver)

## Context

The engine needs a concrete data model for desktop state. A reviewed design proposal (2026-11-13) was evaluated against the PRD's project-first model, the safety invariants in `AGENTS.md`, and the architecture in `docs/architecture/`.

## Decision

**SQLite is Locafy's local control-plane database: it stores configuration, definitions, relationships, and process state — not machine output.** The project is the central entity; applications, services, domains, environment configuration, dependencies, and process records belong to a project.

### Database location

One database per Locafy installation, resolved with `adrg/xdg` (already a Wails dependency) so platform conventions are honored:

```
Windows: %APPDATA%/Locafy/data/locafy.db
macOS:   ~/Library/Application Support/Locafy/data/locafy.db
Linux:   $XDG_DATA_HOME/locafy/data/locafy.db (~/.local/share/locafy/data/locafy.db)
```

The database is independent of user project directories. Deleting a project from Locafy never touches the project's files.

### Schema standards

- Primary keys: UUID text columns.
- `status` (configured state: active/disabled/archived) and `lifecycle_state` (runtime state: stopped/starting/running/stopping/error) are separate columns — an active project can be stopped.
- Projects use soft deletion (`deleted_at`); every project-scoped table references `project_id` with `ON DELETE CASCADE`.
- Flexible settings live in JSON columns (`configuration`), never as core relationships.
- No framework-specific columns (e.g. `artisan_command`, `npm_script`); framework behavior belongs to detection/adapters and configuration.
- Runtimes are machine-level records assigned to applications (`runtime_id`, `ON DELETE SET NULL`), never owned by them.
- Domains target exactly one of application or service, enforced by a `CHECK` constraint.
- Environment variables may hold either a `value` or an OS-credential-store `secret_reference` (`is_secret` flag) — plaintext secrets are never stored in SQLite.
- Dependencies (`resource_dependencies`) are polymorphic (`dependent_type`/`dependent_id`); referential integrity for these is enforced at the service layer together with graph and cycle validation.
- Text-valued state columns carry `CHECK` constraints; the service layer is a second guard, not the only one.
- `updated_at` is maintained by triggers where applicable.

### Schema ownership by feature

Each feature owns its tables and migration, keeping migrations small and reviewable:

| Feature | Tables |
|---|---|
| DESK-DB-002 | `machine`, `settings`, `activity_logs` (`project_settings` moves to DESK-PROJ-001, which owns the `projects` FK) |
| DESK-PROJ-001/002 | `projects` |
| DESK-APP-001, DESK-RUN-001 | `applications`, `runtimes` |
| DESK-SVC-001 | `services`, `service_templates` |
| DESK-NET-001 | `domains`, `port_allocations`, `certificates` |
| DESK-APP-002, DESK-DEVX-001 | `environment_variables`, `resource_dependencies`, `processes` |
| Phase 9 | `tunnels*` (only after the protocol spec; gateway connection state and traffic metrics stay out of SQLite) |

### Explicitly excluded from SQLite

Application output logs, terminal history, and high-volume metrics (rolling log files / metrics storage instead); user project databases and source code (they belong to the user's directories); tunnel traffic (gateway infrastructure). `activity_logs` records engine actions ("Project created", "PostgreSQL started"), not output.

### Deferred companion decision: project manifest

SQLite is the local operational database only. A versioned, portable **project manifest** (project configuration that can be backed up, shared, and recreated on another machine) is planned as a separate decision and feature — designed before project creation is implemented. Until that decision is recorded, project creation implementation is blocked on it (tracked in `docs/FEATURES.md`).

## Consequences

- The schema grows with features instead of a big-bang migration; every table above arrives with the feature that needs it.
- Service modules own all SQL; the UI never queries the database directly.
- Backup story is simple: one file plus OS credential-store entries.
- The polymorphic dependency table trades database-level FK integrity for flexibility; service-layer validation is mandatory, and its tests must cover cycle detection.
- Existing `internal/db.DefaultPath()` moves to the xdg-resolved `data/` subdirectory before any released data exists, so no data migration is needed.
