# ADR-0007: Project manifest and project storage model

- **Status:** Accepted
- **Date:** 2026-11-14
- **Depends on:** ADR-0004 (local data model)

## Context

ADR-0004 makes the project the central entity and defers one companion
decision: a portable project manifest, to be designed before project creation is
implemented. That is what this record settles.

Two kinds of project exist in practice, and they need different treatment:

- a project Locafy creates and owns the folder for, inside its own workspace;
- a project that already lives on disk, which Locafy points at without taking
  ownership of the user's files.

Collapsing them would mean either writing into directories Locafy does not own
or refusing to work with existing code.

## Decision

### Two origins, one record

`projects.kind` is `managed` or `linked`.

- **Managed** — Locafy creates the directory under its workspace root
  (`C:\locafy\projects\<slug>` on Windows, `~/locafy/projects/<slug>` on
  macOS and Linux) and owns the folder. The name produces
  a slugged, unique directory name; collisions get a numeric suffix.
- **Linked** — the operator selects an existing directory anywhere on the
  machine. Locafy records the path and never creates, renames, moves or deletes
  anything inside it.

The workspace root is one directory shared by all managed projects, with a
fixed platform default that is visible and predictable: the drive root on
Windows (`C:\locafy\projects`), the home directory elsewhere
(`~/locafy/projects`). The operator can point managed projects at another
location with a stored preference, which is a settings change rather than a
schema change: the column records the resulting path, not where it came
from. Linked projects ignore the workspace root entirely.

A project's root path is unique among projects that have not been removed.
Comparison is case-insensitive, because Windows and default macOS filesystems
are.

### Manifest

A managed project carries `locafy.project.json` at its root:

```json
{
  "schemaVersion": 1,
  "name": "Etugora",
  "description": ""
}
```

- **Written only into managed projects**, which Locafy owns. Linked directories
  are read-only by default: a manifest found there is read if present, never
  created or rewritten.
- **Not authoritative at runtime.** SQLite holds operational state — status,
  lifecycle, configuration, relationships. The manifest carries the portable
  definition: what to recreate when a project is backed up or shared.
- **Versioned from the first release.** `schemaVersion` is required; an unknown
  version is reported rather than guessed at, because silently ignoring fields
  is how portable formats break.
- **No timestamps or machine-derived values**, so the file diffs cleanly and can
  live in a repository.

Manifest export and import across machines is a later feature; this record fixes
the format and the writing rules so project creation does not invent them
underneath.

### Removal never deletes source files

Removing a project is a soft delete (`deleted_at`), for both kinds. Even a
managed project's directory is left on disk, because a folder can contain work
the operator did after creation: generated files, notes, a lockfile they edited.
Deleting the directory is a separate, explicit action that does not exist yet,
and it is deliberately not a side effect of removal.

### Root path changes

Only linked projects can change their root path. A managed project's path is the
workspace directory Locafy created; letting it move would leave an orphaned
folder with no record of why it is there. The operation validates the new
directory, refuses paths already used by another project, and never moves,
copies or deletes anything — it records the new location and nothing else.

## Consequences

- Creating a project writes a directory and a manifest row; linking one writes
  only database rows. A failed insert cleans up the directory it just created.
- Boundaries are explicit in the data: every project-scoped table references
  `project_id`, and the origin says who owns the files.
- A managed project's manifest can drift from its database row if one is edited
  by hand. The database wins until an import feature reconciles them, and the
  manifest is small enough to regenerate.
- Soft deletion means removed projects still hold their rows. Queries must
  filter on `deleted_at IS NULL`; the unique path index is partial for the same
  reason, so a path can be reused after removal.
