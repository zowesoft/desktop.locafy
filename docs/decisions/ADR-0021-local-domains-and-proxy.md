# ADR-0021: Local domains served by an in-process reverse proxy

- **Status:** Accepted — 2026-11-26, after the implementation review at the end of this record
- **Date:** 2026-11-25
- **Depends on:** ADR-0004 (local data model), ADR-0010 (applications and dependency graph), ADR-0014 (application ports and ownership), ADR-0009 (project workspace tabs)
- **Affects:** DESK-NET-001

## Context

DESK-NET-001 asks for readable local hostnames routed to applications
(`api.etugora.test`), a reverse proxy that survives configuration changes,
WebSocket proxying, conflict reporting and local HTTPS. PRD section 11 names
Caddy for the proxy and mkcert for certificates, and asks that both be
validated before they are adopted.

Both are external binaries. ADR-0020's admission rule — an engine is admitted
only when the project publishes builds for the platforms Locafy supports — applies
to them in the same way, and both bring a second problem the proxy does not
need to have: Caddy owns the listener and its configuration file, so every
routing change is a config write, an external validation run and a reload whose
failure mode is a half-applied configuration; mkcert writes to the operating
system's trust store, which needs elevation and an explanation the user must
accept.

Locafy already supervises processes and allocates ports; the routing itself is a
table lookup from `Host` to `127.0.0.1:<port>`, which Go's standard library can
serve.

## Decision

### Locafy serves the proxy itself

The reverse proxy is part of the desktop process: `net/http` with
`httputil.ReverseProxy` per route, listening on loopback only. No Caddy binary,
no configuration file, no reload protocol.

- **Routing** is a map from a validated hostname to the owning application's
  discovered port, rebuilt from the database whenever a domain, an application
  port or a project changes.
- **Reloads are validated by construction.** Hostnames are validated when they
  are written and conflicts are refused there; a route is only added when the
  application has a live port. A rebuild swaps one map atomically, so a request
  sees either the old table or the new one, never a partial configuration. An
  unknown host answers 404 naming the host rather than proxying anywhere.
- **WebSocket** upgrades pass through `httputil.ReverseProxy`, which forwards
  `101 Switching Protocols` connections; no separate path is required.
- **Loopback only.** The listener binds `127.0.0.1`; the proxy never exposes an
  application to the network, matching PRD section 11.4.
- **Conflicts are reported, not resolved by force.** A hostname claimed twice,
  reserved (`*.locafy.dev`), outside the development suffix or malformed is
  refused with a reason on the record. A domain assigned before its application
  has bound a port is kept as a valid record that routes nothing, so an
  application can be named before it first runs.

### Domains are records, not configuration text

A domain is a row: id, project, application, hostname, timestamps, soft delete.
Uniqueness is a database index on the lowercased hostname, so two projects
cannot claim the same name even across restarts. The hostname is normalised to
lowercase at write time; the interface and the proxy use the same value.

### HTTPS is issued locally rather than delegated

This record first deferred local HTTPS to a separate decision. That decision is
ADR-0023: Locafy generates its own local authority instead of adopting mkcert,
serves TLS on a second loopback listener beside the HTTP one, and leaves the
trust-store step to an explicit user action. The proxy keeps no certificate
state beyond the leaf it serves.

## Consequences

- A dependency disappears: no Caddy install, version pin or configuration
  reload, and the router ships with the application on every platform.
- Locafy owns behaviour a proxy binary would have provided: request logging,
  header forwarding (`X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto`),
  error responses and upgrade handling. The surface is small and tested.
- The listener needs a port. Binding 80 is the natural choice for
  `api.etugora.test` without a port suffix; on Unix that needs privileges the
  desktop app does not have, and on any platform the port may be taken. The
  proxy tries 80 then 8080 (and 443 then 8443 for TLS), reports the address it
  actually bound, and shows that address in the Domains tab, because a URL with
  `:8080` is not the same promise as one without.
- Certificate trust, renewal and removal belong to ADR-0023. Trust stays a user
  action; removal is the inverse of the same store operation plus deleting the
  files.

## Deliberately deferred

- ~~Local HTTPS and the certificate authority/trust-store decision~~ —
  resolved by ADR-0023.
- A user-configured development suffix (the first slice accepts `.test`).
- Path-based routing and wildcard hostnames.
- Proxy access logs as durable records; they belong with DESK-DEVX-001.

## Implementation review (2026-11-26)

Checked against the code that shipped in `internal/network`, not against the plan:

- **Matches.** The proxy is `net/http` + `httputil.ReverseProxy` with one
  loopback listener pair and no configuration file; the table is rebuilt from
  `domains` once a second, compared as hostname-to-target pairs, and swapped
  only when it changed. `SetXForwarded` supplies `X-Forwarded-For`,
  `X-Forwarded-Host` and `X-Forwarded-Proto`, the browser's `Host` is preserved,
  and WebSocket upgrades pass through unmodified. An unknown host answers 404
  naming it; an unreachable upstream answers 502 naming the target. Claims are
  refused by a unique index on `lower(hostname)` and surfaced as their own
  message, and a removed project stops routing because the routing query joins
  live projects.
- **Corrected here.** The conflict rule above now says what the code does: a
  portless application may hold a domain, and the route simply waits for a port.
  The earlier text claimed that case was refused. The port paragraph now names
  the 8080/8443 fallbacks that the code has had since the first bind attempt.
- **Open.** macOS and Linux have not run the proxy or the TLS listener; the
  per-platform evidence is listed in `docs/FEATURES.md` under DESK-NET-001.
