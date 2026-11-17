# ADR-0010: Applications, advisory detection and the dependency graph

- **Status:** Accepted
- **Date:** 2026-11-17
- **Depends on:** ADR-0004 (local data model), ADR-0007 (project storage), ADR-0008 (services and ports), ADR-0009 (project workspace)
- **Affects:** DESK-APP-001, DESK-APP-002, DESK-SVC-001, DESK-PROJ-002

## Context

A project can hold several runnable things: a Laravel API, a Next.js storefront,
a worker. PRD section 8.3 asks Locafy to propose them from what is already in
the folder, while making clear that detection is a suggestion — a folder name or
a manifest is evidence, not proof, and running the wrong command in someone's
repository is worse than asking.

Applications also depend on each other. Starting a worker before its queue, or a
frontend before its API, produces confusing failures; a dependency that loops
back on itself can never be started at all.

This record settles the record shape, where detection looks, and how the graph
is validated. Process supervision, ports and health belong to DESK-APP-002.

## Decision

### The record

An application belongs to exactly one project and is stored in
`applications`:

- `directory` is relative to the project root with forward slashes; `.` is the
  root itself. An absolute path is accepted only when it resolves inside the
  root, and creation refuses anything that resolves outside it, so a record can
  never point Locafy at files the project does not own.
- `language`, `framework`, `marker`, `start_command` and `stop_command` are
  advisory strings copied from detection or typed by the operator. Nothing is
  inferred again at start time.
- `source` is `detected` or `manual`, so the interface can say where a record
  came from without re-running detection.
- `status` holds the configured/process state and starts at `stopped`; runtime
  state in DESK-APP-002 extends it rather than replacing it.
- Removal is a soft delete (`deleted_at`) consistent with projects: files are
  never touched, and a partial unique index on `(project_id, lower(directory))`
  frees the directory for a new record.

### The dependency graph

`application_dependencies(application_id, depends_on_id)` means "this
application needs that one running first". The edge direction is the order of
start, so the reverse order is the safe stop order.

- Edges never cross projects; a dependency must be another live application of
  the same project. The interface has no use for a cross-project start order,
  and it would make one project's start depend on another's.
- Self-dependency is refused in code and by a CHECK constraint.
- The whole graph is validated before any edge is written: the prospective
  edges replace the current ones in memory, a depth-first walk looks for a node
  revisited while still on the current path, and only an acyclic result is
  committed in one transaction. A refused change leaves the previous edges
  exactly as they were.
- Dependencies are replaced as a set, not appended, so the interface can show a
  complete list without a separate remove step.
- Removing an application deletes the edges that name it, so the remaining graph
  never points at a record that is gone.

### Detection

Detection reads file names and two small manifests (`package.json`,
`composer.json`) — it never writes, executes or network-fetches.

- **Where:** the project root, its child folders, and their children. That
  covers a flat project, a folder-per-app project, and the common
  `apps/web` / `packages/api` layout. Deeper trees are not walked, because
  vendored dependencies live there and the result stops being a proposal.
- **What is skipped:** `node_modules`, `vendor`, `.git`, `.next`, `dist`,
  `build`, `bin`, `obj`, `target`, `venv`, `.venv`, `__pycache__`, `coverage`,
  `tmp`, `temp`, and every hidden folder.
- **One proposal per folder,** chosen by a fixed precedence: Node
  (`package.json`), PHP (`composer.json`), Go (`go.mod`), Python (`manage.py`,
  then `pyproject.toml` or `requirements.txt`), Ruby (`Gemfile`), .NET
  (`*.csproj`, then `*.sln`). A folder with several markers gets a stable answer
  rather than an arbitrary one.
- **What is proposed:** language, a framework when a marker proves one (Next.js,
  Vite, Laravel, Django, Rails and their siblings), and a start command only
  when the folder states one — `npm run dev` when that script exists, `php
  artisan serve` with `artisan`, `python manage.py runserver` with `manage.py`,
  `bin/rails server` with a Rails layout, `go run .`, `dotnet run`. Otherwise the
  command is left empty for the operator to fill in, because a guessed command
  is worse than none.
- The operator confirms a proposal into a record and may change every field,
  including the command, before saving.
- Detection results are never stored, so a project that changes on disk is
  re-described on the next look instead of drifting from a cached answer.

## Consequences

- Running detection on a large repository is bounded: three levels, one read of
  the folder per level, two manifest reads per candidate folder.
- A monorepo nested deeper than two levels needs a manual record; the folder
  picker already exists for that case.
- The marker list is data, not behaviour: adding a language means adding a
  branch and its tests, and no schema change.
- Confirmation is not merely a UI step — `Create` accepts an arbitrary command,
  so the trust boundary is "the operator saved this", which DESK-APP-002 must
  respect by running exactly the stored command and nothing composed at runtime.
- Because edges are validated in memory before writing, the cycle rule is tested
  as a pure function as well as through the service.
- Start-order execution, port checks and readiness belong to DESK-APP-002; this
  record only fixes the order the graph describes.
