# Merchant settings capture provenance

These are actual Chromium screenshots from task-owned Next/Go/PostgreSQL with a
signed MOCK IdP and synthetic merchant data; they are not production screenshots.
Credential inputs were empty. No generated artwork is shipped in the UI.

- `desktop.png`, `mobile.png`, `zh-CN.png`, `zh-TW.png`: source
  `output/playwright/settings-real-20260924T155335.960874000/results/`.
  Desktop is full-page 1586×1018; mobile is full-page 390×1669. Locale captures
  show the actual dirty-method conflict flow, not a successful provider connection.
- `hero-repro.png`: source
  `output/playwright/settings-real-20260924T155737.743633000/results/`, genuine
  1586×992 viewport, `fullPage:false`; no crop or image editing.
- Subdirectory for all source captures:
  `settings-real-REAL-PG-A-wi-f3300--preserves-safe-uncertainty/`.
- Approved comparison: `../../mocks/merchant-settings-a.png`.
- Visual verdict `333388a4-8f34-4859-96aa-b55c7bea2968`: only the three original
  presentation fixes were scored resolved; functional/provider acceptance is separate.
