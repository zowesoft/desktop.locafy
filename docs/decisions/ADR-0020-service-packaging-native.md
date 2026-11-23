# ADR-0020: Service packaging — native engine processes, with containers as a per-engine exception

- **Status:** Proposed — awaiting operator approval
- **Date:** 2026-11-23
- **Depends on:** ADR-0001 (cross-platform), ADR-0004 (local data model), ADR-0008 (service engines and instances), ADR-0013 (process supervision), ADR-0014 (ports and ownership), ADR-0015 (health checks), ADR-0019 (managed runtimes and installs)
- **Affects:** DESK-SVC-001, DESK-NET-001, DESK-DL-001

## Status note

This record was first written as accepted, and the PostgreSQL slice of
DESK-SVC-001 was built and verified against it. It now also carries the
engine-by-engine comparison, and it is returned to **proposed** so the operator
can approve the packaging rule before the remaining engines are committed to
it. Nothing in the delivered PostgreSQL slice changes if the rule is approved;
if it is rejected in favour of container-backed instances, that slice is the
part that would need reworking.

## Context

ADR-0008 settled what a service instance *is* — one data directory, one set of
ports, one supervised process, its own configuration — and deliberately left
open how an engine's process comes to exist: native binaries, or containers.

That choice is load-bearing. It decides whether Locafy needs a container
runtime on the machine, who owns port allocation, how a data directory is
exposed to the operator, how an instance is stopped, what the downloads view
installs, and how much per-platform work each new engine costs.

Constraints that already exist in this repository:

- Local-first, no required account (AGENTS.md). A service must start on a
  machine with no network, no cloud account, and no software beyond what Locafy
  installs itself.
- Cross-platform from day one, verified per platform, with CI on Windows, macOS
  and Linux (ADR-0001).
- Locafy already supervises processes it starts: argv-only commands, recorded
  process identity, tree kill, port discovery, TCP health (ADR-0013 to 0015,
  0017).
- Managed tools are unpacked into a directory Locafy owns and are never added to
  the system `PATH` (ADR-0019).

## Decision

**Native is the default and the supported packaging for every service engine.
A container runtime is not required, installed, or assumed. Containers are
allowed only as a per-engine exception, recorded in its own ADR, where no
maintained native build exists for one of the three platforms.**

### What that means concretely

- An engine is a set of executables: discovered in a known installation
  location, or unpacked by Locafy into its own directory by the install
  machinery.
- An instance is exactly the object ADR-0008 describes — a Locafy-created data
  directory, an allocated port, and a process Locafy starts with an argv and
  stops with its tree. No volume, mount, image or container network is modelled.
- The engine's server binary runs in the foreground under supervision, so its
  output and process identity belong to Locafy. Shutdown prefers the engine's
  own graceful stop and falls back to the supervised tree kill.
- **Admission rule:** an engine enters the catalogue only when it has a
  maintained native build for Windows, macOS and Linux, a discovery path, and a
  first-run procedure Locafy can run. An engine that fails the rule is absent
  from the catalogue with a stated reason, not shipped broken.
- The instance object does not record its packaging, so a container-backed
  exception later would not change the schema, the port rules, or the project
  lifecycle.

### Engine-by-engine comparison

Native availability was checked against each project's published downloads, and
PostgreSQL's path on Windows was exercised live in the slice described below.
"Container" means the engine's official image under Docker or Podman.

