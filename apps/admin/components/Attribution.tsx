// Purpose: Owns the read-only merchant attribution report page.
// Depends on: react, next/link, next/navigation, @live-commerce/i18n, @live-commerce/ui, @/lib/presentation-copy, @/lib/model, @/lib/ads-request, @/lib/attribution-client, @/lib/attribution-copy, @/lib/attribution-model, ./WorkspaceFrame, ./attribution.css, ./AttributionPanels, ./ads/parity.css, @live-commerce/format, @/lib/ads-copy, ./AdsResults
// Used by: apps/admin/app/[locale]/ads/attribution/page.tsx
// Invariants: separate report sources, unknown values stay absent, server owns all amounts.
"use client";
// D7/D9 readonly page. BFF GET /api/stores/{store}/ads/attribution -> Go GET /v1/admin/stores/{store}/ads/attribution.
// URL holds only report filters. PG/Go own amounts and order facts; audience-read queues guarded Meta GETs only.
// D7/D9 readonly page. BFF GET /api/stores/{store}/ads/attribution -> Go GET /v1/admin/stores/{store}/ads/attribution.
// URL holds only report filters. PG/Go own amounts and order facts; audience-read queues guarded Meta GETs only.
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { growthCopy, type Locale } from "@live-commerce/i18n";
import { money, displayTime } from "@live-commerce/format";
import { adsCopy } from "@/lib/ads-copy";
import { ReportSection } from "./AdsResults";
import { Field, FormRow, DateControl } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";

import type { Store } from "@/lib/model";
import { validAdsQuery } from "@/lib/ads-request";
import {
  readAttribution,
  type AttributionError,
} from "@/lib/attribution-client";
import { attributionCopy } from "@/lib/attribution-copy";


import type { AttributionReport } from "@/lib/attribution-model";
import { WorkspaceFrame } from "./WorkspaceFrame";

import "./attribution.css";
import "./ads/parity.css";
import { DraftPanel, SessionPanel } from "./AttributionPanels";

