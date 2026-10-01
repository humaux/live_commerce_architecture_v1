"use client";

// Merchant product list (/{locale}/products): one dense table of every product with search, status filter, cover
// thumbnail, active price range and stock summed over all warehouses (unit catalog-core, contracts/storefront-v2.md A).
// BFF GET /api/stores/{store}/catalog-products?q&status&cursor&limit -> Go GET /v1/admin/stores/{id}/catalog-products
// (internal/httpapi/collections.go, catalog:read + inventory:read; internal/catalog.ListProductSummaries). Cover bytes:
// GET .../products/{id}/images/{image} (existing photo preview). Read lifecycle (session fence, cleared when hidden or
// signed out) is useGuardedRead. The wide per-warehouse view stays on the Inventory ledger, linked from the heading.
import { useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readProducts } from "@/lib/catalog-v2-client";
import type { ProductSummary } from "@/lib/catalog-v2-model";
import { catalogCopy } from "@/lib/catalog-v2-copy";
import { imageURL } from "@/lib/images-client";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { ProductPhoto } from "./ProductPhoto";
import { Icon } from "./Icon";
import "./orders.css";
import "./ProductAdmin.css";

export const productStatuses = ["all", "draft", "active", "archived"] as const;

const href = (locale: Locale, store: string, q: string, status: string, after: string) => {
  const params = new URLSearchParams();
  if (store) params.set("store", store);
  if (q) params.set("q", q);
  if (status && status !== "all") params.set("status", status);
  if (after) params.set("after", after);
  return `/${locale}/products${params.size ? `?${params}` : ""}`;
};
export const editHref = (locale: Locale, store: string, id: string) => `/${locale}/products/${id}${store ? `?store=${store}` : ""}`;

export function ProductList({
  locale, stores, store, q, status, after, initialError, renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  q: string;
  status: string;
  after: string;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = catalogCopy[locale];
  const l = c.list;
  const router = useRouter();
  const [draft, setDraft] = useState(q);
  const previous = useRef<string[]>([]);
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${q}|${status}|${after}`,
    store ? (signal) => readProducts(store.id, q, status, after, signal) : null,
    initialError,
  );
  const go = (nextStore: string, nextQ: string, nextStatus: string, nextAfter: string) =>
    router.push(href(locale, nextStore, nextQ, nextStatus, nextAfter));
  function search(event: FormEvent) {
    event.preventDefault();
    previous.current = [];
    go(store?.id ?? "", draft.trim(), status, "");
  }
  const failure =
    read.status === "signed-out" ? l.signedOut
    : read.status === "forbidden" ? l.forbidden
    : read.status === "not-found" ? l.noStore
    : read.status === "unavailable" ? l.unavailable
    : "";
  const page = read.data;
  const sid = store?.id ?? "";
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? l.noStore} active="products">
      <div className="orders-page product-admin" data-testid="products-page">
        <header className="orders-heading product-heading">
          <div>
            <h1>{l.title}</h1>
            <p>{l.subtitle}</p>
          </div>
          <div className="product-heading-actions">
            <Link className="product-link" href={`/${locale}/${sid ? `?store=${sid}` : ""}`} data-testid="products-ledger-link">{l.inventory}</Link>
            {store && (
              <Link className="product-primary" href={`/${locale}/products/new?store=${sid}`} data-testid="product-new">
                <Icon name="product" size={18} />
                {l.newProduct}
              </Link>
            )}
          </div>
        </header>
        <form className="orders-controls" role="search" onSubmit={search}>
          {stores.length > 1 && (
            <label>
              {l.store}
              <select data-testid="store-selector" value={sid} onChange={(e) => { previous.current = []; go(e.target.value, "", "all", ""); }}>
                {stores.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}
              </select>
            </label>
          )}
          <label className="product-search">
            {l.search}
            <input type="search" data-testid="products-search" value={draft} maxLength={120} autoComplete="off" onChange={(e) => setDraft(e.target.value)} />
          </label>
          <label>
            {l.status}
            <select data-testid="products-status" value={status || "all"} onChange={(e) => { previous.current = []; go(sid, draft.trim(), e.target.value, ""); }}>
              {productStatuses.map((s) => <option key={s} value={s}>{s === "all" ? l.all : c.status[s]}</option>)}
            </select>
          </label>
          <button type="submit" data-testid="products-search-submit" disabled={!store || read.status === "loading"}>
            <Icon name="search" size={18} />
            {l.searchButton}
          </button>
          {(q || (status && status !== "all")) && (
            <button type="button" onClick={() => { setDraft(""); previous.current = []; go(sid, "", "all", ""); }}>{l.clear}</button>
          )}
        </form>
        {(read.status === "loading" || read.status === "hidden") && <p className="orders-message" role="status">{l.loading}</p>}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{l.retry}</button>
          </div>
        )}
        {read.status === "ready" && page && (
          <>
            {page.items.length > 0 && (
              <div className="orders-table-scroll">
                <table className="orders-table product-table" data-testid="products-table">
                  <thead>
                    <tr><th>{l.product}</th><th>{l.status}</th><th>{l.price}</th><th>{l.stock}</th></tr>
                  </thead>
                  <tbody>
                    {page.items.map((row) => <Row key={row.id} row={row} locale={locale} store={sid} />)}
                  </tbody>
                </table>
              </div>
            )}
            {page.items.length === 0 && <p className="orders-message" role="status" aria-live="polite">{q || status !== "all" ? l.emptySearch : l.empty}</p>}
            <footer className="orders-pager">
              <span />
              <div>
                <button type="button" data-testid="products-previous" disabled={!previous.current.length && !after} onClick={() => go(sid, q, status, previous.current.pop() ?? "")}>{l.previous}</button>
                <button type="button" data-testid="products-next" disabled={!page.next_cursor} onClick={() => { previous.current.push(after); go(sid, q, status, page.next_cursor); }}>{l.next}</button>
              </div>
            </footer>
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}

function Row({ row, locale, store }: { row: ProductSummary; locale: Locale; store: string }) {
  const c = catalogCopy[locale];
  const l = c.list;
  const price =
    row.price_min_minor === null || row.price_max_minor === null ? l.noVariants
    : row.price_min_minor === row.price_max_minor ? money(locale, row.currency, row.price_min_minor)
    : `${money(locale, row.currency, row.price_min_minor)} – ${money(locale, row.currency, row.price_max_minor)}`;
  return (
    <tr data-testid={`product-row-${row.id}`}>
      <td data-label={l.product}>
        <Link className="product-cell" href={editHref(locale, store, row.id)} aria-label={`${l.edit}: ${row.name}`}>
          <ProductPhoto code="" name={row.name} demo={false} imageSrc={row.cover_image_id ? imageURL(store, row.id, row.cover_image_id) : undefined} />
          <span>
            <strong>{row.name}</strong>
            <small>/{row.slug} · {l.variants(row.sku_count)}</small>
          </span>
        </Link>
      </td>
      <td data-label={l.status}>
        <span className={`orders-badge product-status product-status-${row.status}`}>{c.status[row.status]}</span>
      </td>
      <td data-label={l.price}>{price}</td>
      <td data-label={l.stock}>
        {row.sku_count === 0 ? "—" : row.available <= 0 ? <span className="product-out">{l.outOfStock}</span> : l.units(row.available)}
      </td>
    </tr>
  );
}
