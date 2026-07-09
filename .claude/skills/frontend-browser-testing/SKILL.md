---
name: frontend-browser-testing
description: Run real, headless-browser end-to-end tests against this repo's SvelteKit frontend using Playwright + Chromium, driving the actual built SPA served by the real Go backend against a scratch database. Use this whenever a frontend/UI change needs verification beyond `npm run check`/`npm run build` — clicking through real pages, testing auth-gated or role-gated flows, verifying Svelte 5 + shadcn-svelte/bits-ui component behavior at runtime, or reproducing a UI bug. Also documents known bits-ui gotchas (Tabs.Content never unmounting, Select not exposing role="combobox") and a reliable recipe for starting/stopping the scratch server in this sandboxed environment.
---

# Frontend headless-browser testing

## When to use this

Any time a frontend task needs proof it actually works at runtime, not just that it typechecks/builds:
new pages, new auth/RBAC-gated UI, shadcn-svelte/bits-ui component work, or reproducing a bug that only
shows up when clicking through the real app. `npm run check` and `npm run build` catch type errors, not
broken interactions.

## One-time environment check

Playwright is already a `web/package.json` devDependency and Chromium is normally cached at
`~/.cache/ms-playwright/`. Verify before assuming it's missing:

```bash
grep playwright "$(git rev-parse --show-toplevel)/web/package.json"
ls ~/.cache/ms-playwright/ 2>&1
```

If genuinely absent, ask the user before installing (downloads ~300MB):

```bash
cd web && npm install --save-dev playwright && npx playwright install chromium
```

## The reliable recipe

The Go server serves the **built** SPA from `./web/build` via a relative path (see
`cmd/all-in-one/server/server.go` `spaFileServer("./web/build")`) — it must be run with the repo root as
cwd, and the frontend must be rebuilt after any source change (`npm run dev`'s live-reload server is not
used here).

**Critical lesson learned the hard way:** in this sandboxed Bash tool, chaining multiple steps (kill old
process, reseed DB, start server, curl health check) in *one* tool call is unreliable — a benign non-zero
exit code from an early command (e.g. `pkill` finding nothing to kill) can silently prevent later commands
in the same call from running. **Issue every step below as its own separate Bash tool call**, and verify
real state (`ps aux`, `sqlite3` queries, `curl` status codes) — never trust a chained exit code alone.

Use the session's own scratchpad directory for the DB file, logs, and screenshots — never `/tmp` directly
and never the real `all-in-one.db`.

1. **Rebuild the frontend** (repeat after every source change):
   ```bash
   cd web && npm run build
   ```
2. **Kill any previous scratch server** (own tool call):
   ```bash
   ps aux | grep "all-in-one server" | grep -v grep
   kill -9 <pid> <pid>   # only if something was found
   ```
