# FEATURES.md — desktop.locafy

Feature tracker per PRD section 21. Statuses: `NOT_STARTED` · `READY` · `IN_PROGRESS` · `BLOCKED` · `IN_REVIEW` · `TESTING` · `COMPLETED`.

Rules: a feature is never marked `COMPLETED` without evidence that its acceptance criteria pass; blockers are documented, never bypassed.

## Summary

| ID | Feature | Repository | Phase | Status | Dependencies |
|---|---|---|---|---|---|
| LOC-FOUND-001 | Repository and engineering standards (shared) | All | 0 | IN_PROGRESS | — |
| DESK-BOOT-001 | Wails desktop bootstrap | desktop.locafy | 1 | IN_PROGRESS | LOC-FOUND-001 |
| DESK-DB-001 | SQLite persistence and migrations | desktop.locafy | 1 | IN_PROGRESS | LOC-FOUND-001 |
| DESK-DB-002 | Control-plane schema foundation | desktop.locafy | 1 | IN_PROGRESS | DESK-DB-001 |
| DESK-UI-001 | Navigation and design system | desktop.locafy | 1 | IN_PROGRESS | DESK-BOOT-001 |
| DESK-PROJ-001 | Create and import projects | desktop.locafy | 2 | COMPLETED | DESK-BOOT-001, DESK-DB-001, DESK-UI-001 |
| DESK-PROJ-002 | Project overview and lifecycle | desktop.locafy | 2 | IN_PROGRESS | DESK-PROJ-001 |
| DESK-APP-001 | Add and detect applications | desktop.locafy | 3 | NOT_STARTED | DESK-PROJ-002 |
| DESK-APP-002 | Application process lifecycle | desktop.locafy | 3 | NOT_STARTED | DESK-APP-001 |
| DESK-RUN-001 | Runtime detection and assignment | desktop.locafy | 4 | NOT_STARTED | DESK-APP-002 |
| DESK-DL-001 | Downloads view for install jobs | desktop.locafy | 4 | NOT_STARTED | DESK-RUN-001 |
| DESK-SVC-001 | Service catalogue and lifecycle | desktop.locafy | 5 | NOT_STARTED | DESK-APP-002 |
| DESK-NET-001 | Local domains and reverse proxy | desktop.locafy | 6 | NOT_STARTED | DESK-APP-002, DESK-SVC-001 |
| DESK-DEVX-001 | Integrated terminal and logs | desktop.locafy | 7 | NOT_STARTED | DESK-PROJ-002, DESK-APP-002, DESK-SVC-001 |
| DESK-ANAL-001 | Analytics views | desktop.locafy | 7 | NOT_STARTED | DESK-DB-002, DESK-DEVX-001 |
| TUN-PROTO-001 | Tunnel threat model and protocol (shared) | Gateway · Desktop · API | 9 | NOT_STARTED | API-AUTH-001, API-DEVICE-001, DESK-NET-001 |
| TUN-AGENT-001 | Desktop tunnel agent | desktop.locafy | 9 | NOT_STARTED | TUN-PROTO-001 |
| TUN-E2E-001 | End-to-end public tunnel flow (shared) | Gateway · Desktop · API | 9 | NOT_STARTED | TUN-PROTO-001, TUN-AGENT-001, TUN-GATE-001 |
| DESK-HELP-001 | Help and diagnostics export | desktop.locafy | 10 | NOT_STARTED | DESK-BOOT-001 |
| DESK-EXT-001 | Extensions platform | desktop.locafy | 11 | NOT_STARTED | DESK-UI-001, DESK-DB-001 |

---

## Entries

### LOC-FOUND-001 — Repository and engineering standards (shared, all repositories)

- **Repository:** All — desktop.locafy, web.locafy, api.locafy, gateway.locafy. This ID is tracked once per repository; acceptance criteria are specialised per repository, and the feature is only complete for a given repository when that repository's criteria pass.
- **Description:** Establish repository foundation: buildable/testable repo, documentation structure, formatting/linting/testing/CI/release conventions, verified toolchain versions, initial architecture decisions recorded.
- **Dependencies:** —
- **Acceptance criteria:** Repo can be built and tested independently; boundaries documented; CI builds and tests on Windows, macOS, and Linux; initial architecture and contracts reviewed; no unresolved foundational decision hidden in implementation assumptions (PRD section 20, Phase 0 exit criteria).
- **Status:** IN_PROGRESS
- **Assigned agent:** —
- **Files/modules:** Repository root, CI configuration, `docs/`
- **Tests:** CI pipeline runs build + tests
- **Blockers:** This repository's three-platform criterion is met: `.github/workflows/ci.yml` runs on `windows-latest`, `macos-latest` and `ubuntu-latest` and is green on GitHub at `b373d08` (run 43586549586) — Go build, vet and tests, the frontend build and its tests, and the full Wails build on each platform. Build/test matrices for web.locafy, api.locafy and gateway.locafy are also pending. The theming scope was widened from dark-only to dark and light, which is now recorded in ADR-0006 rather than left as an assumption.
- **Evidence:** Documentation structure in place — `docs/architecture/`, `docs/decisions/` with an index and ADR-0001 (desktop platform support), ADR-0002 (verified toolchain), ADR-0003 (SQLite driver), ADR-0004 (local data model), ADR-0005 (machine monitoring), `docs/development/`, `docs/features/` and this tracker. `AGENTS.md` carries the mandatory agent rules, approved technology, safety invariants and commit conventions; commits follow Conventional Commits with no attribution trailers. Toolchain versions verified in ADR-0002: Wails CLI 3.0.0, Go 1.27.1, Node 22.14.0, npm 11.19.0. Continuous integration runs in `.github/workflows/ci.yml` on Windows, macOS and Linux, mirroring the same commands that are run locally; run 43586549586 on `b373d08` is green on all three jobs, with the `Vet` and `Frontend tests` steps individually successful on each platform. The repository builds and tests independently: `wails3 build` produces `bin/locafy.exe`; `npm run build` (tsc + vite) exits 0; `go build ./...`, `go vet ./...` and `go test ./...` cover the whole module and pass, including the mobile scaffolding under `build/`, which needed a no-op entry point to be buildable off iOS; tests pass for the activity, db, logs, machine, monitor, settings and status packages. Build/test matrix for web.locafy, api.locafy and gateway.locafy still pending.