/** Owns the read-only merchant attribution report page. */
export function Attribution({
  locale,
  store,
  from,
  to,
  draftID,
  sessionID,
  initialError,
}: {
  locale: Locale;
  store: Store | null;
  from: string;
  to: string;
  draftID: string;
  sessionID: string;
  initialError: AttributionError | null;
}) {
  const g = growthCopy[locale];
  const c = attributionCopy[locale],
    router = useRouter();
  const key = `${store?.id}:${from}:${to}`;
  const [result, setResult] = useState<{
    key: string;
    report: AttributionReport;
  } | null>(null);
  const [error, setError] = useState<AttributionError | null>(initialError),
    [retry, setRetry] = useState(0);
  const [range, setRange] = useState({ from, to }),
    [invalid, setInvalid] = useState(false);
  useEffect(() => {
    setRange({ from, to });
    setInvalid(false);
  }, [from, to]);
  useEffect(() => {
    setResult(null);
    setError(initialError);
    if (!store || initialError) return;
    const abort = new AbortController();
    let current = true;
    const expire = () => {
      current = false;
      abort.abort();
      setResult(null);
      setError("signed-out");
    };
    const onStorage = (event: StorageEvent) => {
      if (event.key === "commerce-session-logout") expire();
    };
    window.addEventListener("commerce-session-logout", expire);
    window.addEventListener("storage", onStorage);
    let channel: BroadcastChannel | null = null;
    try {
      channel = new BroadcastChannel("commerce-session");
      channel.onmessage = (event) => {
        if (event.data?.type === "logout") expire();
      };
    } catch {
      /* same-tab + storage remain */
    }
    void readAttribution(store.id, from, to, abort.signal)
      .then((report) => {
        if (current) setResult({ key, report });
      })
      .catch((cause: unknown) => {
        if (!current || abort.signal.aborted) return;
        const code = cause instanceof Error ? cause.message : "unavailable";
        setError(
          code === "signed-out" || code === "forbidden" ? code : "unavailable",
        );
      });
    return () => {
      current = false;
      abort.abort();
      window.removeEventListener("commerce-session-logout", expire);
      window.removeEventListener("storage", onStorage);
      channel?.close();
    };
  }, [store?.id, from, to, retry, initialError, key]);
  const [screen, setScreen] = useState<"index" | "results" | "detail" | "breakdown">(draftID || sessionID ? "detail" : "index");
  const report = result?.key === key && !error ? result.report : null;
  const selectedDraft =
    report?.drafts.find((d) => d.draft_id === draftID) ??
    (!draftID ? report?.drafts[0] : undefined);
  const selectedSession =
    report?.sessions.find((s) => s.session_id === sessionID) ??
    (!sessionID ? report?.sessions[0] : undefined);
  function navigate(fields: {
    from?: string;
    to?: string;
    draft?: string;
    session?: string;
  }) {
    const q = new URLSearchParams({
      ...(store ? { store: store.id } : {}),
      from,
      to,
    });
    if (draftID) q.set("draft", draftID);
    if (sessionID) q.set("session", sessionID);
    for (const [name, value] of Object.entries(fields))
      value ? q.set(name, value) : q.delete(name);
    if (fields.draft || fields.session) setScreen("detail");
    router.push(`/${locale}/ads/attribution?${q}`);
  }
  function index() { setScreen("index"); navigate({ draft: "", session: "" }); }
  const title = screen === "results" ? g.results : screen === "breakdown" ? g.breakdown : screen === "detail" ? (sessionID && !draftID ? c.sessions : g.adAttribution) : g.attribution;
  const loading = !error && !!store && !report;
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? ""}
      active="ads-attribution"
    >
      <div className="growth-task attribution-page" data-testid="ads-attribution">
        <h1>{title}</h1><p className="growth-intro">{screen === "index" ? g.attributionIntro : screen === "breakdown" ? g.breakdownHint : c.intro}</p>
        <div className="growth-actions"><Link data-testid="attribution-back" href={`/${locale}/ads${store ? `?store=${store.id}` : ""}`}>{c.back}</Link>
          {screen === "index" ? <button type="button" onClick={() => setScreen("results")}>{g.results}</button> : <button type="button" onClick={index}>{screen === "results" ? g.viewAttribution : g.backAttribution}</button>}
          {screen === "detail" && selectedDraft && <button type="button" onClick={() => setScreen("breakdown")}>{g.breakdown}</button>}
          {screen === "breakdown" && <button type="button" onClick={() => setScreen("detail")}>{g.backDetail}</button>}
          {screen !== "index" && <button type="button" onClick={() => setRetry(v => v + 1)}>{g.refresh}</button>}
        </div>
        <form
          className="attribution-filters"
          onSubmit={(event) => {
            event.preventDefault();
            if (
              !validAdsQuery(
                `https://local.invalid/?from=${range.from}&to=${range.to}`,
                "ads/attribution",
              )
            ) {
              setInvalid(true);
              return;
            }
            setInvalid(false);
            if (range.from === from && range.to === to) setRetry((v) => v + 1);
            else navigate({ ...range, draft: "", session: "" });
          }}
        >
          <FormRow>
            <Field id="attribution-from" label={c.from} width="short">
              <DateControl emptyLabel={presentationCopy[locale].date}
                id="attribution-from"
                lang={locale}
                type="date"
                required
                value={range.from}
                onChange={(e) =>
                setRange((v) => ({ ...v, from: e.target.value }))
              }
                data-testid="attribution-from"
              />
            </Field>
            <Field id="attribution-to" label={c.to} width="short">
              <DateControl emptyLabel={presentationCopy[locale].date}
                id="attribution-to"
                lang={locale}
                type="date"
                required
                value={range.to}
                onChange={(e) => setRange((v) => ({ ...v, to: e.target.value }))}
                data-testid="attribution-to"
              />
            </Field>
          </FormRow>
          <button
            type="submit"
            disabled={
              loading ||
              !store ||
              error === "signed-out" ||
              error === "forbidden"
            }
            data-testid="attribution-apply"
          >
            {c.apply}
          </button>
        </form>
        {invalid && <p role="alert">{c.invalid}</p>}
        {loading && (
          <p role="status" data-testid="attribution-loading">
            {c.loading}
          </p>
        )}
        {error && (
          <div role="alert" data-testid="attribution-error">
            <p>
              {error === "signed-out"
                ? c.signedOut
                : error === "forbidden"
                  ? c.forbidden
                  : c.unavailable}
            </p>
            {error === "signed-out" ? (
              <Link href={`/${locale}/`}>{c.signIn}</Link>
            ) : (
              error === "unavailable" && (
                <button
                  type="button"
                  data-testid="attribution-retry"
                  onClick={() => {
                    if (initialError) router.refresh();
                    else setRetry((v) => v + 1);
                  }}
                >
                  {c.retry}
                </button>
              )
            )}
          </div>
        )}
        {!store && !error && <p>{c.empty}</p>}
        {report && (
          <>
            <p className="attribution-note" data-testid="attribution-window">
              {report.window.from} – {report.window.to} · {c.orderZone}
            </p>
            {report.truncated && (
              <p role="status" data-testid="attribution-truncated">
                {c.truncated}
              </p>
            )}
            {!report.drafts.length && !report.sessions.length && (
              <p data-testid="attribution-empty">{c.empty}</p>
            )}
            {screen !== "results" && <details data-testid="attribution-selection" open={screen === "index" ? true : undefined}><summary>{g.advanced}</summary>
            <FormRow className="attribution-selection">
              <Field
                id="attribution-draft"
                label={c.selectDraft}
                hint={!report.drafts.length ? c.noDraft : undefined}
                width="long"
              >
                <select
                  id="attribution-draft"
                  aria-describedby="attribution-draft-hint"
                  data-testid="attribution-draft"
                  value={selectedDraft?.draft_id ?? ""}
                  disabled={!report.drafts.length}
                  onChange={(e) => navigate({ draft: e.target.value })}
                >
                  {!selectedDraft && (
                    <option value="">{report.drafts.length ? c.noDraft : "—"}</option>
                  )}
                  {report.drafts.map((d) => (
                    <option key={d.draft_id} value={d.draft_id}>
                      {d.source_ref || d.draft_id}
                    </option>
                  ))}
                </select>
              </Field>
              <Field
                id="attribution-session"
                label={c.selectSession}
                hint={!report.sessions.length ? c.noSession : undefined}
                width="long"
              >
                <select
                  id="attribution-session"
                  aria-describedby="attribution-session-hint"
                  data-testid="attribution-session"
                  value={selectedSession?.session_id ?? ""}
                  disabled={!report.sessions.length}
                  onChange={(e) => navigate({ session: e.target.value })}
                >
                  {!selectedSession && (
                    <option value="">{report.sessions.length ? c.noSession : "—"}</option>
                  )}
                  {report.sessions.map((s) => (
                    <option key={s.session_id} value={s.session_id}>
                      {s.title || s.session_id}
                    </option>
                  ))}
                </select>
              </Field>
            </FormRow>
            </details>}
            {screen === "index" && <div className="growth-grid">{report.drafts.map(d => <section className="growth-card" key={d.draft_id}><h2>{d.source_ref || d.draft_id}</h2><button type="button" data-testid={`attribution-open-${d.draft_id}`} onClick={() => navigate({ draft: d.draft_id, session: "" })}>{g.adAttribution}</button></section>)}{report.sessions.map(s => <section className="growth-card" key={s.session_id}><h2>{s.title || s.session_id}</h2><button type="button" data-testid={`attribution-session-open-${s.session_id}`} onClick={() => navigate({ session: s.session_id, draft: "" })}>{c.sessions}</button></section>)}</div>}
            {screen === "results" && <ReportSection c={{ ...adsCopy[locale], blockOrders: g.orders, blockDelivery: g.delivery, blockReported: g.reported }} locale={locale} store={store!} when={(l, iso, empty) => iso ? displayTime(l, iso) : empty} />}
            {screen === "detail" && selectedDraft && <>
              <div className="growth-columns"><section className="growth-card"><h2>{c.ordersTitle}</h2>{selectedDraft.orders.map(o => <dl className="growth-facts" key={o.path}><div><dt>{o.path === "ad_click" ? c.adClick : c.boosted}</dt><dd>{o.orders} · {money(locale, selectedDraft.currency, o.net_minor)}</dd></div></dl>)}<p>{c.boostedNote}</p></section>
                <section className="growth-card"><h2>{c.metaTitle}</h2><dl className="growth-facts"><div><dt>{c.spend}</dt><dd>{selectedDraft.spend_minor === null ? c.unknown : money(locale, selectedDraft.currency, selectedDraft.spend_minor)}</dd></div><div><dt>{c.purchases}</dt><dd>{selectedDraft.meta.purchases ?? c.unknown}</dd></div><div><dt>{c.purchaseValue}</dt><dd>{selectedDraft.meta.purchase_value_minor === null ? c.unknown : money(locale, selectedDraft.currency, selectedDraft.meta.purchase_value_minor)}</dd></div></dl><p>{c.metaZone}: {selectedDraft.meta_account_timezone ?? c.unknown}</p></section></div>
              <section className="growth-card"><h2>{g.pending}</h2>{selectedDraft.orders.map(o => <dl className="growth-facts" key={o.path}><div><dt>{o.path === "ad_click" ? c.adClick : c.boosted}</dt><dd>{o.pending_orders} · {money(locale, selectedDraft.currency, o.pending_minor)}</dd></div></dl>)}</section>
            </>}
            {screen === "breakdown" && selectedDraft && <DraftPanel c={c} locale={locale} draft={selectedDraft} view="breakdown" />}
            {screen === "detail" && <details data-testid="attribution-advanced"><summary>{g.advanced}</summary>
            {selectedDraft && (
              <DraftPanel c={c} locale={locale} draft={selectedDraft} />
            )}
            {selectedSession && (
              <SessionPanel
                c={c}
                locale={locale}
                session={selectedSession}
                store={store!.id}
              />
            )}
            </details>}
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}
