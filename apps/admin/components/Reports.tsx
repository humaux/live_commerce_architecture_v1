// Purpose: authenticated four-tab sales reports, range controls, permission feedback and one audited CSV action.
// Depends on: React/Next, existing guarded reads/order-actions/session fence, report client/copy and pure ReportViews.
// Used by: app/[locale]/finance/reports/page.tsx; finance group remains the sole primary navigation entry.
// Invariants: I01 server authority, I05 backend-only money, I06 no uncertain export retry, I15 no private cache.
"use client";
import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { TabStrip, DateControl } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { useGuardedRead, ReadError, type ReadCode } from "@/lib/customers-client";
import { financeDay } from "@/lib/customers-model";
import { readOrderActions, OrderReadError } from "@/lib/orders-client";
import { reportNames, validReportsQuery, type ReportName } from "@/lib/reports-request";
import { readReport, downloadReport, exportOutcome, loadUncertainScopes, exportWithLock, type ReportDownload } from "@/lib/reports-client";
import { reportsCopy } from "@/lib/reports-copy";
import type { ProductSort } from "@/lib/reports-presentation";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { ProductReportTable, ChannelReportChart, FunnelReportChart, ManualReportTable } from "./ReportViews";
import "./orders.css";
import "./reports.css";

/** Frozen report page props; initial dates are optional and otherwise use the last 30 Taipei days. */
export type ReportsProps = {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
  initialFrom?: string; initialTo?: string; initialTab?: ReportName;
};

/** Render reports with guarded reads and session-bound CSV download; never owns financial facts. */
export function Reports(props: ReportsProps) {
  // A new server render resets controls and the aborted read scope; uncertain export locks persist in tab sessionStorage.
  return <ReportsWorkspace key={`${props.renderKey}|${props.store?.id ?? ""}`} {...props} />;
}