### DESK-BOOT-001 — Wails desktop bootstrap

- **Repository:** desktop.locafy
- **Description:** Wails v3 application bootstrap; Go backend ↔ React/TypeScript frontend integration; settings and structured logging; native OS information and basic monitoring; packaging/development workflow.
- **Dependencies:** LOC-FOUND-001
- **Acceptance criteria:** Desktop app launches reliably on Windows, macOS, and Linux; frontend can invoke approved Go backend operations; local state persists across restarts; logging and error reporting work (PRD section 20, Phase 1 exit criteria).
- **Status:** IN_PROGRESS
- **Assigned agent:** —
- **Files/modules:** `main.go`, `internal/settings/`, `internal/status/`, `internal/monitor/`, `internal/activity/`, `internal/machine/`, `internal/logs/`, `frontend/src/` (shell, screens, components), `frontend/bindings/`
- **Tests:** Go tests for the settings service, rotating log, database, machine registration, activity, monitoring and status packages; Vitest component tests for the top bar, shell, error screen, theme toggle and not-found screen; CI runs build, vet, both test suites and the full Wails build on Windows, macOS and Linux.
- **Blockers:** The application has been launched and inspected only on Windows. CI builds and tests all three platforms, so a compile or test break on macOS or Linux would be caught, but runtime launch — window creation, WebView startup, the platform window chrome — is unverified there. Everything else in the Phase 1 exit criteria is demonstrated below.
- **Evidence:** The shell is no longer a placeholder. `AppShell` provides the brand mark, the 14 navigation entries from PRD section 6.2 and the top bar (search, status, notifications, theme toggle); `createHashRouter` routes every entry, with a not-found screen for unknown routes and an error element that replaces the router's development notice. Screens are the dashboard, settings, placeholders for unbuilt sections and the planned tunnels/extensions entries. Go bridges five services: settings, status (`Health`, `Notifications`), activity, machine registration and the rotating structured log; TypeScript bindings are generated for the services rather than written by hand. Window behaviour: opens centred, fits the primary screen's work area using its scale factor, and carries the palette's colours in the Windows caption and the webview background. Both themes are audited for contrast, and the light theme is recorded in ADR-0006. Verified locally on Windows: `wails3 build` produces `bin/locafy.exe` and launches cleanly (startup log sequence in `%APPDATA%/Locafy/logs/locafy.log`, database at the ADR-0004 path with WAL sidecars, one upserted machine row, one `Engine started` activity entry per launch); `go build ./...`, `go vet ./...` and `go test ./...` pass; `npm test` runs 29 component tests; the rendered window was captured over CDP in both themes. CI (run `43586549586` at `b373d08`) is green on Windows, macOS and Linux, with the Go checks, frontend build, frontend tests and full `wails3 build` individually successful on each platform.

### DESK-DB-001 — SQLite persistence and migrations

- **Repository:** desktop.locafy
- **Description:** SQLite database with safe, documented migrations as the desktop's local persistence layer. Includes the storage-path policy from ADR-0004 (xdg-resolved `Locafy/data/locafy.db`), a transaction helper, and UUID generation for schema records.
- **Dependencies:** LOC-FOUND-001
- **Acceptance criteria:** Local state persists across restarts; migrations run safely and repeatably; the database resolves to the platform-correct data directory on Windows, macOS, and Linux; upgrade/migration path documented (PRD section 18.3).
- **Status:** IN_PROGRESS
- **Assigned agent:** —
- **Files/modules:** `internal/db/`
- **Tests:** Migration tests (fresh install, upgrade, rollback); path-resolution tests; transaction helper tests
- **Blockers:** — (the database is opened, migrated and logged at every engine startup, and settings are read and written through the bound settings service).
- **Evidence:** `go vet` and `go test ./internal/...` pass locally on Windows and on macOS and Linux in CI (run `43585396744`): embedded ordered migrations, idempotent re-runs, data persists across reopen; `DataHome()` resolves the roaming application-data directory per ADR-0004; `DataPath()` produces `locafy/data/locafy.db`; `WithTx` commits on success and rolls back on error; `NewID()` issues UUIDs.

### DESK-DB-002 — Control-plane schema foundation

