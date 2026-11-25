# ADR-0022: One local engine API, with the CLI and MCP server as clients

- **Status:** Proposed — awaiting operator approval
- **Date:** 2026-11-25
- **Depends on:** ADR-0001 (cross-platform), ADR-0004 (local data model), ADR-0013 (process supervision), ADR-0016 (project lifecycle), ADR-0021 (local proxy)
- **Affects:** DESK-CLI-001, DESK-MCP-001, DESK-AGENT-001

## Context

Everything Locafy's engine can do is reachable only through Wails bindings called
from the React window. A developer scripting a start, or an AI assistant asked to
"add this app and run it", has no surface to call: the SQLite database and the
supervised processes are owned by the desktop process, and a second writer would
race the supervisor and the single-instance guard.

## Decision

The engine exposes one local JSON API, and every non-UI client uses it.

- **Transport.** HTTP/JSON on `127.0.0.1`, an ephemeral port, versioned under
  `/v1`, with a bearer token written to the data directory beside the engine's
  record. Loopback only; no LAN binding and no unauthenticated localhost.
- **One implementation.** Handlers call the same packages the Wails bindings
  call (`projects`, `applications`, `scaffold`, `services`, `network`). The API is
  a transport, not a second business layer, and the bindings stay as they are.
- **Discovery.** The engine writes its port and token to a discovery file the CLI
  reads. A stale file produces "the engine is not running", never an implicit
  start.
- **CLI.** `locafy` is a thin client: JSON-first (`--json`), stable exit codes,
  idempotent lifecycle verbs.
- **MCP.** A separate stdio server (`locafy mcp`) translates MCP tools into the
  same API. Tools are typed; there is no "run a command" tool. Writes require the
  server to be started with `--allow-writes`, and removal tools are not exposed in
  the first version.
- **Attribution.** Mutations through either client write the activity entry the
  interface writes, with an actor (`user`, `cli`, `mcp`), so a project's history
  does not pretend a human clicked.

## Consequences

- The engine gains a public contract to keep stable while the interface is free to
  change; versioning under `/v1` is what makes that possible.
- Authentication is required on loopback even though the listener never leaves the
  machine: any local process can connect otherwise, and the token costs one file.
- A CLI and an MCP server become build and release artefacts of their own
  (packaging, version skew with the desktop app, and a name collision with
  `locafy.exe` to resolve).
- Headless use in CI needs an engine without a window; that is deliberately not
  decided here.

## Deliberately deferred

- `locafy serve --headless` and its single-instance and logging rules.
- An HTTP MCP transport for remote agents; it belongs with the tunnel work.
- The in-app assistant, which is a client of this API and a separate decision
  (model, key, cost limit).
