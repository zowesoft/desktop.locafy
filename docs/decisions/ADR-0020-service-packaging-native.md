# ADR-0020: Service packaging — native engine processes, with containers as a per-engine exception

- **Status:** Accepted — 2026-11-26, with the MySQL slice delivered and verified live on Windows
- **Date:** 2026-11-23
- **Depends on:** ADR-0001 (cross-platform), ADR-0004 (local data model), ADR-0008 (service engines and instances), ADR-0013 (process supervision), ADR-0014 (ports and ownership), ADR-0015 (health checks), ADR-0019 (managed runtimes and installs)
- **Affects:** DESK-SVC-001, DESK-NET-001, DESK-DL-001

## Status note

This record was first written as accepted, and the PostgreSQL slice of
DESK-SVC-001 was built and verified against it. It then carried the
engine-by-engine comparison and was returned to **proposed** so the operator
could approve the packaging rule before the remaining engines were committed to
it. The rule was approved on 2026-11-26 together with the MySQL slice, and both
of those engines are now delivered: MySQL has a discovery path, an initialiser,
supervision and tests in this repository, and Locafy started, queried and stopped
it on Windows. The rows below stay the record of what was checked before each
engine existed.

The engine table was corrected on 2026-11-23. It previously read as though every
engine had been checked against its published downloads; in fact only
PostgreSQL had been. Each row now says what was actually checked, and the
wording that presented an assumption as a check is gone. The Redis-on-Windows
section now carries the licence and practical cost of each option and a
recommendation, so the operator can settle it in one decision rather than four.
The Mailpit and MinIO rows were re-checked on 2026-11-24 against Mailpit's
v1.34.0 assets and MinIO's own download server and registries, which is why
their wording differs from the first pass: MinIO's community binaries are now
withdrawn rather than merely frozen.

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

**What was checked, and what was not.** Only PostgreSQL has been exercised:
the delivered slice installed nothing itself, but it discovered, initialised and
ran the installed PostgreSQL 17 on Windows, and every binary it used came from
one `bin`. Everything else below was read off the vendors' current download
pages, release assets and registry responses on 2026-11-23 and 2026-11-24 — a
published build is evidence that a build exists, not that Locafy can find it,
initialise it and supervise it. PostgreSQL and MySQL have since gained a
discovery path, an initialiser and supervision; Redis, MariaDB, Mailpit and
MinIO still have none, and none of those four has been started by Locafy on any
platform. Their rows are therefore leads, not verified paths.
"Container" means the engine's official image under Docker or Podman.