- **Repository:** desktop.locafy
- **Description:** First control-plane tables per ADR-0004: `machine` (single-row machine record), `settings` (typed key-value preferences), and `activity_logs` (engine action history, not application output). `project_settings` is deferred to the DESK-PROJ-001 migration because it needs the `projects` foreign key.
- **Dependencies:** DESK-DB-001
- **Acceptance criteria:** Machine record is a single-row upsert reflecting hostname, platform, architecture, and OS version; settings values are typed (`string`, `boolean`, `number`, `JSON`) and unique per key; activity log entries record scope, action, and timestamp; all state columns carry CHECK constraints; table access lives behind a settings/machine service module, never in UI code.
- **Status:** IN_PROGRESS
- **Assigned agent:** —
- **Files/modules:** `internal/db/migrations/`, `internal/settings/`, `internal/machine/`, `internal/activity/`
- **Tests:** Upsert/uniqueness tests; typed-value round-trip tests; activity-log append tests
- **Blockers:** — (the machine record is registered at startup and the schema is exercised by the settings, machine and activity services).
- **Evidence:** `go vet . ./internal/...` and `go test ./internal/...` pass locally on Windows and on macOS and Linux in CI (run `43585396744`), which exercises the per-platform OS version lookup on each platform: migration 0002 creates `machine` and `activity_logs` with CHECK constraints, triggers, and index; machine registration upserts exactly one row; settings round-trip string/boolean/number/JSON, reject type mismatches, and return `ErrNotFound` for missing keys; activity entries append with scope/scopeID and read back newest-first with limits; invalid scopes are rejected by the schema CHECK (test-verified).

### DESK-UI-001 — Navigation and design system

