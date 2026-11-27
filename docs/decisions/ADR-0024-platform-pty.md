# ADR-0024: A platform PTY for the integrated terminal

- **Status:** Proposed — awaiting the operator's approval; nothing in this record is implemented yet
- **Date:** 2026-11-27
- **Depends on:** ADR-0001 (cross-platform from day one), ADR-0013 (process supervision and tree termination), ADR-0023 (the CA processes inherit)
- **Affects:** DESK-DEVX-001

## Context

DESK-DEVX-001 asks for an integrated terminal at global, project, application
and service scopes, and its tracker entry names one blocker: the platform PTY
behaviour has to be documented before the terminal is built (PRD section 19).
That is the right place to stop, because a PTY is the one part of the feature
that cannot be written once and run everywhere, and the choice between a
library and in-repo platform code is a technology decision this repository is
required to record rather than assume.

The constraints that shape the decision:

- **Three platforms, no CGO.** ADR-0001 and ADR-0003 fix Windows, macOS and
  Linux and a CGO-free build. Any PTY layer must be pure Go or already part of
  the toolchain.
- **Windows is not a Unix PTY.** Windows has no `openpty`. Since Windows 10
  version 1809 it has ConPTY (`CreatePseudoConsole` plus `ResizePseudoConsole`),
  which is what Windows Terminal and VS Code use; before that the only options
  were third-party helpers such as winpty. ConPTY needs an inheritable pipe
  pair and `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE` in the child's attribute list.
- **macOS and Linux differ in details, not in kind.** Both open a
  pseudo-terminal master/slave pair (`/dev/ptmx`), but Linux relies on
  `grantpt`/`unlockpt`/`ptsname` while macOS's `posix_openpt` does not need the
  same dance, and the window-size ioctl differs in constant name.
- **A terminal is not a log.** Escape sequences, cursor moves and full-screen
  programs make a transcript meaningless as a log line and impossible to mask
  line by line. Whatever the terminal stores must be kept away from the log store
  built for DESK-DEVX-001.

## Decision (proposed)

### One session per tab, owned by the engine, rendered by xterm.js

Each terminal tab starts one shell session inside the engine, attached to a PTY
the engine owns. The window hosts xterm.js as the emulator and streams bytes to
and from the engine; the engine never parses them. A session is identified by an
id, is scoped to the surface that opened it (global or one project), and dies
with the tab, the window, or the application.

Approval for xterm.js already exists: it is named in `AGENTS.md`'s approved
technology list. This record does not reopen it.

### The PTY substrate: two small CGO-free libraries, isolated per platform

Proposed: adopt `github.com/creack/pty` for macOS and Linux and
`github.com/UserExistsError/conpty` for Windows, behind one in-repo interface
(`internal/terminal`) so the rest of the engine never imports either directly.
Versions are pinned and verified at implementation time against the toolchain
recorded in ADR-0002, not asserted here.

Why libraries rather than in-repo syscall code, which `AGENTS.md`'s "Go
platform-specific process/PTY facilities" could also be read to allow:

- ConPTY's attribute-list handshake and the `PROC_THREAD_ATTRIBUTE_*` layout are
  undocumented in places and have changed between Windows releases; the library
  that Windows Terminal's ecosystem uses tracks those changes.
- `creack/pty` is the de facto Go PTY for Unix, has no CGO, and is small enough
  to audit; reimplementing `grantpt`/`unlockpt`/`ptsname_r` per libc adds risk
  for no benefit.
- The alternative — an in-repo `pty_windows.go`/`pty_unix.go` pair built on
  `syscall` and `golang.org/x/sys/unix` — avoids two dependencies but makes the
  repository own the ConPTY interop and the macOS/Linux divergence forever.

Both libraries sit behind a four-method interface (`Start`, `Resize`, `Read`,
`Close`, plus `ExitCode`), so either can be replaced without touching the
session layer:

```go
type Session interface {
    Start(spec Spec) (io.ReadWriteCloser, int, error) // returns the pty and pid
    Resize(pty io.ReadWriteCloser, cols, rows uint16) error
    Close(pty io.ReadWriteCloser) error
}
```

**This paragraph is the decision the operator is being asked to make.** If the
operator prefers no new dependencies, the fallback is the in-repo syscall pair
described above, with the same interface; this record's other decisions do not
change.

### Shell and working directory

- **Windows:** `pwsh.exe` when it is on `PATH`, then `powershell.exe`, then
  `%COMSPEC%` (normally `cmd.exe`). The resolved program, its absolute path and
  the reason it was chosen are shown on the tab's first line, so a machine with
  an unusual shell setup is diagnosable without guessing.
- **macOS and Linux:** the user's login shell from `$SHELL` when it resolves to
  an executable, otherwise `/bin/sh`. Locafy does not assume zsh or bash.
- **Working directory:** the project's root for a project-scoped session, the
  user's home directory for a global one. A session's folder is fixed for its
  life; `cd` inside the shell does not move the record.
- The shell is started with **no arguments** and no command string. Locafy never
  builds a command line from anything the user typed elsewhere; the user types
  their own commands into their own shell.