| Engine | What was checked | Native on Windows | Native on macOS | Native on Linux | Container | Recommendation |
|---|---|---|---|---|---|---|
| **PostgreSQL 17** | **Verified live** on Windows: discovered, initialised and run by the delivered slice | The installed PostgreSQL 17 at `C:\Program Files\PostgreSQL\17\bin` (the EDB installer's layout): `initdb`, `postgres`, `pg_ctl`, `createdb` in one `bin` — **verified live** | Not checked here; Homebrew and the EDB installer are the documented routes | Not checked here; Oracle packages and distro packages are the documented routes | Official `postgres` image | **Native now.** Satisfies the admission rule, needs one initialiser (`initdb`), runs from a plain argv. Delivered. |
| **Redis 8.x** | Published downloads and install docs, 2026-11-23 | **No official native build.** Redis documents Windows as Docker-only and names Memurai its Windows partner. Community forks build it anyway — `redis-windows` and `zkteco-home` both track 8.10.3, `tporadowski` ships a native 5.0.14.1 with an 8.10 port in progress — none with vendor support | Homebrew `redis` — Redis documents Homebrew as the supported macOS route; source builds work | `packages.redis.io` APT/RPM, Snap, distro packages, source | Official `redis` image | **Blocked on Windows.** Not merely "no build": the only vendor-blessed native option is development-only with a forced 10-day restart, which Locafy cannot ship or hide. Needs the operator decision below. |
| **MySQL 8.4 LTS / 9.7 LTS** | Published download page for 8.4.13 LTS (also 26.10 Innovation), 2026-11-23; **delivered and verified live on 8.0.30** on Windows, 2026-11-26 | MSI installer and ZIP archive | Official `.dmg`/pkg installer and `.tar.gz` | Generic `.tar.gz`, Oracle RPM/DEB repositories, distro packages | Official `mysql` image | **Native, delivered.** Discovered from PATH, `Program Files\MySQL`, the Laragon layout and Locafy's managed engines; initialised with `mysqld --initialize-insecure`; run on loopback with its own data directory and port; stopped through `mysqladmin shutdown`. macOS and Linux runs are still outstanding. |
| **MariaDB 13.0** | Published download page and the 13.0.2 file listing, 2026-11-23 | MSI package (`winx64-packages`) | **No official package.** MariaDB's own documentation states there is no `.pkg` installer for macOS and sends users to Homebrew; its 13.0.2 file listing has no macOS directory | Repositories (DEB/RPM), systemd binary tarball | Official `mariadb` image | **Does not satisfy the rule as published.** Windows and Linux are vendor builds; macOS is a third-party Homebrew build that MariaDB merely points at. Either the operator accepts that macOS route, or the relational slot is MySQL only. |
| **Mail capture (Mailpit)** | v1.34.0 release assets and install documentation, 2026-11-24 | `mailpit-windows-amd64.zip`, `mailpit-windows-arm64.zip` | `mailpit-darwin-amd64.tar.gz`, `mailpit-darwin-arm64.tar.gz`, also Homebrew `mailpit` | `mailpit-linux-386`, `-amd64`, `-arm`, `-arm64` `.tar.gz` | Official multi-arch images (`axllent/mailpit`) | **Native — cheapest of the set.** One binary, no install step, no data directory to initialise unless persistence (`--database`) is turned on; two ports, 1025 SMTP and 8025 UI. |
| **Object storage (MinIO)** | Download server and registry responses, repository state and licence, 2026-11-24 | Community edition is **source-only** and archived, and its binaries are gone, not merely frozen: MinIO's own download server answers **HTTP 410 Gone** for every community file, with its own text saying "These files are no longer served from this site. This applies to all community releases and to all hotfix builds". AIStor publishes a paid binary | Same for community; AIStor publishes a `darwin-arm64` binary and a Homebrew tap, both behind a licence key | Same for community; AIStor RPM/DEB/binary behind a licence key | No community images: the archived README offers only "build a Docker image from the provided Dockerfile", and a maintainer declined to publish the October 2025 security release as an image (issue 21647). Docker Hub's API answers **404** for `minio/minio` and quay.io answers **401** to anonymous requests (checked 2026-11-24). MinIO published no dated notice of the removal, so the dates come from the registries, not from MinIO. AIStor images need a licence | **Fails the rule.** There is no maintained community build on any platform — the source is archived and the binaries are withdrawn — and the maintained build (AIStor) is commercial and licensed. Building the archived source ourselves is not a project this catalogue should take on. Garage and SeaweedFS are the alternatives to evaluate (PRD section 23). |

Engines that look attractive but fail the admission rule stay out until they
do not: that is the deliberate cost of a cross-platform promise.

### The MySQL slice (delivered 2026-11-26)

- **Discovery** reads PATH, `%ProgramFiles%\MySQL\MySQL Server *\bin`,
  `C:\laragon\bin\mysql\*\bin` and Locafy's managed engines on Windows, and
  `/usr/local/mysql/bin`, the Homebrew opt paths and MAMP on Unix. The version
  comes from `mysqld --version`, never from a folder name.
- **Initialisation** runs `mysqld --initialize-insecure --datadir=<instance>
  --basedir=<install>`; an instance counts as initialised when `<datadir>/mysql`
  exists. Its `root` account has an empty password and is reachable only on
  loopback, the same local-development posture PostgreSQL's trust rule gives.
- **Supervision** runs `mysqld --datadir=… --port=… --bind-address=127.0.0.1
  --mysqlx=OFF`, plus `--console` on Windows so the transcript stays in Locafy's
  hands instead of an error-log file. A stop asks `mysqladmin --protocol=TCP …
  shutdown` first and falls back to the supervised tree kill.
- **The project's database** is created once the instance answers, with
  `mysql --protocol=TCP -e "CREATE DATABASE IF NOT EXISTS …"`, and `root` is the
  user recorded for the connection URL.

**Port sharing is a real failure mode.** Windows lets a loopback listener and a
wildcard listener hold the same port, so on a machine where another MySQL already
binds `0.0.0.0:3306`, Locafy's instance can bind `127.0.0.1:3306` and a statement
sent before Locafy's server has finished starting reaches the other server. The
first live run did exactly that: the project's database was created in the
machine's own MySQL instead of the instance. The preparation step therefore asks
the server for `@@datadir` and writes only when that directory is the instance's
own, retrying while it is not.

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
  Windows answer is decided. MariaDB is the second if the macOS route is judged
  to be Homebrew rather than a MariaDB build, and object storage is not a native
  candidate at all while the community MinIO edition publishes nothing.
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

Redis Ltd. publishes no Windows build, and its documentation sends Windows users
to Docker or to Memurai. Nothing below is implemented, and nothing below was run
by Locafy. Options as they stand on 2026-11-23:

| Option | Licence | What it costs Locafy |
|---|---|---|
| **Memurai** (Windows only) | Proprietary (Janea Systems). The Developer Edition is free, needs no account, and is licensed for development and testing only: 10 days of maximum uptime before it shuts down until restarted, 50% of system memory, 10 client addresses. The Enterprise Edition is paid. The EULA forbids distributing the software, making extra copies, or working around technical limitations. | No bundling and no auto-restart: the operator installs Memurai, Locafy discovers it as it discovers PostgreSQL, and can only report the shutdown. It exists on one platform, so macOS and Linux still use real Redis from Homebrew and distro packages and every test and doc splits three ways. Closest fit to the existing model, and the one dependency that will stop working after ten days of continuous use. |
| **An unofficial native port** | Community builds of Redis source under Redis's own licence (RSALv2 / SSPLv1 / AGPLv3 since 8.0): `redis-windows/redis-windows` 8.10.3 (October 2026), `zkteco-home/redis-windows` 8.10.3 (November 2026), `tporadowski/redis` native 5.0.14.1 with an 8.10 port in progress. | Free, and cheap to wire: one more discovery path. Expensive to stand behind: no vendor supports the Windows build, no one commits to a security fix on it, and anything Locafy redistributed would carry AGPL or RSAL obligations. This is the option that most quietly turns a supported catalogue into a gamble. |
| **A Windows-only container exception** | Docker Desktop is free for personal use, education, non-commercial open source, and businesses under 250 employees **and** under $10M revenue; otherwise Pro, Team or Business from about $9 per user per month; government entities always pay. Docker Engine is Apache-2.0, and Podman (Apache-2.0) is a free alternative that runs a WSL2 machine. | A second control plane beside the supervisor: image, host port, volume, daemon state, pull failures. A cold image cache breaks the offline first run the rest of the design protects. It also moves a licence decision onto the operator, invisible to Locafy. |
| **WSL2-hosted Redis** | Free; WSL is a Windows optional feature. | Enabling it needs administrator rights, the Virtual Machine Platform feature, a reboot and firmware virtualization (SLAT) before the first service starts. The server then lives in a Linux VM: Locafy holds `wsl.exe`, not the Redis process, so the supervised tree kill reaches the client rather than the server, and `wsl --terminate` would kill everything else in that distro. Health over forwarded localhost works; ownership does not. |
| **Omit Redis from the first release** | — | No cache engine in v1. One less engine to discover, initialise and document, and one story per platform. |

**Recommendation: omit Redis from the first release.** Each alternative buys a
cache at a price the design has otherwise refused to pay — a dependency that
expires after ten days, a binary nobody supports, a paid container runtime, or a
VM Locafy only half owns. The loss is one engine, not one capability: a project
without a cache still runs, and the application that truly needs Redis is the
case a container exception exists for — chosen by the operator, per project,
rather than imposed on every Windows user through the catalogue.

If a cache must ship in v1 anyway, the least-bad route is **Memurai discovered
and never bundled**: report its ten-day shutdown in the Services tab as an engine
fact with a documented restart step, never restart it automatically, and accept
that the Windows engine differs from the other two.

If the operator would rather have a cache that is native on all three platforms,
**Garnet** (Microsoft Research, MIT-licensed, RESP-compatible, .NET) is a better
engine candidate than any Windows port of Redis and would enter the catalogue as
its own engine under the same admission rule. It is not Redis, and it needs the
.NET runtime or a self-contained build — a separate decision, not a packaging
exception.

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
   (`pg_ctl stop -m fast`), then falls back to the supervised tree kill. As with
   application supervision (ADR-0013), the stop is decided from the process's
   own exit rather than only from the record, so a child still holding the
   output pipes cannot make a killed engine read as a failed stop. The output
   readers are released the same way: once the engine has exited they read what
   it wrote and stop, instead of waiting on a pipe an orphan keeps open. Lines
   still draining after Stop finalised the run are folded into the transcript
   the row reads back.
   Restarting the service instance is stop then start, keeping the same
   directory and port. A project restart is a different action and leaves
   running services alone (ADR-0016).
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

- Which engine follows PostgreSQL — MySQL and Mailpit are the recommended next
  two, and each needs its own initialiser work. MariaDB waits on the macOS
  ruling above.
- The object storage engine. MinIO is out under the admission rule above; Garage and SeaweedFS remain candidates, unevaluated.
- Shared versus isolated tenancy defaults for services that support tenancy
  (ADR-0008); the delivered slice is isolated-only.
- Where engine binaries come from for the managed case (download jobs are
  DESK-DL-001's machinery; the slice discovers installed engines and reports a
  missing one).