- **Repository:** desktop.locafy
- **Description:** Global navigation (Overview, Projects, Runtimes, Downloads, Analytics, Logs, Terminal, Tunnels as planned/disabled, Extensions as planned/disabled, Help and support, Settings), with Applications, Services and Domains moved into the project workspace per ADR-0009, and the Locafy design system on shadcn/ui + Tailwind with PRD section 17.2 tokens.
- **Dependencies:** DESK-BOOT-001
- **Acceptance criteria:** Navigation matches PRD section 6.2, with the planned/disabled treatment applied everywhere tunnels and extensions appear (dashboard metrics, quick actions, project tabs), not only in navigation; design tokens per section 17.2 with dark and light themes and a persisted theme toggle in the top bar; shortcut hints use platform-appropriate modifiers; project view is the primary working area; global vs project-scoped settings distinguishable; status never by color alone; the top bar carries machine status and a notification entry point (PRD section 6.3); the brand mark matches the mockup's orange-red hexagon.
- **Status:** IN_PROGRESS
- **Assigned agent:** —
- **Files/modules:** `frontend/src/components/app-shell.tsx`, `frontend/src/components/top-bar-status.tsx`, `frontend/src/screens/`, `frontend/src/router.tsx`, `frontend/src/index.css`, `internal/status/`, `internal/monitor/`
- **Tests:** `go test ./internal/status/` covers health reporting, resource usage and notification listing; `go test ./internal/monitor/` covers sampling for memory and disk, percentage bounds, idempotent close, and a degraded disk reading. The frontend has component tests for the top bar under Vitest with Testing Library: `npm test` runs them in jsdom against a mocked status service, covering the healthy pill and its CPU figure, the checking state, an unhealthy engine, an unreachable backend (pill and popover text), the resource detail with its progress bars, an empty notification list, listed entries with their ages, the nine-plus badge cap, and a failed activity read. The same suite covers the route error screen (a thrown failure with its message and technical details, a route response reported as `404 Not Found` with its status, and the return to the overview) and the theme toggle (initial state read from the document, both directions applied to the document, persisted to storage, and relabelled for the next action). Other screens still have no tests.
- **Blockers:** The status pill and its popover report engine health plus real CPU, memory and disk usage. The remaining PRD section 6.3 item — running processes — is deferred to the applications phase: Locafy cannot launch or supervise an application yet, so a process list would have nothing to manage and nothing real to verify until the application runtime exists. Reported processes should describe processes Locafy started, not whatever happens to be on the machine. The project view, which section 6.2 makes the primary working area, now exists as the projects screen from DESK-PROJ-001, and the interface carries 42 component tests. The Windows caption bar stays dark in the light theme; ADR-0006 records that as an accepted limitation, since the runtime in use exposes no call to re-theme a caption and removing it needs a frameless window. This work began while `LOC-FOUND-001` was incomplete and before this repository had a CI workflow, against that feature's documented gate.
- **Evidence:** Design tokens from PRD section 17.2 applied in `frontend/src/index.css` (dark palette as the default on `.dark`, light palette on `:root`, `--success` added for healthy state). Both palettes set `color-scheme`, and both were audited on the rendered dashboard: every enabled text element clears 4.5:1, or 3:1 where it is large, in dark and in light — which required darkening the light `--success` and `--muted-foreground` tokens that had been tuned against the dark palette. The window background matches the dark palette and an inline stylesheet paints the stored theme before the bundle loads, so a light-theme user does not see a dark flash. `AppShell` provides the shared sidebar and top bar; `createHashRouter` maps each navigation entry to a route with placeholder screens for unbuilt sections, and an unknown hash route renders a not-found screen that names the requested address and offers a way back — it is rendered inside the shell, so the 14 navigation entries stay reachable; verified on an unmatched route (`#/reports?range=week` showed the heading, the requested address and both actions, with the sidebar intact) and covered by a test for the heading, the echoed address and the return to the overview; the dashboard screen implements the global mockup's hero, metrics, project cards, application list and activity rail. Verified: `npm run build` (tsc + vite) and `wails3 build` exit 0; the app launches and persists state; browser checks at 1508x1000 and 813x408 show no horizontal overflow; muted text on card surfaces measures 6.87:1; Ctrl K focuses the top bar search, which filters the navigation to matching screens as it is typed, excludes the planned entries from the results, moves the highlight with the arrow keys, jumps on Enter and clears on Escape or blur; it is a labelled combobox with `aria-expanded`/`aria-activedescendant` over a listbox, and it was verified in the running window (Ctrl K focused the field, `set` filtered to Settings, Enter reached `#/settings` with the field cleared); status shown with text and colour. Window chrome themed to the palette so the caption bar matches the content. Routes carry an error element that replaces the router's development notice with a titled message, the thrown error's message, reload and back-to-overview actions, and a collapsed stack trace; verified by simulating a failing screen: the notice is absent, the heading and message render, and back-to-overview returns to the dashboard. Navigation carries all 14 entries from PRD section 6.2 and routes each one; tunnels and extensions are non-selectable entries labelled PLANNED, and that treatment reaches the dashboard metric, quick actions, local tunnel card and project tabs — verified by reading the rendered DOM: 14 entries, two `aria-disabled`, disabled quick actions carrying a PLANNED label, and the tunnels metric reading `—` with `Planned`. Top bar reports engine health from a backend check and lists recent engine activity; `Health()` pings the database rather than assuming it, and both surfaces label their state in text. The bell also leads with a warning when the data volume has under 5 GiB free, derived from the current reading rather than recorded as an event, so it clears itself when space is freed; warnings carry the word `Warning` next to an icon so the state is not colour-only. Verified against the real engine on a nearly full volume: the list began with `Low disk space — 240 MB free of 237.9 GB on the data volume` and severity `warning`, followed by `info` activity entries. The status pill reports CPU usage next to its label and its popover shows CPU, memory and disk as labelled bars with used-of-total detail, sampled in the background by `internal/monitor` and recorded in ADR-0005; the backend tests read the real machine, asserting installed memory and data-volume capacity are present, used never exceeds total, percentages stay within 0-100, and an unreadable disk does not lose the memory reading. The top bar carries exactly one status pill, one notification bell and one theme toggle, in that order — state first, the appearance control last: the shell draws no status controls of its own and shows only what `App.tsx` passes through `topbarActions`, confirmed on the running app (one status control, one bell, one theme toggle) and held by a shell test that fails if a built-in pill or bell reappears. The top bar carries the status pill, the notification bell and the theme toggle, and the brand mark renders the orange-red faceted hexagon (gradient stops `#FF873D` to `#F05252`); verified in the browser: dark is applied before first paint, the stored theme survives reloads in both directions, computed tokens flip to `#F5F3F8` / `#111116` / `#FFFFFF` / `#D8D5E2` / `#15803D` in light and back to the dark set, the Sun/Moon toggle is labelled with its action, and the console is clean. The notifications bell reads the recorded activity history rather than a separate store, so project work appears there: `Project created`, `Project removed` and their siblings are listed with the project name and age, and the projects screen refetches the bell after every action instead of leaving it to the minute-long poll. Project entries are links that open the project's own workspace at `/projects/<id>`, while machine entries stay plain text; an entry whose project has since been removed lands on that route's `Project not found` state instead of a dead row. The earlier approach — a `?project=<id>` parameter that only highlighted a card on the projects list — was removed once the workspace route existed, since a highlight is not where project work happens. Verified live: every project link rendered as `#/projects/<uuid>`, choosing one opened the workspace with the project's name as its heading and its tabs, and choosing a removed project's entry showed `Project not found`. `Notification` carries `ScopeID` for this, tested in `internal/status` and in both component suites; verified live, where selecting `Open project Nav demo` moved the hash to `#/projects?project=<uuid>`, highlighted the Nav demo card and closed the bell. The bell pages instead of truncating: it asks for ten entries and each Show more activity request raises the page by twenty, up to the service's two-hundred-entry ceiling, and the grown page is remembered in `sessionStorage` under `locafy.notifications.page` so closing the bell or reloading the shell reopens it at that size rather than back at ten (an unusable saved value falls back to ten); verified live, where the list went from 10 rows to 30 and the trigger's name from `10 recent entries` to `30 recent entries`. Entries can be dismissed one at a time or all at once: dismissal sets `activity_logs.dismissed_at` (migration 0004) so the history stays complete for diagnostics while the bell stops counting it, a failed dismissal is reported rather than silently dropped, and the low-disk warning carries no dismiss control because hiding it would not change the condition. Dismissal is covered by three Go tests (`internal/activity` visibility, `internal/status` dismissal and clearing, dismissibility flags) and four component tests; verified live, where dismissing the `Dismiss demo` entry set its `dismissed_at` in the database and the row disappeared from the list. The low-disk warning is actionable rather than only informative: it carries `OpenPath` for the folder holding the engine's data and an `Open folder` control that asks `StatusService.OpenDataFolder` to reveal that folder in the OS file manager (`explorer`, `open` or `xdg-open`), because a webview cannot open a local folder from a link; failures are reported in the popover. Covered by two Go tests (the warning offers its folder while recorded entries offer none, and the opener receives the data directory and propagates a failure) and three component tests. The live click could not be exercised on this machine: the warning only appears below 5 GiB of free space and, by the time the feature was built, the volume had 5.9 GiB free, so the entry was absent; the folder path was confirmed to exist and the `explorer` form of the command to launch it. Which kinds of notification appear is the operator's choice in Settings: `notifications.machine`, `notifications.project` and `notifications.warning` are boolean settings that stay on until turned off, the service filters activity by scope and withholds the derived warning when its type is off, and turning every type off leaves the bell empty while the recorded history stays for diagnostics. The Settings card writes each change through an optimistic cache update, because sending the whole preferences object from a stale cache silently lost the earlier of two quick toggles — a race the browser run found. Ages are relative, so the bell re-renders every thirty seconds while it is open and every age carries the exact time as a tooltip. Covered by four Go tests (defaults fully on, project off hiding only project entries, warnings off withholding the derived warning, everything off persisting), four Settings tests (state shown, one type changed without disturbing others, a stored choice, a rejected change restored) and two bell tests (the choice reacting immediately, an age that advances while open with its exact time in the title). Verified live: the Settings card listed `Warnings`, `Project activity` and `Machine events` all on; turning `Project activity` off left only `Engine started` machine entries in the bell, turning it back on restored five project entries, and an age carried `11/16/2026, 10:51:36 AM` as its tooltip. Seen and dismissed are separate states: opening the bell records `notifications.last_seen`, entries recorded after that mark are `Seen: false` and sit above a `New` divider captured when the bell opened (so the divider survives the refetch that marks them seen), and the badge counts unread entries through `UnreadNotifications` rather than the length of the loaded page — capped at nine-plus and bounded by the same two-hundred-entry ceiling. The derived warning is current state, so it is never unseen and never counts. Covered by four Go tests (unread until opened, new activity unread again after opening, dismissed and disabled types excluded from the count, warnings never unread) and three component tests (unread badge instead of loaded size, divider plus the seen mark, no divider when everything is new). Repeated activity collapses into one counted row, so five identical engine events take a line reading `×5` instead of five lines; the count opens to each occurrence's relative and absolute time. Entries stay apart when their target differs or when one is new and the other already seen, and dismissing a collapsed row dismisses every entry behind it. Covered by four component tests (a repeated action collapsed and expanded, different targets kept apart, new and seen kept apart, a counted row dismissed in full); verified live, where five engine starts on the page appeared as `Engine started ×5` and expanded to five timestamps. An empty bell names its reason instead of always claiming nothing happened: with every type off it says so and links to Settings, while a history that is simply empty still reads `Nothing recorded yet.`; verified live, where turning all three types off produced `Every notification type is turned off. Choose which ones to show in Settings.`, following that link reached `#/settings`, and restoring the types brought the ten entries back.