function ReportsWorkspace({ locale, stores, store, initialError, renderKey, initialFrom, initialTo, initialTab }: ReportsProps) {
  const c = reportsCopy[locale], router = useRouter();
  const [defaults] = useState(() => { const now = new Date(); return { from: financeDay(now, -29), to: financeDay(now) }; });
  const from = initialFrom ?? defaults.from, to = initialTo ?? defaults.to;
  const [draftFrom, setDraftFrom] = useState(from), [draftTo, setDraftTo] = useState(to);
  const [tab, setTab] = useState<ReportName>(initialTab ?? "products");
  // Keep the selected tab in the address so refresh restores it (Show carries it too); replaceState adds no history entry.
  const selectTab = (name: ReportName) => {
    setTab(name);
    try { const u = new URL(window.location.href); u.searchParams.set("report", name); window.history.replaceState(window.history.state, "", u); } catch { /* the in-memory tab still applies */ }
  };
  const [sort, setSort] = useState<ProductSort>("net_minor"), [ascending, setAscending] = useState(false);
  const [exportView, setExportView] = useState<{ scope: string; outcome: ReportDownload | "busy" } | null>(null);
  // I06: unresolved uncertain exports persist per browser tab (sessionStorage by store), so re-renders and refresh never re-enable them.
  const [uncertainScopes, setUncertainScopes] = useState<string[]>([]);
  // A scope still stored on mount is uncertain: it either came back uncertain or its request was cut short (refresh/close mid-flight).
  const storeId = store?.id;
  useEffect(() => { if (storeId) setUncertainScopes((u) => [...new Set([...u, ...loadUncertainScopes(storeId)])]); }, [storeId]);
  const exporting = useRef(false), exportController = useRef<AbortController | null>(null);
  const scope = `${renderKey}|${locale}|${store?.id ?? ""}|${from}|${to}|${tab}`; // read lifecycle: new per server render
  const lockScope = `${store?.id ?? ""}|${from}|${to}|${tab}`; // export lock: stable across renders
  const valid = validReportsQuery("products", `/?from=${draftFrom}&to=${draftTo}`);
  const validQuery = validReportsQuery("products", `/?from=${from}&to=${to}`);
  const permissions = store?.permissions;
  const denied = !!permissions && (!permissions.includes("orders:read") || (tab === "funnel" && !permissions.includes("live:read")));
  const read = useGuardedRead(scope, store && validQuery && !denied ? async (signal) => {
    const view = await readReport(store, tab, from, to, signal);
    if (store.permissions) return { view, canExport: store.permissions.includes("orders:export"), permissionKnown: true };
    // Existing order-actions API supplies a display hint; every CSV is reauthorized on the server.
    try {
      const actions = await readOrderActions(store.id, signal);
      return { view, canExport: actions.orders_export, permissionKnown: true };
    } catch (error) {
      if (error instanceof OrderReadError && error.code === "signed-out") throw new ReadError("signed-out");
      return { view, canExport: false, permissionKnown: false };
    }
  } : null, initialError ?? (denied ? "forbidden" : null));

  useEffect(() => () => { exportController.current?.abort(); }, [scope]);
  useEffect(() => {
    const conceal = () => { exportController.current?.abort(); };
    const visibility = () => { if (document.visibilityState === "hidden") conceal(); };
    window.addEventListener("pagehide", conceal);
    window.addEventListener("commerce-session-logout", conceal);
    document.addEventListener("visibilitychange", visibility);
    return () => {
      window.removeEventListener("pagehide", conceal);
      window.removeEventListener("commerce-session-logout", conceal);
      document.removeEventListener("visibilitychange", visibility);
    };
  }, []);

  const navigate = (storeID: string) => {
    if (!valid || !storeID) return;
    router.push(`/${locale}/finance/reports?store=${storeID}&from=${draftFrom}&to=${draftTo}&report=${tab}`);
  };
  const submit = (event: FormEvent) => { event.preventDefault(); if (store) navigate(store.id); };
  const keyTabs = (event: KeyboardEvent<HTMLDivElement>) => {
    const at = reportNames.indexOf(tab);
    const next = event.key === "ArrowRight" ? (at + 1) % 4 : event.key === "ArrowLeft" ? (at + 3) % 4
      : event.key === "Home" ? 0 : event.key === "End" ? 3 : -1;
    if (next < 0) return;
    event.preventDefault(); selectTab(reportNames[next]);
    event.currentTarget.querySelector<HTMLButtonElement>(`[data-report="${reportNames[next]}"]`)?.focus();
  };
  const tabProps = (name: ReportName) => ({
    id: `reports-tab-${name}`, role: "tab", "aria-selected": tab === name, "aria-controls": `reports-panel-${name}`,
    tabIndex: tab === name ? 0 : -1, "data-report": name, onClick: () => selectTab(name), type: "button" as const,
  });
  const outcome = exportOutcome(lockScope, exportView, uncertainScopes);
  const canExport = read.status === "ready" && !!read.data?.canExport && !exporting.current && outcome !== "uncertain" && valid && draftFrom === from && draftTo === to; // export only the applied range: an edited-but-not-shown range must not download the old one (Codex review P2, PR #3)
  const exportCSV = async () => {
    if (!store || !canExport || !read.boundary) return;
    exporting.current = true;
    const active = new AbortController(); exportController.current = active;
    setExportView({ scope: lockScope, outcome: "busy" });
    const result = await exportWithLock(store.id, lockScope, () => downloadReport(store.id, tab, from, to, read.boundary, active.signal));
    exporting.current = false;
    setExportView({ scope: lockScope, outcome: result });
    if (result === "uncertain") setUncertainScopes((u) => [...new Set([...u, lockScope])]);
    // A signed-out result must hide stale numbers; the guarded read rechecks server session authority.
    if (result === "signed-out") read.reload();
  };
  const csvProps = { type: "button" as const, disabled: !canExport, onClick: exportCSV };
  const csvLabel = outcome === "busy" ? c.exporting : c.csv;
  const failure = read.status === "signed-out" ? c.signedOut : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? (store ? c.notFound : c.noStore) : read.status === "unavailable" ? c.unavailable : "";
  const exportMessage = outcome === "uncertain" ? c.csvUnknown : outcome === "done" ? c.csvDone : outcome === "forbidden" ? c.csvForbidden
    : outcome === "signed-out" ? c.signedOut : outcome === "not-found" ? c.notFound : outcome === "unavailable" ? c.csvUnavailable : "";
  const view = read.status === "ready" ? read.data?.view : null;
  const panel = (name: ReportName) => ({ id: `reports-panel-${name}`, role: "tabpanel", "aria-labelledby": `reports-tab-${name}`, hidden: tab !== name, tabIndex: 0 });
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="finance">
    <div className="orders-page reports-page" data-testid="reports-page">
      <AdminPageHeader locale={locale} title={c.title} description={c.subtitle} />
      <form className="reports-controls" onSubmit={submit}>
        <label>{c.store}<select aria-label={c.store} value={store?.id ?? ""} disabled={stores.length < 2 || !valid}
          onChange={(event) => navigate(event.target.value)}>{!stores.length && <option value="">{c.noStore}</option>}
          {stores.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>
        <label>{c.from}<DateControl emptyLabel={c.from} lang={locale} type="date" value={draftFrom} required data-testid="reports-from"
          onChange={(event) => setDraftFrom(event.target.value)} /></label>
        <label>{c.to}<DateControl emptyLabel={c.to} lang={locale} type="date" value={draftTo} required data-testid="reports-to"
          onChange={(event) => setDraftTo(event.target.value)} /></label>
        <button type="submit" disabled={!store || !valid} data-testid="reports-show">{c.show}</button>
        {!valid && <p role="alert" data-testid="reports-range-invalid">{c.rangeInvalid}</p>}
      </form>
      <div className="reports-basis"><p data-testid="reports-money-basis"><strong>{c.moneyBasis}</strong></p><p>{c.moneyDetails}</p><p>{c.rangeBasis}</p></div>
      <TabStrip role="tablist" className="reports-tabs" label={c.tabsLabel} previousLabel={c.previous} nextLabel={c.next} onKeyDown={keyTabs}>
        <button {...tabProps("products")} data-testid="reports-tab-products">{c.tabs.products}</button>
        <button {...tabProps("channels")} data-testid="reports-tab-channels">{c.tabs.channels}</button>
        <button {...tabProps("funnel")} data-testid="reports-tab-funnel">{c.tabs.funnel}</button>
        <button {...tabProps("manual-orders")} data-testid="reports-tab-manual-orders">{c.tabs["manual-orders"]}</button>
      </TabStrip>
      {(read.status === "loading" || read.status === "hidden") && <p role="status">{read.status === "hidden" ? c.hidden : c.loading}</p>}
      {failure && <div role="status"><p>{failure}</p>{read.status !== "signed-out" && <button type="button" onClick={read.reload}>{c.retry}</button>}</div>}
      {!validQuery && <p role="alert">{c.rangeInvalid}</p>}
      <section {...panel("products")}>
        {view?.name === "products" && <ProductReportTable locale={locale} report={view.report} sort={sort} ascending={ascending}
          onSort={(key) => { setAscending(sort === key ? !ascending : key === "name"); setSort(key); }} />}
        <div className="reports-export"><button {...csvProps} data-testid="reports-csv-products">{csvLabel}</button></div>
      </section>
      <section {...panel("channels")}>
        {view?.name === "channels" && <ChannelReportChart locale={locale} report={view.report} />}
        <div className="reports-export"><button {...csvProps} data-testid="reports-csv-channels">{csvLabel}</button></div>
      </section>
      <section {...panel("funnel")}>
        {view?.name === "funnel" && <FunnelReportChart locale={locale} report={view.report} />}
        <div className="reports-export"><button {...csvProps} data-testid="reports-csv-funnel">{csvLabel}</button></div>
      </section>
      <section {...panel("manual-orders")}>
        {view?.name === "manual-orders" && <ManualReportTable locale={locale} report={view.report} staff={view.staff} sessions={view.sessions} />}
        <div className="reports-export"><button {...csvProps} data-testid="reports-csv-manual-orders">{csvLabel}</button></div>
      </section>
      <p>{c.csvHint}</p>
      {read.status === "ready" && !read.data?.canExport && <p role="status">{read.data?.permissionKnown ? c.csvForbidden : c.exportUnknownPermission}</p>}
      {exportMessage && read.status !== "hidden" && <p role="status" data-testid="reports-export-result">{exportMessage}</p>}
    </div>
  </WorkspaceFrame>;
}
