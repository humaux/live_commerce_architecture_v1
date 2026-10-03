# Unit platform-site: public platform website on xgdwm.com for Meta business verification and App Review

Owner (2026-10-04): the SaaS is operated by **Hong Kong Da Wan Trading Limited**, the company used for Meta business verification. The public site must show this company, or Meta's review crawler may fail verification.

Facts were taken on 2026-10-04 from the owner's registration documents (Certificate of Incorporation and Business Registration Certificate). They are public registry data. Nothing from the NNC1 form (directors' personal details) is used.

| Field | Value |
|---|---|
| Legal name (EN) | Hong Kong Da Wan Trading Limited |
| Legal name (ZH) | 香港大碗貿易有限公司 |
| Companies Registry No. (CR No.) | 81215167 (incorporated 11 Sep 2026 under the Companies Ordinance, Cap. 622) |
| Business Registration Certificate No. | 81215167-000-09-26-2 (valid 11/09/2026 – 10/09/2027) |
| Registered business address (as printed; not translated) | RM 10, 23/F, New Trend Centre, 704 Prince Edward Road East, San Po Kong, Hong Kong |

## Current state (verified 2026-10-04)

`https://xgdwm.com` and `https://www.xgdwm.com` do not answer (no DNS record, no edge block). Only `admin.xgdwm.com` responds.

The storefront's `/privacy` and `/data-deletion` are each merchant store's buyer privacy centre. They are not the platform operator's policy.

No Meta data-deletion callback (signed_request) exists. An instructions URL satisfies Meta's requirement.

## Decisions

1. **Host.** A new edge host `{$LC_PLATFORM_HOST}` (`xgdwm.com`) is served by the **admin Next app**, through a new public route group that needs no session. `www.xgdwm.com` redirects 301 to the apex. The admin host keeps its current behaviour.
   - Caddy gets one block plus the www redirect.
   - compose.env gets `LC_PLATFORM_HOST`.
   - Preflight checks that the host resolves to the edge.
2. **Company information has a single source.** `apps/admin/lib/company.ts` holds the facts above plus a public contact email, supplied by the owner and read from env `LC_COMPANY_CONTACT_EMAIL` (no default value that looks real). Every page footer and every legal page reads from it. The admin login/signup pages show the same footer line.
3. **Pages.** Locales: zh-TW (default), zh-CN, en. Meta reviewers read en. All pages are indexable, with no tracking scripts.
   - `/` — what the service does (live-selling SaaS: Facebook live comment ordering, storefront, payments/logistics, Meta ads), sign-in and sign-up buttons pointing to the admin host, and the company footer.
   - `/privacy` — the platform privacy policy. The operator is the company above. It covers:
     - the categories of data collected: merchant account data; data accessed through Meta permissions (Pages, comments, live videos, ad accounts, insights), used only to provide the merchant's requested features and never sold;
     - buyers' personal data, which the company processes on behalf of merchants, as processor;
     - retention, security, international transfer, merchant and buyer rights;
     - how to disconnect Meta;
     - contact details.
     The wording must match what the code actually does, per the contracts. Anything unverified stays out.
   - `/terms` — terms of service, with the operator named.
   - `/data-deletion` — Meta "Data Deletion Instructions": how to disconnect inside the app, how to e-mail a deletion request, what is deleted, and the timeline.
   - `/contact` — the company block plus the contact email.
4. **Meta domain verification.**
   - Preferred: a DNS TXT record, added by the owner in Cloudflare. This needs no code.
   - Fallback: optional env `LC_META_DOMAIN_VERIFICATION` renders `<meta name="facebook-domain-verification" content=…>` on the apex home page only. When the env is unset, the tag is not rendered.
5. **Legal text.** The agent drafts it and marks it "需 owner/律師審閱" in the delivery summary, not on the page. It contains no invented facts and no promises the system does not keep.
6. **Out of scope.** A marketing CMS, a blog, pricing pages, analytics/Pixel.

## Gates

- **PS1, node test.** Every footer and every legal page renders the company facts exactly from `company.ts`, in 3 locales. The contact email comes from env, and the build fails when the env is missing in production mode.
- **PS2, browser, real clicks** at 390 and 1586 in 3 locales. All links work: home → privacy/terms/data-deletion/contact → sign in (admin host). There is no horizontal scroll, the pages are crawlable (no `noindex`), and a robots/sitemap entry exists.
- **PS3, Caddy.** `caddy adapt` / `validate` passes with the new block and the www → apex 301. Smoke tests the host.
- **PS4.** Without `LC_META_DOMAIN_VERIFICATION` no meta tag is rendered. With it, exactly one tag appears, and only on the apex home page.
- **PS5.** check-gates, test-node, admin tsc, click sweep including the new public pages.

## Owner actions (runbook §4.2 addendum)

1. Cloudflare: A records for `xgdwm.com` and `www` pointing to the pilot edge IP, DNS-only.
2. compose.env: `LC_PLATFORM_HOST=xgdwm.com` and `LC_COMPANY_CONTACT_EMAIL=<public mailbox>`.
3. Meta Business Settings: add the domain, then verify it with the DNS TXT record (preferred) or the meta tag.
4. Meta app 大梦 → Basic settings:
   - Privacy Policy URL `https://xgdwm.com/privacy`
   - Terms URL `https://xgdwm.com/terms`
   - User data deletion `https://xgdwm.com/data-deletion`
   - App domain `xgdwm.com`
   - Website `https://xgdwm.com`
5. Business verification: use the CI and BRC documents. The legal name and address on the site must match them exactly.

## Owner ruling (2026-10-04): product name "DaWan Live"
- **The product name is `DaWan Live`.** It is a constant in `apps/admin/lib/company.ts` (`productName`), next to the legal entity. It is never translated, and it reads the same in all three locales.
- **Where it appears:**
  - the platform-site header, `<title>` and copy;
  - the admin login, signup and reset pages: brand text above the form, and `<title>` as "DaWan Live · <page>";
  - the W0 shell rail header;
  - the line "DaWan Live is operated by Hong Kong Da Wan Trading Limited" in every platform footer and in the platform legal pages.
- **Where it does not appear:** merchant storefronts keep the merchant's own brand. There is no "Powered by" badge for now (YAGNI; ask the owner first).
- **Domain:** the owner will buy `dawanlive.com` later. All hosts stay env-driven (`LC_PLATFORM_HOST`, `LC_ADMIN_HOST`, `LC_STORE_BASE_DOMAIN`), so a move is an env + DNS + Meta-settings change, with no code. Hard-coding `xgdwm.com` in UI or legal copy is forbidden: render hosts from config.
- **Gate:** PS1 additionally asserts the product name and the "operated by" line come from `company.ts`. A grep gate checks that no UI or legal copy contains a literal `xgdwm.com`.


Owner 2026-10-04: public contact email `ailun@xgdwm.com` (Spacemail mailbox) -> `LC_COMPANY_CONTACT_EMAIL` on the pilot.