### DESK-PROJ-001 — Create and import projects

- **Repository:** desktop.locafy
- **Description:** Create and import projects; project listing, metadata, configuration, and persistence; renaming a project; changing a project's root path with validation; safe project removal.
- **Dependencies:** DESK-BOOT-001, DESK-DB-001, DESK-DB-002, DESK-UI-001
- **Acceptance criteria:** Users can create/import and manage projects (create, import, rename, change root path, remove from Locafy); changing the root path validates the selected directory, reports conflicts, and never moves, copies, or deletes files; project data persists; project boundaries are enforced; removing a project from Locafy does not delete its source files (PRD sections 7.2, 20).
- **Status:** COMPLETED
- **Assigned agent:** —
- **Files/modules:** `internal/projects/` (records, validation, manifest), `internal/db/migrations/0003_projects.sql`, `internal/db/projectsdir_windows.go` and `projectsdir_unix.go` (workspace default), `frontend/src/screens/projects.tsx`, `frontend/src/components/ui/dialog.tsx` and `alert-dialog.tsx`, `frontend/bindings/locafy.dev/desktop/internal/projects/`
- **Tests:** 15 Go tests in `internal/projects/projects_test.go` — create with directory, manifest and record; slug collisions; blank and overlong names; directory cleanup when the insert fails; link without writing into the directory; missing, file and blank paths; case-insensitive duplicate detection; rename; managed projects refusing a root change; root-change validation and conflict reporting; removal keeping files and releasing the path; list ordering; workspace preference and its use by create. Three Go tests in `internal/projects/import_test.go` cover a folder scan listing child directories with advisory markers, scan path validation, and `LinkMany` importing every selected path while reporting per-path failures. 22 screen tests in `frontend/src/screens/projects.test.tsx` cover the empty state, the list with managed and linked badges, create, link, rename, folder change offered only for linked projects, removal text, a rejected action shown inside its dialog, and the batch import flow (scan, hidden unmarked folders, selection counts, per-folder failure reasons, a rejected scan).
- **Blockers:** — The manifest decision ADR-0007 removed the recorded blocker. Manifest import/export between machines remains a later feature.
- **Evidence:** ADR-0007 records the managed-versus-linked model, the manifest format (`locafy.project.json`, `schemaVersion: 1`, written only into projects Locafy created) and the rule that removal is a soft delete which never touches files. `0003_projects.sql` creates `projects` — UUID key, name and root path guards, `kind` and `status`/`lifecycle_state` CHECK constraints, JSON `configuration`, `deleted_at`, a partial unique index on the live root path, and the `updated_at` trigger per ADR-0004. Verified in the running application over CDP: the empty screen offers both actions and names the workspace (`C:\locafy\projects`); creating `Etugora demo` produced `C:\locafy\projects\etugora-demo` containing `locafy.project.json` with `"schemaVersion": 1` and the project name, and the list showed it as Managed; linking `D:\...\linked-folder` showed it as Linked and left the directory with exactly its original `package.json`; removing the linked project dropped it from the list while the directory and file stayed; renaming to `Etugora platform` changed the label while `C:\locafy\projects\etugora-demo` kept its folder name. `go vet ./...`, `go build ./...` and `go test ./...` pass; `npm test` passes 42 tests and `npm run build` exits 0. Project boundaries hold: every operation goes through the service, paths are canonicalised and compared case-insensitively, and a second project cannot take a live project's directory. The settings screen exposes the workspace folder: the field shows the effective location, Save stores an absolute path through `SetWorkspaceDir`, and Use default clears the preference. Verified live: setting `C:\locafy\projects-demo` updated the field, the next managed project was created at `C:\locafy\projects-demo\custom-workspace-demo`, and Use default restored `C:\locafy\projects`; five settings tests cover the displayed value, saving, resetting, a rejected relative path and the disabled state of an unchanged value. Project actions reach the top bar's notifications: every create, link, rename, root change and remove is recorded in the activity history the bell reads, and the projects screen invalidates the notifications query after each action so the entry appears without waiting for the minute-long poll. Verified live: creating `Bell demo` made `project · Bell demo` appear in the open bell with its `Project created` action, alongside earlier `Project removed` entries; the bell loads ten entries and grows by twenty each time Show more activity is chosen, up to a two-hundred-entry ceiling, so a burst of project work cannot hide machine entries permanently. Batch import: `ScanFolder` lists a folder's child directories that are not already live project roots and marks the ones holding a recognizable project marker (advisory — folders without a marker are still offered behind a filter), and `LinkMany` links every selected path, returning per-path outcomes so one failure does not stop the rest; verified in the running application over CDP — Import folder opened the dialog, a scan of a parent folder listed its children with markers, folders without a marker stayed hidden until the filter was enabled, and importing the selection linked each folder as a Linked project with per-folder failures kept listed with their reason. `go vet ./...`, `go test ./...` and `go build ./...` pass; `npx vitest run` passes 101 tests across 9 files and `npm run build` exits 0.

