# ADR-0017: Project-scoped resource usage

- **Status:** Accepted
- **Date:** 2026-11-18
- **Depends on:** ADR-0005 (machine monitoring), ADR-0013 (application process supervision), ADR-0014 (application ports), ADR-0016 (project lifecycle orchestration)
- **Affects:** DESK-PROJ-002, DESK-DEVX-001, DESK-ANAL-001

## Context

The project overview shows machine CPU, memory and disk. That answers "is this
computer loaded", not "is my project heavy". PRD section 13.3 requires every
displayed metric to state its scope — machine-wide, Locafy-managed, or
project-specific — and the project overview is where the project-specific
reading belongs. The supervisor already knows the process trees it started
(the roots ADR-0014 walks for port ownership), which is the attribution
boundary Locafy can honestly defend.

## Decision

### What is measured

A project's usage is the sum over the process trees of its **running
applications**: the roots the supervisor launched plus their descendants,
walked through the same parent/child ownership the port reading uses.

- **CPU** is the delta of accumulated CPU seconds (user + system) between
  runtime ticks — the same two-second loop that reads ports and health —
  divided by the wall time between the samples and normalized by the logical
  core count, so a managed tree reads on the same scale as the machine meter
  (100% = every core). The first sample after a process starts has no
  previous delta and reports zero rather than a guess.
- **Memory** is the sum of resident memory (RSS) over the tree, read once per
  tick.

A process that disappears between ticks contributes nothing to that tick and
its baseline is dropped; a tree that gains a child starts counting it from the
next tick.

### What is not measured, deliberately

- Processes the developer started outside Locafy are invisible to this
  number. A project's true footprint is larger than what Locafy runs; the
  meter is labelled for the scope it can defend ("Managed", the applications
  Locafy runs for this project), and the machine meters stay machine-scoped.
- Disk I/O, network and per-service usage wait for the Phase 7 monitoring
  work; this slice covers the two readings the overview needs today.

### Where it lives

Usage is runtime state of a live process, not a record: it lives on the
supervisor's run and is reported per application as `application:usage` events
keyed by application id, aggregated per project by a service method. Nothing
is stored in the database; history belongs to the analytics work
(DESK-ANAL-001) once retention is designed. A stopped application
contributes zero and a project with nothing running reports zero with a zero
application count, which the interface presents as "nothing running" rather
than as a failing meter.

## Consequences

- The overview can show what the project itself costs, next to the machine
  readings it sits inside — the two scales agree because both are normalized
  to the whole machine.
- A busy build inside a managed application can push the managed CPU above
  the machine meter's current reading only when the machine itself is nearly
  saturated; the two meters must never be summed by the interface.
- Attribution is exactly as honest as the process tree: an application that
  forks a detached helper survives in the tree only while the parent link
  does. Detached grandchildren are not chased.
