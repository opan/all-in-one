# Theme / Color Palette — Progress Tracker

Companion to [THEME_IMPLEMENTATION_PLAN.md](./THEME_IMPLEMENTATION_PLAN.md). Branch: `feat/user-themes`.

## Status

| # | Step | File(s) | Status |
|---|------|---------|--------|
| 1 | Palette CSS token blocks | `web/src/app.css` | ✅ done |
| 2 | Theme store (`PALETTES`, `setPalette`, `initPalette`) | `web/src/lib/stores/theme.ts` (new) | ✅ done |
| 3 | No-flash pre-paint script | `web/src/app.html` | ⏳ pending |
| 4 | Wire `<ModeWatcher />` + `initPalette()` | `web/src/routes/+layout.svelte` | ⏳ pending |
| 5 | Settings → General section (palette + mode pickers) | `web/src/routes/settings/+page.svelte` | ⏳ pending |

## Step 1 — done

Added 6 CSS blocks to `web/src/app.css` between the existing `.dark` block and `@theme inline`:
- `[data-palette="ocean"]` + `[data-palette="ocean"].dark`
- `[data-palette="slate"]` + `[data-palette="slate"].dark`
- `[data-palette="forest"]` + `[data-palette="forest"].dark`

Each block overrides only accent tokens (`--primary`, `--primary-foreground`, `--ring`,
`--sidebar-primary`, `--sidebar-primary-foreground`, `--chart-1..3`). The `default` palette
needs no block — it falls through to base `:root` / `.dark`.

Specificity: `[data-palette="x"].dark` (0,2,0) > `.dark` (0,1,0), so dark-mode overrides win.
`[data-palette="x"]` ties with `:root` and wins by source order (placed after).

### Notable contrast choices
- **Ocean dark**: `--primary: #4BB8FA` with `--primary-foreground: #0a1929` — white on #4BB8FA
  fails WCAG (~2.5:1); dark text gives ~7:1.
- **Slate** ring is `#FF5722` (orange) over the muted teal primary for focus-ring pop.
- **Forest** ring is `#FF9D23` (orange) over the green primary, same idea.

No element sets `data-palette` yet, so visually nothing changes until Step 3/4.

## Step 2 — done

New file `web/src/lib/stores/theme.ts` exports:
- `PaletteKey`, `PaletteMeta`, `PALETTES` (4 entries with label + 4 swatch hexes)
- `palette` writable store (subscribe via `$palette`)
- `setPalette(key)` — updates store + localStorage + `documentElement.dataset.palette`, with a
  `// TODO: sync to backend (PATCH /api/v1/users/preferences)` seam
- `initPalette()` — reads `localStorage['aio-palette']`, falls back to `"default"`

SSR-safe via `import { browser } from "$app/environment"` (same pattern as `lib/api.ts`).
Uses classic `writable` rather than a `.svelte.ts` rune module to match existing project style.
`svelte-check`: 0 errors related to this file (14 pre-existing warnings in other files).

## Decisions still open

None — values are placeholders that can be tuned in later steps if the live preview reveals issues.

## Out of scope (deferred to future DB-sync phase)

Backend `user_preferences` table, `GET/PATCH /api/v1/users/preferences`, store sync.
The theme store (Step 2) will leave a `// TODO: sync to backend` seam so this slots in later
without touching callers.