### DESK-PROJ-002 — Project overview and lifecycle

- **Repository:** desktop.locafy
- **Description:** Projects dashboard and project workspace with project-scoped tabs (ADR-0009); project overview screen (PRD section 17.3) and start/stop/restart orchestration foundation; project-scoped terminal/logging context foundations.
- **Dependencies:** DESK-PROJ-001
- **Acceptance criteria:** Lifecycle follows PRD section 7.3 order (validate → check runtimes → check ports → services → readiness → applications → health checks → report); component-level status is reported, not a misleading single status; partial running states visible; stop order is safe and managed processes are distinguishable from unrelated ones.
- **Status:** IN_PROGRESS
- **Assigned agent:** —
- **Files/modules:** `frontend/src/screens/projects.tsx` (projects dashboard), `frontend/src/screens/project-workspace.tsx` (workspace and tabs), `frontend/src/components/lifecycle-badge.tsx`, `frontend/src/lib/projects-query.ts`, `frontend/src/components/app-shell.tsx`, `frontend/src/App.tsx`, `frontend/src/router.tsx`
- **Tests:** Frontend only so far: dashboard cards, filtering and links; the stats row (totals, running, stopped and errored counts read from labelled groups), the lifecycle words scoped to each card, the Quick Start tile opening the create dialog, and the More menu (rename, change folder offered only for linked projects, remove); workspace header, overview, tabs, planned tunnels tab, section switching, unknown project and read failure; shell navigation limited to global entries; the project switcher (current project, listing, switching while keeping the section, unknown project) and its presence only inside a workspace; the folder picker (fills the path, cancel keeps typed text, failure shown, offered only where a folder is chosen). The dialogs package has a Go test for its title handling. Lifecycle orchestration tests and partial-failure state tests are still to write.
- **Blockers:** Start, stop and restart, component-level status, and the lifecycle order from PRD section 7.3 are not built; they need applications and services to exist (DESK-APP-002, DESK-SVC-001). Every workspace tab except Overview is a not-built-yet section. The Processes tab shows the not-built state until process supervision exists. The dashboard has been inspected in the running window over CDP; the workspace screens have been verified by tests and a production build only.
- **Evidence:** The sidebar holds the 11 global entries from ADR-0009 and no project list. `/projects` is a dashboard: four stat tiles (total, running, stopped, with issues) label each count in words with an icon that only echoes it, then project cards in a two-column grid — a stable accent border hashed from the project id, name, description, Managed or Linked, lifecycle state in words top-right, a meta row pairing the mono root path with the creation age, and one primary Open action beside a More menu holding rename, change folder (linked projects only) and remove. A dashed Quick Start tile closes the grid with a way to create the next project. A filter by name or folder sits above the grid. Verified in the running window over CDP: the tiles read the live counts, cards show their accent border and badges, the More menu opens with exactly the right items per project kind, and the Quick Start button opens the create dialog. `/projects/:projectId` is the workspace: header with name, state, origin and folder, then tabs for Overview, Applications, Services, Domains, Tunnels (planned, not selectable), Environment, Logs, Terminal and Settings, each addressable by URL. The Overview tab shows the record's real fields and no invented counts. The Projects sidebar entry stays selected inside a workspace. While a workspace is open, the top bar shows the project's name and state, a back arrow to the dashboard, and a switcher that lists every project and keeps the current tab when another is chosen; it is absent on the overview and the dashboard. The workspace has a Processes tab. Linking a folder and changing a linked project's folder offer a Browse button backed by a native folder picker (`internal/dialogs`, `ChooseFolder`); cancelling keeps the typed path and a failing picker is reported in the dialog. PRD section 6.2 was rewritten to match ADR-0009. Verified on the committed snapshot in a clean checkout: `npx tsc --noEmit` is clean, `npx vitest run` passes 72 tests across 9 files, and `go build ./...`, `go vet` and the dialogs and projects tests pass. The native folder dialog has not been opened in the running window.

