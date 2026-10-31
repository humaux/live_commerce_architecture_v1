// Purpose: Signed dashboard help, attention, navigation, quick-start and capability task states.
// Depends on: existing authenticated design/settings/Meta clients, guarded reads, seller locale copy.
// Used by: Dashboard on the existing localized root route with a validated view discriminator.
// Invariants: preserve design destinations/document fields and CAS; no guessed delivery services or write retries.
"use client";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { entryParityCopy, type Locale } from "@live-commerce/i18n";
import { get, useGuardedRead } from "@/lib/customers-client";
import { readDraft, saveDraft } from "@/lib/design-client";
import type { Draft } from "@/lib/design-model";
import { writeSettings } from "@/lib/settings-client";
import { serviceInput } from "@/lib/settings-v2";
import type { Market, Service } from "@/lib/settings-model";
import type { Page } from "@/lib/model";
import { readMetaHealth, recheckMetaHealth } from "@/lib/meta-health-client";
import { metaHealthCopy } from "@/lib/meta-health-copy";
export type DashboardView = "overview" | "help" | "attention" | "navigation" | "delivery" | "first-product" | "capabilities";
type Props = { locale: Locale; store: string; currency: string; view: DashboardView; scope: string; link: (path: string, extra?: Record<string, string>) => string };

