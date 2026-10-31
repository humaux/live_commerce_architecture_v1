# RVFIX2-403

| finding | change (file:line) | RED run | GREEN run |
|---|---|---|---|
| P1 duplicate tw POST after failed/lost policy PUT | apps/admin/components/SettingsWizard.tsx:998-1013 (created row into markets + v0 baseline; retryPending :807-812 already inserts) | see red.log | pending |
| CI 41231545040 bundle:06 settings-real.spec.ts:270 | tests/admin/settings-real.spec.ts:263-266 (home2 -> home triggers the held read instead of reload) | 41231545040 | pending |
