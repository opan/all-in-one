# Listing Schema Presets — Implementation Plan

## Context

In the listing app, a **Topic** owns a JSONForms `form_schema` (`schema` + `uischema`) that defines
the custom fields of its **Items**. Today the only way to author that schema is to hand-type raw JSON
into a `<Textarea>` in `web/src/components/edit-topic-modal.svelte` — error-prone and unfriendly.

**Goal:** let users start from a curated preset schema, chosen from a **card gallery** where each
card previews its fields, then still tweak the JSON before saving.

Frontend-only: no API, model, migration, or type changes.

## Step 1 — Preset catalog: `web/src/lib/data/listing-schema-presets.ts` (new)

Reuse `FormSchema` from `web/src/lib/types/json-forms.ts`. Do not invent new schema types.

```ts
import type { FormSchema } from "$lib/types/json-forms";

export interface TopicSchemaPreset {
  id: string;
  name: string;
  description: string;
  icon?: string; // emoji, matching the modal's existing emoji style
  formSchema: FormSchema;
}

export const topicSchemaPresets: TopicSchemaPreset[] = [ /* 4 presets below */ ];
```

All field types must stay within what the item renderer in
`web/src/routes/listing/topics/[id]/+page.svelte` supports (enum → Select, boolean → Checkbox,
number/integer → Input, `format: date|date-time|email|uri`). Each preset carries a `uischema`
(`VerticalLayout`, one `Control` per property with `scope: "#/properties/<key>"`) so it passes the
modal's `handleFormSchemaInput()` validation.

### Preset definitions

1. **Product listing** (`🛍️`) — `title` (string, req), `price` (number), `quantity` (integer),
   `category` (enum: electronics/furniture/clothing/other), `in_stock` (boolean, default true),
   `description` (string).
2. **Contact** (`👤`) — `full_name` (string, req), `email` (string, format:email), `phone` (string),
   `company` (string), `notes` (string).
3. **Task / To-do** (`✅`) — `title` (string, req), `status` (enum: todo/in-progress/done,
   default todo), `priority` (enum: low/medium/high, default medium), `due_date` (string,
   format:date), `done` (boolean, default false).
4. **Event** (`📅`) — `name` (string, req), `start` (string, format:date-time), `location` (string),
   `url` (string, format:uri), `notes` (string).

Use `internal/listing/seed/seed.go` and `.context/JSONFORMS_SCHEMA_GUIDE.md` as references for valid
payload shape.

## Step 2 — Preset card gallery: `web/src/components/edit-topic-modal.svelte` (modified)

Keep the existing textarea + live-preview intact. Additions:

- Import `topicSchemaPresets` / `TopicSchemaPreset` and reuse the already-imported `Card` + `Button`.
- Above the "Form Schema (JSON)" textarea in the **left column**, add a *"Start from a preset"*
  section: a compact responsive grid of clickable preset cards. Each card shows `icon` + `name`, the
  `description`, and a **mini field preview** — wrapped chips like `title`, `price (number)`,
  `category (enum)` derived from `Object.entries(preset.formSchema.schema.properties)`, reusing the
  existing `bg-muted px-1.5 py-0.5 rounded` chip styling from the live-preview pane.
- `function applyPreset(preset: TopicSchemaPreset)`:
  ```ts
  formData.form_schema_json = JSON.stringify(preset.formSchema, null, 2);
  handleFormSchemaInput(); // updates jsonError + parsedFormSchema → live preview
  ```
- Optional polish: mark the active preset card (compare current textarea JSON to each preset) and
  keep the empty state reachable (clearing the textarea already shows "No schema defined yet").
  Presets are a starting point only — manual editing and fully custom schemas stay supported.

## Non-goals

- No backend / DB / `json-forms.ts` type / item-renderer changes.
- No new npm dependency (no code editor, no real JSONForms runtime) — consistent with current code.

## Verification

1. `cd web && npm run check` and `npm run build` — clean (a malformed preset fails the type check).
2. Drive the UI (frontend-browser-testing skill, or `npm run dev` + `go run main.go all-in-one server`):
   - Add New Topic → 4 preset cards render with field-chip previews.
   - Click a preset → textarea fills, right-hand live preview shows the same fields; switching
     presets replaces cleanly.
   - Manually edit after applying → confirms preset is only a starting point.
   - Save topic → add an Item → dynamic form renders the preset's fields (enum dropdown, checkbox,
     date picker, etc.).
   - Edit an existing topic with a schema → its schema still loads; presets don't clobber unless clicked.
