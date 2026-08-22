# Landing Page — Progress Tracker

**Branch:** `feat/landing-page` (10 commits on top of `main` @ `770e6a6`)
**Status:** 🚧 In progress — three follow-up improvements requested by opan.
**Last updated:** 2026-08-08

## Goal
Polish the public landing page (`web/src/routes/+page.svelte`) with three changes:

1. **Refresh product screenshots** — the committed shots in `web/static/landing/*.png` show the
   *old* rounded/shadowed theme (black "Add New Topic", bright-red "Delete", rounded inputs). Regenerate
   them from the current **Modernist Blue sharp-edge** theme so the landing matches the real app.
2. **Features: diagonal → carousel** — replace the diagonal cascade (`.features-grid` + `.flow-line`
   SVG + staggered `.feature-*` cards) with a **manual, one-slide-at-a-time carousel** (big screenshot +
   copy per slide, prev/next arrows + dot indicators, loops, no autoplay). Also removes the
   reveal-on-scroll blank-space issue that hid cards below the fold.
3. **Show the security feature (2FA)** — 2FA/TOTP is already shipped (`9746350 feat: add 2FA-TOTP (#15)`;
   Settings → Advanced tab, `/api/v1/users/2fa/*`). Add a **separate "Security built in" band** below
   the carousel with the real 2FA settings screenshot + copy.

### Locked decisions
- Carousel: **one slide at a time, manual** (arrows + dots, loop, no autoplay).
- 2FA: **separate security band**, not a 4th carousel slide.
- Carousel is **hand-rolled** (Svelte 5 transform slider) — no embla/shadcn carousel dep (keep deps
  minimal per CLAUDE.md; also nothing to `npx add` in this sandbox).

## Screenshots to regenerate (from the sharp-edge themed app, seeded scratch DB)
- [x] `dashboard.png` — hero, `/home` full app (with sidebar), admin ✅ new sharp-edge shot
- [x] `listing.png` — ✅ re-shot from the REAL `/listing/topics` page (Bookshelf / Home Inventory /
      Recipes). The first attempt mistakenly used the orphan `/listing` stub; corrected.
- [x] `chat.png` — `/chat` main content area ✅
- [x] `shortener.png` — `/shortener` main content area ✅ (populated 6 sample links w/ click counts)
- [x] `twofa.png` — NEW, the QR "Set Up Two-Factor Authentication" setup card ✅

## Listing: CORRECTED (see .context/LISTING_BACKEND_DISCONNECT.md)
An earlier note here claimed the listing feature was broken. **That was wrong** — the real pages
`/listing/topics` and `/listing/topics/[id]` work (the `-maxdepth 2` search had hidden them). The app's
nav links to `/listing/topics`, which the feature actually uses. The **only** broken thing was the
orphaned bare `/listing` stub (nothing linked to it), now fixed with a `307` redirect to
`/listing/topics`. The landing `listing.png` was re-shot from the real page.

## Checklist
- [x] Rework features section in `+page.svelte`: carousel markup + state + CSS; remove diagonal CSS.
- [x] Add "Security built in" band (2FA) below the carousel.
- [x] Regenerate screenshots into `web/static/landing/` (all except listing — blocked).
- [x] `npm run check` (0 errors, landing file clean) + `npm run build` clean.
- [x] Live headless-browser verify of the new landing (light + dark) — carousel + security band work.
- [x] Resolve listing slide — quick-unblock (load → `/api/v1/topics`), sharp-edge shot installed.
- [ ] ADR note in `docs/adr/` (carousel choice / security band) — pending, once opan signs off.
- [ ] `git add` the new/changed screenshots + `+page.svelte` + `listing/+page.ts` and commit.

## Notes
- App routes: `/home`, `/listing`, `/chat`, `/shortener`, `/settings`. Main content = `<main class="flex-1 overflow-auto">`.
- Seeded users: `admin`/`admin123`, `user`/`user123`, `demo`/`demo123`.
- Landing page is self-contained: its own Modernist Blue tokens in the component `<style>` (light `:root`
  + `:global(.dark) .landing`), independent of the app-wide theme tokens.