| Engine | Native on Windows | Native on macOS | Native on Linux | Container | Recommendation |
|---|---|---|---|---|---|
| **PostgreSQL 17** | Official installer (EDB) — **verified live** (`initdb`, `postgres`, `pg_ctl`, `createdb` from one `bin`) | Official installer / Homebrew | Official packages / distro packages | Official `postgres` image | **Native now.** It satisfies the admission rule, needs one initialiser (`initdb`), and runs from a plain argv. Delivered. |
| **Redis** | **No maintained build.** The last Microsoft port is 3.2, unmaintained; Memurai is a commercial fork | Homebrew `redis` | Distro packages, official source | Official `redis` image | **Blocked on Windows.** Native on macOS and Linux, so the catalogue entry cannot meet the admission rule without a Windows answer (see the decision this needs). |
| **MySQL 8 / MariaDB 11** | Official installer / ZIP archive | Homebrew, official DMG | Official archives, distro packages | Official `mysql` and `mariadb` images | **Native, next after PostgreSQL.** Satisfies the rule; costs its own initialiser (`mysqld --initialize-insecure` or `mariadb-install-db`) and a port. |
| **Mail capture (Mailpit)** | Single static binary | Single static binary (also Homebrew) | Single static binary | Community image | **Native — cheapest of the set.** Go binary, no install step, no data directory to initialise; two ports (SMTP and UI). |
| **Object storage** | MinIO is a single binary, but AGPL-3.0 licensing and its distribution/packaging changes need validation before Locafy bundles or launches it; Garage and SeaweedFS are alternatives | Same | Same | MinIO image | **Deferred.** The engine is still undecided (PRD section 23); a native decision here would be premature. |

Engines that look attractive but fail the admission rule stay out until they
do not: that is the deliberate cost of a cross-platform promise.

### Why native for this catalogue

1. **It satisfies local-first without a second installer.** Requiring Docker
   would make the first service start depend on software Locafy cannot install,
   licence or repair, and on a daemon that may be absent, stopped, or not
   permitted for the current user — and on Windows a WSL2 distribution.
2. **The hard parts are already built.** Port allocation, process identity,
   tree kill, port discovery and TCP health exist and are tested. A container
   does not remove that work — it still needs a host port, a mapped directory
   and a lifecycle — it adds a second control plane on top.
3. **The data directory is what the operator cares about.** With a native
   process it is an ordinary folder to open, back up, copy or delete
   deliberately. A volume adds indirection that a single-machine, single-user
   tool gains nothing from.
4. **It is smaller.** One server process per instance, no daemon, no image
   layers to pull or store, on a machine that is also running the operator's
   applications.
5. **It is verifiable here.** PostgreSQL 17 initialises and runs from an argv on
   this machine; that is the whole contract the decision needs, and it is now
   demonstrated rather than assumed.

### Trade-offs, stated plainly

| Dimension | Native (recommended) | Container-backed |
|---|---|---|
| First run, offline | Works as soon as the engine is present | Image must be pulled first; a cold cache breaks offline first run |
| Machine dependency | Per-engine runtime needs (e.g. VC++ runtime) | Docker/Podman daemon, and WSL2 on Windows |
| Data location | Ordinary folder, named in the UI | Named volume or bind mount; data is not directly inspectable |
| Process control | Locafy owns pid, tree, output, exit | Daemon owns the lifecycle; output comes from the log driver |
| Ports | Locafy allocates and verifies | Still a host port mapping — the same conflicts remain |
| Isolation | By directory, port and loopback binding; **not** sandboxing | Stronger (namespaces/cgroups), but still publishes host ports |
| Upgrade | Replace binaries; data directory pinned to a major version | Replace tag; volume pinned to a major version |
| Per-engine cost | Initialiser, server flags and discovery per engine **per platform** | One image, but a runtime to install and a pull policy to design |
| Footprint | One process per instance | Daemon plus image layers, resident even when idle |
| Failure modes | Engine missing, port held, initialisation failed | Daemon down, pull failed, mount permission, orphaned container |
| Secrets | Engine's own auth on loopback | Image env vars / secrets plus engine auth |

The honest summary: containers move work from Locafy into an external
dependency, and native moves work from the external dependency into Locafy.
For a single-user desktop tool that must work offline, paying it in Locafy is
the better trade — but it is paid engine by engine, and that is why the
catalogue grows one verified engine at a time.

## Consequences

- **Engine availability becomes Locafy's responsibility.** Each engine needs a
  native build per platform, a discovery path, and a verified first-run
  procedure. Redis is the first casualty: absent from the catalogue until its
  Windows answer is decided.
