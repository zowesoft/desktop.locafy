# ADR-0013: Application process supervision

- **Status:** Accepted
- **Date:** 2026-11-18
- **Depends on:** ADR-0010 (applications and dependency graph), ADR-0011 (framework recipes and scaffold jobs)
- **Affects:** DESK-APP-002, DESK-RUN-001, DESK-PROJ-002

## Context

Applications exist as records with a folder and a start command, but nothing
runs them. DESK-APP-002 asks for start, stop and restart, visible failures
without corrupting other records, and no orphaned processes on any platform.

The scaffold runner already solved the same platform problems for installers:
argv-only commands, Windows `.bat`/`.cmd` shims, hidden consoles, a process group
on Unix, and a tree kill. Reusing that behaviour is the difference between one
supervision model and two that drift.

Two constraints come from earlier records. ADR-0010 says the trust boundary is
"the operator saved this command", so a start must run exactly the stored
`start_command` and nothing composed at runtime. ADR-0004's status vocabulary
already constrains `applications.status`, so process state extends it rather
than replacing it.

## Decision

### One supervision model, shared with scaffolds

Process creation and tree killing move to `internal/process`:

- `process.CommandFor` builds an `exec.Cmd` from an argv. Windows executables
  start hidden, and `.bat`/`.cmd` shims go through `cmd.exe` with a fixed argv
  that is never a shell string. Unix commands start in their own process group.
- `process.KillTree` stops a pid and everything below it: `taskkill /T /F` on
  Windows, `kill(-pid, SIGKILL)` on Unix.

The scaffold runner and the application supervisor both call these; nothing
else spawns a process. No shell is ever involved: the stored start command is
split into an argv by a quote-aware tokenizer, and the first token is resolved
to an executable (directly when it names a path, otherwise through `PATH` and
Windows `PATHEXT`).

### Where an application runs

An application starts with its working directory set to
`project.root_path + application.directory`. Its environment is Locafy's own;
nothing from the scaffold runner's CI/non-interactive variables applies, because
a development server should behave as it does in a terminal. Child input is
closed: Locafy is not an interactive terminal for applications.

If the folder is gone, the command is empty or the executable cannot be
resolved, nothing starts and the record is marked `error` with the reason, so
the failure is visible after a refresh instead of only in a toast.

### Status, exit codes and restart

`applications.status` gains the process states its CHECK constraint already
allowed: `starting`, `running`, `stopping` and `error`, alongside `stopped`.
The last outcome is stored beside it: `exit_code` (NULL when unknown, for
example a process ended by a signal), `error` (the message explaining a
failure, empty otherwise) and `started_at`.

- Start writes `running` only after `exec.Start` succeeded; a process that exits
  on its own moves to `stopped` (code 0) or `error` (any other outcome).
- Stop writes `stopping`, kills the tree, and waits up to ten seconds for the
  exit. A stop the operator asked for ends `stopped` even when the kill's exit
  code is nonzero, because the exit is the expected result.
- Restart is stop-then-start through the same paths; it never launches a second
  copy of a process that did not stop.
- One application failing does not touch another record: every transition is a
  single-row update keyed by the application id.

Locafy does not reattach to processes after it restarts. On startup every
record left in a process state is reconciled to `stopped` with an explanatory
message, because the owner process and its environment are gone. Orphans are
prevented at the other end: quitting Locafy kills every supervised tree.

### Observable output

Each process's stdout and stderr are streamed line by line as
`application:output`, and every status change as `application:state`, both keyed
by application id. The supervisor keeps the most recent 500 lines per live
process in memory for a late-opening view; it writes no log files, because
durable application logs belong to the integrated log work (DESK-DEVX-001).
Output is not persisted: a process that is gone has no buffer to read.

### Deliberately deferred

- Port tracking, conflict reporting and health checks are the next DESK-APP-002
  slice; nothing here binds or probes a port.
- Dependency-ordered starts and project-level orchestration (PRD section 7.3)
  still belong to DESK-PROJ-002. This slice starts exactly one application.
- Runtime selection (DESK-RUN-001) is not consulted; the stored command runs
  against the machine's `PATH` as an operator would run it in a terminal.

## Consequences

- The scaffold runner keeps its behaviour and loses its private copies of the
  platform code, so a future Windows flag or Unix signal fix lands once.
- A start command is a string parsed at start time, not an argv stored in
  columns. Editing the command remains the operator's responsibility, and the
  parsed form is never cached.
- A requested stop records `stopped` with no exit code, while an unexpected
  exit keeps its code and message, so the interface can tell a crash from a
  stop without another column.
- Marks in process states after an unclean shutdown are corrected once, on
  startup, rather than being trusted.
- The 500-line in-memory cap means a long-running, noisy server loses its oldest
  output; durable logs arrive with DESK-DEVX-001.
