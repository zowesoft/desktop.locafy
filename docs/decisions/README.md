# Decisions — desktop.locafy

Architecture decision records (ADRs) for this repository.

## Recorded decisions

| ADR | Decision | Status |
|---|---|---|
| [ADR-0001](ADR-0001-desktop-platform-support.md) | Cross-platform desktop support from day one (Windows, macOS, Linux) | Accepted |
| [ADR-0002](ADR-0002-verified-toolchain.md) | Verified toolchain: Wails v3.0.0, Go 1.27.1, Node 22.14.0 | Accepted |
| [ADR-0003](ADR-0003-sqlite-driver.md) | SQLite driver: modernc.org/sqlite (pure Go, CGO-free builds) | Accepted |
| [ADR-0004](ADR-0004-local-data-model.md) | Local data model: SQLite as control-plane DB; project-centric schema standards | Accepted |
| [ADR-0005](ADR-0005-machine-monitoring.md) | Machine monitoring: gopsutil sampled in the background, CGO-free | Accepted |
| [ADR-0006](ADR-0006-light-theme.md) | Light theme alongside the dark default, and the caption-bar limitation it accepts | Accepted |
| [ADR-0007](ADR-0007-project-manifest-and-storage.md) | Managed and linked projects, and the portable project manifest (manifest rules superseded by ADR-0012) | Superseded in part |
| [ADR-0008](ADR-0008-service-engines-and-instances.md) | Service engines are machine-level; instances are shared by default, with isolated per-project instances as the alternative (packaging settled by ADR-0020) | Accepted |
| [ADR-0009](ADR-0009-navigation-and-project-workspace.md) | Global-only sidebar, a projects dashboard, and a project workspace with Applications, Services and Domains as tabs | Accepted |
| [ADR-0010](ADR-0010-applications-and-dependency-graph.md) | Applications are project-scoped records with a confirmed, advisory detection and a cycle-checked dependency graph | Accepted |
| [ADR-0011](ADR-0011-framework-recipes-and-scaffold-jobs.md) | Framework recipes are shipped data, scaffolds run argv-only with version choice, and only successful installs are registered | Accepted |
| [ADR-0012](ADR-0012-opt-in-project-override-file.md) | Project settings live in SQLite; `locafy.project.json` is an opt-in override file (supersedes ADR-0007's manifest rules) | Accepted |
| [ADR-0013](ADR-0013-application-process-supervision.md) | Applications run argv-only from their stored command in their stored folder, one supervised process each, killed with its tree on stop | Accepted |
| [ADR-0014](ADR-0014-application-ports-and-ownership.md) | Application ports are discovered from the live process tree; a held recorded port refuses the start instead of killing its holder | Accepted |
| [ADR-0015](ADR-0015-application-health-checks.md) | Health is a TCP connection attempt on the discovered port: healthy, unhealthy, or unknown | Accepted |
| [ADR-0016](ADR-0016-project-lifecycle-orchestration.md) | Project start/stop/restart run applications in dependency order with readiness waits; the displayed state is derived, never stored | Accepted |
| [ADR-0017](ADR-0017-project-resource-usage.md) | Project usage is the CPU and memory of the process trees Locafy started, sampled on the runtime loop and normalized to the machine scale | Accepted |
| [ADR-0018](ADR-0018-runtime-registry-and-assignment.md) | The runtime registry is data with a shared tools discovery; an application can pin a copy, resolved ahead of PATH at start | Accepted |
| [ADR-0019](ADR-0019-install-jobs-and-managed-runtimes.md) | Installs are portable archives unpacked into a runtimes directory Locafy owns; jobs are rows with progress, outcome and retry; managed copies surface through the registry | Accepted |
| [ADR-0020](ADR-0020-service-packaging-native.md) | Services run as native engine processes Locafy supervises; no container runtime, and only engines with a verified native build per platform enter the catalogue | Accepted |
| — | Desktop stack selections made in the PRD (Wails v3, Go, React/TS, SQLite) are recorded there and considered approved baseline. | — |

## Decisions pending (do not guess — PRD section 23)

| Decision | Resolve in |
|---|---|
| Initial runtime list and supported versions (PRD section 9.1) | Phase 4 |
| Object storage engine | Phase 5 |
| Cloud credential and device-identity storage (joint with `api.locafy` auth design) | Phase 8 |
| Reverse proxy (Caddy candidate) validation | Phase 6 |
| Local certificate tooling (mkcert candidate) validation | Phase 6 |
| Auto-update mechanism | Phase 10 |

## Template

When recording a new ADR, include: number, title, date, status (proposed/accepted/superseded), context, decision, consequences, and affected repositories/features.
