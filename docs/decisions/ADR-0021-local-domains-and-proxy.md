# ADR-0021: Local domains served by an in-process reverse proxy

- **Status:** Proposed — awaiting operator approval
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
  reserved (`*.locafy.dev`), outside the development suffix, or pointing at an
  application with no port is refused with a reason on the record.

### Domains are records, not configuration text

A domain is a row: id, project, application, hostname, timestamps, soft delete.
Uniqueness is a database index on the lowercased hostname, so two projects
cannot claim the same name even across restarts. The hostname is normalised to
lowercase at write time; the interface and the proxy use the same value.

### HTTPS is deferred, not guessed

Local HTTPS needs a locally trusted certificate authority and a documented
trust-store step; doing that well is its own decision (generated CA versus
mkcert, where the certificate lives, how removal works). This record does not
choose one, and DESK-NET-001 stays open on that criterion. HTTP routing is
complete without it.

## Consequences

- A dependency disappears: no Caddy install, version pin or configuration
  reload, and the router ships with the application on every platform.
- Locafy owns behaviour a proxy binary would have provided: request logging,
  header forwarding (`X-Forwarded-For`, `X-Forwarded-Host`, `X-Forwarded-Proto`),
  error responses and upgrade handling. The surface is small and tested.
- The listener needs a port. Binding 80 is the natural choice for
  `api.etugora.test` without a port suffix; on Unix that needs privileges the
  desktop app does not have, and on any platform the port may be taken. The
  proxy reports a failed bind as its status with the reason, and routing stays
  inspectable in the interface.
- Certificate trust, renewal and removal remain an open decision, so HTTPS is
  explicit in the tracker rather than a half-built switch.

## Deliberately deferred

- Local HTTPS and the certificate authority/trust-store decision.
- A user-configured development suffix (the first slice accepts `.test`).
- Path-based routing and wildcard hostnames.
- Proxy access logs as durable records; they belong with DESK-DEVX-001.
