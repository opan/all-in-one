# Bug: Listing page is disconnected from the backend (items vs topics)

**Found:** 2026-08-08, during landing-page screenshot refresh (`feat/landing-page`).
**Present on:** `main`, `feat/landing-page`, `feat/listing-schema-presets` (shared frontend/backend).
**Severity:** the `/listing` page was 500-ing for every user (now stopgapped).

## Symptom
Visiting `/listing` rendered the SvelteKit **500 "Internal Error"** page.

## Root cause
`web/src/routes/listing/+page.ts` `load` fetched **`/api/v1/items`**, which the backend never
registers. `internal/listing/handler/handler.go` only registers:
- `GET/POST /api/v1/topics`, `GET/PUT/DELETE /api/v1/topics/{id}`
- `GET/POST /api/v1/topics/{topic_id}/items`, `.../items/{id}` (GET/PUT/DELETE)

There is **no `/api/v1/items`** (unscoped). The request fell through to the SPA fallback file server,
which returns `index.html` with HTTP **200**. `load` saw `res.ok === true`, called `res.json()` on
HTML → threw → SvelteKit rendered the 500 page.

## Stopgap applied (2026-08-08)
`listing/+page.ts` load repointed to `/api/v1/topics` so the page renders (unblocks the landing
screenshot and stops the 500). See the NOTE comment in that file.

## Still broken — needs a real fix (NOT done)
1. `listing/+page.svelte` renders a **hardcoded** `Item[]` placeholder (`First Item`…`Fourth Item`) and
   **ignores** the load's `data.listings`. Real data never shows.
2. Create/edit/delete in that page call `/api/v1/items` and `/api/v1/items/{id}` (`apiPost`/`apiPut`/
   `apiDelete`) — also nonexistent → those actions fail.
3. The page's domain model (`Item{title,description}`) doesn't match the backend (`topics`, and
   items are always nested under a topic: `/topics/{topic_id}/items`).

## Suggested proper fix (separate task)
Decide the page's intent (topics list, or items-under-a-topic), then wire `load` + render + CRUD to the
matching real endpoints, replace the hardcoded array with `data`, and align the `Item`/`Topic` types.
Consider the `feat/listing-schema-presets` branch — it may already be reworking this area.
