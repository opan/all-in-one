# ADR: "Modernist Blue" app-wide theme

Records the decision to adopt the "Modernist Blue" design handoff (from cloud design) as the app's
default theme. Add a new entry here for future changes to the global design tokens.

---

## ADR-001: Adopt the handoff design system app-wide via base theme tokens

### Status
Accepted

### Context
The app used a neutral gray shadcn theme (`--radius: 0.625rem`, oklch grays, system font) that looked
rough and inconsistent. A cloud-design handoff ("Modernist Blue": Archivo font, flat/zero-radius, no
shadows, blue `#3b7bff` accent, full light/dark palettes) was produced. The app uses Tailwind v4 +
shadcn-svelte with theme tokens as CSS variables in `web/src/app.css` (`:root` / `.dark`), a
`--radius` token, and a `data-palette` override system wired through `web/src/lib/stores/theme.ts`
and pre-painted in `web/src/app.html`.

### Decision
Map the handoff tokens onto the **existing base CSS variables** rather than a scoped stylesheet:
- Rewrote `:root` (light) and `.dark` (dark) color tokens to the Modernist Blue palette
  (light ground `#f4f7ff` / text `#17203a`; dark ground `#0d1526` / text `#e8edf7`; accent `#3b7bff`).
- Set `--radius: 0` to flatten every shadcn component globally (no shadows).
- Added Archivo as `--font-sans` via a Tailwind v4 `@theme` block, loaded from Google Fonts in
  `app.html` (preconnect + `display=swap`).
- Kept the `data-palette` override system (ocean/slate/forest) intact; the base default is now blue
  and the "default" palette swatches were updated to match.

### Rationale
- The design must apply **app-wide**, not to one page. The base tokens are the single lever that
  restyles every component (buttons, inputs, cards, dialogs, sidebar) with no per-component churn,
  because shadcn's `--radius` and `@theme inline` color mappings already fan out everywhere.
- Reusing the token layer keeps dark mode (`mode-watcher`) and the palette picker working unchanged.

### Alternatives Considered
| Option | Rejected because |
|---|---|
| Scoped, page-local styles | Would leave the rest of the app on the old neutral theme; contradicts app-wide intent. |
| New `data-palette="blue"` default | Palettes only override `--primary`/`--ring`/`--chart`/`--sidebar-primary`, not grounds, borders, radius, or font — insufficient for the full handoff. |
| Self-host Archivo | More setup; Google Fonts with `display=swap` is adequate and matches the handoff note. Revisit for offline/self-hosted needs. |

### Consequences
- Every page now renders flat (radius 0) with the blue palette; verified on the topics list and
  topic modal. The existing landing page inherits the new look.
- `--radius-sm/md` derive to negative values via `calc(0 - Npx)`; browsers clamp negative radius to 0
  (cosmetically fine; can hard-set to `0` later).
- Non-default palettes now sit on blue grounds — acceptable, pre-existing behavior.
- Screenshots taken in a sandboxed browser fall back from Archivo when external Google Fonts can't
  load; the real app loads Archivo.
