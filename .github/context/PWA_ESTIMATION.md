# PWA Estimation

Recorded: 2026-05-10
Status: Not started — kept for future reference.

## Overview

SvelteKit + Vite has first-class PWA support via `@vite-plugin-pwa`. Most of the
work is configuration rather than custom code, making this feasible without a
large time investment.

## Effort Estimate

| Piece | What it involves | Estimate |
|---|---|---|
| Web manifest + icons | `manifest.json`, icons in ~6 sizes, theme color, display mode | 0.5 day |
| Service worker + asset caching | Auto-precache static assets via `@vite-plugin-pwa` | 0.5 day |
| Offline fallback page | Show a friendly page when network is gone | 0.5 day |
| Offline-aware UI | Detect `navigator.onLine`, disable chat/reload when offline | 0.5 day |
| Push notifications | New backend endpoint (Web Push), service worker handler, permission UI | 2–3 days |
| Background sync | Queue failed API calls and replay when online | 2–3 days |

**Installable + usable offline for cached pages:** ~2 days
**Full PWA with push notifications + background sync:** ~7–8 days

## Caveats for This App

### Works well as PWA
- Listing / topics browsing (cache-first, works offline)
- Settings
- General navigation

### Tricky parts
- **Chat** — WebSockets cannot be intercepted by a service worker. Offline
  graceful degradation is needed but true offline chat is not possible.
- **JWT cookie auth** — service workers intercept all fetches; token refresh
  flow needs to remain intact.
- **Push on iOS** — Web Push on iOS Safari requires iOS 16.4+ and the app must
  be installed to the home screen. Still more limited than native.

## Suggested Starting Point

1. Install `@vite-plugin-pwa` in `web/`
2. Configure manifest (name, icons, theme colour, `display: standalone`)
3. Choose caching strategy: `NetworkFirst` for API routes, `CacheFirst` for
   static assets
4. Add an offline fallback route in SvelteKit

This alone gives the "Add to Home Screen" prompt — the biggest UX win — in
roughly half a day.

## Bottom Line

If the goal is to avoid a native app for **listing and settings**, a PWA covers
~80% of the use case in ~2 days. For real-time chat parity with native, the gap
does not fully close regardless of PWA investment.
