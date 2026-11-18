# ADR-0016: Project lifecycle orchestration

- **Status:** Accepted
- **Date:** 2026-11-18
- **Depends on:** ADR-0010 (applications and dependency graph), ADR-0013 (application process supervision), ADR-0014 (application ports), ADR-0015 (application health checks)
- **Affects:** DESK-PROJ-002, DESK-APP-002, DESK-SVC-001

## Context

Applications can now be started, stopped and restarted one at a time, and their
dependency graph is validated at write time (ADR-0010). A project still has no
Start button that works: starting five applications by hand, in the right
order, waiting for each database before the API that needs it, is exactly the
repetitive work Locafy exists to remove. PRD section 7.3 describes the
intended order, and services — the step before applications — do not exist yet
(DESK-SVC-001).

## Decision

### Orchestration lives beside the supervisor

Project start, stop and restart are methods of the application supervisor:
it already owns process state, the dependency edges are already loaded with the
records, and every transition keeps its single-row keyed update. The projects
package stays storage and metadata only; it never spawns or kills a process.

### Start order and readiness

Start walks the project's runnable applications in topological order — a
dependency always appears before its dependents, and name order breaks ties so
the order is stable. Applications without a start command are never started;
a dependent of one is skipped with that reason.

Before a dependent starts, each of its dependencies must be ready:

- **Running** is the floor. A dependency that exited, was refused, or sits in
  `error` fails its dependents immediately, with the dependency's own message.
  A successful launch is watched briefly for an immediate exit, because a
  command that crashes on boot must fail its dependents, not pose as a running
  dependency; the watch costs one poll in the happy path once the observation
  window has passed.
- **Answering** is the readiness signal when the dependency has a discovered
  port: the dependent waits for `healthy` (ADR-0015) up to a ceiling. An
  unhealthy dependency at the ceiling does not start its dependents — a
  half-up database is worse than a clearly skipped API.
- **Portless is not unhealthy.** A dependency that runs but binds no port
  within a grace window is treated as ready: a worker cannot answer a socket,
  and running is all it can prove.

The ceilings are short and fixed. Orchestration is not a watchdog; an
application that needs longer than the ceiling to bind still runs — its
dependents simply wait for the next start.

### Stop and restart

Stop walks the reverse topological order, so a dependent's process ends before
the infrastructure it talked to. Restart is stop then start through the same
paths. Only processes Locafy started are ever touched (ADR-0013); a foreign
listener on a project port is reported, never killed.

One project action at a time: a second start or a stop while an orchestration
is running is refused until it finishes. Manual single-application actions stay
available throughout.

### Reporting

The result is per-application, never a single word hiding the details:

- each application ends `started`, `already running`, `skipped` (with the
  reason written to the record's `error` column, so the reason survives a
  refresh), or `failed` (the supervisor's own record error);
- a `Project started` / `Project stopped` / `Project restarted` activity entry
  summarises the run at project scope, so it reaches the notifications bell;
- the derived project state is computed from component statuses:
  `starting`/`stopping` while transient states exist, `running` when every
  runnable application runs, `partial` when some run and others do not,
  `error` when none run but some are in `error`, and `stopped` otherwise.

The stored `projects.lifecycle_state` column stays untouched: its CHECK
vocabulary cannot express `partial`, and a stored single state would drift from
the components it summarises. The interface derives the state it displays from
the component statuses; the column remains the schema's coarse placeholder
until a consumer needs it stored.

### Deliberately deferred

- Services are not in the graph yet (DESK-SVC-001). When they exist they take
  the position before applications in the PRD section 7.3 order; the
  orchestrator's step list is where they will slot in.
- A project-level health summary beyond the derived state (for example a
  readiness percentage) waits for the analytics work.
- Runtime checking (PRD section 7.3 step 2) arrives with DESK-RUN-001; the
  supervisor's PATH resolution is the only runtime behaviour today.
