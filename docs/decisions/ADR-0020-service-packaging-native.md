# ADR-0020: Service packaging — native engine processes, no container runtime

- **Status:** Accepted
- **Date:** 2026-11-23
- **Depends on:** ADR-0001 (cross-platform), ADR-0004 (local data model), ADR-0008 (service engines and instances), ADR-0013 (process supervision), ADR-0014 (ports and ownership), ADR-0019 (managed runtimes and installs)
- **Affects:** DESK-SVC-001, DESK-NET-001, DESK-DL-001

## Context

ADR-0008 settled what a service instance is — one data directory, one set of
ports, one supervised process, its own configuration — and explicitly left open
how an engine's process comes to exist: native binaries, or containers.

The choice is load-bearing. It decides whether Locafy needs a container runtime
on the machine, who owns port allocation, how a data directory is mounted, how
an instance is stopped, what the downloads view installs, and what the
cross-platform matrix has to verify per engine.

Constraints that already exist in this repository:

- Local-first and no required account (AGENTS.md). A service must start on a
  machine with no network, no cloud account, and no additional software beyond
  what Locafy installs itself.
- Cross-platform from day one, per platform, with CI on Windows, macOS and
  Linux (ADR-0001).
- Locafy already supervises processes it starts: argv-only commands, recorded
  process identity, tree kill, discovered ports, TCP health (ADR-0013, 0014,
  0015, 0017).
- Managed tools are unpacked into a directory Locafy owns and are never added
  to the system `PATH` (ADR-0019).

## Decision

**Services run as native engine processes that Locafy starts and supervises
directly. Locafy does not require, install, or talk to a container runtime in
this phase.**

### What that means concretely

- An engine is a set of executables on the machine: either discovered in a
  known installation location, or unpacked by Locafy into its own directory by
  the install machinery. Nothing about a service depends on Docker, Podman, or
  a virtual machine.
- An instance is exactly the object ADR-0008 describes: a Locafy-created data
  directory, an allocated port, and a process Locafy starts with an argv and
  stops with its tree. No volume, mount, image, or container network is
  modelled.
- The engine's own server binary runs in the foreground under supervision, so
  its output and its process identity belong to Locafy. Shutdown prefers the
  engine's own graceful stop command and falls back to the supervised tree kill.
- Only engines with a maintained native build for all three platforms enter the
  catalogue. An engine that needs a compatibility layer is not offered until
  that has been verified, and its absence is reported as "not available on this
  platform" rather than failing at start.

### Why native

1. **It satisfies the local-first rule without a second installer.** Requiring
   Docker would make the first service start depend on software Locafy cannot
   install, license, or repair, and on a daemon that may be absent, stopped, or
   not permitted for the current user.
2. **The hard parts are already built.** Port allocation, process identity,
   tree kill, port discovery and TCP health exist and are tested. Containers
   would not remove that work — a container still needs a host port, a mapped
   directory and a lifecycle — they would add a second control plane on top.
3. **The data directory is the unit the operator cares about.** With native
   processes it is an ordinary folder the operator can open, back up, copy, or
   delete deliberately. A volume adds indirection with no benefit for a
   single-machine, single-user tool.
4. **It is smaller.** One server process per instance, no image layers to pull
   or store, no daemon memory. This matters on a machine that is also running
   the operator's applications.
5. **It is verifiably cross-platform for the engine that matters first.**
   PostgreSQL ships maintained native installers for Windows, macOS and Linux.
   The 17 release installed on this machine initialises and runs a data
   directory from a plain argv, which is the whole contract this decision
   needs.

### Why not containers

- A container runtime is an extra dependency, an extra failure mode, and an
  extra support surface, and it duplicates capabilities the supervisor already
  provides.
- Image tags would need their own pinning, pulling and storage policy, and
  offline first-run would break as soon as an image is not cached.
- Isolation would still be conventional: every container would publish a host
  port and read a host directory, so the port-conflict and data-retention rules
  in ADR-0008 apply unchanged.
- Nothing in DESK-SVC-001's acceptance criteria requires container isolation.
  PRD section 10.4 asks for per-project isolation, which separate data
  directories, ports, processes and loopback binding provide.

### Consequences and costs

- **Engine availability is Locafy's problem.** Each engine added to the
  catalogue needs a native build per platform and a verified discovery path.
  Redis has no official Windows build; it is therefore absent from the
  catalogue until that is resolved, rather than shipped broken.
- **Initialisation is per engine.** PostgreSQL needs `initdb`; MySQL needs
  `mysqld --initialize`; each engine's first-run procedure is code and tests of
  its own. This is the real cost of the decision and it is paid engine by
  engine.
- **We own the server configuration surface.** Options a container image would
  have baked in (`listen_addresses`, a port override, an encoding) are flags
  Locafy passes to each engine. They must be verified per engine version.
- **Isolation is by convention, not sandboxing.** Separate directories, ports
  and loopback binding. The interface must not describe it as a container.
- **Credentials need a store.** Native engines expect authentication. Loopback
  trust authentication is acceptable for a local development instance and is
  recorded in the slice that introduces it; anything beyond loopback is not
  offered.
- **Containers stay possible.** Because the instance object does not name its
  packaging, a later ADR can add container-backed instances for an engine that
  has no native build, without changing the schema, the port rules, or the
  project lifecycle.

## Not decided here

- Which engines follow PostgreSQL, and in what order (Redis needs a native
  Windows answer first).
- The object storage engine (MinIO remains a candidate only).
- Where engine binaries come from for the managed case (download jobs are
  DESK-DL-001's machinery; this slice discovers installed engines and reports a
  missing one).
