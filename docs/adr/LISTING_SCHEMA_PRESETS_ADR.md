# ADR: Listing Schema Presets (app-feature)

This document records design decisions made when adding **schema presets** to the listing app's
topic editor — a set of curated, ready-made JSONForms `form_schema` starting points a user can pick
from a card gallery (with a per-preset field preview) instead of hand-typing raw JSON. Add a new
entry here for any future change that touches listing schema presets.

Implementation status is tracked in `.context/LISTING_SCHEMA_PRESETS_PROGRESS.md`; the build recipe
is in `.context/LISTING_SCHEMA_PRESETS_IMPLEMENTATION_PLAN.md`. This document records the *decisions*.

---

## ADR-001: Presets as a frontend-only, static catalog reusing the existing `FormSchema`

### Status
Accepted

### Context
A Topic's custom item fields are defined by its `form_schema` (JSONForms `schema` + `uischema`).
Today that schema is authored only by hand-typing raw JSON into a `<Textarea>` in
`web/src/components/edit-topic-modal.svelte` — error-prone and unfriendly to non-technical users.
The requirement: let users select a preset standard schema and preview each one, with at least 3
presets available.

### Decision
Add a static, typed preset catalog in `web/src/lib/data/listing-schema-presets.ts`
(`topicSchemaPresets: TopicSchemaPreset[]`, each carrying a `FormSchema`), and render it as a **card
gallery** in the existing topic modal. Selecting a preset writes its JSON into the current schema
textarea and reuses the existing `handleFormSchemaInput()` so the existing live-preview pane updates.
No backend, DB, API, `json-forms.ts` type, or item-renderer changes.

### Rationale
- The backend already persists/serves arbitrary `form_schema`; presets are purely an authoring
  convenience, so a frontend-only change is the smallest correct surface.
- Reusing the existing `FormSchema` type means every preset is type-checked at build time — a
  malformed preset fails `npm run check` rather than reaching a user.
- Reusing `handleFormSchemaInput()` + the existing live preview means "preview each preset" needs no
  new rendering engine, and applied presets remain fully hand-editable (a starting point, not a lock).

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Dropdown `Select` that only injects JSON into the textarea | Weakest "preview each preset" experience — the user must read raw JSON to compare presets. The card gallery shows each preset's fields at a glance. |
| Store presets in the DB / serve via an API | No runtime-editability requirement; presets are a fixed, code-reviewed set. A static TS catalog is simpler, versioned with the code, and type-checked. |
| Adopt a real JSONForms runtime (`@jsonforms/*`) for preview/render | Large new dependency; the app deliberately uses a hand-rolled renderer today. Out of scope for adding presets. |

### Consequences
- Adding or changing a preset is a code change (new entry in `listing-schema-presets.ts`) — intended,
  since presets are curated defaults.
- Presets must stay within the item renderer's supported field types (enum → Select, boolean →
  Checkbox, number/integer → Input, `format: date|date-time|email|uri`) and include a matching
  `uischema`; otherwise the produced item form would render fields the renderer can't handle.
- No migration or data backfill; existing topics are unaffected. Editing a topic that already has a
  schema still loads that schema, and presets never overwrite it unless explicitly clicked.
