"use client";
import { DateControl } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
// Extracted ads panel: existing BFF ads-client calls -> Go /v1/admin/stores/{store}/ads; no command or DTO changes.
import { useCallback, useEffect, useMemo, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Field, FormRow } from "@live-commerce/ui";
import { decimal } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import { readReport, AdsReadError } from "@/lib/ads-client";
import {
  formatMinor,
  localDate,
  maxReportDays,
  validCapi,
  validReportWindow,
  dayMs,
  type Report,
  type Settings,
} from "@/lib/ads-model";
import { errorText, type AdsCopy } from "@/lib/ads-copy";

export function ReportSection({
  c,
  locale,
  store,
  when,
}: {
  when: (locale: Locale, iso: string | null, empty: string) => string;
  c: AdsCopy;
  locale: Locale;
  store: Store;
}) {
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [state, setState] = useState<
    | { kind: "idle" }
    | { kind: "loading" }
    | { kind: "invalid" }
    | { kind: "error"; text: string }
    | { kind: "ready"; report: Report }
  >({ kind: "idle" });
  const load = useCallback(
    async (a: string, b: string, signal: AbortSignal) => {
      if (!validReportWindow(a, b)) {
        setState({ kind: "invalid" });
        return;
      }
      setState({ kind: "loading" });
      try {
        const report = await readReport(store.id, a, b, signal);
        if (!signal.aborted) setState({ kind: "ready", report });
      } catch (error) {
        if (signal.aborted) return;
        const code = error instanceof AdsReadError ? error.code : "unavailable";
        setState({
          kind: "error",
          text:
            code === "signed-out"
              ? c.signedOut
              : code === "forbidden"
                ? c.forbidden
                : code === "unavailable" || code === "not-found"
                  ? c.unavailable
                  : errorText(c, code),
        });
      }
    },
    [store.id, c],
  );
  useEffect(() => {
    // Default window: the last 7 days ending today (browser-local dates, set after mount to keep SSR identical).
    const now = Date.now();
    const a = localDate(now - 6 * dayMs);
    const b = localDate(now);
    setFrom(a);
    setTo(b);
    const active = new AbortController();
    void load(a, b, active.signal);
    return () => active.abort();
  }, [load]);
  const money = (currency: string, minor: number) =>
    formatMinor(locale, currency, minor);
  const num = (n: number) => decimal(locale, n, 0);
  const fetched = (iso: string | null) =>
    `${c.reportFetched}: ${when(locale, iso, c.reportNotFetched)}`;
  const r = state.kind === "ready" ? state.report : null;
  return (
    <section
      className="ads-section"
      aria-labelledby="ads-report-h"
      data-testid="ads-report"
    >
      <div className="ads-section-head">
        <h2 id="ads-report-h">{c.reportTitle}</h2>
      </div>
      <form
        className="ads-controls"
        onSubmit={(e) => {
          e.preventDefault();
          void load(from, to, new AbortController().signal);
        }}
      >
        <FormRow>
          <Field id="ads-report-from" label={c.reportFrom} width="short">
            <DateControl emptyLabel={presentationCopy[locale].date}
              id="ads-report-from"
              lang={locale}
              type="date"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              data-testid="ads-report-from"
            />
          </Field>
          <Field id="ads-report-to" label={c.reportTo} width="short">
            <DateControl emptyLabel={presentationCopy[locale].date}
              id="ads-report-to"
              lang={locale}
              type="date"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              data-testid="ads-report-to"
            />
          </Field>
        </FormRow>
        <button
          type="submit"
          disabled={state.kind === "loading"}
          data-testid="ads-report-load"
        >
          {c.reportLoad}
        </button>
      </form>
      <p className="ads-note">{c.reportRefreshNote}</p>
      {state.kind === "loading" && (
        <p className="ads-message" role="status">
          {c.reportLoading}
        </p>
      )}
      {state.kind === "invalid" && (
        <p className="ads-bad" role="alert">
          {c.reportInvalid} ({maxReportDays})
        </p>
      )}
      {state.kind === "error" && (
        <p className="ads-bad" role="alert">
          {state.text}
        </p>
      )}
      {r && (
        <>
          <p className="ads-note" data-testid="ads-report-separate">
            {c.reportSeparate}
          </p>
          <div className="ads-blocks">
            <article
              className="ads-block"
              data-testid="ads-block-orders"
              aria-labelledby="ads-b1"
            >
              <h3 id="ads-b1">{c.blockOrders}</h3>
              <p className="ads-sub">{c.blockOrdersSub}</p>
              {r.orders === null ? (
                <p className="ads-empty" data-testid="ads-orders-na">
                  {c.notAvailable}
                </p>
              ) : (
                <dl className="ads-facts">
                  <div>
                    <dt>{c.captured}</dt>
                    <dd>{money(r.orders.currency, r.orders.captured_minor)}</dd>
                  </div>
                  <div>
                    <dt>{c.refunded}</dt>
                    <dd>{money(r.orders.currency, r.orders.refunded_minor)}</dd>
                  </div>
                  <div>
                    <dt>{c.net}</dt>
                    <dd>
                      <strong>
                        {money(r.orders.currency, r.orders.net_minor)}
                      </strong>
                    </dd>
                  </div>
                </dl>
              )}
              {r.orders?.note === "card_payments_only" && (
                <p className="ads-hint">{c.cardOnly}</p>
              )}
              <p className="ads-meta">
                {c.reportWindow}: {r.window.from} – {r.window.to} ·{" "}
                {c.reportTimezone}: {r.timezone}
                {r.orders ? ` · ${fetched(r.orders.fetched_at)}` : ""}
              </p>
            </article>
            <article
              className="ads-block"
              data-testid="ads-block-delivery"
              aria-labelledby="ads-b2"
            >
              <h3 id="ads-b2">{c.blockDelivery}</h3>
              <p className="ads-sub">{c.blockDeliverySub}</p>
              <dl className="ads-facts">
                <div>
                  <dt>{c.spend}</dt>
                  <dd>
                    {money(
                      r.meta_delivery.currency,
                      r.meta_delivery.spend_minor,
                    )}
                  </dd>
                </div>
                <div>
                  <dt>{c.impressions}</dt>
                  <dd>{num(r.meta_delivery.impressions)}</dd>
                </div>
                <div>
                  <dt>{c.clicks}</dt>
                  <dd>{num(r.meta_delivery.clicks)}</dd>
                </div>
                {r.meta_delivery.final_through && (
                  <div>
                    <dt>{c.finalThrough}</dt>
                    <dd>{r.meta_delivery.final_through}</dd>
                  </div>
                )}
              </dl>
              <p className="ads-meta">
                {c.reportWindow}: {r.window.from} – {r.window.to} ·{" "}
                {c.reportTimezone}: {r.meta_delivery.account_timezone} ·{" "}
                {fetched(r.meta_delivery.fetched_at)}
              </p>
            </article>
            <article
              className="ads-block"
              data-testid="ads-block-reported"
              aria-labelledby="ads-b3"
            >
              <h3 id="ads-b3">{c.blockReported}</h3>
              <p className="ads-sub">{c.blockReportedSub}</p>
              <dl className="ads-facts">
                <div>
                  <dt>{c.purchases}</dt>
                  <dd>{num(r.meta_reported.purchases)}</dd>
                </div>
                <div>
                  <dt>{c.purchaseValue}</dt>
                  <dd>
                    {money(
                      r.meta_reported.currency,
                      r.meta_reported.purchase_value_minor,
                    )}
                  </dd>
                </div>
              </dl>
              <p className="ads-meta">
                {c.reportWindow}: {r.window.from} – {r.window.to} ·{" "}
                {c.reportTimezone}: {r.meta_delivery.account_timezone} ·{" "}
                {fetched(r.meta_reported.fetched_at)}
              </p>
            </article>
          </div>
        </>
      )}
    </section>
  );
}