### DESK-APP-001 — Add and detect applications

- **Repository:** desktop.locafy
- **Description:** Add/import applications; framework/language detection from project markers (PRD section 8.3); configurable working directory, commands, runtime assignment model; dependency graph with cycle validation.
- **Dependencies:** DESK-PROJ-002
- **Acceptance criteria:** A project can contain multiple applications; detection is advisory and overridable; detected commands require user confirmation before first execution; dependency graph validated for cycles.
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Detection tests per marker type; dependency-graph validation tests
- **Blockers:** —
- **Evidence:** —

### DESK-APP-002 — Application process lifecycle

- **Repository:** desktop.locafy
- **Description:** Process start/stop/restart; port tracking and conflict reporting; application logs and health state; independent and project-lifecycle-driven operation.
- **Dependencies:** DESK-APP-001
- **Acceptance criteria:** Applications start/stop independently and via project actions; failures are visible and do not corrupt unrelated application state; port conflicts reported without terminating unrelated processes; no orphaned processes (PRD sections 18.3, 20, Phase 3).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Process lifecycle tests; port conflict tests; failure isolation tests
- **Blockers:** Platform-specific process behavior must be documented (PRD section 19)
- **Evidence:** —

### DESK-RUN-001 — Runtime detection and assignment

- **Repository:** desktop.locafy
- **Description:** Detect installed runtimes (PHP, Node.js, Python, Go, Java, Ruby, .NET — final list decided in implementation planning); display versions/locations; per-application selection; install additional versions where feasible; compatibility validation.
- **Dependencies:** DESK-APP-002
- **Acceptance criteria:** Applications use explicitly selected runtime versions; missing versions produce clear errors; existing system runtimes are never silently overwritten; Locafy-managed installs are distinguishable from system installs (PRD sections 9.2, 20, Phase 4).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Runtime detection tests per platform; assignment/validation tests
- **Blockers:** Initial runtime list and per-OS installation strategy undecided (PRD section 23)
- **Evidence:** —

### DESK-DL-001 — Downloads view for install jobs

- **Repository:** desktop.locafy
- **Description:** Global machine-scoped view of in-progress and completed download/install jobs — runtimes and other Locafy-managed components — with progress, outcome, and retry (PRD section 6.5).
- **Dependencies:** DESK-RUN-001
- **Acceptance criteria:** Install jobs appear with live progress; completed and failed jobs are retained with their outcome; retry re-runs a failed job; the view is machine-scoped and works without an account (PRD sections 6.5, 9, Phase 4).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Job list and progress updates; retry flow; history retention
- **Blockers:** Job creation depends on the runtime installation strategy (PRD section 23)
- **Evidence:** —

### DESK-SVC-001 — Service catalogue and lifecycle

- **Repository:** desktop.locafy
- **Description:** Service catalogue and project attachment (priority: PostgreSQL → Redis → Mail capture → MySQL/MariaDB → S3-compatible storage); configuration, persistent data directories, start/stop/restart, health checks, logs, connection details; safe removal with data-retention warnings.
- **Dependencies:** DESK-APP-002
- **Acceptance criteria:** A project can run an application with its required managed services; service data persists per documented behavior; service failures reported independently; removing a service configuration does not silently delete data; per-project isolation (PRD sections 10.4, 20, Phase 5).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Service lifecycle tests; isolation tests; data-retention tests
- **Blockers:** Service packaging strategy (native versus container-backed) and object storage engine undecided (PRD section 23). The engine and instance model, with a shared default and isolated opt-in, is recorded in ADR-0008. Services are delivered as a project tab, not a global screen (ADR-0009).
- **Evidence:** —

### DESK-NET-001 — Local domains and reverse proxy

- **Repository:** desktop.locafy
- **Description:** Reverse-proxy integration (Caddy candidate), local domain assignment and routing, local HTTPS (mkcert candidate), WebSocket proxying, port/domain conflict detection, network diagnostics, validated configuration reloads.
- **Dependencies:** DESK-APP-002, DESK-SVC-001
- **Acceptance criteria:** Multiple applications/projects use distinct local domains; configuration changes are validated before reload; local HTTPS works on all supported platforms (Windows, macOS, Linux); loopback-by-default enforced; port owners visible (PRD sections 11, 20, Phase 6).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Routing/domain conflict tests; certificate workflow tests
- **Blockers:** Reverse proxy and certificate tooling candidates must be validated (PRD section 23)
- **Evidence:** —

### DESK-DEVX-001 — Integrated terminal and logs

