# Unit ui-w0-shell — new admin shell, route registry and format contract (R5, UI architecture v2 W0)

Owner 2026-10-02 approved the R5 comps ("还行"): `output/r5-research/comps-webgpt/` (01 = shell + overview; README lists the 4 places where the comps contradict the rules — follow the rules, not the picture).
Architecture: docs/engineering/ui-architecture.md §5, §6, §10.1, §10.2, §10.5, §10.6, §10.7 (W0 row).
Starts only after unit stop-bleed is merged (both touch WorkspaceFrame.tsx, globals.css and the money helpers).

Read: the sections above, output/r5-research/UI-INVENTORY.md §1.1, §2, §4.1, §5.1, §6, apps/admin/components/WorkspaceFrame.tsx, apps/admin/app/[locale]/layout.tsx, apps/admin/lib/team-model.ts (navNeeds/navVisible), the stop-bleed money and time helpers, and packages/i18n.

## Decisions
1. **Shell** (comp 01). It replaces WorkspaceFrame for every signed-in admin page.
   - Dark rail 220px with the 10 groups of §10.2 in that order; 设置 is pinned to the bottom. A group shows only when it has at least one route the user may open. 消息中心 stays hidden until W3 adds its first page: no empty or placeholder entries.
   - Groups expand to their second-level entries; the active route is highlighted.
   - Top bar: store switcher, language, help link, account menu with 退出登录.
   - **No control without behaviour.** No global search, task centre or notification bell until a backend serves them. This is the lesson of the hard-coded channel status (D04).
   - Below 1024px the rail becomes a drawer opened from the top bar. At 375px there is no horizontal page scroll, and 设置 and 退出登录 stay reachable at 1366×768.
2. **Route registry = single source** (§5.1, revised by §10.5).
   - Each domain declares its routes in `apps/admin/src/features/<domain>/routes.ts`; `apps/admin/src/routes.ts` aggregates them.
   - Each entry is `{id, path, group, labelKey, icon, permission | public, template, nav: boolean, spec}`. `spec` is the browser spec that covers the route.
   - The rail, breadcrumbs, document titles, the 403 page and check-gates all read the registry.
   - The permission is UX only; Go still checks every read and write.
   - Mapping of today's routes:
     - 总览 `/`
     - 直播与贴文 `/studio`, `/studio/claims`
     - 订单与发货 `/orders`; `/orders/new` and `/orders/cvs-print` with nav=false
     - 商品与库存 `/products`, `/collections`, `/inventory`; `/products/import` with nav=false
     - 顾客 `/customers`; `/customers/{id}` with nav=false
     - 营销优惠 `/promotions`, `/ads`
     - 网店 `/design`
     - 收款与报表 `/finance`
     - 设置 `/settings`, `/team`, `/billing`
     - AuthLayout: `/invite/{token}`, `/reset`, `/signup` (public)
3. **Legacy pages move into the shell unchanged.** W0 does not redesign any page body. Domains are replaced in W1 and later.
   - Before switching, record a screenshot baseline of every legacy page at 1586×992 and 390×844 under `tests/admin/baselines/w0/`. It is reviewed by a human and committed.
4. **packages/ui** (tokens + only what the shell uses):
   - `tokens.css` holds the comp design system as CSS custom properties: brand #FF6A00, page #F5F6F8, surface #FFFFFF, border #E5E7EB, text #1F2937, rail #1F2937, radius 8px, success/warning/danger/info, the spacing scale, and 44px touch targets.
   - Shell components use CSS Modules.
   - Remove the old shell rules from globals.css. Page rules stay until their domain migrates.
   - Further components (DataTable, MoneyInput, …) arrive with W1 and are not built here.
5. **packages/format**: one implementation of money and time.
   - Money: parse in major units, TWD whole dollars, display NT$ without decimals.
   - Time: store-local time with a label (Asia/Taipei → 台北时间 / 台北時間).
   - Move the stop-bleed helpers here and re-export them; do not copy them.
   - Status wording stays in each domain's i18n (§10.5).
6. **Copy:** shell strings live in a typed copy module (`typeof en`, three locales). A node test fails on missing or extra keys.

## Gate (red first, then green)
- **G-UI1** (node test, run by check-gates):
  - every `app/[locale]/**/page.tsx` has a registry entry and every entry has a page;
  - every entry has a labelKey in three locales, a permission or `public`, and an existing spec file;
  - no duplicate ids or paths;
  - the rail is generated from the registry, with no literal nav array left.
- **G-UI2** (browser, new mode `--browser-admin-shell`):
  - shell geometry at 1366×768, 1586×992, 2000×1100, 1024×768, 768×1024, 390×844 and 360×740;
  - no horizontal scroll;
  - no overlap of the rail, top bar or main area (elementFromPoint);
  - every nav entry is reachable and clickable for real;
  - the drawer opens and closes below 1024px and keyboard focus returns;
  - 设置 and 退出登录 are reachable;
  - zh-CN, zh-TW and en, with a long English store name.
- **Roles:**
  - a staff role without `catalog:read` does not see 商品与库存, and the direct URL shows the 403 page, not data;
  - the owner sees everything;
  - switching store does not show the previous store's data (cross-store negative).
- **G-UI3** (lint, check-gates):
  - no `Intl.NumberFormat` / `toLocaleString` / `toLocaleDateString` outside packages/format;
  - no 最小货币单位 / 最小貨幣單位 / "minor unit" in user-visible copy;
  - existing offenders go on an explicit allowlist that may only shrink.
- **G-UI4:** axe on the shell finds no serious or critical issues; focus order runs rail → top bar → main; touch targets are ≥44px under 1024px.
- **G-UI5:** new files are ≤500 lines (warn) and ≤800 lines (fail; existing over-limit files on a shrinking allowlist); fetch appears only in api.ts or client modules of the BFF layer; no cross-feature deep imports and no cycles, checked with the TypeScript compiler API or an existing tool. A new dependency needs a line in docs/engineering/dependencies.md.
- **G-UI7:** every existing admin browser mode stays green: at least `--browser-admin-legacy`, `--browser-catalog-core`, `--browser-catalog-media`, `--browser-promotions`, `--browser-ops-polish`, `--browser-customers-billing`, `--browser-studio-ui`, `--browser-merchant-orders-ui`, `--browser-cvs`, `--browser-meta-connect` and `--browser-design`. Locators that pointed at the old nav are updated without weakening any assertion.
- **Visual QA:** a different agent (K3) compares the shell screenshots with comp 01. P1 count must be 0.

## Executor
K2.8 (ui_worker) in its own worktree. K3 does the independent visual QA and review. Claude does the final review and the merge.
