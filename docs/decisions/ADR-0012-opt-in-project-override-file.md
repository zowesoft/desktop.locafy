# ADR-0012: Project settings live in SQLite; the override file is opt-in

- **Status:** Accepted
- **Date:** 2026-11-18
- **Supersedes:** the manifest writing and authority rules in ADR-0007

## Context

ADR-0007 had project creation write `locafy.project.json` into every managed
project. In practice that puts a Locafy file into directories the operator
sees and versions as their own, before they have asked for one, and the file
carried nothing the database did not already hold. Project settings belong to
the application's own store; a file in the project folder should only exist
because someone wants to change those settings from outside the application.

## Decision

### The database is the store

SQLite holds project settings for managed and linked projects alike. Creating a
project — blank, imported or scaffolded — writes its directory and its rows and
nothing else.

### `locafy.project.json` is an opt-in override

`internal/projects` exposes two operations:

- **Override file status** — the file's path in the project root and whether it
  exists.
- **Set override file** — writes the file with the stored values when enabled,
  and removes it when disabled.

The file is created only by that explicit request, from the project's Settings
tab. The operator turns it on to override a setting from the project folder and
off to return to the stored values.

When the file exists and parses with `schemaVersion: 1`, its `name` and
`description` override the stored values wherever a project is read, and the
project list is ordered by the effective name. Anything malformed, unknown or
empty is ignored per field, so a broken file never hides a project. Renaming in
Locafy rewrites the file, so an override the operator opted into never masks
their own edit.

Linked projects may carry an override file too: writing it is always the
operator's explicit choice, so it no longer contradicts ADR-0007's rule that
Locafy never writes into a linked directory on its own.

## Consequences

- Project folders stay clean by default; the file appears only where an
  override was asked for, and disappears when the override is turned off.
- Two sources exist for name and description, with a stated precedence: the
  file wins while it is enabled, and the database wins otherwise.
- Removing a project still never deletes files, including an override file.
- Later settings that need overriding extend the file's schema, which is
  versioned from the start.
