# PARITY-ENTRY-FIX2 GEMINI REPORT

| Failing Job / Test | Class | What Changed | File:Line |
|---|---|---|---|
| 124446852658 (`gate (bundle:06)` / `--browser-inbox`) / `TestBrowserInboxUIRealChain` (`inbox-ui.spec.ts:110:1 (INU01)`, `inbox-ui.spec.ts:590:3 (INU06 en)`, `inbox-v2-ui.spec.ts:733:3 (PARITY inbox zh-CN)`, `inbox-v2-ui.spec.ts:763:5 (PARITY inbox desktop closed A)`) | (b) | Fixed shared setup helper `enterMessages` to check responsive viewport width before attempting to click the mobile drawer button `"Open navigation"`, preventing desktop runs from timing out on `display: none` menu button while maintaining mock and mobile drawer behavior | `tests/admin/inbox-browser-support.ts:76-78` |
| 124446852816 (`gate (bundle:08)` / `--browser-password-auth`) / `TestBrowserPasswordAuth` (`tests/admin/parity-entry.spec.ts:69-160`, `tests/admin/password-auth.spec.ts:179:1`) | (b) | Reset `.entry-parity .entry-form label > span::before { content: none; }` to eliminate inherited `*` pseudo-element from `globals.css:1331`, matching signed Figma frame `20:443` literal label text and fixing `getByRole` accessible name matching that previously hit 840s context deadline | `apps/admin/components/auth/entry.css:17` |
| 124446852658 / 124446852816 (`receipts`, `required`, `aggregate`) | (b) | Derived failures from `--browser-inbox` and `--browser-password-auth`; regenerated browser ownership matrix via `scripts/dev/derive-browser-ownership.mjs` | `output/ci-select-backend-browser/browser-ownership.json:1-1200` |

STATUS: DONE
