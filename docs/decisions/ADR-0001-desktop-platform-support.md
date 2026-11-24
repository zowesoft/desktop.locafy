# ADR-0001: Cross-platform desktop support from day one

- **Date:** 2026-11-13
- **Status:** Accepted (product decision)
- **Supersedes:** The PRD's original position that the first supported operating system would be selected in Phase 0.

## Context

The PRD initially allowed selecting a single first supported operating system (with Windows as a practical development target) during Phase 0 planning. Wails v3 is inherently cross-platform, and deferring platform support would leave platform-specific behavior — process lifecycle and signals, PTY, runtime installation, service management, paths and permissions, local DNS resolution, certificate trust stores, and packaging — as unreviewed risk accumulating until a later phase.

## Decision

Locafy desktop supports Windows, macOS, and Linux from the first release. Every build targets all three platforms. Platform-specific behavior is implemented and tested per platform as each feature is built. CI is configured to build and test on all three platforms (`.github/workflows/ci.yml`, matrix `windows-latest`, `macos-latest`, `ubuntu-latest`); whether those jobs are green is read on the CI service, not from this repository, so this record claims configuration rather than a passing run.

## Consequences

- A feature is not complete unless it works on all supported platforms; single-platform implementations are incomplete, not shippable.
- CI must run build/test jobs on Windows, macOS, and Linux from the first development phase (part of LOC-FOUND-001).
- Platform abstraction is part of the initial architecture design; per-platform code paths are isolated and documented rather than scattered.
- Development effort per feature is higher than a single-OS approach; the trade-off is accepted to avoid a costly retrofit later.
- The PRD (sections 19, 23, 24) has been updated to match this decision.

## Affected features

LOC-FOUND-001 (CI matrix), DESK-BOOT-001 (launch reliability on all platforms), DESK-RUN-001 (per-OS runtime installation), DESK-SVC-001 (per-OS service management), DESK-NET-001 (per-OS certificate trust), DESK-DEVX-001 (per-OS PTY behavior).
