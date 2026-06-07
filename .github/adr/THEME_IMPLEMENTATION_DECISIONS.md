# Theme / Color Palette — Architecture Decision Record

**Date**: 2026-06-07
**Status**: ✅ Implemented (branch `feat/user-themes`)
**Companion docs**: [THEME_IMPLEMENTATION_PLAN.md](../context/THEME_IMPLEMENTATION_PLAN.md), [THEME_PROGRESS.md](../context/THEME_PROGRESS.md)

---

## Decision summary

Ship user-selectable theming as a **client-only**, **accent-driven** feature. Persist the palette
choice in `localStorage`; let `mode-watcher` continue owning light/dark mode. Expose a Settings →
General section as the single entry point. Defer the DB-backed cross-device sync to a future phase
behind a clearly-marked seam.

---

## Context

Two gaps existed before this work:

1. **Dark mode was dead.** `mode-watcher` was installed but never rendered — there was no
   `<ModeWatcher />` in any layout, so the `.dark` class never toggled. The CSS variables for dark
   mode in `web/src/app.css` were unreachable.
2. **No palette concept.** Every user got the same neutral zinc accents.

The CSS already exposed every color as an `oklch` variable under `:root` and `.dark`, which is the
ideal hook for swappable palettes — so the work was wiring, not redesign.

---

## Decisions

### 1. Accent-driven theming, not full re-skin

Only these tokens are overridden per palette:
`--primary`, `--primary-foreground`, `--ring`, `--sidebar-primary`,
`--sidebar-primary-foreground`, `--chart-1..3`.

Background, foreground, border, muted, card, popover all stay on the neutral base.

**Why:** Full re-skinning risks unreadable contrast across the long tail of shadcn components. The
accent surface area is small, easy to audit, and gives a visibly different feel per palette
without forcing every screen through manual contrast review.

### 2. `[data-palette="..."]` attribute on `<html>`, composed with `.dark` class

Each palette gets two blocks: `[data-palette="x"]` (light) and `[data-palette="x"].dark` (dark).
The `default` palette has no block — it falls through to base `:root` / `.dark`.

Specificity ordering:
- `:root` (0,1,0) vs `[data-palette="x"]` (0,1,0) — palette wins by source order (placed after).
- `.dark` (0,1,0) vs `[data-palette="x"].dark` (0,2,0) — palette wins by specificity.

**Why an attribute, not a class?** Two reasons:
- Class would collide with `.dark` (one `class` attribute, multiple classes is fine but mixes
  concerns). Attribute keeps palette and mode orthogonal.
- `mode-watcher` writes to `class`; we write to `dataset`. No coordination needed.

### 3. Hex values, not oklch

The plan deliberately used hex codes from ColorHunt directly. We did not convert to `oklch`.

**Why:** The CSS custom properties are consumed as colors with no need for the oklch interpolation
benefits. The 4 hex codes per palette are the source-of-truth in `PALETTES` metadata; reusing them
in CSS keeps the swatch preview in Settings exactly matching the rendered accent.

**Tradeoff:** No automatic perceptual lightness scaling between modes. We tune contrast manually
per `(palette, mode)` pair. Notable case: ocean dark uses `--primary-foreground: #0a1929` because
white on `#4BB8FA` fails WCAG (~2.5:1).

### 4. `localStorage` now, DB-ready

The theme store writes to `localStorage['aio-palette']` and leaves a single comment seam in
`setPalette`:

```ts
// TODO: sync to backend (PATCH /api/v1/users/preferences) once the endpoint exists.
```

**Why:** A user-preferences table requires migration + Postgres parity + repository + handler +
otel + store sync — easily 3-4× the work of the visible feature. Shipping the visible feature
first delivers value; the seam guarantees the future sync slots in without touching callers.

**Tradeoff:** Choice doesn't follow the user across devices yet. Acceptable for a personal-learning
app; would not be acceptable in a multi-device prod setting.

### 5. Two-script application: inline pre-paint + `onMount` belt-and-suspenders

A small inline IIFE in `app.html` reads localStorage and sets `dataset.palette` before first
paint. `initPalette()` runs again in the root layout's `onMount`.

**Why both?**
- Pre-paint script prevents the flash of wrong-palette during hydration.
- `onMount` keeps the Svelte store (`palette`) in sync with the DOM so the Settings UI shows the
  correct selected card.

The pre-paint script duplicates the valid-key list (`['default','ocean','slate','forest']`). The
comment in `app.html` flags this as a sync point with `PALETTES` in the theme store. Acceptable
duplication given there's no clean way to share a constant between inline boot script and module
code.

### 6. Mode owned by `mode-watcher`, not duplicated

`mode-watcher` already persists mode to its own localStorage key and renders the `.dark` class.
The theme store doesn't touch mode at all.

**Why:** Duplicating mode state risks drift between two sources. The Settings UI calls
`mode-watcher`'s `setMode` / `resetMode` directly.

### 7. `userPrefersMode.current`, not `mode.current`, drives the toggle highlight

The Light/Dark/System buttons in Settings highlight based on `userPrefersMode.current` — the
*preference* — not `mode.current` — the *resolved* value.

**Why:** If the user picks "System" and the system is dark, `mode.current === 'dark'` but the
user's intent was "follow system". Highlighting "Dark" would be misleading. The preference is
what the UI control represents.

### 8. Classic Svelte `writable` store, not `.svelte.ts` rune module

`web/src/lib/stores/theme.ts` uses `writable<PaletteKey>` from `svelte/store`, accessed via
`$palette` auto-subscription in templates.

**Why:** No `.svelte.ts` rune modules existed elsewhere in the codebase. Introducing one for this
single store would be inconsistent and require a separate justification. The classic store works
fine for the access patterns here (read in template via `$palette`, write via `setPalette`).

---

## Files touched

| File | Change |
|------|--------|
| `web/src/app.css` | +6 palette blocks (ocean/slate/forest × light/dark) |
| `web/src/app.html` | +inline IIFE pre-paint script |
| `web/src/lib/stores/theme.ts` | new — `PALETTES`, `palette`, `setPalette`, `initPalette` |
| `web/src/routes/+layout.svelte` | +`<ModeWatcher />`, +`initPalette()` in `onMount` |
| `web/src/routes/settings/+page.svelte` | +`general` nav item & section (palette cards + mode buttons) |

---

## Out of scope (future work)

The future DB-sync phase will add:

- SQLite migration `05_user_preferences.sql` + Postgres parity
- `UserPreferences` model with `palette` field
- Repository + `GET/PATCH /api/v1/users/preferences` handler
- otel instrumentation on the new endpoint (per CLAUDE.md backend guidelines)
- Store sync: on login load, fetch preferences and `setPalette(...)`; in `setPalette`, fire-and-forget the PATCH
- Migration path for users who already have a localStorage value (server value wins if both exist
  and they conflict — fresh login = source of truth)

The single `// TODO` comment in `setPalette` is the only code that needs to change in the
existing store; everything else is additive.

---

## Verification (manual, post-merge owner)

- [ ] Settings → General is the landing tab.
- [ ] Each of the 4 palette cards applies instantly when clicked; primary buttons + active sidebar
      item recolor; backgrounds stay readable.
- [ ] Light / Dark / System buttons flip `.dark` on `<html>` and the active button stays
      highlighted correctly across all three states.
- [ ] Reload page → palette and mode persist with no flash.
- [ ] Mobile width (`< md`): General section + "← Back" button behave like other sections.
- [ ] Login/register pages also respect mode + palette (`<ModeWatcher />` is outside the
      `isAuthPage` branch).
