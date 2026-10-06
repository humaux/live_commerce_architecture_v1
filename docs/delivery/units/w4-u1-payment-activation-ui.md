# Unit W4-U1: enable card payments (platform Stripe) and settlement statements (UI, Codex)

**Status.** DRAFT, re-scoped 2026-10-06 after the owner decided 「不使用PAYUNi，直接允许商家使用平台的stripe进行收款」.
It waits for integrator freeze. The earlier PAYUNi activation scope of this brief is **cancelled**.
- Base: `r3/integration` `2fa471a1`. No migration.
- Worktree `.worktrees/w4-u1-card-payments-ui`.

**Read first:** PREAMBLE → AGENTS.md → PROCESS.md → this file. Then read
`contracts/stripe-platform-account-v1.md` §2, §3.3 (`set/read_platform_stripe`), §5 and §6.4 (`read_store_settlements`),
and the frozen W4-S1 and W4-S2 briefs.

## Integrator 裁决 (overrides the body)

- There is **no credential input of any kind**. The merchant only accepts terms and toggles card payments. Never show
  an account id, key or approval id.
- The enable dialog quotes the terms summary from contract §5 verbatim and shows `descriptor_preview`. If the platform
  is not OPEN, the page says 「信用卡收款尚未開放」 and hides the toggle. In the BLOCKED state it says 「已被平台暫停，請聯絡客服」
  and allows disable only.
- The storefront disclosure line (contract §5, three languages) appears above the Stripe pay button and on the order
  payment page, whenever the hosted view returns `collector`.

## Scope

1. **`apps/admin/components/CardPayments.tsx`** (new):
   - state badge (NONE / ENABLED / DISABLED / BLOCKED, plus the platform state);
   - the enable dialog: terms checkbox, optional descriptor suffix with a live length check against the server's rule,
     and the resulting preview;
   - disable;
   - shows the currency and the min/max per order (NT$25 – NT$20,000);
   - 409 → reload and retry message.
2. **`SettingsWizard.tsx`:** the 收款 step links to the new page (entry only).
3. **`apps/admin/components/Settlements.tsx`** (new): statements list (period, captured, refunded, disputes, Stripe fee,
   platform fee, carried-in, net payable, paid / pending with the payout reference), a detail view of lines, and an empty
   state. Read-only, and requires `billing:manage` (otherwise the page is hidden).
4. **`apps/storefront`:** a `collector` disclosure copy key in `lib/payment-copy.ts` (three languages) and its render in
   the Stripe pay section.
5. **BFF:** `apps/admin/app/api/stores/**` (new only) mirrors `GET/PUT …/payments/card` and `GET …/settlements[/{id}]`.

## Write paths

- `apps/admin/components/{CardPayments.tsx,Settlements.tsx}` (new), `SettingsWizard.tsx` (entry);
- `apps/admin/lib/card-payments-*.ts`;
- `apps/admin/app/api/stores/**` (new only);
- `apps/admin/src/features/settings/routes.ts` (two lines);
- `apps/storefront/lib/payment-copy.ts` and the Stripe pay-section component (disclosure only);
- `tests/admin/card-payments.spec.ts`, `tests/admin/settlements.spec.ts`.

## Tests / gates

- **New mode** `bash scripts/dev/test-local.sh --browser-card-payments` (MOCK backend). It covers:
  - enable and disable;
  - every state badge;
  - suffix length refusal;
  - BLOCKED allows disable only;
  - platform CLOSED hides the toggle;
  - the settlements list and detail;
  - no `acct_`/`sk_`/`rk_` string in DOM, network or storage;
  - the storefront disclosure in three languages.
- **Regression:** `--stripe-browser`, `--browser-payment`, `--browser-click-sweep`, `--browser-visual-lint`.
  These are **CI gates**, run on GitHub.
- **Local:** `test-node.sh`, typecheck, build, `check-gates.sh`.

## Evidence / role / dependencies

BROWSER (MOCK). Role: Codex, with K3 and Claude review. Depends on W4-S1 (card page, disclosure) and W4-S2 (settlements
tab). Ship the card page first if S2 lags.
