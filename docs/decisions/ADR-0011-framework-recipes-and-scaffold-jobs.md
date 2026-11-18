# ADR-0011: Framework recipes, argv-only scaffolds and toolchain ownership

- **Status:** Accepted
- **Date:** 2026-11-17
- **Depends on:** ADR-0004 (local data model), ADR-0007 (project storage), ADR-0010 (applications)
- **Affects:** DESK-SCAF-001, DESK-SCAF-002, DESK-SCAF-003, DESK-PROJ-003, DESK-RUN-001, DESK-DL-001

## Context

Adding a project or an application should not begin with a blank folder and a
trip to a framework's documentation. Locafy can run the framework's own
installer, but doing that safely means answering four questions before any
command runs: what exactly will execute, with which versions, where it writes,
and what happens when it fails halfway.

The same feature could easily become either an unsafe "run anything" console or
a hardcoded Laravel-only wizard. Neither matches PRD section 8.3 (detection is
advisory, the operator confirms) or the project's cross-stack constraint.

## Decision

### Recipes are shipped data, never fetched

`internal/scaffold/recipes.json` is embedded in the binary. It is trusted
input, reviewed like code, and validated when loaded: unique ids, closed option
enums with a default that is one of them, known placeholders only, no shell
characters in arguments, argv arrays, at least one command, and declared
verification markers. A broken catalogue fails at startup rather than halfway
through an install.

Each recipe declares its language, framework, documentation, the versions it has
been checked against, selectable version/option values, required tools with
minimum versions, ordered commands, files to verify, and the resulting
application's start command. Adding a framework is a data change plus tests, not
a new screen.

### Version choice is explicit

Framework versions are closed options (`Laravel 13/12`, `Next.js 16/15`, `Vite
8/7`, `Django 6.1/5.2`, `.NET 11/10`) and are bound into the command by the
catalogue itself — `create-next-app@15`, `laravel/laravel:^12.0`,
`django~=5.2.0`, `--framework net10.0`. The interface shows the values and the
resulting command, never a free-text version string.

Tool versions are chosen from what is installed. The probe reports every copy it
finds (PATH plus Laragon, XAMPP, Composer's user bin, the .NET installer,
Homebrew and system locations), and the operator picks one when several satisfy
the recipe. Installing a runtime is deliberately not part of this record:
DESK-RUN-001 owns machine-level runtime installation, and the probe is the
shared seed of that work.

### Execution is argv, not a shell

Commands are `exec.CommandContext` argv arrays. Options are bound from closed
enums, so no text from the interface reaches argv except the validated target
folder and project name, which are contained and slugged. Windows `.bat`/`.cmd`
shims (Composer, the Laravel installer) run through `cmd.exe /c` with that fixed
argv because Go cannot execute them directly; no shell string is ever composed.

Installers run non-interactively: `--no-interaction`, closed stdin, `CI=1`,
`COMPOSER_NO_INTERACTION=1`, `npm_config_yes=true`, `NO_COLOR=1`, and a
twenty-minute ceiling. Output streams to the dialog and to
`logs/scaffold-<id>.log`; cancel kills the process tree.

### Containment and atomicity

- Applications land in a new folder inside the project root; the project root
  itself is not an application target for scaffolding.
- A destination with files is refused. The one existing-empty-folder case is
  allowed only for recipes whose commands run inside the target (Django's
  virtual environment).
- A managed project's path is allocated but not created; the installer creates
  it. The project row, manifest and application record are written only after
  the installer's markers exist on disk. A failed scaffold registers nothing and
  offers to remove a folder Locafy created — never one that was already there.
- Scaffolded applications are saved as `detected` (the click that started the
  scaffold is the confirmation) with `scaffold_recipe` and `scaffold_options`
  recording provenance. The applications `source` CHECK is unchanged, so no
  table rebuild is needed.

### Laravel is one click, with a fallback

Defaults are starter kit `none` and database `sqlite`. When the Laravel
installer is present and version 13 is chosen, `laravel new` runs; otherwise —
older pinned version or no installer — the recipe falls back to
`composer create-project laravel/laravel:^<version>.0`, which needs no global
install. The only preparation step Locafy runs itself is
`composer global require laravel/installer`, shown before it runs, user-level and
never elevated. System installers (winget, choco, apt) are never invoked.

### Jobs are recorded

`scaffold_jobs` keeps one row per run with its recipe, target, options, command
summary, log path and outcome, so the interface can survive closing the dialog
and the future downloads view (DESK-DL-001) has a history to read.

### The operator can open a terminal at the run

A scaffold only ever runs Locafy-defined commands. Everything after that —
`php artisan migrate`, extra packages, debugging a half-finished install —
is the operator's own session. Locafy offers an "Open terminal" button on
a failed run and on every entry of the scaffold history: the platform's
terminal launches at the run's folder (`wt -d` on Windows with an explicit
path, never through cmd's parser, with a cmd fallback only when the path is
quote- and percent-free; `open -a Terminal` on macOS; gnome-terminal, konsole,
xfce4-terminal, kitty, alacritty or wezterm on Linux, argv throughout).
Typed commands in that terminal are the operator's actions, not Locafy
actions, so they are never confirmed, rebuilt or logged as commands.

## Consequences

- The advertised framework list is small and verified: five official
  scaffolders, each smoke-tested on Windows before its card is shown as ready.
  Recipes without a live check carry that limitation in the tracker rather than
  in the UI.
- A recipe that changes its flags fails visibly: the tool's own output is in the
  log, verification markers catch a half-finished install, and nothing is
  registered.
- Toolchain installation remains a separate feature; the scaffold only reads.
  When DESK-RUN-001 lands, it can reuse the probe and extend the version picker
  from "installed copies" to "install another".
- Django is the riskiest recipe: it owns a multi-step virtual-environment flow
  whose paths differ by platform, which is why it is verified separately.
- Logs are per job and bounded by the same rotation principles as engine logs;
  a failed run's folder remains until the operator removes it.
