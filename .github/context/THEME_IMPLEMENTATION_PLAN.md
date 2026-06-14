# Theme / Color Palette Implementation Plan

## Context

Users currently have no control over the app's appearance. The design system in
[web/src/app.css](../../web/src/app.css) already exposes every color as an oklch CSS variable under
`:root` (light) and `.dark` (dark), which is the ideal hook for swappable palettes. Two gaps:

1. `mode-watcher` is installed but **never wired up** — there is no `<ModeWatcher />` in the root
   layout and no light/dark toggle, so dark mode is effectively dead today.
2. There is no palette concept at all.

This change adds a new **Settings → General** section where the user can pick a **color palette**
(3 ColorHunt-based options + the existing default) and a **light/dark/system mode**. Step 1 stores
the choice in `localStorage` only (instant apply, no flash, no backend work); the store is structured
so a DB-backed cross-device sync can be layered in later without rework.

Decisions (confirmed with user):
- **Persistence:** Browser/localStorage now, DB later (store designed for future sync).
- **Theming scope:** Accent-driven — keep neutral background/foreground for readability; the palette
  only recolors `--primary` / `--ring` / `--sidebar-primary` / chart accents.
- **Light/dark:** Wire up `mode-watcher` now and add the toggle in the General section.

## Palettes

Each palette is an `[data-palette="..."]` block overriding only accent tokens, plus a matching
`[data-palette="..."].dark` block. Background/foreground/border stay on the neutral base.

| Key | ColorHunt colors | Primary (light → dark) |
|-----|------------------|------------------------|
| `default` | existing neutral zinc | no override (base `:root`/`.dark`) |
| `ocean` | 2C5EAD · 1591DC · 4BB8FA · C4E2F5 | `#2C5EAD` → `#4BB8FA` |
| `slate` | F5F5F5 · 76ABAE · 303841 · FF5722 | `#76ABAE` (accent/ring `#FF5722`) |
| `forest` | 5B7E3C · FFD65A · FF9D23 · EA5252 | `#5B7E3C` (accent/ring `#FF9D23`) |

For each palette set: `--primary`, `--primary-foreground` (white on the dark primaries), `--ring`,
`--sidebar-primary`, `--sidebar-primary-foreground`, and `--chart-1..3` from the 4 hex colors. Exact
foreground/contrast values get tuned during implementation; hex is fine since these vars are consumed
directly as colors (no need to convert to oklch).

## Implementation

### 1. CSS — palette token blocks
[web/src/app.css](../../web/src/app.css)
- After the existing `:root` / `.dark` blocks, add `[data-palette="ocean"]`, `["slate"]`,
  `["forest"]` (light) and their `.dark` counterparts overriding the accent tokens above.
- `default` needs no block (falls through to base). The `data-palette` attribute lives on
  `document.documentElement`; it composes with the `.dark` class that `mode-watcher` toggles.

### 2. Theme store (palette) — `localStorage` now, DB-ready
New file `web/src/lib/stores/theme.ts`
- Export `PALETTES` metadata (key, label, the 4 swatch hexes — reused by the settings UI to render
  preview swatches).
- Svelte store / runes module holding current palette; `setPalette(key)` writes
  `localStorage['aio-palette']` and sets `document.documentElement.dataset.palette`.
- `initPalette()` reads localStorage (default `'default'`) and applies it on app start.
- Leave a clearly-marked seam (`// TODO: sync to backend`) in `setPalette` so the future
  GET/PATCH `/api/v1/users/preferences` call slots in without touching callers.
- Mode itself stays owned by `mode-watcher` (it already persists to localStorage); the store does not
  duplicate it.

### 3. No-flash pre-paint script
[web/src/app.html](../../web/src/app.html)
- Add a tiny inline `<script>` in `<head>` that reads `localStorage['aio-palette']` and sets
  `document.documentElement.dataset.palette` before first paint. (`mode-watcher` handles the `.dark`
  class equivalently for mode.)

### 4. Wire up mode-watcher + init palette
[web/src/routes/+layout.svelte](../../web/src/routes/+layout.svelte)
- Import and render `<ModeWatcher />`.
- Call `initPalette()` in `onMount` (belt-and-suspenders alongside the app.html script).

### 5. New "General" settings section
[web/src/routes/settings/+page.svelte](../../web/src/routes/settings/+page.svelte)
- Add `{ id: 'general', label: 'General', icon: '🎨' }` to `navItems` (place it first).
- Add an `{:else if activeSection === 'general'}` block rendering:
  - **Color palette** field: a row of selectable cards, one per `PALETTES` entry, each showing the 4
    swatches + label + a selected ring. Clicking calls `setPalette(key)` → applies instantly.
  - **Appearance (mode)** field: Light / Dark / System buttons calling `setMode('light'|'dark')` /
    `resetMode()` from `mode-watcher`, with the active one highlighted via `mode.current`.
- Follow the existing `Field`/`Separator` markup conventions already used in this file.

### Out of scope (future / DB phase)
- Backend `user_preferences` (sqlite migration + Postgres parity), `User` model field,
  repository + `GET/PATCH /api/v1/users/preferences` handler with otel instrumentation, and store
  sync. Captured as the `// TODO` seam in step 2.

## Verification
- `cd web && npm run dev`, log in, open **Settings → General**.
- Select each palette → primary buttons, sidebar active item, and accents recolor immediately;
  background stays readable. Reload → choice persists with no flash.
- Toggle Light/Dark/System → `.dark` class flips, palette accents adapt per-mode; the existing
  Toaster (which reads `mode.current` in
  [web/src/lib/components/ui/sonner/sonner.svelte](../../web/src/lib/components/ui/sonner/sonner.svelte))
  follows the mode.
- Check mobile width: the General section and back-button behavior match the other sections.
- `npm run build` succeeds (type-check clean).
