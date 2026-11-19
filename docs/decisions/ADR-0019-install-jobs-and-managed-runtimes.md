# ADR-0019: Install jobs and Locafy-managed runtimes

- **Status:** Accepted
- **Date:** 2026-11-19
- **Depends on:** ADR-0018 (runtime registry and assignment), ADR-0011 (scaffold jobs as the proven job pattern), ADR-0004 (SQLite control plane)

## Context

DESK-RUN-001 delivered the runtime registry as a read-only fact of the
machine: Locafy sees what is installed but never installs anything. The
Downloads view (DESK-DL-001) is where managed components arrive, and the
operator stance is recorded: Locafy-managed runtimes are the primary path,
with the machine's own copies still visible and still usable. The scaffold
already runs long jobs with a row, a log file, live events and a cancel.

## Decision

### Installs are portable archives in a directory Locafy owns

An install job downloads a portable archive (zip or tar.gz, both stdlib)
into `<data home>/Locafy/runtimes/<kind>/<version>/`, verifies it opens and
contains the expected executable, and unpacks it there. No elevation, no
system PATH writes, no uninstaller, no registry entries — removing a managed
runtime is removing a directory. The archive's checksum verification arrives
with the catalogue work that pins versions; until then a successful install
is one whose archive opens and names the expected binary, which the record
states plainly.

### Managed copies surface through the ordinary registry

Discovery gains one more location per kind: the managed runtimes directory.
A copy found there carries the source `managed`, the catalog lists it before
the machine's copies of the same kind, and the runtime picker shows it
marked. Nothing about assignment changes — ADR-0018's resolution applies.

### Jobs are machine-scoped rows with a proven lifecycle

`install_jobs` mirrors `scaffold_jobs`: id, kind, label, the spec as JSON,
state (`queued`, `running`, `succeeded`, `failed`, `cancelled`), progress in
bytes against the archive size, an error message and a log file path. A job
runs on a goroutine that writes its log as it goes and emits the row as a
`downloads:jobs` event, the same shape the scaffold manager uses. Retry
creates a fresh job from the failed spec, so history keeps every attempt;
Cancel interrupts a running download and marks it cancelled. Removal takes a
finished row out of the list without touching the installed files it
produced.

### The first catalogue is small and data-driven

The installable catalogue is data: kind, version, per-OS archive URL,
archive format, the path inside the archive that must exist, and the root
folders to strip. The first entry is Node.js (the zip for Windows, the
tar.gz for macOS and Linux, pinned versions of the official distribution).
Adding PHP, Python or Go is a catalogue entry, not code.

## Consequences

- The Downloads screen lists in-progress and finished jobs with progress,
  outcome and retry, machine-scoped, working without an account.
- After an install, the runtime appears in the Runtimes screen as `managed`
  without any registration step, and the picker prefers it.
- A machine's own runtimes remain first-class; Locafy prefers its managed
  copies but never hides or refuses the machine's.
- Checksum-pinned catalogue entries are a follow-up; the job record keeps
  the archive it downloaded only long enough to unpack it.