// ---------- CAPI ----------
export function CapiSection({
  c,
  settings,
  busy,
  uncertain,
  save,
  onRetry,
}: {
  c: AdsCopy;
  settings: Settings;
  busy: boolean;
  uncertain: boolean;
  onRetry: () => void;
  save: (enabled: boolean, dataset: string, test: string) => Promise<void>;
}) {
  const datasets = useMemo(
    () => settings.connections.filter((x) => x.provider === "meta_dataset"),
    [settings],
  );
  const [enabled, setEnabled] = useState(settings.capi.enabled);
  const [dataset, setDataset] = useState(
    settings.capi.dataset_binding_id ?? "",
  );
  const [test, setTest] = useState(settings.capi.test_event_code ?? "");
  const [invalid, setInvalid] = useState(false);
  useEffect(() => {
    setEnabled(settings.capi.enabled);
    setDataset(settings.capi.dataset_binding_id ?? "");
    setTest(settings.capi.test_event_code ?? "");
    // Primitive deps: a background reload returns a new settings object and must not wipe unsaved edits.
  }, [
    settings.capi.enabled,
    settings.capi.dataset_binding_id,
    settings.capi.test_event_code,
  ]); // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <section
      className="ads-section"
      aria-labelledby="ads-capi-h"
      data-testid="ads-capi"
    >
      <div className="ads-section-head">
        <h2 id="ads-capi-h">{c.capiTitle}</h2>
      </div>
      <p className="ads-note">{c.capiIntro}</p>
      <p className="ads-note">{c.capiConsent}</p>
      <form
        className="ads-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (!validCapi(enabled, dataset, test)) return setInvalid(true);
          setInvalid(false);
          void save(enabled, dataset, test);
        }}
      >
        <fieldset disabled={busy || uncertain}>
          <label className="ads-check">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              data-testid="ads-capi-enabled"
            />
            {c.capiEnable}
          </label>
          <Field
            id="ads-capi-dataset"
            label={c.capiDataset}
            hint={datasets.length === 0 ? c.capiNoneConnected : undefined}
          >
            <select
              id="ads-capi-dataset"
              aria-describedby="ads-capi-dataset-hint"
              value={dataset}
              onChange={(e) => setDataset(e.target.value)}
              data-testid="ads-capi-dataset"
            >
              <option value="">{c.capiNoDataset}</option>
              {datasets.map((x) => (
                <option key={x.binding_id} value={x.binding_id}>
                  {x.asset_id}
                </option>
              ))}
            </select>
          </Field>
          <Field id="ads-capi-test" label={c.capiTest} hint={c.capiTestHint}>
            <input
              id="ads-capi-test"
              aria-describedby="ads-capi-test-hint"
              value={test}
              onChange={(e) => setTest(e.target.value)}
              maxLength={64}
              autoComplete="off"
              data-testid="ads-capi-test"
            />
          </Field>
          {invalid && (
            <p className="ads-bad" role="alert">
              {c.capiInvalid}
            </p>
          )}
          <div className="ads-actions">
            <button
              type="submit"
              className="primary"
              data-testid="ads-capi-save"
            >
              {busy ? c.sending : c.capiSave}
            </button>
          </div>
        </fieldset>
        {uncertain && (
          <p className="ads-actions">
            <button type="button" onClick={onRetry}>
              {c.retrySame}
            </button>
          </p>
        )}
      </form>
    </section>
  );
}
