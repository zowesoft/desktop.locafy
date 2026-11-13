# FEATURES.md — desktop.locafy

Feature tracker per PRD section 21. Statuses: `NOT_STARTED` · `READY` · `IN_PROGRESS` · `BLOCKED` · `IN_REVIEW` · `TESTING` · `COMPLETED`.

Rules: a feature is never marked `COMPLETED` without evidence that its acceptance criteria pass; blockers are documented, never bypassed.

## Summary

| ID | Feature | Repository | Phase | Status | Dependencies |
|---|---|---|---|---|---|
| LOC-FOUND-001 | Repository and engineering standards (shared) | All | 0 | NOT_STARTED | — |
| DESK-BOOT-001 | Wails desktop bootstrap | desktop.locafy | 1 | NOT_STARTED | LOC-FOUND-001 |
| DESK-DB-001 | SQLite persistence and migrations | desktop.locafy | 1 | NOT_STARTED | LOC-FOUND-001 |
| DESK-UI-001 | Navigation and design system | desktop.locafy | 1 | NOT_STARTED | DESK-BOOT-001 |
| DESK-PROJ-001 | Create and import projects | desktop.locafy | 2 | NOT_STARTED | DESK-BOOT-001, DESK-DB-001, DESK-UI-001 |
| DESK-PROJ-002 | Project overview and lifecycle | desktop.locafy | 2 | NOT_STARTED | DESK-PROJ-001 |
| DESK-APP-001 | Add and detect applications | desktop.locafy | 3 | NOT_STARTED | DESK-PROJ-002 |
| DESK-APP-002 | Application process lifecycle | desktop.locafy | 3 | NOT_STARTED | DESK-APP-001 |
| DESK-RUN-001 | Runtime detection and assignment | desktop.locafy | 4 | NOT_STARTED | DESK-APP-002 |
| DESK-SVC-001 | Service catalogue and lifecycle | desktop.locafy | 5 | NOT_STARTED | DESK-APP-002 |
| DESK-NET-001 | Local domains and reverse proxy | desktop.locafy | 6 | NOT_STARTED | DESK-APP-002, DESK-SVC-001 |
| DESK-DEVX-001 | Integrated terminal and logs | desktop.locafy | 7 | NOT_STARTED | DESK-PROJ-002, DESK-APP-002, DESK-SVC-001 |
| TUN-PROTO-001 | Tunnel threat model and protocol (shared) | Gateway · Desktop · API | 9 | NOT_STARTED | API-AUTH-001, API-DEVICE-001, DESK-NET-001 |
| TUN-AGENT-001 | Desktop tunnel agent | desktop.locafy | 9 | NOT_STARTED | TUN-PROTO-001 |
| TUN-E2E-001 | End-to-end public tunnel flow (shared) | Gateway · Desktop · API | 9 | NOT_STARTED | TUN-PROTO-001, TUN-AGENT-001, TUN-GATE-001 |

---

## Entries

### LOC-FOUND-001 — Repository and engineering standards (shared, all repositories)

- **Repository:** All — desktop.locafy, web.locafy, api.locafy, gateway.locafy. This ID is tracked once per repository; acceptance criteria are specialised per repository, and the feature is only complete for a given repository when that repository's criteria pass.
- **Description:** Establish repository foundation: buildable/testable repo, documentation structure, formatting/linting/testing/CI/release conventions, verified toolchain versions, initial architecture decisions recorded.
- **Dependencies:** —
- **Acceptance criteria:** Repo can be built and tested independently; boundaries documented; CI builds and tests on Windows, macOS, and Linux; initial architecture and contracts reviewed; no unresolved foundational decision hidden in implementation assumptions (PRD section 20, Phase 0 exit criteria).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** Repository root, CI configuration, `docs/`
- **Tests:** CI pipeline runs build + tests
- **Blockers:** — (Verified toolchain recorded in ADR-0002 — Wails v3.0.0, Go 1.27.1, Node 22.14.0; desktop platform support decided in ADR-0001.)
- **Evidence:** desktop.locafy builds independently — `wails3 build` compiles the Go binary and frontend, producing `bin/locafy.exe`. Build/test matrix for web.locafy, api.locafy, and gateway.locafy still pending.

### DESK-BOOT-001 — Wails desktop bootstrap

- **Repository:** desktop.locafy
- **Description:** Wails v3 application bootstrap; Go backend ↔ React/TypeScript frontend integration; settings and structured logging; native OS information and basic monitoring; packaging/development workflow.
- **Dependencies:** LOC-FOUND-001
- **Acceptance criteria:** Desktop app launches reliably on Windows, macOS, and Linux; frontend can invoke approved Go backend operations; local state persists across restarts; logging and error reporting work (PRD section 20, Phase 1 exit criteria).
- **Status:** IN_PROGRESS
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Launch/reliability test; backend operation invocation tests
- **Blockers:** Cross-platform launch validation requires CI (ADR-0001); Windows build verified locally
- **Evidence:** `wails3 build` on Windows produces `bin/locafy.exe` (v3.0.0, Go 1.27.1); TypeScript bindings generated from Go services into `frontend/bindings/`.

