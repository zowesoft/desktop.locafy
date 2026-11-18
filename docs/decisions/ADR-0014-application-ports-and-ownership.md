# ADR-0014: Application ports and ownership

- **Status:** Accepted
- **Date:** 2026-11-18
- **Depends on:** ADR-0013 (application process supervision)
- **Affects:** DESK-APP-002, DESK-PROJ-002, DESK-NET-001

## Context

ADR-0013 gave an application a supervised process but deliberately left ports
out: it neither discovered what a running application listens on nor explained a
port that is already taken. The PRD asks for port tracking and conflict reporting
that never terminates an unrelated process, so that stays true even when it would
be the quickest way to "fix" the conflict.

Locafy knows two things about a port: which applications it started, and which
TCP ports are listening on the machine. It does not know, in advance, which port
a command will choose — a framework decides that, may read a flag, and may fall
back to the next free port.

## Decision

### Discovery, not configuration

The supervisor discovers ports from the live process tree rather than parsing
them out of a command or asking the operator to declare them:

- Every listening TCP port belongs to a pid (`net.Connections`).
- A port is the application's when its owning pid is the supervised process or
  one of its descendants. Descent is walked through parent pids, so `npm run
  dev` → node → the server all count.
- The lowest such port becomes the application's recorded `port`; every observed
  port is emitted as `application:ports` so the interface can show the rest.
- Scanning repeats while the process lives. A scan that fails, or sees nothing
  yet, leaves the last recorded value alone instead of flapping to "no port".
- The recorded port survives the process: it is the last port the application
  used, and it is what makes a conflict detectable on the next start.

### A conflict is a refusal to start, with the reason

`port` is only a record, so a conflict is checked at start time against it:

- If a listener already holds the application's recorded port, the start is
  refused, and the record gets `status = error` plus a `port_conflict` message.
  Nothing is killed, and no process is spawned.
- The message names what holds the port: another Locafy application by name, or a
  process Locafy did not start, with its process name and pid, followed by the
  fact that Locafy will not stop it.
- A refusal is not an app failure: `error` stays empty and `port_conflict`
  carries the explanation, so the interface can present it as a port problem
  rather than a crash.
- The first run has no recorded port, so it is never refused; whatever it binds
  is recorded for the next one.
- A scan failure at start time does not block a start. A missing answer is not a
  conflict.

### Only processes Locafy started are terminated

- Stop and quit kill the pid of the process Locafy spawned and its tree, exactly
  as ADR-0013 records. The supervisor never looks up a port's owner to kill it,
  and a conflict path never reaches the kill code.
- A process that has already exited is removed from the live set before any kill
  can be attempted, and a run marked exited is never killed even if a pid is
  still present, so a recycled pid cannot be hit.

## Consequences

- An application whose previous port is now held cannot be started until the
  operator frees it. That is intentional: the alternative is a confusing bind
  failure or a silent move to another port. An app that would have chosen a
  different port still needs the operator to stop the holder, which the message
  says plainly.
- Two Locafy applications cannot fight over the same recorded port; the second
  one is named against the first.
- The port shown for a stopped application is its last known one, not a live
  reading; "Running" plus a port is the only combination that claims a live
  listener.
- No port is reserved or checked before it is recorded, so a first run can still
  collide with a foreign process. That collision surfaces through the app's own
  failure output, and the next start is refused with the ownership message.
- Ports learned here become the natural input for local domains and the reverse
  proxy (DESK-NET-001); nothing in this slice configures them.
- Health checks remain out of scope; a listening port is evidence of a bound
  socket, not proof the application is ready.
