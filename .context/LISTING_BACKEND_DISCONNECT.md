# Listing route cleanup — corrected findings

**Updated:** 2026-08-08. **This supersedes an earlier, incorrect version of this file.**

## Correction
An earlier investigation concluded "the listing frontend never worked / is a stub." **That was wrong** —
it was based only on the orphaned `/listing` route and missed the real pages (a `-maxdepth 2` file search
hid them). The listing feature **works** and always has.

## Actual state (verified live, 2026-08-08)
- **`/listing/topics`** — real topics list. Loads `/api/v1/topics`, renders real data, full topic CRUD
  (create/edit/delete) via `EditTopicModal`. Works.
- **`/listing/topics/[id]`** — real item view under a topic. Loads `/api/v1/topics/{id}` +
  `/api/v1/topics/{id}/items`, full item CRUD, plus edit/delete topic. Works.
- The app's **sidebar** and **home dashboard** both link "Listings" → `/listing/topics` (the working page).
- Backend is fully functional: topics CRUD + nested items CRUD all verified (201/200).
- These routes exist on `main`, `feat/landing-page`, and `feat/listing-schema-presets` alike.
  (`feat/listing-schema-presets` adds JSON-schema presets to the topic editor — one extra commit.)

## The only real problem (now fixed)
The bare **`/listing`** route was a leftover **orphan stub**: it fetched the nonexistent `/api/v1/items`
(→ SPA-fallback HTML → `res.json()` threw → 500) and rendered hardcoded `First Item…` placeholders with
CRUD pointed at `/api/v1/items[/id]`. **Nothing in the app linked to it.**

**Fix applied:** `/listing/+page.ts` now `throw redirect(307, "/listing/topics")`; the stub
`+page.svelte` is reduced to a redirect fallback. (This replaces the earlier stopgap that repointed the
stub's load to `/api/v1/topics` but still rendered the hardcoded table.)

## Landing follow-up (done)
The landing carousel's `listing.png` had been shot from the broken `/listing` stub ("First Item…"). It's
been re-captured from the real `/listing/topics` page (Bookshelf / Home Inventory / Recipes).