### DESK-DB-001 — SQLite persistence and migrations

- **Repository:** desktop.locafy
- **Description:** SQLite database with safe, documented migrations as the desktop's local persistence layer.
- **Dependencies:** LOC-FOUND-001
- **Acceptance criteria:** Local state persists across restarts; migrations run safely and repeatably; upgrade/migration path documented (PRD section 18.3).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Migration tests (fresh install, upgrade, rollback)
- **Blockers:** —
- **Evidence:** —

### DESK-UI-001 — Navigation and design system

- **Repository:** desktop.locafy
- **Description:** Main navigation (Overview, Projects, Applications, Services, Runtimes, Domains, Tunnels as planned/disabled, Logs, Terminal, Settings) and the Locafy design system on shadcn/ui + Tailwind with PRD section 17.2 tokens.
- **Dependencies:** DESK-BOOT-001
- **Acceptance criteria:** Navigation matches PRD section 6.2; design tokens per section 17.2; project view is the primary working area; global vs project-scoped settings distinguishable; status never by color alone.
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** UI component/snapshot tests
- **Blockers:** —
- **Evidence:** —

### DESK-PROJ-001 — Create and import projects

- **Repository:** desktop.locafy
- **Description:** Create and import projects; project listing, metadata, configuration, and persistence; renaming a project; changing a project's root path with validation; safe project removal.
- **Dependencies:** DESK-BOOT-001, DESK-DB-001, DESK-UI-001
- **Acceptance criteria:** Users can create/import and manage projects (create, import, rename, change root path, remove from Locafy); changing the root path validates the selected directory, reports conflicts, and never moves, copies, or deletes files; project data persists; project boundaries are enforced; removing a project from Locafy does not delete its source files (PRD sections 7.2, 20).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Project CRUD + boundary enforcement tests; root-path change validation tests (missing directory, conflicting project, unreadable path); removal-does-not-delete-sources test
- **Blockers:** —
- **Evidence:** —

### DESK-PROJ-002 — Project overview and lifecycle

- **Repository:** desktop.locafy
- **Description:** Project overview screen (PRD section 17.3) and start/stop/restart orchestration foundation; project-scoped terminal/logging context foundations.
- **Dependencies:** DESK-PROJ-001
- **Acceptance criteria:** Lifecycle follows PRD section 7.3 order (validate → check runtimes → check ports → services → readiness → applications → health checks → report); component-level status is reported, not a misleading single status; partial running states visible; stop order is safe and managed processes are distinguishable from unrelated ones.
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Lifecycle orchestration tests; partial-failure state tests
- **Blockers:** —
- **Evidence:** —

### DESK-APP-001 — Add and detect applications

- **Repository:** desktop.locafy
- **Description:** Add/import applications; framework/language detection from project markers (PRD section 8.3); configurable working directory, commands, runtime assignment model; dependency graph with cycle validation.
- **Dependencies:** DESK-PROJ-002
- **Acceptance criteria:** A project can contain multiple applications; detection is advisory and overridable; detected commands require user confirmation before first execution; dependency graph validated for cycles.
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Detection tests per marker type; dependency-graph validation tests
- **Blockers:** —
- **Evidence:** —

### DESK-APP-002 — Application process lifecycle

- **Repository:** desktop.locafy
- **Description:** Process start/stop/restart; port tracking and conflict reporting; application logs and health state; independent and project-lifecycle-driven operation.
- **Dependencies:** DESK-APP-001
- **Acceptance criteria:** Applications start/stop independently and via project actions; failures are visible and do not corrupt unrelated application state; port conflicts reported without terminating unrelated processes; no orphaned processes (PRD sections 18.3, 20, Phase 3).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Process lifecycle tests; port conflict tests; failure isolation tests
- **Blockers:** Platform-specific process behavior must be documented (PRD section 19)
- **Evidence:** —

### DESK-RUN-001 — Runtime detection and assignment

- **Repository:** desktop.locafy
- **Description:** Detect installed runtimes (PHP, Node.js, Python, Go, Java, Ruby, .NET — final list decided in implementation planning); display versions/locations; per-application selection; install additional versions where feasible; compatibility validation.
- **Dependencies:** DESK-APP-002
- **Acceptance criteria:** Applications use explicitly selected runtime versions; missing versions produce clear errors; existing system runtimes are never silently overwritten; Locafy-managed installs are distinguishable from system installs (PRD sections 9.2, 20, Phase 4).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Runtime detection tests per platform; assignment/validation tests
- **Blockers:** Initial runtime list and per-OS installation strategy undecided (PRD section 23)
- **Evidence:** —

### DESK-SVC-001 — Service catalogue and lifecycle

