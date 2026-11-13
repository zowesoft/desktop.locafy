# AGENTS.md — desktop.locafy

Instructions and boundaries for AI coding agents working in this repository.
Product context: see `Locafy-PRD-v1.0.md` in the parent workspace (`../Locafy-PRD-v1.0.md`).

## Purpose

Locafy desktop application: the desktop UI and local engine for managing projects, applications, services, runtimes, domains, environment configuration, logs, terminals, monitoring, and (later) tunnels.

**Approved technology (do not change without a recorded decision):**
Wails v3 · Go · React + TypeScript · Tailwind CSS · shadcn/ui · Lucide · Zustand (client state) · TanStack Query (async state) · SQLite (local database) · xterm.js (terminal) · Go platform-specific process/PTY facilities.

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

## Decisions you must NOT guess (PRD section 23)

- First supported desktop OS — undecided; Windows is a practical development target, not a support commitment. Resolve in Phase 0.
- Wails v3 / Go / React versions — verify against official compatibility sources when initializing; do not hardcode unverified versions into scripts.
- Service packaging and installation strategy — undecided (evaluate native vs container-backed in Phases 4–5).
- Object storage engine — MinIO is a candidate, not a decision (Phase 5).
- Reverse proxy — Caddy is the initial candidate, to be validated in Phase 6.
- Local certificates — mkcert is the initial candidate, to be validated in Phase 6.
- Auto-update mechanism — undecided (Phase 10).
- Tunnel gateway technology and transport/protocol — intentionally undecided; do not select them.
- API database and authentication/session design — owned by `api.locafy`; do not assume them here.

## Safety invariants (PRD sections 3.6, 7.2, 8.3, 11.4, 12, 18.1)

- Never silently delete project source files or persistent service data; removal from Locafy and data deletion are separate, confirmed operations.
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
