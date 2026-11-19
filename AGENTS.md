# AGENTS.md — desktop.locafy

Instructions and boundaries for AI coding agents working in this repository.
Product context: see `Locafy-PRD-v1.0.md` in the parent workspace (`../Locafy-PRD-v1.0.md`).

## Purpose

Locafy desktop application: the desktop UI and local engine for managing projects, applications, services, runtimes, domains, environment configuration, logs, terminals, monitoring, and (later) tunnels.

**Approved technology (do not change without a recorded decision):**
Wails v3 · Go · React + TypeScript · Tailwind CSS · shadcn/ui · Lucide · Zustand (client state) · TanStack Query (async state) · SQLite (local database) · xterm.js (terminal) · Go platform-specific process/PTY facilities.

## Local-first (non-negotiable)

Core desktop functionality must work with no Locafy account, no cloud API, and no Internet connection.

- Never require sign-in, cloud reachability, or a network round trip to create or import a project, start or stop applications and services, use local domains, read logs, open a terminal, or view monitoring.
- Cloud features (device registration, tunnels, reserved domains, synchronization) are optional layers on top. When the account, device, or network is unavailable, they degrade to a clearly explained disabled state — never to a broken or blocked local experience.
- Local state is authoritative. Cloud state must never overwrite local project configuration without an explicit user-initiated action.
- A cloud or authentication failure must never abort a local operation that does not inherently depend on it (PRD sections 3.1, 14.3, 25).

## Current status

Repository scaffolded — **no implementation yet** (Phase 0/1 pending per PRD section 20).
Do not implement features before the repository foundation feature `LOC-FOUND-001` is completed.

## Mandatory agent rules (PRD section 21.3)

1. Read this file, `docs/`, and the relevant PRD sections before changing code.
2. Confirm the feature's scope and acceptance criteria in `docs/FEATURES.md` before starting.
3. Check dependencies and existing implementation before starting.
4. Work within this repository unless cross-repository work is explicitly requested.
5. Do not change approved architecture or technology choices without recording a proposal in `docs/decisions/` and receiving approval.
6. Do not implement unrelated features during a task.
7. Add or update tests for changed behavior.
8. Run the relevant test, lint, type-check, and build commands before reporting completion.
9. Report exactly what was changed and which validations passed or failed.
10. Update `docs/FEATURES.md` (status, evidence, blockers) as part of the task.
11. Never mark a feature `COMPLETED` without evidence that its acceptance criteria pass.
12. Document blockers rather than silently bypassing them.

## Cross-repository contracts

- Consume the cloud API contract from `api.locafy` via its published OpenAPI specification (generated or versioned clients) — never hand-roll drifting types.
- The tunnel protocol is specified and versioned in Phase 9 (`TUN-PROTO-001`); do not improvise it.
- Do not copy implementation code between repositories. Shared contracts over shared source.

## Platform policy

**Cross-platform from day one — decided, do not revisit.** Every build must support Windows, macOS, and Linux. Wails v3 is cross-platform, so:

- Platform-specific behavior (process lifecycle and signals, PTY, runtime installation, service management, paths and permissions, local DNS resolution, certificate trust stores, packaging) is implemented and tested per platform as each feature is built — never deferred to a later phase.
- CI builds and tests on all three platforms; a feature is not complete if it works on only one.
- Avoid Windows-only or Unix-only assumptions outside clearly isolated, per-platform code paths.
- Tests are held to the same rule: expectations must not depend on the host running them. Branch filesystem-layout assertions on `runtime.GOOS`, normalise path separators before comparing simulated paths, assert absolute-ness for the platform under test, and never use pid 1 as an unrelated process-tree root. The full patterns and how CI proves them are in `docs/development/README.md` (Cross-platform test portability).

## Decisions you must NOT guess (PRD section 23)

- ~~First supported desktop OS~~ — resolved: cross-platform from day one (see Platform policy and ADR-0001).
- ~~Wails v3 / Go / React versions~~ — verified and recorded in ADR-0002: Wails CLI v3.0.0, Go 1.27.1 (min 1.26), Node 22.14.0, npm 11.19.0. Re-verify before any upgrade.
- Service packaging and installation strategy — undecided (evaluate native vs container-backed in Phases 4–5).
- Object storage engine — MinIO is a candidate, not a decision (Phase 5).
- Reverse proxy — Caddy is the initial candidate, to be validated in Phase 6.
- Local certificates — mkcert is the initial candidate, to be validated in Phase 6.
- Auto-update mechanism — undecided (Phase 10).
- Tunnel gateway technology and transport/protocol — intentionally undecided; do not select them.
- API database and authentication/session design — owned by `api.locafy`; do not assume them here.
- Initial runtime list and supported versions are not finalised (PRD section 9.1) — resolve in Phase 4.
- Where cloud credentials and device identity are stored, and in what form, is undecided — resolve with the cross-client authentication design in Phase 8.

## Safety invariants (PRD sections 3.1, 3.6, 7.2, 8.3, 11.4, 12, 14.3, 18.1)

- Never silently delete project source files or persistent service data; removal from Locafy and data deletion are separate, confirmed operations.
- Store cloud credentials and device tokens using operating-system-appropriate secure storage. Never place them in project files, repository contents, generated logs, or unencrypted application data.
- Never collect or upload project source code, environment files, or logs. Any transfer is an explicit user action with clear disclosure of exactly what is sent.
- Validate user-supplied paths before reading, writing, or executing against them; never follow a path outside the location the user confirmed.
- Bind services to loopback by default; never expose databases or management ports publicly by default.
- Never terminate unrelated processes to resolve port conflicts.
- Framework/language detection is advisory; users must review and confirm before first execution of detected commands.
- Mask secrets; never print secrets in Locafy-generated logs or terminal output.
- Never build shell commands from untrusted input; identify commands before execution.
- Communicate status with text or icons, never color alone.

## Commit messages

Follow Conventional Commits in every commit:

```
<type>(<scope>): short imperative summary

Optional body explaining what changed and why.
```

- Types: `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `chore`, `build`, `ci`.
- Scope: the feature area in kebab-case, e.g. `feat(projects):`, `fix(services):`, `docs(agents):`.
- Subject: imperative mood, no trailing period, 72 characters or fewer.
- Body: separated from the subject by a blank line, wrapped at 72 characters, and explains the motivation and notable decisions. Omit it for trivial changes.
- One logical change per commit. Never mix unrelated work.
- Never add `Co-Authored-By`, `Signed-off-by`, `Generated with`, or any other attribution, trailer, or signature to a commit message. A commit message is the Conventional Commits subject plus an optional explanatory body, and nothing else.

Example:

```
feat(projects): import existing directories as projects

Scan the selected root directory, validate that it exists, and store
project metadata in SQLite without modifying any source files.
```

## Comments in code

- Write self-explanatory code; comment only to explain intent that cannot be expressed in code.
- Never reference the PRD or any other document inside a comment. Context and decisions live in `docs/`.
- Do not leave commented-out code behind; delete it.
- Keep comments brief and professional, written the way an experienced software engineer would write them.

## Documentation ownership

- `README.md` — purpose, setup, development, basic usage
- `AGENTS.md` — this file
- `docs/architecture/` — architecture and technical design
- `docs/features/` — feature specifications and acceptance criteria
- `docs/development/` — local setup, testing, build, release procedures
- `docs/decisions/` — architecture decision records
- `docs/FEATURES.md` — feature tracker
