# ADR-0005: Machine monitoring through gopsutil

- **Status:** Accepted
- **Date:** 2026-11-14

## Context

The top bar is meant to report whether the machine is keeping up, not only
whether the engine process is alive (PRD section 6.3). That needs CPU, memory
and disk readings on Windows, macOS and Linux from a single binary.

The readings must be available to the webview without blocking it, and the
engine must stay CGO-free so builds work without a C toolchain on any of the
three platforms.

## Decision

Use `github.com/shirou/gopsutil/v4` for CPU, memory and disk readings, sampled
in a background goroutine every two seconds in `internal/monitor`. The status
service reads the cached sample, so the frontend never waits on a measurement.

Disk usage describes the volume holding the engine's data directory, since
that is the volume Locafy fills.

## Alternatives considered

- **Per-platform syscalls behind build tags.** Avoids a dependency, but means
  three implementations and three sets of tests, and only the Windows path
  would be exercised in current development. gopsutil's platform handling is
  tested upstream on all three per its own CI — not measured here, where the
  Windows path is the only one that has run.
- **`golang.org/x/sys` directly on Windows only.** Would leave macOS and Linux
  without monitoring, which contradicts the cross-platform commitment in
  ADR-0001.

## Consequences

- The module gains a dependency tree (gopsutil, pure Go; `go-ole` and WMI on
  Windows). It adds to binary size and to the set of dependencies to track for
  updates.
- Monitoring stays CGO-free, so cross-compilation keeps working.
- Readings are samples, not a metrics pipeline: values older than two seconds
  are never served, and no history is retained. Anything requiring history
  needs a store, not a bigger sampler.
- A resource that cannot be read leaves its field zero instead of failing the
  whole status request, so partial readings remain useful.
