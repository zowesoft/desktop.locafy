# CLI, MCP and the in-app assistant — plan

Status: proposed, not started. Tracked as DESK-CLI-001, DESK-MCP-001 and
DESK-AGENT-001 in [`../FEATURES.md`](../FEATURES.md); the architectural decision
is [ADR-0022](../decisions/ADR-0022-engine-api-cli-and-mcp.md). This document is
the specification those features move to `READY` against; it is not an
implementation.

## Why

Locafy's engine already knows how to create a project, detect applications,
scaffold a framework, start a dependency-ordered project and route a domain —
but only through a Wails binding called from the React window. Two kinds of user
cannot use any of it:

- a developer in a terminal or a script, who wants `locafy start` without
  clicking;
- an AI assistant, which needs a stable, documented, machine-readable surface
  and cannot drive a WebView.

Doing the first well produces the second: one local API, two clients. The
in-app assistant then becomes a third client of the same tools instead of a
parallel integration.

## Principles

1. **One engine.** The CLI and MCP server never open the SQLite database, never
   spawn a process and never route a request themselves. They call the running
   engine, which already owns those things. A second writer would race the
   supervisor and the single-instance guard.
2. **Machine-readable first.** Every command has a JSON form (`--json`) with a
   stable shape; human output is a rendering of the same data. Errors carry a
   code and a message, and the exit status distinguishes "bad input" from
   "engine refused" from "engine unavailable".
3. **Read before write.** Reads need no confirmation; writes are explicit verbs.
   Destructive actions (`remove`, `stop --force`) need an explicit flag from the
   CLI and an explicit confirmation parameter from MCP, and the MCP surface
   should default to read-only unless the client was started with write access.
4. **Attribution.** Every CLI/MCP mutation writes the same activity entry the
   interface writes, with an actor (`user`, `cli`, `mcp`, `assistant`). Today's
   activity rows have no actor column; adding one is part of the API work.
5. **Loopback and a token.** The API listens on `127.0.0.1` on an ephemeral port
   and requires a bearer token stored beside the engine's data with restrictive
   permissions. No LAN binding, no unauthenticated localhost, no token in logs.
6. **No shell.** MCP exposes typed operations, never "run this command".

## Architecture

```
locafy CLI ─┐
MCP server ─┼─ HTTP/JSON, loopback + token ─→ engine process (DB, supervisor, proxy)
assistant ──┘                                  └─ same service packages the Wails bindings call
```

- **`internal/api`** — a versioned JSON API (`/v1/...`) over the existing
  packages. It is a transport, not a second implementation: handlers validate
  input, call `projects`, `applications`, `scaffold`, `services`, `network`,
  and serialise the result. The Wails bindings stay as they are.
- **Discovery** — the engine writes `%APPDATA%/Locafy/engine.json` with the port,
  the token and the engine version. The CLI reads it; if it is stale it probes
  the recorded port once and reports "the engine is not running" rather than
  starting one implicitly.
- **`locafy` binary** — a thin client. `locafy-cli` is the working name if the
  `locafy` name collides with the desktop executable; the collision is an open
  question below.
- **MCP server** — a separate stdio process (`locafy mcp`), so any MCP client
  can launch it without knowing about Wails. Tools are a curated subset of the
  API; each declares its JSON schema and whether it mutates. Resources expose
  project lists and status as read-only documents.

### Headless / CI

The API needs an engine. Running the desktop window in CI is wrong; a
`locafy serve --headless` mode (engine without a window) is the likely answer,
but it changes startup, single-instance behaviour and where logs go, so it is
its own decision and its own slice. Until then the CLI requires the desktop app
to be running, and says so.

## Command surface

