# Theme / Color Palette — Progress Tracker

Companion to [THEME_IMPLEMENTATION_PLAN.md](./THEME_IMPLEMENTATION_PLAN.md). Branch: `feat/user-themes`.

## Status

| # | Step | File(s) | Status |
|---|------|---------|--------|
| 1 | Palette CSS token blocks | `web/src/app.css` | ✅ done |
| 2 | Theme store (`PALETTES`, `setPalette`, `initPalette`) | `web/src/lib/stores/theme.ts` (new) | ✅ done |
| 3 | No-flash pre-paint script | `web/src/app.html` | ✅ done |
| 4 | Wire `<ModeWatcher />` + `initPalette()` | `web/src/routes/+layout.svelte` | ✅ done |
| 5 | Settings → General section (palette + mode pickers) | `web/src/routes/settings/+page.svelte` | ✅ done |

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

## Step 3 — done

Added an inline IIFE pre-paint script to `web/src/app.html` (between `<meta viewport>` and
`%sveltekit.head%`). Reads `localStorage['aio-palette']`, validates against the hardcoded list
`['default','ocean','slate','forest']`, and sets `document.documentElement.dataset.palette`
before first paint. Wrapped in `try/catch` to survive disabled localStorage.

**Sync point:** the hardcoded list must match `PALETTES` in `web/src/lib/stores/theme.ts` —
comment in the script notes this.

## Step 4 — done

`web/src/routes/+layout.svelte`:
- Added `import { onMount } from 'svelte'`, `import { ModeWatcher } from 'mode-watcher'`,
  `import { initPalette } from '$lib/stores/theme'`.
- `onMount(() => initPalette())` — belt-and-suspenders alongside the app.html script.
- Rendered `<ModeWatcher />` once after `<svelte:head>`, **outside** the `isAuthPage` branch so
  login/register pages also respect mode + palette.

After this step, dark mode is live for the first time (mode-watcher was installed but never
rendered before). Palette wiring is also live — visiting any page reads localStorage and
applies the palette via CSS. No UI to change it yet — that's Step 5.

## Step 5 — done

`web/src/routes/settings/+page.svelte`:
- Added imports for `mode-watcher` (`setMode`, `resetMode`, `userPrefersMode`) and theme store
  (`PALETTES`, `palette`, `setPalette`).
- Prepended `{ id: 'general', label: 'General', icon: '🎨' }` to `navItems`; default
  `activeSection` changed from `'account'` to `'general'`.
- Inserted a new `{#if activeSection === 'general'}` branch as the first arm of the conditional
  chain. Contains:
  - **Color palette**: `grid-cols-2 sm:grid-cols-4` of 4 cards (default + 3 palettes). Each card
    shows the 4 swatches + label. Selected card uses `border-ring`, which itself is palette-driven
    — the selected ring inherits the chosen accent.
  - `<Separator />`
  - **Appearance**: 3 buttons (Light / Dark / System). Active button uses `variant="default"`,
    inactive uses `variant="outline"`. Active state derived from `userPrefersMode.current` (not
    `mode.current`) so "System" stays highlighted regardless of resolved value.

`svelte-check`: 0 errors. `npm run build`: ✓ done.

## Whole feature — done

All 5 steps complete on branch `feat/user-themes`. ADR: [THEME_IMPLEMENTATION_DECISIONS.md](../adr/THEME_IMPLEMENTATION_DECISIONS.md).
Next move: PR.

## Decisions still open

None — values are placeholders that can be tuned in later steps if the live preview reveals issues.

## Out of scope (deferred to future DB-sync phase)

Backend `user_preferences` table, `GET/PATCH /api/v1/users/preferences`, store sync.
The theme store (Step 2) will leave a `// TODO: sync to backend` seam so this slots in later
without touching callers.
