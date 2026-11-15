# ADR-0009: Global navigation and the project workspace

- **Status:** Accepted
- **Date:** 2026-11-15
- **Depends on:** ADR-0004 (local data model), ADR-0007 (project storage)
- **Related:** ADR-0008 (service engines and instances)
- **Affects:** DESK-UI-001, DESK-PROJ-001, DESK-PROJ-002, DESK-APP-001, DESK-SVC-001, DESK-NET-001
- **Amends:** PRD section 6.2 (main navigation)

## Context

The sidebar currently lists 14 flat entries. Some are machine-wide, and others
describe things that belong to a project. Applications, services and domains
are project resources: an application is a workload of one project, a service
is infrastructure attached to one project, and a domain routes to one
application or service. Presenting them as top-level lists detaches them from
the project that owns them, which the PRD warns against when it says the project
overview should "avoid presenting applications and services as unrelated global
lists".

The second mockup also puts a list of projects in the sidebar. That was
reviewed and rejected: projects get their own dashboard instead.

## Decision

### Sidebar: global items only

The sidebar holds machine-level entries and the projects entry point. It holds
no list of projects.

| Entry | Scope |
|---|---|
| Overview | Global |
| Projects | Global hub: the projects dashboard |
| Runtimes | Global |
| Downloads | Global |
| Analytics | Global, filterable by project |
| Logs | Global, filterable by project |
| Terminal | Global, opens at machine scope |
| Tunnels | Global view of tunnels; planned |
| Extensions | Global; planned |
| Help & Support | Global |
| Settings | Global |

Applications, Services and Domains leave the sidebar and become tabs of the
project workspace. The sidebar therefore has 11 entries. Tunnels and Extensions
keep the planned and disabled treatment everywhere they appear.

### Projects dashboard

`Projects` opens a dashboard that lists every project: name, description,
origin (managed or linked), lifecycle state, and its applications and services
in summary. It offers search, creating a managed project, and linking an
existing directory. It is backed by the projects service. Opening a project
enters its workspace.

### Project workspace

A project opens into its own workspace, where everything scoped to the project
is a tab:

- Overview
- Applications
- Services
- Domains
- Tunnels (planned)
- Environment
- Logs
- Terminal
- Settings

A tab whose feature is not built yet shows the existing not-built state rather
than disappearing, so the hierarchy is visible from the start. The workspace
shows project-scoped settings separately from global settings, which keeps the
PRD requirement that the two stay distinguishable.

Logs and Terminal exist at both levels. The global entries work at machine scope
and can be filtered by project; the workspace tabs are fixed to their project.
Every metric and log states its scope.

### Engines

Installed service engines are machine-level (ADR-0008). They are managed
alongside runtimes rather than inside a project, and service instances appear in
the project's Services tab. The exact screen is settled with DESK-RUN-001.

## Not decided here

- **Project switcher in the top bar.** Whether the workspace shows a chip for
  switching between projects, or switching always goes through the projects
  dashboard.
- **Whether the workspace gets a Processes tab.** It depends on process
  supervision, which does not exist yet.

## Consequences

- PRD section 6.2 lists Applications, Services and Domains as main navigation
  entries. This record amends that for the desktop; the PRD text should be
  updated to match.
- The navigation tests, the router and the search results that assume 14 flat
  entries change with the implementation. Search should keep finding projects
  and, within a project, its resources.
- The global Overview stays a machine-level summary. Project detail lives in
  the workspace, not on the overview.
- Features that placed their screens in global navigation (DESK-APP-001,
  DESK-SVC-001, DESK-NET-001) now deliver project tabs instead.
- A missing project tab never blocks navigation to the others, and
  disabled planned entries never lead to a broken screen.
