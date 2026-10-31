# GEMINI-REPORT — PARITY-CHECKOUT-FIX3 (PR #351)

| Failing job / test | Class (a/b/c) | What changed (`file:line`) |
|---|---|---|
| `124237319015` (`gate (bundle:06)` / `--browser-home-cod`) → `TestBrowserHomeCod` (`COD-255 gap-up: server due refresh required` at `tests/storefront/home-cod-buyer.mjs:255`) | **b** | `apps/storefront/components/OrderFlow.tsx:231-240`: Always invoke `refreshCodQuote(context, currentQuote, found)` when COD is offered and `cod_due_minor` mismatches the current option surcharge (even when initial `selected` mode defaults to `"card"`), so `gap-up` / `gap-down` / `gap-zero` / `refresh-failed` perform the authoritative `GET /api/buyer/quotes/{id}` read; fall back to `currentQuote` on card/online review only when `GET quotes/{id}` returns 200 with a still-stale optional COD due (preserving pinned card re-quote), while rethrowing on `optionRefresh`, `selected === "cash_on_delivery"`, or any HTTP transport failure (`reason.status !== undefined`). Regenerated `output/ci-select-backend-browser/browser-ownership.json:1-1470` and recorded analysis in `output/parity-checkout/FIX3.md:1-25`. |
| `124237319091` (`click-sweep@5/10`), `124237319763` (`click-sweep@7/10`), `124237319491` (`visual-lint@3/4`), `124237318093` (`--browser-webkit`), `124250898837` (`receipts`), `124250898768` (`aggregate`), `124252653668` (`required`) | Consequential / infra (`NOT_RUN` artifact download failure) | No code change; documented in `output/parity-checkout/FIX3.md:13-17`. |

STATUS: DONE
