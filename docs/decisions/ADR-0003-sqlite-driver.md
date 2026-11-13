# ADR-0003: SQLite driver — modernc.org/sqlite (pure Go)

- **Date:** 2026-11-13
- **Status:** Accepted
- **Depends on:** ADR-0001 (cross-platform support from day one)

## Context

The product database is SQLite (approved in the PRD). Two established Go drivers exist:

- `github.com/mattn/go-sqlite3` — the de facto standard, but requires CGO. Every build platform then needs a working C toolchain, and cross-compilation becomes substantially harder (a C cross-compiler per target OS/arch).
- `modernc.org/sqlite` — a pure-Go translation of SQLite. No C toolchain, cross-compilation with plain `go build`, identical `database/sql` usage.

ADR-0001 commits to building and testing on Windows, macOS, and Linux from day one, with CI on all three.

## Decision

Use `modernc.org/sqlite` behind `database/sql` as the SQLite driver. No CGO anywhere in the desktop engine.

## Consequences

- Builds for all supported platforms need only a Go toolchain; CI images stay simple.
- Cross-compilation for release packaging remains plain `go build`.
- The pure-Go driver is measurably slower than mattn's in some workloads; for Locafy's state (configuration, metadata, logs index) this is immaterial — local throughput is bounded by the OS, not the driver.
- The dependency is large (it embeds translated SQLite source); acceptable in exchange for the toolchain simplicity.
- Feature parity caveat: some compile-time SQLite options differ from the C build; none used by Locafy are affected. Revisit only if a specific SQLite extension is ever required (record as a new decision then).