| Command | Does | JSON |
|---|---|---|
| `locafy status` | engine up/down, version, proxy state, counts | yes |
| `locafy projects` | projects with derived lifecycle | yes |
| `locafy project <id\|name>` | one project, its applications and services | yes |
| `locafy project create <name> --path <dir> [--kind managed\|linked]` | create a project | yes |
| `locafy project import <path>` | import an existing folder | yes |
| `locafy apps <project>` | detected and recorded applications | yes |
| `locafy app add <project> --name --dir --command` | record an application | yes |
| `locafy start <project>` / `stop <project>` / `restart <project>` | lifecycle actions, component results | yes |
| `locafy services <project>` / `service add <project> --engine postgresql` | service list and attach | yes |
| `locafy domain add <project> <application> <hostname>` / `domain rm <hostname>` | local domains | yes |
| `locafy logs <project> [--app <id>] [--follow]` | recent output, optional stream | lines / NDJSON |
| `locafy scaffold <framework> --dir <path>` | framework scaffold via the job system | yes |
| `locafy mcp` | start the MCP server on stdio | — |

Rules: `<project>` accepts id or unambiguous name; commands are idempotent where
the operation is (`start` on a running project reports `already running` instead
of failing); `--json` is the documented interface, prose output is for humans;
exit codes are `0` ok, `2` usage, `3` refused by the engine, `4` engine not
running, `5` unexpected failure.

## MCP surface

Tools (read-only unless marked):

| Tool | Arguments | Notes |
|---|---|---|
| `locafy_status` | — | engine, proxy, counts |
| `locafy_list_projects` | — | id, name, derived state |
| `locafy_get_project` | `project` | components and their status |
| `locafy_create_project` | `name`, `path`, `kind?` | write |
| `locafy_import_project` | `path` | write |
| `locafy_list_applications` | `project` | detection included |
| `locafy_add_application` | `project`, `name`, `directory`, `command` | write |
| `locafy_start_project` / `locafy_stop_project` | `project`, `confirm?` for stop | stop is destructive-adjacent |
| `locafy_assign_domain` | `project`, `application`, `hostname` | write, validated |
| `locafy_read_logs` | `project`, `application?`, `lines?` | read |

Safety: writes allowed only when the server runs with `--allow-writes`; removal
tools are not exposed at all in the first version; every call is logged with
actor `mcp`. Resources: `locafy://projects`, `locafy://project/{id}`.

## In-app assistant (later)

A panel that answers questions and performs actions through the same tool
surface, server-side, with the owner's model key (the pattern the AI templates
already use: key stays in `.env`/Sites secrets, never in the client). It can
create a project, add an application, start it and explain the failure from the
same JSON the CLI prints — no fourth integration. It is deliberately last: the
tool surface must be stable before a conversational layer is worth building.

## Phasing

- **A — read-only API + CLI reads** (`status`, `projects`, `apps`, `logs`).
- **B — writes** (`project create/import`, `app add`, `start/stop`, domains),
  activity attribution, `--json` frozen, docs.
- **C — MCP server**, read tools first, then writes behind `--allow-writes`.
- **D — in-app assistant** over the same tools.

Each phase ships with tests against the HTTP API (not the Wails bindings) and a
live check on Windows; macOS/Linux remain CI-only until a machine is available.

## Open questions (decisions before implementation)

- **Binary name.** The desktop executable is `locafy.exe`; a CLI called `locafy`
  collides on Windows PATH. Options: rename the desktop binary, ship the CLI as
  `locafy-cli`, or put the CLI behind `locafy cli` inside the desktop binary.
- **Headless mode.** Separate decision: engine without a window, its single
  instance rules, and where its logs go.
- **Token lifetime.** A token per engine run, rotated on start, versus a
  long-lived token in the data directory. Per-run is safer and cheap because the
  CLI discovers the file each time.
- **MCP transport.** stdio only for the first version; whether an HTTP transport
  is needed for remote agents belongs with the tunnel work.
- **Assistant provider and cost.** Which model, whose key, and what the daily
  limit is — the template pattern exists, but this is an owner decision.

## Acceptance criteria (first slice, phase A)

- `locafy status` reports engine version, uptime and proxy state with the engine
  running, and exit code 4 with a clear message with it stopped.
- `locafy projects --json` returns the same projects and derived states the
  dashboard shows, verified side by side.
- The API refuses requests without the token, and the token file is not readable
  by other users.
- A stale discovery file produces one clear "engine is not running" message, not
  a hang or a stack trace.
- No code path in the CLI opens the SQLite database or spawns a process.