/** Preserve the current nav kinds/targets and every unrelated field while editing labels. */
function Navigation({ locale, store, scope, link }: Props) {
  const c = entryParityCopy[locale].dashboard;
  const read = useGuardedRead(scope, signal => readDraft(store, signal), null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [busy, setBusy] = useState(false), [message, setMessage] = useState("");
  const [locked, setLocked] = useState(false);
  const working = useRef(false), live = useRef(true);
  useEffect(() => { live.current = true; return () => { live.current = false; }; }, []);
  useEffect(() => { setDraft(read.data); setLocked(false); }, [read.data]);
  async function save() {
    if (!draft || !read.boundary || read.status !== "ready" || locked || working.current) return;
    working.current = true; setBusy(true); setMessage("");
    try {
      const result = await saveDraft(store, draft.version, draft.document, read.boundary);
      if (!live.current) return;
      if (result.ok && await read.refresh()) setMessage(c.saved);
      else { setLocked(true); setMessage(!result.ok && result.code === "conflict" ? c.conflict : c.saveFailed); }
    } finally { working.current = false; if (live.current) setBusy(false); }
  }
  const labels = { home: c.home, all_products: c.products, collection: c.collection, page: c.page, url: c.url };
  return <section className="entry-task-surface">
    <h2>{c.navItems}</h2><p>{c.navBody}</p>
    {read.status !== "ready" ? <p role="status">{read.status === "loading" ? c.loading : c.unavailable}</p> : draft && <>
      <div className="entry-nav-grid">{(["header", "footer"] as const).flatMap(area => draft.document.nav[area].map((item, i) => <div className="entry-nav-item" key={`${area}:${i}`}>
        <h3>{i + 1}. {labels[item.kind]}</h3>
        <label><span>{c.displayName}</span><input value={item.label} maxLength={60} disabled={busy || locked} onChange={event => {
          const label = event.target.value;
          setDraft(current => current && ({ ...current, document: { ...current.document, nav: { ...current.document.nav,
            [area]: current.document.nav[area].map((row, index) => index === i ? { ...row, label } : row),
          } } }));
        }} /></label>
        <p className="entry-task-muted">{c.destination}: {labels[item.kind]}{item.target ? ` / ${item.target}` : ""}</p>
      </div>))}</div>
      {!draft.document.nav.header.length && !draft.document.nav.footer.length && <p>{c.emptyMenu} <Link href={link("design")}>{c.editDesign}</Link></p>}
      <button className="entry-task-primary" disabled={busy || locked || !read.boundary} onClick={() => void save()}>{busy ? c.saving : c.saveMenu}</button>
    </>}
    {message && <p role="status">{message}</p>}
    {(locked || !["ready", "loading"].includes(read.status)) && <button onClick={read.reload}>{c.reload}</button>}
  </section>;
}

/** Discover only a single matching market; never substitute or create an unconfigured service. */
async function deliveryServices(store: string, currency: string, signal: AbortSignal) {
  const parsePage = <T,>(value: unknown): Page<T> => {
    const p = value as Page<T>;
    if (!p || !Array.isArray(p.items) || typeof p.next_cursor !== "string" || p.next_cursor) throw new Error("incomplete_delivery");
    return p;
  };
  const markets = await get(`/api/stores/${store}/markets`, parsePage<Market>, signal);
  const candidates = markets.items.filter(m => m.active && m.currency === currency);
  if (candidates.length !== 1 || !/^[0-9a-f-]{36}$/.test(candidates[0].id)) throw new Error("ambiguous_market");
  const market = candidates[0];
  const result = await get(`/api/stores/${store}/markets/${market.id}/countries/TW/delivery-services`, parsePage<Service>, signal);
  const services = result.items.filter(s => s.mode === "MANUAL" && (s.delivery_kind === "home" || s.delivery_kind.startsWith("cvs_")));
  if (services.some(s => s.market_id !== market.id || s.country !== "TW" || s.currency !== currency || !/^[a-z][a-z0-9_-]{0,39}$/.test(s.code) || !Number.isSafeInteger(s.version) || s.version < 1 || typeof s.enabled !== "boolean")) throw new Error("invalid_delivery");
  if (new Set(services.map(s => s.delivery_kind)).size !== services.length) throw new Error("ambiguous_delivery");
  return services;
}

/** Delivery toggles save existing versioned services, then read the server before advancing. */
function Delivery(props: Props) {
  const { locale, store, currency, scope, link } = props, c = entryParityCopy[locale].dashboard;
  const read = useGuardedRead(scope, signal => deliveryServices(store, currency, signal), null);
  const [selection, setSelection] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false), [locked, setLocked] = useState(false), [message, setMessage] = useState("");
  const working = useRef(false), live = useRef(true);
  useEffect(() => { live.current = true; return () => { live.current = false; }; }, []);
  useEffect(() => { if (read.data) setSelection(Object.fromEntries(read.data.map(s => [s.code, s.enabled]))); }, [read.data]);
  const home = read.data?.filter(s => s.delivery_kind === "home") ?? [];
  const cvs = read.data?.filter(s => s.delivery_kind.startsWith("cvs_")) ?? [];
  async function next() {
    if (!read.data || !read.boundary || read.status !== "ready" || locked || working.current || !read.data.some(s => selection[s.code])) return;
    working.current = true; setBusy(true); setMessage("");
    try {
      for (const service of read.data.filter(s => selection[s.code] !== s.enabled)) {
        const resource = `markets/${service.market_id}/countries/TW/delivery-services/${service.code}`;
        const result = await writeSettings<Service>(store, { resource, method: "PUT", key: crypto.randomUUID() }, JSON.stringify(serviceInput(service, selection[service.code])), read.boundary);
        if (result.error || result.uncertain || !result.value) throw new Error("save_failed");
      }
      if (!live.current || !await read.refresh()) throw new Error("read_failed");
      window.location.assign(link("", { view: "first-product" }));
    } catch { if (live.current) { setLocked(true); setMessage(c.saveFailed); } }
    finally { working.current = false; if (live.current) setBusy(false); }
  }
  const choice = (label: string, description: string, services: Service[]) => <label className="entry-delivery-choice">
    <input type="checkbox" checked={services.length > 0 && services.every(s => selection[s.code])} disabled={read.status !== "ready" || !services.length || busy || locked}
      onChange={event => { const enabled = event.target.checked; setSelection(current => ({ ...current, ...Object.fromEntries(services.map(s => [s.code, enabled])) })); }} />
    <span><strong>{label}</strong><span className="entry-task-muted">{description}</span></span>
  </label>;
  return <div className="entry-quickstart">
    <p className="entry-quickstart-progress">{c.quickstart} · 2 / 3</p><h1>{c.deliveryTitle}</h1><p>{c.deliveryBody}</p>
    {read.status === "loading" && <p role="status">{c.loading}</p>}
    {read.status !== "ready" && read.status !== "loading" && <p role="status">{c.deliveryMissing} <Link href={link("settings")}>{c.settings}</Link></p>}
    <section className="entry-task-surface">{choice(c.homeDelivery, c.homeDescription, home)}{choice(c.cvs, c.cvsDescription, cvs)}</section>
    <p className="entry-task-note">{c.deliveryNote}</p>
    {message && <p role="status">{message}</p>}
    {locked && <button onClick={() => { setLocked(false); setMessage(""); read.reload(); }}>{c.reload}</button>}
    <div className="entry-task-actions"><Link href={link("settings")}>{c.back}</Link><button className="entry-task-primary" disabled={busy || locked || read.status !== "ready" || !read.data?.some(s => selection[s.code])} onClick={() => void next()}>{busy ? c.saving : c.nextProduct}</button></div>
  </div>;
}

