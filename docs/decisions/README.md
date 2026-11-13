# Decisions — desktop.locafy

Architecture decision records (ADRs) for this repository.

## Recorded decisions

| ADR | Decision | Status |
|---|---|---|
| [ADR-0001](ADR-0001-desktop-platform-support.md) | Cross-platform desktop support from day one (Windows, macOS, Linux) | Accepted |
| — | Desktop stack selections made in the PRD (Wails v3, Go, React/TS, SQLite) are recorded there and considered approved baseline. | — |

## Decisions pending (do not guess — PRD section 23)

| Decision | Resolve in |
|---|---|
| Verified Wails v3 / Go / React versions | Phase 0 |
| Service packaging and installation strategy | Phases 4–5 |
| Initial runtime list and supported versions (PRD section 9.1) | Phase 4 |
| Object storage engine | Phase 5 |
| Cloud credential and device-identity storage (joint with `api.locafy` auth design) | Phase 8 |
| Reverse proxy (Caddy candidate) validation | Phase 6 |
| Local certificate tooling (mkcert candidate) validation | Phase 6 |
| Auto-update mechanism | Phase 10 |

## Template

When recording a new ADR, include: number, title, date, status (proposed/accepted/superseded), context, decision, consequences, and affected repositories/features.
