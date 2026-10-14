# COMPANY-SITE delivery

Role ui_worker · base r3/integration 62554448 · branch unit/company-site · not pushed.

## Files
- `apps/admin/lib/company.ts` — `about` added to `platformPages`; `operatedBy` (export name kept) now "DaWan Live is a live-commerce platform of Hong Kong Da Wan Trading Limited"; `operatorSentence` zh-TW/zh-CN/en use the owner's ownership wording.
- `apps/admin/lib/platform-copy.ts` — `pages.about`, `about` paragraphs (3 locales; facts only, platform Stripe + contracting party per owner decision); label 營運公司/运营公司/Operator → 公司資訊/公司信息/Company.
- `apps/admin/lib/platform-legal.ts` — wording only: "operator/營運/运营" role words → "company / we"; no retention, rights or refund text changed.
- `apps/admin/components/platform/PlatformDocument.tsx` — new `PlatformAbout` (H1 = legal English + Chinese names, ownership sentence, paragraphs, single `CompanyFacts` block, links to terms/privacy/data-deletion/contact); footer uses `operatorSentence(locale)` (was English-only `operatedBy`) and links `/about`.
- `apps/admin/app/(public)/site/[locale]/[page]/page.tsx` — renders About, header nav link to About. Verification meta still home-only. Sitemap picks up `/about` automatically via `platformPages`.
- `apps/admin/app/(public)/site/[locale]/layout.tsx` — comment only.
- `OperatorFooter.tsx` unchanged: it already calls `operatorSentence`, so the in-app footer gets the new wording.

## Tests
- Updated `tests/admin/platform-site.test.ts`: ADM10 sentence (3 locales, literal owner strings), PS1 `operatedBy` text, PS2 routes include `about`, PS1 copy-completeness excludes `about` from the legal-document key check (it is not a legal doc; assertion otherwise unchanged).
- New: `/about` route/copy in 3 locales, sitemap source, nav + footer links, H1 = legal names, facts via CompanyFacts, verification home-only; guard that copy/legal/sentences contain no "operated by"/"operator"/營運/运营.
- Existing browser tests (`platform-brand-browser.mjs`, `shell-browser.mjs`) compare against `operatorSentence(locale)` so follow the new text; unchanged.

## Not run
- NOT_RUN: all tests, typecheck, build (CI per instruction). Screenshots / 點擊台账 for `/about` header, footer and body links: NOT_RUN (CI/browser).

## Risks
- Static source-regex assertions for nav/footer/H1 (node test cannot render .tsx); a browser click ledger for the new links is still owed.
- zh-CN sentence hard-codes the Simplified company name (`香港大碗贸易有限公司`), as specified; `company.legalChinese` is Traditional.
- Header gets an extra nav link reusing `ps-languages` styling; mobile layout unverified.
- Legal wording touched (role words only) — owner/legal should skim.
