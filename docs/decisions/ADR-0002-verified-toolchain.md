# ADR-0002: Verified desktop toolchain versions

- **Date:** 2026-11-13
- **Status:** Accepted
- **Depends on:** ADR-0001 (cross-platform support from day one)

## Context

The PRD requires that Wails v3, Go, and supporting toolchain versions be verified against official sources before any version is hardcoded into implementation scripts or documentation (PRD section 6.1, section 23). Verification was performed against the official Wails v3 installation documentation (v3.wails.io/quick-start/installation/, last updated 2026-11-13) and confirmed by running `wails3 doctor` on a Windows 11 machine.

**Re-checked on 2026-11-24:** `wails3 version` → v3.0.0, `go version` → go1.27.1 windows/amd64, `node --version` → v22.14.0, `npm --version` → 11.19.0, and `wails3 doctor` reports Windows 11 Pro (build 22000) with WebView2 157.0.4341.60. The Go, Node and npm pins still match what is recorded; the WebView2 runtime has moved past the build this record first named, which is expected of an Evergreen runtime. Only Windows was re-checked — macOS and Linux were not run, here or in the first pass.

## Decision

The desktop repository targets the following verified toolchain:

| Tool | Version | Verified by |
|---|---|---|
| Wails CLI (`wails3`) | v3.0.0 (stable) | `wails3 version` |
| Go | 1.27.1 (minimum 1.26 per Wails docs) | `go version`, `wails3 doctor` |
| Node.js | 22.14.0 (LTS) | `node --version` |
| npm | 11.19.0 | `npm --version` |
| WebView2 Runtime (Windows) | 157.0.4341.46 recorded; 157.0.4341.60 on 2026-11-24 | `wails3 doctor` — Evergreen, so the build number moves with Microsoft's updater |

Everything in the Verified by column was run on the Windows 11 machine that built the first slice; nothing in the table was measured on macOS or Linux. The macOS and Linux requirements below come from the Wails documentation, not from a run here.

Supporting facts verified from the docs:

- The CLI command is `wails3` (not `wails`, which belongs to v2). Install with `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`.
- `wails3 init -n <name> -t react` scaffolds a Go + React + TypeScript + Vite project with a `Taskfile.yml` build system.
- Service structs registered via `application.NewService()` get type-safe TypeScript bindings generated into `frontend/bindings/` during `wails3 dev` / `wails3 build`; generated bindings are never edited by hand.
- `wails3 dev` provides Go rebuild/restart and frontend hot reload; `wails3 build` produces native executables per platform.
- Platform dependencies: Windows needs only WebView2 (verified present); macOS needs Xcode CLT; Linux needs GTK4 + WebKitGTK 6.0 (Ubuntu 24.04+/Debian 13+), with a legacy `gtk3` build tag for older distributions (supported through v3.0.x, removed in v3.1).
- Cross-platform builds have first-class documentation support (guides/build/cross-platform).

Known optional gaps (not Phase 0/1 blockers; relevant from Phase 10 packaging):
- NSIS not installed (Windows installer builds).
- MakeAppx/SignTool not installed (MSIX/signing).
- Docker not installed (cross-compilation container images).

## Consequences

- Implementation scripts and documentation may pin Go >= 1.26 and reference `wails3` v3.0.0 explicitly.
- The Linux requirement (WebKitGTK 6.0, Ubuntu 24.04+/Debian 13+) constrains the minimum supported Linux distributions; the legacy `gtk3` tag covers older ones and its removal in v3.1 must be tracked.
- CI (per ADR-0001/LOC-FOUND-001) must provide build/test jobs on Windows, macOS, and Linux; Linux CI images need the GTK4/WebKitGTK 6.0 packages.
- Version pins must be re-verified when any component is upgraded.
