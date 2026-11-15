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
| [ADR-0007](ADR-0007-project-manifest-and-storage.md) | Managed and linked projects, and the portable project manifest | Accepted |
| [ADR-0008](ADR-0008-service-engines-and-instances.md) | Service engines are machine-level; instances are isolated per project by default, with explicit opt-in shared instances (packaging left open) | Proposed |
| — | Desktop stack selections made in the PRD (Wails v3, Go, React/TS, SQLite) are recorded there and considered approved baseline. | — |

## Decisions pending (do not guess — PRD section 23)

| Decision | Resolve in |
|---|---|
| Service packaging and installation strategy | Phases 4–5 |
| Initial runtime list and supported versions (PRD section 9.1) | Phase 4 |
| Object storage engine | Phase 5 |
| Cloud credential and device-identity storage (joint with `api.locafy` auth design) | Phase 8 |
| Reverse proxy (Caddy candidate) validation | Phase 6 |
| Local certificate tooling (mkcert candidate) validation | Phase 6 |
| Auto-update mechanism | Phase 10 |

## Template

When recording a new ADR, include: number, title, date, status (proposed/accepted/superseded), context, decision, consequences, and affected repositories/features.
