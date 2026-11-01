// Purpose: Owns the merchant subscription status and hosted billing entry controls.
// Depends on: react, @live-commerce/i18n, @/lib/model, @/lib/client, @/lib/billing-client, @/lib/customers-client, @/lib/billing-model, @/lib/orders-model, @/lib/billing-copy, ./WorkspaceFrame, ./AdminPageHeader, @live-commerce/ui, @/lib/presentation-copy, ./orders.css, ./order-actions.css, ./customers.css, ./finance/money.css
// Used by: apps/admin/app/[locale]/billing/page.tsx
"use client";

// Merchant billing page (/{locale}/billing): standing, plan choices -> Stripe Checkout redirect, "Manage payment &
// invoices" -> Stripe portal redirect, subscription rows, usage this period (U4).
// BFF GET /api/stores/{store}/billing (billing:manage), POST .../billing/checkout {price_id}, POST .../billing/portal
// (keyless) -> Go /v1/admin/stores/{id}/billing[/checkout|/portal] (internal/httpapi/billing.go; internal/billing).
// The returned Stripe URL goes straight to location.assign: never kept in state, storage or logs (I11).
// Coming back with `?checkout=done` shows "updating…" and re-GETs once; the server (webhook) stays the authority on
// standing, so nothing here marks a subscription active.
import { useEffect, useRef, useState } from "react";
import { moneyCopy, type Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { readBilling, openPortal, startCheckout } from "@/lib/billing-client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { hasLiveSubscription, type BillingStatus } from "@/lib/billing-model";
import { displayTime } from "@/lib/orders-model";
import { billingCopy, type BillingCopy } from "@/lib/billing-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { Badge } from "@live-commerce/ui";
import "./orders.css";
import "./order-actions.css";
import "./customers.css";
import "./finance/money.css";

type Phase = "idle" | "updating" | "slow";
const errorText = (c: BillingCopy, code: string) => c.errors[code] ?? c.errors.default;

/** Owns the merchant subscription status and hosted billing entry controls. User actions request hosted checkout or portal navigation through billing-client. */
export function Billing({
  locale,
  stores,
  store,
  checkout,
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  checkout: "" | "done" | "cancel";
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = billingCopy[locale], p = moneyCopy[locale];
  const [portal, setPortal] = useState(false);
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}`,
    store ? (signal) => readBilling(store.id, signal) : null,
    initialError,
  );
  const [phase, setPhase] = useState<Phase>(checkout === "done" ? "updating" : "idle");
  const rechecked = useRef(false);
  const storeID = store?.id ?? "";

  const refreshNow = useRef(read.refresh);
  refreshNow.current = read.refresh;
  // checkout=done: the webhook may land a moment after the redirect. One delayed re-GET, then say so honestly.
  useEffect(() => {
    if (checkout !== "done" || read.status !== "ready" || rechecked.current) return;
    rechecked.current = true;
    const timer = setTimeout(async () => {
      const ok = await refreshNow.current();
      setPhase(ok ? "idle" : "slow");
      // Drop the return marker so a reload does not repeat it; path only, no bearer data is in this URL.
      window.history.replaceState(window.history.state, "", `/${locale}/billing${storeID ? `?store=${storeID}` : ""}`);
    }, 2500);
    return () => clearTimeout(timer);
  }, [checkout, read.status, locale, storeID]);

  const failure =
    read.status === "signed-out" ? c.signedOut
    : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? (store ? c.notFound : c.noStore)
    : read.status === "unavailable" ? c.unavailable
    : "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="billing">
      <div className="money-page" data-testid="billing-page">
        <h1>{portal ? p.manage : failure ? p.billingRefusal : p.billing}</h1>
        {!portal && !failure && <p className="money-muted">{p.subscriptionIntro}</p>}
        {checkout === "cancel" && <p className="orders-message" role="status" data-testid="billing-cancelled">{p.cancelled}</p>}
        {phase === "updating" && <p className="orders-message" role="status" data-testid="billing-updating">{p.updating}</p>}
        {phase === "slow" && <p className="orders-message" role="status">{c.checkoutDoneSlow}</p>}
        {(read.status === "loading" || read.status === "hidden") && <p className="orders-message" role="status">{c.loading}</p>}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && read.data && store && (
          <Sections status={read.data} store={store.id} boundary={read.boundary} refresh={read.refresh} locale={locale} c={c} portal={portal} setPortal={setPortal} />
        )}
      </div>
    </WorkspaceFrame>
  );
}

function Sections({
  status,
  store,
  boundary,
  refresh,
  locale,
  c,
  portal,
  setPortal,
}: {
  status: BillingStatus;
  store: string;
  boundary: string;
  refresh: () => Promise<boolean>;
  locale: Locale;
  c: BillingCopy;
  portal: boolean;
  setPortal: (value: boolean) => void;
}) {
  const p = moneyCopy[locale];
  const [busy, setBusy] = useState("");
  const [problem, setProblem] = useState("");
  const live = hasLiveSubscription(status);
  const time = (value: string | null) => (value ? displayTime(locale, value) : "—");

  async function redirect(kind: string, run: () => ReturnType<typeof startCheckout>) {
    if (busy) return;
    setBusy(kind);
    setProblem("");
    const result = await run();
    if (result.ok) {
      // I11: the bearer URL is used once, right here, and dropped; the page is left on purpose so `busy` stays set.
      window.location.assign(result.value);
      return;
    }
    setBusy("");
    setProblem(errorText(c, result.code));
    // The server may know better than this page (e.g. subscription_exists after a second tab): re-read.
    if (result.code === "subscription_exists" || result.code === "no_billing_customer") void refresh();
  }
  if (portal) return <section className="money-card money-editor"><p>{p.portalIntro}</p><div className="money-actions"><button onClick={() => setPortal(false)}>{p.backBilling}</button><button className="primary" data-testid="billing-portal" disabled={busy !== ""} onClick={() => void redirect("portal", () => openPortal(store, boundary))}>{busy === "portal" ? c.portalOpening : p.manage}</button></div>{problem && <p role="alert" data-testid="billing-problem">{problem}</p>}</section>;
  const standing = status.standing === "GOOD" ? p.good : status.standing === "UNBILLED" ? p.unbilled : status.standing === "GRACE" ? p.overdue : p.restricted;
  return <>
    {status.stale && <p role="status" data-testid="billing-stale">{c.stale}</p>}
    {status.subscriptions.some(item => item.cancel_at_period_end) && <p className="money-card">{p.cancellationHint}</p>}
    <div className="money-grid">
      <div className="money-stack">
      <section className="money-card"><h2>{p.standing}</h2><p><Badge tone={status.standing === "GOOD" ? "success" : status.standing === "UNBILLED" ? "neutral" : "warning"} data-testid="billing-standing" data-standing={status.standing}>{standing}</Badge></p><p>{c.standingText[status.standing]}</p>
        {status.payment_pending && <p role="status" data-testid="billing-pending">{c.paymentPending}</p>}
      </section>
      <section className="money-card money-subscription"><h2>{p.subscription}</h2>
        {status.subscriptions.length > 0 ? <div data-testid="billing-subscriptions">{status.subscriptions.map(item => {
          const plan = status.plans.find(candidate => candidate.price_id === item.price_id);
          return <dl key={`${item.price_id}-${item.retrieved_at}`}><div><dt>{p.plan}</dt><dd>{plan?.name ?? "—"}{plan && <span> · {money(locale,plan.currency,plan.amount_minor)} / {c.per[plan.interval] ?? plan.interval}</span>}</dd></div><div><dt>{p.status}</dt><dd>{c.subStatus[item.status]}</dd></div><div><dt>{p.period}</dt><dd>{time(item.current_period_start)} – {time(item.current_period_end)}</dd></div>{item.cancel_at_period_end && <div><dt>{p.cancellation}</dt><dd>{time(item.current_period_end)}</dd></div>}</dl>;
        })}</div> : <p>{c.plansNone}</p>}
        {!live && <ul className="billing-plans" data-testid="billing-plans">{status.plans.map(plan => <li key={plan.price_id} data-testid={`plan-${plan.price_id}`}><span><strong>{plan.name}</strong>{money(locale,plan.currency,plan.amount_minor)} / {c.per[plan.interval] ?? plan.interval}</span><button className="primary" data-testid={`plan-choose-${plan.price_id}`} disabled={busy !== ""} onClick={() => void redirect("checkout", () => startCheckout(store,plan.price_id,boundary))}>{busy === "checkout" ? c.choosing : c.choose}</button></li>)}</ul>}
        {live && <p className="money-note">{c.plansHave}</p>}
        {problem && <p role="alert" data-testid="billing-problem">{problem}</p>}
      </section>
      </div>
      <div className="money-stack">
      <section className="money-card"><h2>{p.payment}</h2><p>{p.paymentIntro}</p>{status.customer_pinned ? <><button type="button" className="primary" data-testid="billing-portal-open" onClick={() => setPortal(true)}>{p.manage}</button><p className="money-note">{p.portalIntro}</p></> : <p>{p.noCustomer}</p>}</section>
      <section className="money-card"><h2>{p.usage}</h2><dl data-testid="billing-usage">{[[c.usagePeriod,`${status.usage.period_start.slice(0,10)} – ${status.usage.period_end.slice(0,10)}`],[c.paidOrders,status.usage.paid_orders],[c.claimWindows,status.usage.claim_windows_opened],[c.privateReplies,status.usage.private_replies_sent],[c.members,status.usage.members]].map(([label,value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl></section>
      </div>
    </div>
    <p className="money-muted" data-testid="billing-test-note">{c.testNote}</p>
  </>;
}
