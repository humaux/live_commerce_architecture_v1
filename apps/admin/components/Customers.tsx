// Purpose: Owns the paginated merchant customer search page.
// Depends on: react, next/link, next/navigation, @live-commerce/i18n, @live-commerce/ui, @/lib/model, @/lib/client, @/lib/customers-client, @/lib/customers-model, @/lib/customers-copy, ./CustomerTags, @/lib/customer-tags-client, @/lib/customer-tags-copy, ./WorkspaceFrame, ./AdminPageHeader, ./Icon, ./orders.css, ./order-actions.css, ./customers.css
// Invariants: server-scoped reads; signed simple search; optional existing tools stay collapsed.
// Used by: apps/admin/app/[locale]/customers/page.tsx
"use client";

// Merchant customers list (/{locale}/customers, U1 audit-first: one table, no card grid, orders table classes).
// BFF GET /api/stores/{store}/customers?limit&after&q -> Go GET /v1/admin/stores/{id}/customers
// (internal/httpapi/customers.go, customers:read; internal/customers.List). Rows link to CustomerDetail.
// Read lifecycle (session fence, PII cleared when hidden/signed out) is useGuardedRead; nothing is cached client-side.
import { useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { Badge } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { readCustomers, useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { canSeeReports, type Customer } from "@/lib/customers-model";
import { readTagCatalog } from "@/lib/customer-tags-client";
import { customerTagsCopy } from "@/lib/customer-tags-copy";
import { CustomerTagManager, TagBadges } from "./CustomerTags";
import { customersCopy, type CustomersCopy } from "@/lib/customers-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { Icon } from "./Icon";
import "./orders.css";
import "./order-actions.css";
import "./customers.css";

function url(locale: Locale, store: string, q: string, after: string, tag = "") {
  const params = new URLSearchParams();
  if (store) params.set("store", store);
  if (q) params.set("q", q);
  if (after) params.set("after", after);
  if (tag) params.set("tag", tag);
  return `/${locale}/customers${params.size ? `?${params}` : ""}`;
}
/** Owns the paginated merchant customer search page. Loads customer results through customers-client. */
export const detailHref = (locale: Locale, store: string, id: string) =>
  `/${locale}/customers/${id}${store ? `?store=${store}` : ""}`;

/** Owns the paginated merchant customer search page. Loads customer results through customers-client. */
export function Customers({
  locale,
  stores,
  store,
  q,
  after,
  tag = "",
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  q: string;
  after: string;
  tag?: string;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = customersCopy[locale];
  const p = c.people;
  const router = useRouter();
  const [draft, setDraft] = useState(q);
  const previous = useRef<string[]>([]);
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${q}|${after}|${tag}`,
    store ? (signal) => readCustomers(store.id, q, after, signal, tag) : null,
    initialError,
  );
  const catalog = useGuardedRead(`${renderKey}|${locale}|${store?.id ?? ""}|tag-catalog`,
    store ? (signal) => readTagCatalog(store.id, signal) : null, initialError);
  const tc = customerTagsCopy[locale];
  const go = (nextStore: string, nextQ: string, nextAfter: string, nextTag = tag) => router.push(url(locale, nextStore, nextQ, nextAfter, nextTag));
  function search(event: FormEvent) {
    event.preventDefault();
    previous.current = [];
    go(store?.id ?? "", draft.trim(), "");
  }
  const failure =
    read.status === "signed-out" ? c.signedOut
    : read.status === "forbidden" ? c.forbidden
    : read.status === "not-found" ? (store ? c.notFound : c.noStore)
    : read.status === "unavailable" ? c.unavailable
    : "";
  const page = read.data;
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="customers">
      <div className="orders-page customers-page people-v2" data-testid="customers-page">
        <AdminPageHeader locale={locale} title={p.title} />
        {/* The search <form> is display:contents inside the controls row: the tag manager dialog has its own <form>s,
            which must not nest in (and bubble submit to) the search form (Codex review P2, PR #3). */}
        <div className="orders-controls">
        <form className="customers-search-form" role="search" onSubmit={search}>
          <label className="customers-search">
            <span className="people-sr-only">{p.search}</span>
            <input
              type="search"
              data-testid="customers-search"
              placeholder={p.search}
              value={draft}
              maxLength={40}
              autoComplete="off"
              onChange={(event) => setDraft(event.target.value)}
            />
          </label>
          <button type="submit" data-testid="customers-search-submit" disabled={!store || read.status === "loading"}>
            <Icon name="search" size={18} />
            {c.searchButton}
          </button>
          {q && (
            <button
              type="button"
              onClick={() => {
                setDraft("");
                previous.current = [];
                go(store?.id ?? "", "", "");
              }}
            >
              {p.clear}
            </button>
          )}
        </form>
        </div>
        <details className="people-advanced" data-testid="people-list-tools"><summary>{p.advanced}</summary>
          <div className="orders-controls">
          <button type="button" data-testid="customers-refresh" disabled={read.status === "loading"} onClick={read.reload}>
            <Icon name="refresh" size={18} />
            {c.refresh}
          </button>
          <label className="customers-search">{tc.filterLabel}
            <select data-testid="customers-tag-filter" value={tag} disabled={!store || catalog.status !== "ready"}
              onChange={event => { previous.current = []; go(store?.id ?? "", q, "", event.target.value); }}>
              <option value="">{tc.filterAll}</option>
              {catalog.data?.items.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}
              {tag && !catalog.data?.items.some(item => item.id === tag) && <option value={tag}>{tc.editorEmpty}</option>}
            </select>
          </label>
          {catalog.status === "unavailable" && <button type="button" onClick={catalog.reload}>{tc.retry}</button>}
          {/* After a write the catalogue is re-read IN PLACE (refresh): reload() would flip the status to loading, unmount the
              manager and its open dialog before the success notice shows (Codex review P2, PR #3). */}
          {store && catalog.status === "ready" && <CustomerTagManager locale={locale} store={store} boundary={catalog.boundary}
            onChanged={() => { void catalog.refresh(); read.reload(); }}
            onScopeLost={() => { catalog.reload(); read.reload(); }} />}
          {store && canSeeReports(store) && <Link className="orders-export" href={`/${locale}/finance/reports?store=${store.id}`} data-testid="customers-reports">{c.reportsLink}</Link>}
          {store?.permissions?.includes("customers:privacy") && <Link className="orders-export" href={`/${locale}/customers/import?store=${store.id}`} data-testid="customers-import">{c.importLink}</Link>}
          </div>
        </details>
        {(read.status === "loading" || read.status === "hidden") && (
          <p className="orders-message" role="status">{c.loading}</p>
        )}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.retry}</button>
          </div>
        )}
        {read.status === "ready" && page && (
          <>
            <h2 className="people-result-count">{q ? p.found(page.items.length) : p.storeCount(page.items.length)}</h2>
            <div className="orders-table-scroll">
              <table className="orders-table customers-table" data-testid="customers-table">
                <thead>
                  <tr>
                    <th>{c.customer}</th>
                    <th>{c.orders}</th>
                    <th>{p.net}</th>
                    <th>{p.consent}</th>
                  </tr>
                </thead>
                <tbody>
                  {page.items.map((row) => (
                    <CustomerRow key={row.customer_id} row={row} c={c} locale={locale} store={store?.id ?? ""} q={q} tag={tag} />
                  ))}
                </tbody>
              </table>
            </div>
            {page.items.length === 0 && (
              <p className="orders-message" role="status" aria-live="polite">{q || tag ? c.emptySearch : c.empty}</p>
            )}
            {(after || page.next_cursor) && <footer className="orders-pager">
              <span>{c.pageCount}: {page.items.length}</span>
              <div>
                <button
                  type="button"
                  data-testid="customers-previous"
                  disabled={!previous.current.length}
                  onClick={() => go(store?.id ?? "", q, previous.current.pop() ?? "")}
                >
                  {c.previous}
                </button>
                <button
                  type="button"
                  data-testid="customers-next"
                  disabled={!page.next_cursor}
                  onClick={() => {
                    previous.current.push(after);
                    go(store?.id ?? "", q, page.next_cursor);
                  }}
                >
                  {c.next}
                </button>
              </div>
            </footer>}
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}

function CustomerRow({ row, c, locale, store, q = "", tag = "" }: { row: Customer; c: CustomersCopy; locale: Locale; store: string; q?: string; tag?: string }) {
  const m = (value: number) => (row.currency ? money(locale, row.currency, value) : String(value));
  const name = row.display_name ?? c.noName;
  const p = c.people;
  return (
    <tr data-testid={`customer-row-${row.customer_id}`}>
      <td data-label={c.customer}>
        <Link href={`${detailHref(locale, store, row.customer_id)}${q ? `&q=${encodeURIComponent(q)}` : ""}${tag ? `&tag=${encodeURIComponent(tag)}` : ""}`} data-testid={`customer-open-${row.customer_id}`}
          aria-label={`${c.open}: ${name}`}>
          <strong>{name}</strong>
          <small>{row.phone_last3 && <>{c.phoneEnding} {row.phone_last3} · </>}{p.customerID} {row.customer_id.slice(0, 8)}</small>
          <span className="people-view">{p.view}</span>
        </Link>
        {!row.active && <Badge tone="neutral">{c.erased}</Badge>}
        {row.imported && <Badge tone="neutral">{c.imported}</Badge>}
        <TagBadges tags={row.tags} locale={locale} />
      </td>
      <td data-label={c.orders}>{row.orders_count}<span className="people-mobile-label"> {p.orders}</span></td>
      <td data-label={p.net}><span className="people-mobile-label">{p.net} </span>{row.currency ? m(row.captured_minor - row.refunded_minor) : "—"}</td>
      <td data-label={p.consent}><span>{c.marketing}: {row.consents.marketing_messages ? c.consentGranted : c.consentNone}</span></td>
    </tr>
  );
}