/** Render observed capabilities; schedule only once and never call a scheduling receipt a successful probe. */
function Capabilities({ locale, store, scope, link }: Props) {
  const c = entryParityCopy[locale].dashboard, m = metaHealthCopy[locale];
  const read = useGuardedRead(scope, signal => readMetaHealth(store, signal), null);
  const [busy, setBusy] = useState(false), [message, setMessage] = useState("");
  const working = useRef(false), live = useRef(true);
  useEffect(() => { live.current = true; return () => { live.current = false; }; }, []);
  async function recheck() {
    if (!read.data?.pages.length || !read.boundary || read.status !== "ready" || working.current) return;
    working.current = true; setBusy(true); setMessage("");
    try {
      // Each identified Page gets exactly one explicit schedule request; no provider send or automatic retry.
      for (const page of read.data.pages) {
        const result = await recheckMetaHealth(store, page.page_id, read.boundary);
        if (!result.ok) { if (live.current) setMessage(result.uncertain ? m.uncertain : c.recheckFailed); return; }
      }
      if (live.current) setMessage(c.scheduled);
    } finally { working.current = false; if (live.current) setBusy(false); }
  }
  return <><h1>{c.capabilities}</h1><h2>{c.capabilityTitle}</h2>
    {read.status === "ready" ? read.data?.pages.map(page => <section className="entry-task-surface" key={page.page_id}><h3>{page.page_name}</h3>
      <dl>{page.capabilities.map(row => <div className="entry-capability" key={`${row.binding_id}:${row.capability}`}><dt>{row.provider === "facebook" ? "Facebook" : "Instagram"} · {row.capability ? m.capability[row.capability] : c.connection}</dt><dd>{m.state[row.state]}</dd></div>)}</dl>
    </section>) : <p role="status">{read.status === "loading" ? c.loading : m.unavailable}</p>}
    <section><h2>{c.restrictions}</h2><p>{c.restrictionBody}</p></section>
    {message && <p role="status">{message}</p>}
    <div className="entry-task-actions"><Link href={link("settings")}>{c.settings}</Link><button className="entry-task-primary" disabled={busy || !!message || read.status !== "ready" || !read.data?.pages.length} onClick={() => void recheck()}>{c.recheck}</button></div>
  </>;
}

/** Each state stays on the real root route and uses existing module destinations. */
export function DashboardTasks(props: Props) {
  const { locale, view, link } = props, c = entryParityCopy[locale].dashboard;
  if (view === "delivery") return <Delivery {...props} />;
  if (view === "capabilities") return <Capabilities {...props} />;
  if (view === "first-product") return <div className="entry-quickstart"><p className="entry-quickstart-progress">{c.quickstart} · 3 / 3</p>
    <h1>{c.firstProduct}</h1><p>{c.firstBody}</p><section className="entry-task-surface"><ul className="entry-product-checklist">{[c.productName, c.productPrice, c.productImage].map(label => <li key={label}>{label}</li>)}</ul></section>
    <div className="entry-task-actions"><Link href={link("", { view: "delivery" })}>{c.back}</Link><Link className="entry-task-primary" href={link("products/new")}>{c.firstAction}</Link></div><Link href={link("settings")}>{c.skip}</Link>
  </div>;
  if (view === "navigation") return <><Link className="entry-task-back" href={link("")}>{c.back}</Link><h1>{c.navigation}</h1><Navigation {...props} /></>;
  if (view === "help") return <><h1>{c.help}</h1><p>{c.helpBody}</p>
    {[[c.helpSetup, c.helpSetupBody, "settings"], [c.helpLive, c.helpLiveBody, "studio"], [c.helpOrders, c.helpOrdersBody, "orders"]].map(([title, body, path]) => <section className="entry-task-surface entry-help-row" key={path}><h2>{title}</h2><p>{body}</p><Link href={link(path)}>{c.helpAction}</Link></section>)}
  </>;
  return <><h1>{c.attention}</h1><p>{c.attentionBody}</p>
    <section className="entry-task-surface"><h2>{c.connection}</h2><Link href={link("", { view: "capabilities" })}>{c.capabilities}</Link></section>
    <section className="entry-task-surface"><h2>{c.todos}</h2><Link href={link("orders")}>{c.helpOrders}</Link></section>
    <p className="entry-task-note">{c.attentionNote}</p>
  </>;
}