- **Environment:** the engine's own environment, plus the CA variables ADR-0023
  gives every process Locafy starts, so `curl` inside the terminal trusts local
  HTTPS. Project environment injection (DESK-DEVX-001's environment half) does
  not exist yet; when it does, a project session inherits it and this record will
  be amended with the precedence rule rather than inventing one now.

### Terminal size

The window sends `cols`/`rows` from xterm.js on open and on every resize,
debounced to one message per settled size. On Unix the engine sets `TIOCSWINSZ`
on the master; on Windows it calls `ResizePseudoConsole`. A resize before the
shell has started is queued and applied once, not dropped.

### Ending a session

Closing the tab or the window ends the session the way ADR-0013 ends an
application: the engine writes end-of-file, waits a short grace, then terminates
the process tree and releases the PTY. Quitting Locafy ends every session before
the database closes; no shell outlives the engine. The tab shows the exit code
(or the signal) once, and a session that exits on its own leaves the tab in
place with a "press Enter to start a new session" state rather than vanishing.

### Nothing is logged, nothing is persisted

- Terminal bytes are **not** written to the log store, the activity history, a
  file, or the database. They exist in the xterm.js buffer for the tab's life.
  Masking arbitrary escape-sequence streams is not tractable, and a terminal
  transcript is exactly where secrets are typed.
- Scrollback is xterm's in-memory buffer, bounded by the emulator, and is lost
  when the tab closes. A "save transcript" action is deliberately deferred
  (below), not forgotten.
- The engine records only the session's start, end and exit code in its own
  structured log — the same fact an application start records today, never the
  output.

### Failure is stated, never faked

- **Windows older than 10 1809, or a ConPTY allocation failure:** the tab says
  the interactive shell is unavailable on this Windows build and names the
  required version. Locafy does not silently degrade to a pipe-only shell that
  would break full-screen programs; a pipe mode can be proposed later as an
  explicit choice.
- **No usable shell:** the tab names what was looked for and where.
- **A PTY read error:** the session ends and the reason is shown once, not
  retried in a loop.
- Nothing about a terminal session reaches the network, with or without an
  account; the local-first rule applies exactly as it does to a project action.

### Verification before the feature can be called done

- **Unit:** shell resolution per platform (branch on `runtime.GOOS`, never assume
  a host layout), environment assembly (the CA variables are present, the normal
  environment is inherited unchanged), the resize message mapping, and that a
  session start with an unknown shell id fails with the documented message.
- **Integration, per platform:** open a PTY, run the resolved shell with
  `echo locafy-pty-ok`, assert the marker arrives through the PTY and the exit
  code is `0`; resize and assert the child observes the new size (`stty size` on
  Unix, `$Host.UI.RawUI.WindowSize` on Windows); close and assert the process
  tree is gone, the same way ADR-0013's tree tests prove it.
- **Windows capability probe:** a test (or build-tag-guarded skip with a printed
  reason) that reports whether ConPTY is available on the running build rather
  than assuming every CI image has it.
- **CI:** the three-platform matrix already runs Go tests on Windows, macOS and
  Linux; the PTY tests run there, not only locally, and a test that cannot run on
  a platform says why instead of passing silently.
- **Manual:** type in the tab, run a full-screen program (`vim`, `top`), resize
  the window, close the tab mid-program, and quit Locafy with a session open;
  confirm no shell process survives.

## Consequences

- Locafy owns two dependencies it did not have. Both are small, CGO-free, and
  confined to `internal/terminal`; removing them later means one file per
  platform.
- The Windows experience matches other terminals on the platform because it uses
  the same ConPTY; the cost is the Windows 10 1809 floor, stated in the failure
  message.
- Three platforms means three behaviours to test. The records above make the
  differences explicit instead of letting a Windows-only assumption leak into
  the Unix path.
- Terminal output stays out of the log store, so DESK-DEVX-001's secret-masking
  rule holds by construction rather than by a filter that might miss something.
- No transcript means no export. If operators ask for one, it is a separate
  decision with its own disclosure, because it writes secrets to disk.

## Deliberately deferred

- Saving or exporting a terminal transcript.
- Terminal scopes beyond global and project (application- and service-scoped
  tabs), which are a routing question rather than a PTY one.
- Pipe-only shells for pre-ConPTY Windows.
- WSL, Git Bash and other alternative shells as first-class choices.
- Remote or shared terminals — a tunnel feature, not this one.
- Project environment injection into terminal sessions, which waits for the
  environment precedence rules DESK-DEVX-001 still owes.

## Questions this record puts to the operator

1. **The substrate:** adopt `creack/pty` and `UserExistsError/conpty`, or keep
   the PTY layer in-repo on `syscall` and `golang.org/x/sys/unix`? The rest of the
   design is written to work either way.
2. **The Windows floor:** is Windows 10 version 1809 an acceptable minimum for
   the terminal, with a stated refusal below it?
3. **The shell default:** is "pwsh, then powershell, then cmd" the right order on
   Windows, or should the terminal always start `cmd` unless the operator picks?
4. **Transcripts:** confirm that no terminal output is stored, including no
   "export terminal" in this phase.

Approval of this record — or a change to these four answers — is what unblocks
the terminal half of DESK-DEVX-001. No code for it is written until then.
