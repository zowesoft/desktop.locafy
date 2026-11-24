# ADR-0006: Support a light theme alongside the dark default

- **Status:** Accepted
- **Date:** 2026-11-14

## Context

The design system shipped dark-only, with the palette comment recording that
the dark theme was the only one until a light-mode decision existed. A light
palette and a top-bar toggle have since been added, so the scope has widened
without a record. The palette, the shell and the native window chrome all made
dark-only assumptions that the light theme has to answer for.

## Decision

Support both themes as first-class palettes.

- `:root` holds the light palette and `.dark` holds the dark one. The document
  carries `dark` by default, matching the design mockups.
- The choice is persisted in `localStorage` under `locafy-theme` and applied by
  an inline script in `index.html` before first paint, so a light-theme user
  never sees the dark palette render first.
- Each palette sets `color-scheme`, so native scrollbars, focus rings and form
  controls follow the active theme rather than staying dark.
- The window background colour is set to the dark palette's background, which
  is the default theme, and the inline stylesheet paints the correct colour for
  the stored theme before the bundle loads.
- Tokens that carry meaning adjust per theme. In light, `--success` and
  `--muted-foreground` are darkened until status and secondary text clear a
  4.5:1 contrast ratio on the tints they actually sit on, not only on white —
  measured on 2026-11-24 from the token values, with the ratios in Consequences.

## Windows caption bar

The caption bar is drawn by Windows, and Wails applies its theme when the
window is created; the version in use exposes no runtime call to re-theme it.
The window therefore declares a dark caption with the palette's colours, and a
light-theme session keeps a dark caption.

This is an accepted, visible limitation rather than an oversight. Removing it
requires either a frameless window with an application-drawn caption, or an
upstream runtime theming call; both are larger changes than the mismatch
justifies today.

## Consequences

- Colour is never the only signal: the palettes must keep text labels next to
  their status colours, since a colour that reads well on one palette may not
  on the other.
- Component styles must use semantic tokens. A literal colour that looks right
  in dark is a defect in light. The two tokens this record adjusts were measured
  on 2026-11-24 from `frontend/src/index.css`: `--success` `#0B6B33` reads
  6.02:1 on `--background`, 6.63:1 on a card and 5.71:1 on a 10% success tint,
  and `--muted-foreground` `#5C596B` reads 6.16:1, 6.78:1 and 5.83:1 on the same
  surfaces — every pair above 4.5:1. What the two earlier failures were is not
  recorded anywhere, so the ratios above are the check that stands rather than
  a recollection of finding them.
- The toggle lives in the top bar and is labelled with its action. The system
  preference is not followed: the product default is dark until the user
  chooses otherwise.
