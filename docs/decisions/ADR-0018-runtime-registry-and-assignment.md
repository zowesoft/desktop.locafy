# ADR-0018: Runtime registry and assignment

- **Status:** Accepted
- **Date:** 2026-11-18
- **Depends on:** ADR-0002 (verified toolchain), ADR-0013 (application process supervision), ADR-0011 (framework recipes and scaffold jobs)
- **Affects:** DESK-RUN-001, DESK-DL-001, DESK-APP-002

## Context

Applications start with a stored command resolved against the machine's PATH
(ADR-0013), which is how an operator would run it in a terminal. That is
correct as a default and ambiguous as a promise: a machine with seven PHP
copies (Laragon, XAMPP, a manual install) runs whichever PATH finds first, and
the interface cannot say which. DESK-RUN-001 asks for detection, display, and
per-application selection.

The framework scaffold already solves discovery for its own tools: a probe
that searches PATH, the Windows Python launcher and per-platform install
locations, runs each copy's own version flag, and caches the answers.

## Decision

### The registry is data

The initial kinds are the seven from PRD section 9.1 — PHP, Node.js, Python,
Go, Java, Ruby and .NET. Each kind names the binary the probe looks for and
the per-platform install locations to search beside PATH; Go, Java and Ruby
gain the location entries their installers use. Adding a kind is a data
change, not a screen.

A **copy** is one installed executable: its path, the version its own
`--version` (or PHP's `-v`) flag reports, and its source — `PATH` when PATH
found it, `installed` when a location glob did. Copies are read-only facts of
the machine: detection never writes, moves or uninstalls anything, so an
existing system install cannot be damaged.

### Identification and freshness

A copy is identified as `kind@path`. Identification is by path, not by
version: two installs of the same version at different paths are different
copies, because the operator points at a place on disk, not a number. The
probe caches versions for the life of the process, so an uninstalled runtime
can still appear until Locafy restarts; a start validates the assignment and
refuses with a clear message when the path no longer exists.

### Assignment

An application stores an optional runtime id (empty means the machine's
default PATH order, exactly ADR-0013's behaviour today). At start, the
supervisor resolves the assignment and **prepends the copy's directory to the
process PATH**, ahead of the inherited PATH — the stored command then resolves
the assigned copy first while everything else keeps the operator's machine
environment. The command is never rewritten; only the search order changes.

A start with an assignment the machine no longer has is refused before any
process spawns, with the reason on the record.

Minimum-version requirements are deliberately absent: the initial supported
versions are not finalised (PRD section 9.1), and a minimum recorded now would
be a guess. Assignment validates existence, not a version.

### Where managed installs fit

Every copy today is system-sourced and the interface labels it as such.
Locafy-managed installs — and the `managed` source that distinguishes them —
arrive with the install work (DESK-DL-001), whose job strategy is still an
open decision. Nothing here assumes an installer.

### Discovery moves to a shared tools package

The probe started in the framework scaffold. Wiring the registry into the
applications supervisor surfaced an import cycle: `scaffold` already imports
`applications` to register a scaffolded application, and `applications` now
needs discovery to resolve an assigned runtime. The discovery core therefore
moves to `internal/tools` — candidates, version probing, per-platform install
locations — and `scaffold` keeps its recipes, preflight and fixes as a thin
consumer. The runtime registry becomes the package's second consumer, and the
move happened for a structural reason, not a hypothetical third one.

## Consequences

- The Runtimes screen lists kinds with every copy's version, location and
  source, straight from data.
- An application can pin the copy its command uses without Locafy editing
  commands or composing shells.
- A developer who changes PATH while Locafy runs sees the new order on the
  next start, except where an assignment pins a copy.
- Runtime *installation* is not part of this record; the downloads view and
  install jobs follow their own decision.
