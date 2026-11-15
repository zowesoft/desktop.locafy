# ADR-0008: Service engines and service instances

- **Status:** Proposed
- **Date:** 2026-11-15
- **Depends on:** ADR-0001 (cross-platform), ADR-0004 (local data model), ADR-0007 (project storage)
- **Affects:** DESK-SVC-001, DESK-NET-001, DESK-RUN-001, DESK-DL-001

## Context

A service is infrastructure that applications use, and it belongs to a project.
The engine behind it, such as PostgreSQL or Redis, is a program that is
installed once on the machine. Without containers, nothing in the operating
system ties an installed engine to a single project, so the question is what
makes a service project-scoped.

Installing an engine and running a service are different operations. This
record separates them and settles how projects get isolated services, and how
they can opt into sharing one. It does not decide how engines are obtained or
packaged, which remains open.

## Decision

**Install globally. Configure and run per project.**

### Engines and instances

| | Engine | Service instance |
|---|---|---|
| What it is | An installed program at a specific version | One configured, runnable server of an engine |
| Scope | Machine | Project (isolated) or machine (shared) |
| Examples | PostgreSQL 17 binaries | Etugora's PostgreSQL |
| Cardinality | One per engine and version | Any number |

This mirrors runtimes and applications: a runtime is machine-level, and an
application selects one. A service instance selects an engine version.

An engine is never started directly. Only instances run.

### What an instance owns

Every instance owns four things that no other instance shares:

1. **Data directory.** One per instance, never shared. Its location and
   persistence behaviour are visible to the operator.
2. **Ports.** One or more named ports (for example `smtp` and `web` for a mail
   service, `api` and `console` for object storage), allocated by Locafy,
   recorded, and checked for conflicts before start. Instances prefer the
   engine's conventional port and take the next free one when it is taken.
3. **Process.** Started and tracked by the engine. Locafy records the PID
   together with the process start time and executable path and verifies that
   identity before signalling, so a reused PID is never mistaken for a managed
   process.
4. **Credentials and configuration.** Secrets are stored through the operating
   system credential store, never in SQLite or project files. Instances bind to
   loopback by default.

### Isolation modes

**Isolated (default).** The instance is owned by exactly one project. Two
projects using the same engine version share binaries and nothing else:

```text
engines/postgresql/17/bin/                  one installed engine
services/<instance-a>/data                  project A: own port, process, secrets
services/<instance-b>/data                  project B: own port, process, secrets
```

Project lifecycle is the instance lifecycle: starting a project starts its
instances, and stopping it stops only those.

**Shared (explicit opt-in).** One instance is owned by the machine, and several
projects bind to it. Each binding gives a project its own logical tenant on the
server: a database and role for relational engines, a bucket and credentials
for object storage. Binding credentials go to the credential store.

- Stopping a project never stops a shared instance. The operator stops it
  explicitly. Whether an idle shared instance may stop automatically is open.
- Server-level configuration, engine version, restarts and resource use are
  shared by every bound project, and the interface says so when the operator
  opts in.
- Removing a project or a binding never drops its databases or buckets. That is
  a separate, confirmed data-deletion action, consistent with ADR-0007.
- Shared mode is offered only for engines whose capabilities declare real
  tenancy: PostgreSQL and MySQL/MariaDB through databases and roles, and
  object storage through buckets and users. Redis and mail capture offer no
  meaningful per-project separation and are isolated-only unless a template
  proves otherwise.
- Moving a project between modes is a dump-and-restore migration, never a
  toggle.

### Engine capabilities

Every service template declares what its engine supports, so Locafy never
assumes that every engine behaves like PostgreSQL:

- multiple instances on one machine
- custom ports
- independent data directories
- automatic initialisation
- health checks
- shared mode, and the kind of tenant it provides

Instance creation, mode selection and the interface are driven by these
declarations. An engine that cannot run more than one instance, or has fixed
ports, is reported as such rather than failing at start.

### Data location

Instance data lives in a central Locafy-managed directory under the platform
data home, keyed by instance ID so a rename does not move it:

```text
<data home>/Locafy/services/<instance-id>/
```

The location is configurable per instance, with a sensible default. Locafy
never places service data inside a linked project directory unless the operator
chooses that location explicitly, because ADR-0007 forbids writing into
directories Locafy does not own. When the operator chooses a project directory
or a cloud-synchronised folder, the interface warns that database files in
version-controlled or synchronised locations can be committed or corrupted.

### Rules

- Locafy-managed engines live under Locafy's own directory, are not added to
  the system `PATH`, and are distinguishable from system installations. Locafy
  never starts, stops, reconfigures or upgrades a system-installed engine, and
  never binds a port it did not allocate.
- An instance records the engine version it was initialised with. Changing
  versions is an explicit operation, because data directories are not portable
  across major versions.
- Removing an instance configuration never deletes its data directory.
- Isolation is by separate directories, ports, processes and loopback binding.
  It is not enforced by the operating system, and the interface and
  documentation must not describe it as sandboxing.
- Missing engines are reported with an offer to install them through the
  downloads view, never installed silently as a side effect of starting a
  project.

### Schema direction

Consistent with ADR-0004, with these additions:

- `service_templates` describes what an engine type needs: capabilities,
  default ports, initialisation, start and health commands, and connection
  shape. It holds no project data.
- Installed engines are machine-level records with the version, the executable
  path, and whether they are Locafy-managed or system installations.
- `services` holds instances. Each records its engine and version, data
  directory, configuration, and a scope of `project` or `shared`. Project
  instances reference `project_id` with `ON DELETE CASCADE`; shared instances
  have no project and are owned by the machine.
- A binding table links a project to a shared instance and records the tenant
  (database and role, or bucket) and the credential reference. Binding rows
  cascade with the project; the tenant's data does not.
- Allocated ports are named, per instance, in `port_allocations`.
- Running processes are recorded in the existing planned `processes` table,
  shared with applications, including process identity.
- Configured `status` and runtime `lifecycle_state` stay separate columns.

Final column definitions belong to the DESK-SVC-001 migration. Container-style
volumes and mount paths are not modelled, because there are no mounts without
containers.

## Not decided here

These stay open and must not be guessed (PRD section 23):

- **Packaging strategy.** Native engine binaries versus container-backed
  instances. This record is compatible with either: an instance is the same
  object whether its process is native or a container. The native route is
  workable for engines that take a data directory and a port on the command
  line, but per-platform availability must be verified engine by engine before
  a choice is made. Known concerns include Redis on Windows, Erlang for
  RabbitMQ, and MinIO licensing.
- **Object storage engine.** MinIO remains a candidate only.
- **Where engines appear in the interface.** Installed engines are
  machine-level and sit naturally beside runtimes, with instances inside the
  project. That is a navigation decision and is recorded separately.
- **Idle shared instances.** Whether they auto-stop.
- **Engine acquisition and download jobs.** Handled with runtime installation in
  Phase 4 and surfaced in the downloads view.

## Consequences

- Services remain project-first without requiring containers: the project owns
  its data, ports, process and credentials, while binaries are shared.
- Locafy takes on bookkeeping that containers would otherwise provide:
  allocating and tracking ports, supervising processes, and creating and
  initialising data directories per engine. This is substantial per-platform
  work and belongs to DESK-SVC-001 and DESK-APP-002.
- Shared mode trades isolation for memory. It adds a machine-owned instance, a
  binding model and a lifecycle that no single project controls, which makes
  its tests and interface more involved than isolated mode.
- Memory use in isolated mode grows with the number of running instances.
  Stopping a project releases its instances, which keeps idle projects cheap.
- Conflicts with a system-installed engine are avoided by port allocation, but
  port owners must still be shown to the operator (PRD section 11.4).
- Because isolation is by convention, tests for DESK-SVC-001 must prove two
  projects can run the same engine simultaneously without sharing data, ports
  or lifecycle, and that a shared binding never grants access to another
  project's tenant.