- **Repository:** desktop.locafy
- **Description:** Service catalogue and project attachment (priority: PostgreSQL → Redis → Mail capture → MySQL/MariaDB → S3-compatible storage); configuration, persistent data directories, start/stop/restart, health checks, logs, connection details; safe removal with data-retention warnings.
- **Dependencies:** DESK-APP-002
- **Acceptance criteria:** A project can run an application with its required managed services; service data persists per documented behavior; service failures reported independently; removing a service configuration does not silently delete data; per-project isolation (PRD sections 10.4, 20, Phase 5).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Service lifecycle tests; isolation tests; data-retention tests
- **Blockers:** Service packaging strategy and object storage engine undecided (PRD section 23)
- **Evidence:** —

### DESK-NET-001 — Local domains and reverse proxy

- **Repository:** desktop.locafy
- **Description:** Reverse-proxy integration (Caddy candidate), local domain assignment and routing, local HTTPS (mkcert candidate), WebSocket proxying, port/domain conflict detection, network diagnostics, validated configuration reloads.
- **Dependencies:** DESK-APP-002, DESK-SVC-001
- **Acceptance criteria:** Multiple applications/projects use distinct local domains; configuration changes are validated before reload; local HTTPS works on all supported platforms (Windows, macOS, Linux); loopback-by-default enforced; port owners visible (PRD sections 11, 20, Phase 6).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Routing/domain conflict tests; certificate workflow tests
- **Blockers:** Reverse proxy and certificate tooling candidates must be validated (PRD section 23)
- **Evidence:** —

### DESK-DEVX-001 — Integrated terminal and logs

- **Repository:** desktop.locafy
- **Description:** Integrated terminal at global/project/application/service scopes (xterm.js + Go PTY); centralized logs with filtering and source attribution; process/CPU/memory/disk monitoring; environment management and secret masking.
- **Dependencies:** DESK-PROJ-002, DESK-APP-002, DESK-SVC-001
- **Acceptance criteria:** Developers can diagnose common local problems without leaving Locafy; metrics and logs have clear scope and source attribution; secrets are not exposed in logs or UI; environment precedence rules documented (PRD sections 12, 13, 20, Phase 7).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Terminal/PTY tests; log attribution tests; secret-masking tests
- **Blockers:** Platform PTY behavior must be documented (PRD section 19)
- **Evidence:** —

### TUN-PROTO-001 — Tunnel threat model and protocol (shared: Gateway/Desktop/API)

- **Repository:** Gateway · Desktop · API — shared. The canonical protocol specification is maintained in `gateway.locafy/docs/architecture/`; this repository records the agent-side obligations it must satisfy.
- **Description:** Threat model and versioned tunnel protocol specification before any gateway implementation (PRD section 16).
- **Dependencies:** API-AUTH-001, API-DEVICE-001 (api.locafy), DESK-NET-001 (this repo)
- **Acceptance criteria:** Protocol and threat model documented; gateway technology decision recorded; security and operational requirements approved (PRD section 16.4 delivery gate items 5–7).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** `docs/architecture/` (protocol spec)
- **Tests:** —
- **Blockers:** Full section 16.4 gate (desktop lifecycle stable, networking established, auth + devices working)
- **Evidence:** —

### TUN-AGENT-001 — Desktop tunnel agent

- **Repository:** desktop.locafy
- **Description:** Desktop tunnel agent establishing authenticated outbound connections to the public gateway; tunnel creation/deletion, visibility and revocation of active exposure.
- **Dependencies:** TUN-PROTO-001
- **Acceptance criteria:** Only authorized devices and configured applications can be exposed; users can inspect and revoke active tunnels; heartbeats/recovery behavior per protocol spec (PRD section 16.2, section 20, Phase 9).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Agent authentication, connection recovery, revocation tests
- **Blockers:** Gateway implementation (gateway.locafy) and protocol spec
- **Evidence:** —

### TUN-E2E-001 — End-to-end public tunnel flow (shared: Gateway/Desktop/API)

- **Repository:** Gateway · Desktop · API — shared. Tracked in each participating repository; the gateway entry coordinates the combined flow.
- **Description:** Validate this repository's side of the complete tunnel flow: authorized device → desktop agent → gateway → public URL → local application, including WebSocket support where in scope. Desktop contributes the agent-side behaviour (device authorization, tunnel start/stop/revoke, heartbeat and reconnect).
- **Dependencies:** TUN-PROTO-001, TUN-AGENT-001, TUN-GATE-001 (gateway.locafy)
- **Acceptance criteria:** An exposed application is reachable through its public URL; revoking a device or tunnel takes effect immediately; disconnection and reconnect behaviour is predictable (PRD sections 20, 24).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Agent-side end-to-end tests; revocation and reconnection scenarios
- **Blockers:** TUN-AGENT-001 and TUN-GATE-001 not implemented; section 16.4 delivery gate not satisfied
- **Evidence:** —
