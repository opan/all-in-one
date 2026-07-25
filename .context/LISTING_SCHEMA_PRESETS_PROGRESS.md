# Listing Schema Presets — Progress

## Feature summary

Let users author a Topic's `form_schema` by starting from a curated **preset** instead of a blank
JSON textarea. Presets are shown as a **card gallery** in the topic create/edit modal; each card
previews the fields it defines. Selecting a preset fills the schema editor and the existing live
preview, and the user can still hand-edit or clear afterward.

This is **frontend-only** — the backend already stores/serves `form_schema` unchanged.

## Key files

- `web/src/lib/data/listing-schema-presets.ts` — **new**, preset catalog.
- `web/src/components/edit-topic-modal.svelte` — **modified**, preset card gallery + `applyPreset`.
- `web/src/lib/types/json-forms.ts` — reused `FormSchema` type (unchanged).
- Reference: `internal/listing/seed/seed.go`, `.context/JSONFORMS_SCHEMA_GUIDE.md`,
  `.context/LISTING_UI_IMPLEMENTATION.md`.

## Presets shipped (4)

1. **Product listing** — title (req), price (number), quantity (integer), category (enum),
   in_stock (boolean), description.
2. **Contact** — full_name (req), email (format:email), phone, company, notes.
3. **Task / To-do** — title (req), status (enum), priority (enum), due_date (format:date),
   done (boolean).
4. **Event** — name (req), start (format:date-time), location, url (format:uri), notes.

## Task checklist

- [x] Documentation (this file, implementation plan, ADR)
- [x] Create `listing-schema-presets.ts` with 4 presets typed against `FormSchema`
- [x] Add preset card gallery + `applyPreset` to `edit-topic-modal.svelte`
- [x] Verify: `npm run check` (0 errors) + `npm run build` (clean)
- [x] Verify: drive the UI (apply preset → preview → save topic → item form renders fields) — 11/11 e2e checks passed

## Status

Complete. All four presets render as a card gallery with field-chip previews; selecting one fills the
schema editor and the existing live preview; switching presets replaces cleanly; saved topics produce
item forms with the preset's fields. Verified via `npm run check`, `npm run build`, and a Playwright
end-to-end run (11/11 checks). No backend changes.

## Notes for resuming

- All preset field types must stay within what the item renderer in
  `web/src/routes/listing/topics/[id]/+page.svelte` supports: enum → Select, boolean → Checkbox,
  number/integer → Input, `format: date|date-time|email|uri`.
- Each preset needs a `uischema` (`VerticalLayout` with one `Control` per property,
  `scope: "#/properties/<key>"`) so it passes the modal's `handleFormSchemaInput()` validation.
- Applying a preset reuses the existing `handleFormSchemaInput()` so the live preview updates for free.