- **Repository:** desktop.locafy
- **Description:** Integrated terminal at global/project/application/service scopes (xterm.js + Go PTY); centralized logs with filtering and source attribution; process/CPU/memory/disk monitoring; environment management and secret masking.
- **Dependencies:** DESK-PROJ-002, DESK-APP-002, DESK-SVC-001
- **Acceptance criteria:** Developers can diagnose common local problems without leaving Locafy; metrics and logs have clear scope and source attribution; secrets are not exposed in logs or UI; environment precedence rules documented (PRD sections 12, 13, 20, Phase 7).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Terminal/PTY tests; log attribution tests; secret-masking tests
- **Blockers:** Platform PTY behavior must be documented (PRD section 19)
- **Evidence:** —

### DESK-ANAL-001 — Analytics views

- **Repository:** desktop.locafy
- **Description:** Local historical views: activity over time derived from the engine action history, resource usage trends with explicit scope labels, and application/service status history including restarts and failures (PRD section 13.4).
- **Dependencies:** DESK-DB-002, DESK-DEVX-001
- **Acceptance criteria:** Activity trend view works from local history without an account; every metric states its scope (machine-wide, Locafy-managed, or project-specific); status history covers restarts and failures; retention is bounded and configurable (PRD sections 6.3, 13.3, 13.4).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Trend queries over stored history; retention pruning; scope-label presence
- **Blockers:** Metric collection arrives with Phase 7 monitoring (DESK-DEVX-001)
- **Evidence:** —

### TUN-PROTO-001 — Tunnel threat model and protocol (shared: Gateway/Desktop/API)

- **Repository:** Gateway · Desktop · API — shared. The canonical protocol specification is maintained in `gateway.locafy/docs/architecture/`; this repository records the agent-side obligations it must satisfy.
- **Description:** Threat model and versioned tunnel protocol specification before any gateway implementation (PRD section 16).
- **Dependencies:** API-AUTH-001, API-DEVICE-001 (api.locafy), DESK-NET-001 (this repo)
- **Acceptance criteria:** Protocol and threat model documented; gateway technology decision recorded; security and operational requirements approved (PRD section 16.4 delivery gate items 5–7).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** `docs/architecture/` (protocol spec)
- **Tests:** —
- **Blockers:** Full section 16.4 gate (desktop lifecycle stable, networking established, auth + devices working)
- **Evidence:** —

### TUN-AGENT-001 — Desktop tunnel agent

- **Repository:** desktop.locafy
- **Description:** Desktop tunnel agent establishing authenticated outbound connections to the public gateway; tunnel creation/deletion, visibility and revocation of active exposure.
- **Dependencies:** TUN-PROTO-001
- **Acceptance criteria:** Only authorized devices and configured applications can be exposed; users can inspect and revoke active tunnels; heartbeats/recovery behavior per protocol spec (PRD section 16.2, section 20, Phase 9).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Agent authentication, connection recovery, revocation tests
- **Blockers:** Gateway implementation (gateway.locafy) and protocol spec
- **Evidence:** —

### TUN-E2E-001 — End-to-end public tunnel flow (shared: Gateway/Desktop/API)

- **Repository:** Gateway · Desktop · API — shared. Tracked in each participating repository; the gateway entry coordinates the combined flow.
- **Description:** Validate this repository's side of the complete tunnel flow: authorized device → desktop agent → gateway → public URL → local application, including WebSocket support where in scope. Desktop contributes the agent-side behaviour (device authorization, tunnel start/stop/revoke, heartbeat and reconnect).
- **Dependencies:** TUN-PROTO-001, TUN-AGENT-001, TUN-GATE-001 (gateway.locafy)
- **Acceptance criteria:** An exposed application is reachable through its public URL; revoking a device or tunnel takes effect immediately; disconnection and reconnect behaviour is predictable (PRD sections 20, 24).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Agent-side end-to-end tests; revocation and reconnection scenarios
- **Blockers:** TUN-AGENT-001 and TUN-GATE-001 not implemented; section 16.4 delivery gate not satisfied
- **Evidence:** —

### DESK-HELP-001 — Help and diagnostics export

- **Repository:** desktop.locafy
- **Description:** Offline help entry point: built-in guidance for common tasks and problems, a way to open the documentation, and an exportable diagnostics bundle for reporting issues (PRD section 6.4).
- **Dependencies:** DESK-BOOT-001
- **Acceptance criteria:** Help content is usable without a network connection; a diagnostics bundle can be exported only after explicit user review; secrets and environment values are masked in the bundle (PRD sections 6.4, 12, 18.1).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Offline help rendering; bundle export; secret-masking checks on bundle contents
- **Blockers:** —
- **Evidence:** —

### DESK-EXT-001 — Extensions platform

- **Repository:** desktop.locafy
- **Description:** Extension model with a built-in catalogue that works offline; install, enable, disable, and uninstall flows with clear permission disclosure; an optional online catalogue layered on the account system later (PRD Phase 11).
- **Dependencies:** DESK-UI-001, DESK-DB-001
- **Acceptance criteria:** Extensions can be managed without an account or network connection; uninstalling an extension leaves user projects and data intact; extensions operate under the same safety invariants as the core application (PRD Phase 11).
- **Status:** NOT_STARTED
- **Assigned agent:** —
- **Files/modules:** —
- **Tests:** Install/enable/disable/uninstall flows; offline catalogue; data-integrity checks after uninstall
- **Blockers:** The extension model and API definition must be recorded as a decision before implementation (PRD Phase 11)
- **Evidence:** —
