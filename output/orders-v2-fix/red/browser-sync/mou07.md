# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: orders-ui.spec.ts >> MOU07 v2 private search, SQL queues, filters and three-language ledger
- Location: tests/admin/orders-ui.spec.ts:685:1

# Error details

```
Error: expect(locator).toHaveText(expected) failed

Locator:  getByTestId('orders-total')
Expected: "Matching orders: 1"
Received: "Matching orders: 20"
Timeout:  10000ms

Call log:
  - Expect "toHaveText" getByTestId('orders-total') with timeout 10000ms
  - waiting for getByTestId('orders-total')
    22 × locator resolved to <p class="orders-v2-total" data-testid="orders-total">Matching orders: 20</p>
       - unexpected value "Matching orders: 20"

```

```yaml
- paragraph: "Matching orders: 20"
```

# Test source

```ts
  606 |           await route.fulfill({
  607 |             status: 503,
  608 |             contentType: "application/json",
  609 |             headers: { "cache-control": "private, no-store" },
  610 |             body: '{"code":"retry_later","details":{}}',
  611 |           });
  612 |         else await route.fulfill({ response: real });
  613 |       } catch {
  614 |         /* A canceled old request may already be gone. */
  615 |       }
  616 |     };
  617 |     await page.route(path, handler);
  618 |     return { intercepted, release, remove: () => page.unroute(path, handler) };
  619 |   }
  620 |   const oldStore = await delayed(`**/api/stores/${store}/orders?*`, false);
  621 |   // Shell switching performs full navigation, canceling the old request. The
  622 |   // filter/locale cases below still exercise late-response generation fences.
  623 |   await page.getByTestId("state-filter").selectOption("DRAFT");
  624 |   await oldStore.intercepted;
  625 |   await switchOrderStore(page, foreignStore);
  626 |   await page.getByTestId("state-filter").selectOption("DRAFT");
  627 |   await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  628 |   oldStore.release();
  629 |   await oldStore.remove();
  630 |   await expect(page.getByTestId(`order-row-${ids.draft0}`)).toHaveCount(0);
  631 |   const oldFilter = await delayed(
  632 |     `**/api/stores/${foreignStore}/orders?*`,
  633 |     true,
  634 |   );
  635 |   await page.getByTestId("state-filter").selectOption("CONFIRMED");
  636 |   await oldFilter.intercepted;
  637 |   await page.getByTestId("state-filter").selectOption("CANCELLED");
  638 |   await expect(page.getByTestId("merchant-orders")).toBeVisible();
  639 |   oldFilter.release();
  640 |   await oldFilter.remove();
  641 |   await expect(page).toHaveURL(/state=CANCELLED/);
  642 |   const oldLocale = await delayed(
  643 |     `**/api/stores/${foreignStore}/orders?*`,
  644 |     false,
  645 |   );
  646 |   await page.getByTestId("state-filter").selectOption("all");
  647 |   await oldLocale.intercepted;
  648 |   await page.getByTestId("locale-switch").selectOption("zh-CN");
  649 |   await expect(page).toHaveURL(/\/zh-CN\/orders/);
  650 |   await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  651 |   oldLocale.release();
  652 |   await oldLocale.remove();
  653 |   await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  654 |   const oldSession = await delayed(
  655 |     `**/api/stores/${foreignStore}/orders/${foreignOrder}`,
  656 |     false,
  657 |   );
  658 |   await page.getByTestId(`order-expand-${foreignOrder}`).click();
  659 |   await oldSession.intercepted;
  660 |   await session(context, noOrdersToken);
  661 |   // G-UI8 audit [EXTERNAL-MOCK]: injects a window focus event after the session cookie changed: the default headless page cannot receive OS focus; native visibility/focus returns are covered by MOU03
  662 |   await page.evaluate(() => window.dispatchEvent(new Event("focus")));
  663 |   oldSession.release();
  664 |   await oldSession.remove();
  665 |   await noPII(page);
  666 | });
  667 | 
  668 | async function paymentBadgesFit(page: Page) {
  669 |   // Global fixed-table column rules must never hide "authorized, not captured".
  670 |   const badges = await page
  671 |     .locator("[data-testid=order-payment-cell] .orders-badge")
  672 |     .evaluateAll((elements) => elements.map((element) => {
  673 |       const cell = element.closest("td")!.getBoundingClientRect();
  674 |       const badge = element.getBoundingClientRect();
  675 |       return {
  676 |         withinCell: badge.left >= cell.left - 1 && badge.right <= cell.right + 1,
  677 |         textFits: element.scrollWidth <= element.clientWidth + 1,
  678 |         label: element.textContent,
  679 |       };
  680 |     }));
  681 |   expect(badges.length).toBeGreaterThan(0);
  682 |   expect(badges.filter((badge) => !badge.withinCell || !badge.textFits)).toEqual([]);
  683 | }
  684 | 
  685 | test("MOU07 v2 private search, SQL queues, filters and three-language ledger", async ({ page }) => {
  686 |   test.setTimeout(120_000);
  687 |   await mkdir(v2Evidence, { recursive: true });
  688 |   await signedLogin(page);
  689 |   const clicks: Array<{control:string; result:string}> = [];
  690 |   await switchOrderStore(page, foreignStore);
  691 |   await page.getByTestId("state-filter").selectOption("all");
  692 |   await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  693 |   await expect(page.getByTestId(`order-row-${ids.pending}`)).toHaveCount(0);
  694 |   await page.reload();
  695 |   await expect(page.getByTestId(`order-row-${foreignOrder}`)).toBeVisible();
  696 |   await expect(page.getByTestId(`order-row-${ids.pending}`)).toHaveCount(0);
  697 |   await switchOrderStore(page, store);
  698 |   await expect(page.getByTestId(`order-row-${ids.pending}`)).toBeVisible();
  699 |   await expect(page.getByTestId(`order-row-${foreignOrder}`)).toHaveCount(0);
  700 |   await page.getByTestId("state-filter").selectOption("all");
  701 |   clicks.push({control:"shell store A → B → reload B → A", result:"real shell selection + Orders navigation; selected store persisted; other store rows absent in both directions"});
  702 |   async function apply(control: string, action: () => Promise<unknown>) {
  703 |     const response = page.waitForResponse(r => r.url().includes(`/api/stores/${store}/orders`) && r.url().includes("view=v2")).then(async r => { expect(r.status(), control).toBe(200); expect(r.headers()["cache-control"]).toBe("private, no-store"); return r.json(); });
  704 |     await action();
  705 |     const data = await response;
> 706 |     await expect(page.getByTestId("orders-total")).toHaveText(`Matching orders: ${data.total}`);
      |                                                    ^ Error: expect(locator).toHaveText(expected) failed
  707 |     for (const [bucket, count] of Object.entries(data.counts)) await expect(page.getByTestId(`orders-count-${bucket}`)).toHaveText(String(count));
  708 |     clicks.push({ control, result: `visible SQL total ${data.total}; counts match response` });
  709 |     return data;
  710 |   }
  711 |   await apply("explicit cancelled state", () => page.getByTestId("state-filter").selectOption("CANCELLED"));
  712 |   await apply("cancelled queue retains state", () => page.getByTestId("orders-bucket-cancelled").click());
  713 |   await expect(page.getByTestId("state-filter")).toHaveValue("CANCELLED");
  714 |   await expect(page).toHaveURL(/state=CANCELLED/);
  715 |   await page.reload();
  716 |   await expect(page.getByTestId("state-filter")).toHaveValue("CANCELLED");
  717 |   await expect(page.getByTestId("orders-bucket-cancelled")).toHaveAttribute("aria-pressed", "true");
  718 |   await apply("all queue retains state", () => page.getByTestId("orders-bucket-all").click());
  719 |   await expect(page.getByTestId("state-filter")).toHaveValue("CANCELLED");
  720 |   await apply("default hide drafts", () => page.getByTestId("state-filter").selectOption("active"));
  721 |   await expect(page.getByTestId(`order-row-${ids.draft0}`)).toHaveCount(0);
  722 |   for (const bucket of ["unpaid","transfer_review","ready_to_ship","ready_to_consign","shipped","completed","cancelled","all"]) {
  723 |     await apply(`queue ${bucket}`, () => page.getByTestId(`orders-bucket-${bucket}`).click());
  724 |     await expect(page.getByTestId(`orders-bucket-${bucket}`)).toHaveAttribute("aria-pressed","true");
  725 |   }
  726 |   for (const [label, query, target] of [["order",ids.shipped,ids.shipped],["tracking","SYNTHETIC-MOU-TRACK",ids.shipped],["phone last4","0001",ids.shipped],["recipient","Synthetic Buyer",ids.shipped],["frozen SKU",frozenSKU,ids.shipped]] as const) {
  727 |     await page.getByTestId("orders-search").fill(query);
  728 |     const data = await apply(`search ${label}`, () => page.getByTestId("orders-apply").click());
  729 |     expect(data.total).toBeGreaterThan(0);
  730 |     await expect(page.getByTestId(`order-row-${target}`)).toBeVisible();
  731 |     expect(new URL(page.url()).searchParams.has("q")).toBe(false);
  732 |     await noPII(page);
  733 |   }
  734 |   await page.reload();
  735 |   await expect(page.getByTestId("orders-search")).toHaveValue(""); // deliberate privacy rule, not storage
  736 |   clicks.push({control:"refresh private search",result:"query cleared, not persisted in URL or storage"});
  737 |   for (const payment of ["cash_on_delivery","bank_transfer","pay_at_pickup","card"]) {
  738 |     await page.getByTestId("orders-payment-filter").selectOption(payment);
  739 |     await apply(`payment ${payment}`, () => page.getByTestId("orders-apply").click());
  740 |     await expect(page).toHaveURL(new RegExp(`payment_mode=${payment}`));
  741 |   }
  742 |   await page.getByTestId("orders-delivery-filter").selectOption("home");
  743 |   await apply("delivery home", () => page.getByTestId("orders-apply").click());
  744 |   await page.reload();
  745 |   await expect(page.getByTestId("orders-payment-filter")).toHaveValue("card");
  746 |   await expect(page.getByTestId("orders-delivery-filter")).toHaveValue("home");
  747 |   await apply("reset filters", () => page.getByTestId("orders-reset").click());
  748 |   await apply("inspect drafts for recorded live claim", () => page.getByTestId("state-filter").selectOption("all"));
  749 |   await page.getByTestId("orders-session-filter").selectOption(ids.live_session);
  750 |   const live = await apply("live session", () => page.getByTestId("orders-apply").click());
  751 |   expect(live.total).toBe(1);
  752 |   await expect(page.getByTestId(`order-row-${ids.live_order}`)).toContainText("Live claim");
  753 |   await page.reload();
  754 |   await expect(page.getByTestId("orders-session-filter")).toHaveValue(ids.live_session);
  755 |   await apply("reset live filter", () => page.getByTestId("orders-reset").click());
  756 |   const today = new Intl.DateTimeFormat("en-CA",{timeZone:"Asia/Taipei",year:"numeric",month:"2-digit",day:"2-digit"}).format(new Date());
  757 |   await page.getByTestId("orders-from").fill(today);
  758 |   await page.getByTestId("orders-to").fill(today);
  759 |   const dated = await apply("Taipei day", () => page.getByTestId("orders-apply").click());
  760 |   expect(dated.total).toBeGreaterThan(0);
  761 |   await page.reload();
  762 |   await expect(page.getByTestId("orders-from")).toHaveValue(today);
  763 |   await expect(page.getByTestId("orders-to")).toHaveValue(today);
  764 |   await apply("reset before visual acceptance", () => page.getByTestId("orders-reset").click());
  765 |   await apply("active ledger", () => page.getByTestId("state-filter").selectOption("active"));
  766 |   for (const locale of ["zh-TW","zh-CN","en"]) {
  767 |     await page.getByTestId("locale-switch").selectOption(locale);
  768 |     await expect(page.getByTestId("orders-table")).toBeVisible();
  769 |     for (const [width,height] of [[1586,992],[1366,768],[390,844]]) {
  770 |       await page.setViewportSize({width,height});
  771 |       if (width === 390) {
  772 |         // Real responsive transition must finish; never screenshot a half-open rail.
  773 |         await expect.poll(() => page.locator("aside[data-shell-rail]").evaluate(node => node.getBoundingClientRect().right)).toBeLessThanOrEqual(0);
  774 |         const more = page.getByTestId("orders-more-filters");
  775 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  776 |         await expect(page.getByTestId("orders-payment-filter")).toBeHidden();
  777 |         await expect(page.getByTestId("state-filter")).toBeHidden();
  778 |         await more.click();
  779 |         await expect(more).toHaveAttribute("aria-expanded", "true");
  780 |         await page.getByTestId("orders-payment-filter").selectOption("card");
  781 |         await page.getByTestId("orders-apply").click();
  782 |         await expect(page).toHaveURL(/payment_mode=card/);
  783 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  784 |         await more.click();
  785 |         await expect(page.getByTestId("orders-payment-filter")).toHaveValue("card");
  786 |         await expect(page.getByTestId("orders-from")).toBeVisible();
  787 |         await page.getByTestId("orders-reset").click();
  788 |         await expect(page).not.toHaveURL(/payment_mode=/);
  789 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  790 |         await more.click();
  791 |         const refreshRead = page.waitForResponse(r => r.url().includes(`/api/stores/${store}/orders?`) && r.status() === 200);
  792 |         await page.getByTestId("orders-refresh").click();
  793 |         await refreshRead;
  794 |         await expect(page.getByTestId("orders-table")).toBeVisible();
  795 |         await more.click();
  796 |         await expect(more).toHaveAttribute("aria-expanded", "false");
  797 |         clicks.push({control:`mobile refresh ${locale}`,result:"open secondary controls, real refresh/readback, collapse"});
  798 |       }
  799 |       if (width === 390) {
  800 |         const rail = page.getByTestId("orders-tabs");
  801 |         // Native click/keyboard drive scrolling; evaluate below only measures geometry.
  802 |         await rail.hover();
  803 |         await page.mouse.wheel(1200, 0);
  804 |         await expect.poll(() => rail.evaluate(node => node.scrollLeft)).toBeGreaterThan(0);
  805 |         await page.getByTestId("orders-bucket-cancelled").click();
  806 |         await expect(page.getByTestId("orders-bucket-cancelled")).toHaveAttribute("aria-pressed", "true");
```