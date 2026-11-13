# Development — desktop.locafy

Verified toolchain and workflow for this repository. The toolchain decision and evidence are recorded in [`decisions/ADR-0002-verified-toolchain.md`](decisions/ADR-0002-verified-toolchain.md).

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.27.1 (minimum 1.26) | Required by Wails v3 |
| Node.js + npm | 22.14.0 / 11.19.0 | Frontend tooling |
| Wails CLI | v3.0.0 | Command is `wails3` (not `wails`) |
| WebView2 Runtime | 157.x | Preinstalled on Windows 10/11 |

Platform dependencies per OS:

- **Windows:** WebView2 Runtime only (usually preinstalled).
- **macOS:** Xcode Command Line Tools (`xcode-select --install`).
- **Linux:** GTK4 + WebKitGTK 6.0 dev packages (Ubuntu 24.04+ / Debian 13+). Older distributions build with `-tags gtk3` (supported through v3.0.x only).

## Install the Wails CLI

```
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
wails3 doctor
```

`wails3 doctor` must report no issues before development starts. If `wails3` is not found, add `%USERPROFILE%\go\bin` (Windows) or `~/go/bin` (macOS/Linux) to PATH.

## Project workflow

```
wails3 dev     # development with hot reload; generates bindings into frontend/bindings/
wails3 build   # production build into bin/
wails3 init -l # list available templates
```

## Verification commands

```
go test ./internal/...    # engine package tests
go vet . ./internal/...   # static checks
go build . ./internal/... # compile check
```

Note: `build/ios` and `build/android` contain mobile build scaffolding that only compiles under `ios`/`android` build tags, so `go build ./...` reports an empty `main` package there. Scope vet/build/test to `.` and `./internal/...`; `wails3 build` handles platform targets itself.

Rules:

- Files in `frontend/bindings/` are generated during build/dev and are never edited by hand.
- Build orchestration lives in `Taskfile.yml`; prefer adding tasks there over ad-hoc scripts.
- Cross-platform builds (per ADR-0001) follow the official cross-platform guide; CI builds and tests on Windows, macOS, and Linux.

## Packaging (later phase)

Windows installers require NSIS; MSIX requires MakeAppx/SignTool. These are not required for development and are addressed in Phase 10.

