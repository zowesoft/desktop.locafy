# ADR-0015: Application health checks

- **Status:** Accepted
- **Date:** 2026-11-18
- **Depends on:** ADR-0013 (process supervision), ADR-0014 (application ports)
- **Affects:** DESK-APP-002, DESK-PROJ-002, DESK-NET-001

## Context

A supervised process that is still running is not the same as an application
that serves. Servers crash after binding, workers restart, and a port can be
listened on while the application is wedged. DESK-APP-002 asks for health state
alongside process state so the interface does not present a running pid as a
working application.

ADR-0014 already discovers the port an application listens on. That is the only
address Locafy can know without configuration: there is no health path, no
expected status code and no per-framework probe in the model.

## Decision

### What is checked

Health is a TCP connection attempt to `127.0.0.1` on the application's primary
discovered port:

- **healthy** — a connection was accepted within 1.5 seconds.
- **unhealthy** — the attempt failed (refused, timed out, or no route).
- **unknown** — the application is not running, has no discovered port yet, or
  has not been checked since it started.

No HTTP request is made. The model has no path or expected response, and a
worker or a raw TCP service is not worse than an HTTP app: "answers" means a
socket accepts a connection, which is what can be verified for every kind
without inventing per-application configuration.

### When it runs

Port discovery and health share one runtime loop, once every two seconds, so
one tick reads the sockets, records a changed port, then probes the current one:

- A port that has not been found yet leaves health unknown rather than
  unhealthy: an application that has not bound is not failing a check.
- A changed port is probed on the same tick, so health follows a server that
  restarts on another port.
- Only a changed result is written and emitted (`application:health`). A stable
  application writes nothing on each tick.
- Process exit, stop and a refused start clear health: a stopped record must not
  keep a stale "healthy".
- A failed port scan skips the port part but still checks the last known port;
  a failed health probe is a result, not an error to hide.

The probe is a dial, not a request: it opens a connection, closes it, and does
not write bytes. Locafy therefore never triggers application work beyond the
connection itself.

### Where it lives

`health` and `health_detail` are columns beside the process state, and the last
checked time is recorded. The process status stays the process's; health answers
the different question of whether anything accepts connections. The interface
shows both, and a conflict or error keeps its own presentation.

## Consequences

- An HTTP application that returns 500 to every request is still "healthy" by
  this definition. Response-level checks need a configured path and expected
  status, which is a later feature; calling this "answering" rather than
  "working" keeps the label honest.
- A non-TCP application (a worker with no listener) stays unknown while it runs.
  That is accurate for a check that can only speak to sockets, and the process
  status still reports it as running.
- Two-second polling means a crash is visible within a few seconds, and a
  short-lived outage may be missed between ticks.
- Health checks make no claim about readiness before the first successful
  probe; the interface must not show a green state for a process that just
  started.
- The discovered port remains the input for local domains and proxying
  (DESK-NET-001); health gives those features a signal to gate on later.
