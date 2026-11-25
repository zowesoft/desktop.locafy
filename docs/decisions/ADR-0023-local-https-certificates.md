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

### The leaf names a revocation list the proxy serves

A client that insists on a definitive revocation answer refuses a certificate
whose chain offers none. The first release documented that; the second looked for
a way to satisfy the client instead, and found one that does not require
regenerating the authority.

### The leaf carries a distribution point, the proxy serves the list

Revocation checking does not require the *authority* to publish anything: it
requires the certificate being validated to name a list. So the leaf issued by the
proxy gains a CRL distribution point — `http://<the bound HTTP address>/.well-known/locafy/ca.crl`
— and the proxy serves an empty, CA-signed CRL there. Existing installations keep
their trusted CA: only the leaf changes, and it is reissued when the address does.

Details that matter:

- **The URL uses the address actually bound.** Port 80 is often taken; the proxy
  falls back to 8080, and the leaf points at whichever listener it has, so the
  check does not depend on a port Locafy does not own.
- **The list is served before hostname routing.** The client fetching it is
  validating a certificate for one of our hostnames, so the fetch must not depend
  on that validation having succeeded. Any `Host` gets the same bytes.
- **The list is CA-signed and dated**, renewed when it is within a day of expiry
  (thirty-day validity), so it never becomes the reason a check fails.
- **Reissue is tied to the address.** A new listener means a new pointer; a
  certificate still naming the old address would fail every revocation check, so
  the leaf is reissued when the address changes, not only when hostnames do.
- **Nothing is revoked**, because nothing can be: the authority is disposable and
  its key lives beside the list. A non-empty CRL is for a future where Locafy
  manages a longer-lived authority.

Measured on Windows against `https://api.health-smoke.test` after trusting the CA:

| Client | Revocation | Result |
|---|---|---|
| PowerShell `Invoke-WebRequest` | as configured (no check) | 200 |
| .NET `HttpClient`, defaults | off | 200 |
| .NET `HttpClient`, `CheckCertificateRevocationList = true` | required | **200** |
| `curl` (schannel) | check on by default | **200**, no flag |
| Chromium | soft-fail for non-public roots | 200, no interstitial |

Before the list existed, the two required-revocation rows were rejections; the
Domains tab said so. The remaining limitation is different: the answer comes from
a loopback listener, so if no listener is running there is nothing to check
against, and a client that requires revocation fails until one is. That is stated
where HTTPS is offered.

Rejected alternatives: an OCSP responder (more moving parts than a static list,
for the same answer), a fixed CRL port independent of the proxy (a second listener
to collide with, and still no answer when Locafy is not running), and putting the
distribution point on the CA (which would mean reissuing the authority and
invalidating every trust already granted).

### Processes Locafy starts trust the CA through the environment

The Windows trust store is not the only store that matters. Node ships its own
Mozilla bundle and ignores the system store, so a Next.js dev server fetching
`https://api.project.test` fails with `UNABLE_TO_VERIFY_LEAF_SIGNATURE` even
after the user trusts Locafy's CA in Windows. The usual workaround is a wrapper
script per project that sets `NODE_EXTRA_CA_CERTS` before booting Next, which
puts developer-machine knowledge into every repository.

Both supervisors reach that helper through a replaceable function, and the helper
takes its certificate directory as a parameter (`CertificateEnvironmentAt`), so
the test suites point at a temporary directory and never create a certificate in
the user's data. The resolved path is cached per directory: every start asks for
the environment, and the answer cannot change while the file stays where the
authority put it.

Locafy starts the process, so it supplies the variable itself: a started
application **and a started service** receive `NODE_EXTRA_CA_CERTS`,
`SSL_CERT_FILE`, `REQUESTS_CA_BUNDLE` and `CURL_CA_BUNDLE` pointing at Locafy's
CA, from one helper (`network.CertificateEnvironment`) so the two supervisors
cannot drift. No script edit,
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
- An OCSP responder, and a CRL with actual revocation entries.
- Injecting the CA into processes Locafy did not start (a terminal, an editor's
  task runner), which is what trusting the CA in the user's store covers.
- A user-configurable CA name and organisation (the generated one is "Locafy
  Local Development CA").
- Public HTTPS, which is the tunnel gateway's job and must never be confused with
  this.