3. **Reseed a fresh scratch DB** (own tool call — always start from a known state; a stale DB from a
   previous run causes confusing `UNIQUE constraint` 500s that look like app bugs but aren't):
   ```bash
   cd "$(git rev-parse --show-toplevel)"
   rm -f <scratchpad>/smoke.db
   ALLINONE_STORAGE_SQLITE_DB_PATH=<scratchpad>/smoke.db go run ./cmd/all-in-one db:seed
   ```
4. **Start the server in the background** (own tool call, plain `&` — `nohup ... & disown` was tried and
   made things *worse*, e.g. the log file didn't even get created):
   ```bash
   ALLINONE_STORAGE_SQLITE_DB_PATH=<scratchpad>/smoke.db ALLINONE_SERVER_PORT=18090 \
     go run ./cmd/all-in-one server > <scratchpad>/server.log 2>&1 &
   ```
5. **Verify health before touching Playwright** (own tool call):
   ```bash
   sleep 3
   curl -s -o /dev/null -w "%{http_code}\n" http://localhost:18090/api/v1/health
   ```
   Only proceed once this prints `200`. A `000` or connection-refused means the server didn't actually
   start — check `<scratchpad>/server.log`, don't just retry blindly.
6. **Run the Playwright script**, redirecting output to a file rather than piping directly (makes it easy
   to `grep`/re-read specific sections without re-running):
   ```bash
   cd web && node <scratchpad>/e2e.js > <scratchpad>/e2e_output.txt 2>&1
   grep -n "SUMMARY\|passed\|FAILED" <scratchpad>/e2e_output.txt
   ```
7. **Clean up when done**: kill the server PID, `rm -f` the scratch DB/logs, `rm -rf` the screenshots dir.
   Never leave a scratch server running or a stale scratch DB behind between sessions.

Seeded default users (from `internal/authnz/seed/seed.go`): `admin`/`admin123`, `user`/`user123`,
`demo`/`demo123`.

## Script skeleton (known-working selectors)

```js
const path = require('path');
const { execSync } = require('child_process');
// The script lives outside web/ (in the scratchpad), so plain require('playwright')
// won't resolve — Node resolves require() relative to the FILE's own location, not
// the process cwd, even if you `cd web` first. Resolve the repo root via git instead.
const repoRoot = execSync('git rev-parse --show-toplevel').toString().trim();
const { chromium } = require(path.join(repoRoot, 'web', 'node_modules', 'playwright'));

const BASE = 'http://localhost:18090';
const SHOT_DIR = '<scratchpad>/shots';

const results = [];
function check(name, cond, extra) {
  results.push({ name, pass: !!cond, extra: extra || '' });
  console.log(`${cond ? 'PASS' : 'FAIL'} - ${name}${extra ? ' :: ' + extra : ''}`);
}
async function shot(page, name) {
  const p = path.join(SHOT_DIR, `${name}.png`);
  await page.screenshot({ path: p, fullPage: true });
  console.log(`screenshot: ${p}`);
}
async function login(page, username, password) {
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.fill('#username', username);
  await page.fill('#password', password);
  await page.click('button[type="submit"]');
  await page.waitForURL(`${BASE}/`, { timeout: 10000 }).catch(() => {});
  await page.waitForTimeout(500);
}
async function logout(page) {
  await page.goto(`${BASE}/`, { waitUntil: 'networkidle' });
  const trigger = page.locator('button:has-text("admin"), button:has-text("user")').first();
  await trigger.click();
  await page.waitForTimeout(200);
  await page.click('text=Log out');
  await page.waitForURL(`${BASE}/login`, { timeout: 10000 });
}

(async () => {
  // --no-sandbox is required in this environment (no normal browser sandbox perms).
  const browser = await chromium.launch({ args: ['--no-sandbox'] });
  const page = await (await browser.newContext()).newPage();

  const networkFailures = [];
  page.on('response', (res) => {
    if (res.status() >= 400 && res.url().includes('/api/v1/')) {
      networkFailures.push(`${res.status()} ${res.request().method()} ${res.url()}`);
    }
  });

  await login(page, 'admin', 'admin123');
  // ... drive the page, call check(...) for each assertion ...

  console.log('\n=== SUMMARY ===');
  const passed = results.filter((r) => r.pass).length;
  console.log(`${passed}/${results.length} checks passed`);
  await browser.close();
  process.exit(results.some((r) => !r.pass) ? 1 : 0);
})();
```

To check a logged-in user's *backend*-enforced permissions directly (not just what the UI shows), call
`fetch` inside the page context after login — cookies are sent automatically:

```js
const res = await page.evaluate(async (url) => {
  const r = await fetch(url, { credentials: 'include' });
  return { status: r.status, body: await r.json().catch(() => null) };
}, `${BASE}/api/v1/chats`);
```

## bits-ui / shadcn-svelte gotchas (cost real debugging time — check these first)

- **`Tabs.Content` never unmounts inactive panels.** A tab component that fetches its own data in
  `onMount` will only ever fetch once per page load — switching tabs does NOT remount it. If data created
  in one tab doesn't appear after switching to another tab that should show it, this is almost certainly
  why. Fix pattern: thread an `active: boolean` prop from the tab host down to each panel and replace
  `onMount(load)` with `$effect(() => { if (active) load(); })`.
- **`Select.Trigger` does not render `role="combobox"`** (bits-ui 2.11 uses `aria-haspopup="listbox"`
  instead, per the current ARIA pattern). A locator like `button[role="combobox"]` will silently match
  zero elements for *every* shadcn-svelte Select in this codebase, not just a broken one. Use
  `[data-slot="select-trigger"]` instead. `Select.Item` does correctly render `role="option"`, so
  `[role="option"]:has-text("...")` is fine for picking an option after opening the dropdown.
- **Dialog scoping:** shadcn-svelte's `Dialog.Content` always carries `data-slot="dialog-content"` — use
  that as the scoping selector (`.last()` for "the most recently opened dialog") rather than guessing at
  `[role="dialog"]` or text-matching the title, which is more fragile against whitespace/em-dash content.
- If a real component bug is suspected, dump the actual DOM before assuming: e.g.
  `locator('button').evaluateAll(els => els.map(el => ({ role: el.getAttribute('role'), dataSlot: el.getAttribute('data-slot'), text: el.textContent })))`
  — this is what surfaced the `role="combobox"` gotcha above; the buttons were there and working, just
  not matching the guessed selector.
