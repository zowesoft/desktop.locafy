# ADR-0008: Service engines and service instances

- **Status:** Accepted
- **Date:** 2026-11-15
- **Depends on:** ADR-0001 (cross-platform), ADR-0004 (local data model), ADR-0007 (project storage)
- **Related:** ADR-0009 (navigation and project workspace)
- **Affects:** DESK-SVC-001, DESK-NET-001, DESK-RUN-001, DESK-DL-001

## Context

A service is infrastructure that applications use, and it belongs to a project.
The engine behind it, such as PostgreSQL or Redis, is a program that is
installed once on the machine. Without containers, nothing in the operating
system ties an installed engine to a single project, so the question is what
makes a service project-scoped.

Installing an engine and running a service are different operations. This
record separates them and settles how projects get service instances: through
one shared instance, or through an isolated instance of their own. It does not
decide how engines are obtained or packaged, which remains open.

## Decision

**Install globally. Configure and run per project.**

### Engines and instances

| | Engine | Service instance |
|---|---|---|
| What it is | An installed program at a specific version | One configured, runnable server of an engine |
| Scope | Machine | Machine (shared) or project (isolated) |
| Examples | PostgreSQL 17 binaries | The shared PostgreSQL, or Etugora's own PostgreSQL |
| Cardinality | One per engine and version | At most one shared instance per engine version; any number of isolated ones |

This mirrors runtimes and applications: a runtime is machine-level, and an
application selects one. A service instance selects an engine version.

An engine is never started directly. Only instances run.

### What an instance owns

Every instance owns four things that no other instance shares:

1. **Data directory.** One per instance, never shared between instances. Its
   location and persistence behaviour are visible to the operator.
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

**Shared (default where the engine supports it).** One instance is owned by the
machine, and several projects bind to it. Each binding gives a project its own
logical tenant on the server: a database and role for relational engines, a
bucket and credentials for object storage. Binding credentials go to the
credential store.

- A shared instance stops only when the operator stops it. Stopping or removing
  a project never stops it, and there is no automatic idle stop.
- Server-level configuration, engine version, restarts and resource use are
  shared by every bound project, and the interface says so wherever the mode is
  chosen.
- Removing a project or a binding never drops its databases or buckets. That is
  a separate, confirmed data-deletion action, consistent with ADR-0007.
- A binding never grants access to another project's tenant.

**Isolated.** The instance is owned by exactly one project. Two projects using
the same engine version share binaries and nothing else:

```text
engines/postgresql/17/bin/                  one installed engine
services/<instance-a>/data                  project A: own port, process, secrets
services/<instance-b>/data                  project B: own port, process, secrets
```

Project lifecycle is the instance lifecycle: starting a project starts its
isolated instances, and stopping it stops only those.

Moving a project between modes is a dump-and-restore migration, never a toggle.

### Default mode

The default is a stored preference with two values: **shared** and **isolated**.
It starts as shared.

- It applies when a service is attached to a project. An engine whose template
  declares tenancy follows the preference. An engine without tenancy is always
  isolated, whatever the preference says, because there is nothing to share
  safely.
- The operator can override the mode for a single service when attaching it.
- Changing the preference affects only services attached afterwards. Existing
  instances keep their mode.
- First-run onboarding is one place to ask. The preference is also changeable at
  any time in settings, and Locafy works with the shared default if onboarding
  is skipped. It never requires an account.

### Engine capabilities

Every service template declares what its engine supports, so Locafy never
assumes that every engine behaves like PostgreSQL:

- multiple instances on one machine
- custom ports
- independent data directories
- automatic initialisation
- health checks
- tenancy, and the kind of tenant it provides (database and role, or bucket and
  user), which is what makes shared mode available

PostgreSQL and MySQL/MariaDB provide databases and roles, and object storage
provides buckets and users. Redis and mail capture offer no meaningful
per-project separation and are isolated-only unless a template proves
otherwise. An engine that cannot run more than one instance, or has fixed ports,
is reported as such rather than failing at start.

### Data location

Instance data lives in a central Locafy-managed directory under the platform
data home, keyed by instance ID so a rename does not move it:

```text
<data home>/Locafy/services/<instance-id>/
```

The location is configurable per instance. Locafy never places service data
inside a linked project directory unless the operator chooses that location
explicitly, because ADR-0007 forbids writing into directories Locafy does not
own. When the operator chooses a project directory or a cloud-synchronised
folder, the interface warns that database files in version-controlled or
synchronised locations can be committed or corrupted.

### Rules

- Locafy-managed engines live under Locafy's own directory, are not added to
  the system `PATH`, and are distinguishable from system installations. Locafy
  never starts, stops, reconfigures or upgrades a system-installed engine, and
  never binds a port it did not allocate.
- An instance records the engine version it was initialised with. Changing
  versions is an explicit operation, because data directories are not portable
  across major versions.
- Removing an instance configuration never deletes its data directory.
- Isolation is by separate directories, ports, processes and loopback binding,
  and in shared mode by separate tenants. It is not enforced by the operating
  system, and the interface and documentation must not describe it as
  sandboxing.
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
  directory, configuration, and a scope of `project` or `shared`. Isolated
  instances reference `project_id` with `ON DELETE CASCADE`; shared instances
  have no project and are owned by the machine.
- A binding table links a project to a shared instance and records the tenant
  (database and role, or bucket) and the credential reference. Binding rows
  cascade with the project; the tenant's data does not.
- Allocated ports are named, per instance, in `port_allocations`.
- Running processes are recorded in the existing planned `processes` table,
  shared with applications, including process identity.
- Configured `status` and runtime `lifecycle_state` stay separate columns.
- The default-mode preference is a typed value in the existing `settings`
  table.

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
- **Engine acquisition and download jobs.** Handled with runtime installation in
  Phase 4 and surfaced in the downloads view.

## Consequences

- Services remain project-first without requiring containers: each project has
  its own tenant or its own instance, with its own credentials, while binaries
  are shared.
- Making shared the default favours a light machine: one server process per
  engine rather than one per project. It also weakens isolation compared with
  isolated instances. Projects on one shared server share its configuration,
  version, restarts and failures, and a heavy query or a crash in one project
  can affect another. The interface must make this visible where the mode is
  chosen. PRD section 10.4 is met through separate tenants for engines that
  support them.
- Locafy takes on bookkeeping that containers would otherwise provide:
  allocating and tracking ports, supervising processes, creating and
  initialising data directories per engine, and provisioning and tracking
  tenants. This is substantial per-platform work and belongs to DESK-SVC-001
  and DESK-APP-002.
- A shared instance has a lifecycle that no single project controls, so
  project start must start or reuse it, and project stop must leave it running.
  Memory is released only when the operator stops it.
- Conflicts with a system-installed engine are avoided by port allocation, but
  port owners must still be shown to the operator (PRD section 11.4).
- Because isolation is by convention, tests for DESK-SVC-001 must prove two
  projects can run the same engine simultaneously without sharing data, ports
  or lifecycle in isolated mode, and that a shared binding never reaches
  another project's tenant.
