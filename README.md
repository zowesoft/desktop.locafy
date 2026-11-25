# desktop.locafy

The Locafy desktop application — a local-first, project-first development environment control plane built with **Wails v3, Go, React, and TypeScript**.

Locafy turns a developer's computer into a manageable local development environment: projects contain applications and the services they depend on, with local domains, environment configuration, logs, terminals, monitoring, and (in a later phase) secure public tunnels.

> **Projects are environments. Applications are workloads. Services are infrastructure.**

## Status

🚧 In active development. On Windows it builds and runs with the SQLite control plane, projects, applications with process supervision, ports and health, native service instances (PostgreSQL today, following the packaging approach ADR-0020 proposes), framework scaffolds, the runtime registry and managed installs, plus project lifecycle orchestration and the activity feed. macOS and Linux are compile-and-test only: CI is configured for all three and no manual run has been done there. Delivery follows the phased plan in the PRD (section 20); see `docs/FEATURES.md` for the feature tracker.

## Documentation

| Path | Contents |
|---|---|
| `AGENTS.md` | Rules and boundaries for AI coding agents |
| `docs/architecture/` | Architecture and technical design |
| `docs/features/` | Feature specifications and acceptance criteria |
| `docs/development/` | Local setup, testing, build, and release procedures |
| `docs/decisions/` | Architecture decision records |
| `docs/FEATURES.md` | Feature tracker |

## Development

Verified toolchain versions and the local development workflow are documented in [`docs/development/README.md`](docs/development/README.md); the verification evidence is recorded in [`docs/decisions/ADR-0002-verified-toolchain.md`](docs/decisions/ADR-0002-verified-toolchain.md).