- **Initialisation is per engine.** PostgreSQL needs `initdb`; MySQL needs
  `mysqld --initialize`; each is code and tests of its own. This is the real
  cost, and it is not hidden behind a generic installer.
- **We own the server configuration surface.** Options a container image bakes
  in (`listen_addresses`, port override, encoding) become argv flags, and must
  be verified per engine version.
- **Isolation is conventional.** Separate directories, ports and loopback
  binding; the interface must never describe it as container isolation.
- **Authentication needs a store.** Loopback trust is acceptable for a local
  development instance and is what the PostgreSQL slice uses; anything bound
  beyond loopback is not offered. A generated password would need the OS
  credential store, which is not built yet.
- **Containers remain a legal exception.** Because the instance object does not
  name its packaging, a container-backed engine can be added later for a
  platform where native is impossible, without a schema change.

## Decision this needs from the operator (Redis on Windows)

The recommendation is to keep Redis out of the catalogue until one of these is
chosen, in this order of preference:

1. **Memurai or another maintained native Windows build**, if its licence
   permits redistribution or a documented free install.
2. **A documented container exception for Windows only**, with macOS and Linux
   staying native. This is the first case that would exercise the exception
   clause above, and it makes Docker a requirement for Redis on Windows only.
3. **WSL2-hosted Redis**, which is a container in spirit and an extra
   dependency in practice.
4. **Drop Redis from the initial catalogue.** Caches are the least essential
   service; the first release could ship without one.

## First slice — PostgreSQL instance attached to a project (delivered)

The outline below is what DESK-SVC-001 built first, and what the packaging
decision is judged on. It is implemented, tested, and verified live; the
evidence is in `docs/FEATURES.md`.

1. **Attach.** The project's Services tab offers the catalogue entry with the
   installed engine version, or the reason it is unavailable. Attaching takes an
   optional name, allocates the port, creates the record and does not start
   anything. The data directory is
   `<data home>/Locafy/services/<instance id>/data`, keyed by instance id so a
   rename never moves it, and never inside a linked project (ADR-0007).
2. **Port.** The engine's conventional port first (5432), otherwise the next
   free one, skipping ports another live instance records or a listener holds.
   The port is recorded and shown with its owner, and a held port refuses the
   start instead of killing the holder (ADR-0014's rule, applied to services).
3. **Start.** The initialiser runs once when the data directory has no cluster
   (`initdb` with the instance's role and trust auth restricted to loopback),
   then the server runs in the foreground on `127.0.0.1` with the allocated
   port. Once the port first answers, the project's database is created with
   the engine's own tool, so the connection details are usable immediately.
4. **Stop and restart.** Stop asks the engine to shut down gracefully
   (`pg_ctl stop -m fast`), then falls back to the supervised tree kill.
   Restart is stop then start, keeping the same directory and port.
5. **Health.** A TCP connection attempt on the allocated port, on the runtime
   loop, reported as answering or not answering with the port named. A stopped
   instance clears the reading rather than showing a stale one.
6. **Connection details.** Loopback host and port, database, role, a
   `postgresql://` URL and the `DATABASE_URL` line, each copyable.
7. **Removal.** Stops the instance, deletes only the record, and keeps the data
   directory; the confirmation names the directory before acting (ADR-0008's
   rule that data deletion is separate).
8. **Out of scope for this slice, and recorded as blockers:** shared-mode
   tenancy, application-to-service start ordering, durable service logs, and
   the remaining engines.

## Not decided here

- Which engine follows PostgreSQL — MySQL/MariaDB and Mailpit are the
  recommended next two, and each needs its own initialiser work.
- The object storage engine (MinIO remains a candidate only).
- Shared versus isolated tenancy defaults for services that support tenancy
  (ADR-0008); the delivered slice is isolated-only.
- Where engine binaries come from for the managed case (download jobs are
  DESK-DL-001's machinery; the slice discovers installed engines and reports a
  missing one).
