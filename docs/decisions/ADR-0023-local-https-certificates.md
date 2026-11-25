# ADR-0023: Local HTTPS from a Locafy-generated certificate authority

- **Status:** Proposed — awaiting operator approval
- **Date:** 2026-11-25
- **Depends on:** ADR-0004 (local data model), ADR-0021 (in-process proxy)
- **Affects:** DESK-NET-001

## Context

ADR-0021 left local HTTPS open on purpose: it needs a certificate authority and
a trust-store story, and both are decisions rather than details. PRD section 11.3
names mkcert as the initial candidate and requires that certificate installation,
renewal, trust changes and removal be explained clearly. ADR-0020's admission
rule — an external tool is adopted only when the project publishes builds for
every supported platform — applies to mkcert as it does to service engines, and
mkcert would also mean Locafy shells out for something its own standard library
can do.

Two further facts decide the shape: binding 80 and 443 is privileged on Unix, and
a root certificate is a security-relevant change to the user's machine that must
never happen silently.

## Decision

### Locafy generates its own local CA

On first use, Locafy creates a local certificate authority in its data directory
(`certs/ca.crt`, `certs/ca.key`, key file restricted to the user) with a ten-year
life. Leaves are issued for the current set of local domains as subject
alternative names, valid for a year, and are regenerated when that set changes.
The CA key never leaves the machine and is never written to logs.

- The CA is created lazily, the first time HTTPS is started, not at install.
- One leaf covers every assigned `*.test` hostname; adding or removing a domain
  reissues it and reloads the listener.
- There is no wildcard certificate. A leaf lists exactly the hostnames Locafy
  routes, so a typo in a hostname cannot be silently trusted.

### HTTPS is an explicit, loopback-only listener

The proxy serves TLS on `127.0.0.1`, trying 443 first and falling back to 8443,
mirroring the HTTP fallback (80, then 8080). No port is silently chosen: the
interface states the address it is serving, and a domain URL is shown with the
port when the default was unavailable. Nothing binds outside loopback.

### Trust is a user action, stated plainly

Locafy never installs its CA implicitly. The Domains tab offers "Trust Locafy's
certificate" with the exact command it will run, and the platform determines the
command:

- **Windows:** `certutil -user -addstore Root <ca.crt>` — the current user's
  store, no elevation.
- **macOS:** `security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db <ca.crt>`
  — the login keychain, no elevation.
- **Linux:** the system store needs root and differs per distribution, so Locafy
  prints the command for the detected family instead of escalating itself.

Removal is documented as the inverse of the same store operation, and deleting
the certificate files is the second half. Browsers trust the leaf once the CA is
trusted; until then the certificate warning is expected and explained.

### Applications Locafy starts trust the CA through the environment

The Windows trust store is not the only store that matters. Node ships its own
Mozilla bundle and ignores the system store, so a Next.js dev server fetching
`https://api.project.test` fails with `UNABLE_TO_VERIFY_LEAF_SIGNATURE` even
after the user trusts Locafy's CA in Windows. The usual workaround is a wrapper
script per project that sets `NODE_EXTRA_CA_CERTS` before booting Next, which
puts developer-machine knowledge into every repository.

Locafy starts the application, so it supplies the variable itself: a started
application receives `NODE_EXTRA_CA_CERTS`, `SSL_CERT_FILE`,
`REQUESTS_CA_BUNDLE` and `CURL_CA_BUNDLE` pointing at Locafy's CA. No script edit,
no `NODE_TLS_REJECT_UNAUTHORIZED=0` (which disables verification instead of
adding an anchor), and no effect on the rest of the machine. A variable the
application already sets is left alone, because an application may trust a CA
Locafy knows nothing about; Go and .NET read the system store and need nothing.

### Resolution stays with the hosts file

A `.test` name reaches this machine only because something maps it to
`127.0.0.1`. Editing the hosts file needs elevation on every supported platform,
and a wildcard resolver would mean Locafy running a DNS server on port 53, which
is both privileged and a much larger security surface than a hosts line. Locafy
therefore edits nothing: the Domains tab shows the exact line to add
(`127.0.0.1 api.project.test`) and names the wildcard-resolver alternative, so
the step is explicit and reversible like the trust step.

### Verification, not assurance

The tests verify the generated chain with the CA as root (a TLS client that
trusts only Locafy's CA completes the handshake), that the leaf names exactly the
assigned hostnames, and that an unknown host still gets the plain 404. A green
run proves the certificates and routing; it does not prove that a browser on
another platform trusts the CA, which stays a stated, manual step.

## Consequences

- No mkcert install, version pin or PATH dependency; the trust step is one
  command the user can read before running.
- Locafy owns certificate lifecycle: expiry (regenerate before it lapses),
  rotation on hostname change, and a documented removal path. A ten-year CA with
  yearly leaves keeps the operator's involvement to almost never.
- A private key lives in the data directory. Protecting it matters only on a
  shared machine — a local development CA cannot impersonate anything but
  `*.test` — but the file is still restricted and never logged.
- The Linux trust step is manual; that is a documented limitation, not a hidden
  one.

## Deliberately deferred

- Automatic expiry renewal while Locafy runs without a restart.
- Editing the hosts file or running a local DNS resolver for `.test` names.
- Injecting the CA into processes Locafy did not start (a terminal, an editor's
  task runner), which is what trusting the CA in the user's store covers.
- A user-configurable CA name and organisation (the generated one is "Locafy
  Local Development CA").
- Public HTTPS, which is the tunnel gateway's job and must never be confused with
  this.
