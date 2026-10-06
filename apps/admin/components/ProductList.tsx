// Purpose: Owns catalog product browsing, filtering and bulk-edit controls.
// Depends on: react, next/link, next/navigation, @live-commerce/i18n, @live-commerce/ui, @/lib/model, @/lib/client, @/lib/customers-client, @/lib/catalog-v2-client, @/lib/catalog-v2-model, @/lib/catalog-v2-copy, @/lib/product-editor-copy, @/lib/product-document, @/lib/catalog-v2-write, @/lib/use-product-leave-guard, @/lib/orders-model, @/lib/images-client, ./WorkspaceFrame, ./AdminPageHeader, ./ProductPhoto, ./ProductQuickEdit, ./Icon, ./orders.css, ./ProductAdmin.css, ./ProductDocument.css
// Used by: apps/admin/app/[locale]/products/page.tsx
"use client";

// Merchant product list (/{locale}/products): one dense table of every product with search, status filter, cover
// thumbnail, active price range and stock summed over all warehouses (unit catalog-core, contracts/storefront-v2.md A).
// BFF GET /api/stores/{store}/catalog-products?q&status&cursor&limit -> Go GET /v1/admin/stores/{id}/catalog-products
// (internal/httpapi/collections.go, catalog:read + inventory:read; internal/catalog.ListProductSummaries). Cover bytes:
// GET .../products/{id}/images/{image} (existing photo preview). Read lifecycle (session fence, cleared when hidden or
// signed out) is useGuardedRead. The wide per-warehouse view stays on the Inventory ledger, linked from the heading.
import {
  useEffect,
  useRef,
  useState,
  type FormEvent,
  type ReactNode,
} from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { Badge, TabStrip, TableFrame } from "@live-commerce/ui";
import type { Store } from "@/lib/model";
import { money } from "@/lib/client";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import {
  command,
  readProducts,
  readCollections,
  readCollection,
} from "@/lib/catalog-v2-client";
import {
  productStatuses,
  parseCreated,
  type ProductSummary,
  type ProductStatus,
  type Collection,
} from "@/lib/catalog-v2-model";
import { catalogCopy, catalogPresentationCopy } from "@/lib/catalog-v2-copy";
import { productEditorCopy } from "@/lib/product-editor-copy";
import { parseBulk, type BulkResult } from "@/lib/product-document";
import { useWrite } from "@/lib/catalog-v2-write";
import { useProductLeaveGuard } from "@/lib/use-product-leave-guard";
import { displayTime } from "@/lib/orders-model";
import { imageURL } from "@/lib/images-client";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { ProductPhoto } from "./ProductPhoto";
import { ProductQuickEdit } from "./ProductQuickEdit";
import { Icon } from "./Icon";
import "./orders.css";
import "./ProductAdmin.css";
import "./ProductDocument.css";

const href = (
  locale: Locale,
  store: string,
  q: string,
  status: string,
  after: string,
) => {
  const params = new URLSearchParams();
  if (store) params.set("store", store);
  if (q) params.set("q", q);
  if (status && status !== "all") params.set("status", status);
  if (after) params.set("after", after);
  return `/${locale}/products${params.size ? `?${params}` : ""}`;
};
/** Builds the localized product editor URL without a network request. */
export const editHref = (locale: Locale, store: string, id: string) =>
  `/${locale}/products/${id}${store ? `?store=${store}` : ""}`;

/** Owns catalog product browsing, filtering and bulk-edit controls. User actions submit catalog commands through catalog-v2-client. */
export function ProductList({
  locale,
  store,
  q,
  status,
  after,
  initialError,
  renderKey,
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
  const pc = productEditorCopy[locale];
  const l = c.list;
  const router = useRouter();
  const [draft, setDraft] = useState(q);
  const previous = useRef<string[]>([]);
  const [selected, setSelected] = useState<string[]>([]),
    [quick, setQuick] = useState<{
      id: string;
      field: "price" | "stock";
      inline: boolean;
      store: string;
    } | null>(null),
    [results, setResults] = useState<BulkResult[]>([]),
    [collections, setCollections] = useState<Collection[] | null>(null),
    [collection, setCollection] = useState(""),
    [loadingCollections, setLoadingCollections] = useState(false);
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${q}|${status}|${after}`,
    store ? (signal) => readProducts(store.id, q, status, after, signal) : null,
    initialError,
  );
  const go = (
    nextStore: string,
    nextQ: string,
    nextStatus: string,
    nextAfter: string,
  ) => {
    setSelected([]);
    router.push(href(locale, nextStore, nextQ, nextStatus, nextAfter));
  };
  function search(event: FormEvent) {
    event.preventDefault();
    previous.current = [];
    go(store?.id ?? "", draft.trim(), status, "");
  }
  const failure =
    read.status === "signed-out"
      ? l.signedOut
      : read.status === "forbidden"
        ? l.forbidden
        : read.status === "not-found"
          ? l.noStore
          : read.status === "unavailable"
            ? l.unavailable
            : "";
  const page = read.data;
  const sid = store?.id ?? "";
  useEffect(() => {
    setSelected([]);
    setQuick(null);
    setResults([]);
    setCollections(null);
    setCollection("");
  }, [sid]);
  const write = useWrite(
    sid,
    read.boundary,
    (code) => (code in pc ? pc[code as keyof typeof pc] : pc.failed),
    pc.uncertain,
    pc.listRecoveryRequired,
  );
  const locked = write.busy || write.message?.kind === "uncertain" || !!quick;
  useProductLeaveGuard(
    { dirty: false, locked: write.busy || write.message?.kind === "uncertain" },
    pc.leave,
  );
  const rows = page?.items ?? [];
  async function bulkStatus(target: ProductStatus) {
    if (locked || !selected.length) return;
    if (
      target === "active" &&
      selected.some(
        (id) => !page?.items.find((r) => r.id === id)?.cover_image_id,
      )
    ) {
      write.fail(pc.imageRequired);
      return;
    }
    await write.run(
      command("POST", "products/bulk-status", {
        ids: selected,
        status: target,
      }),
      parseBulk,
      async (value) => {
        setResults(value);
        setSelected([]);
        await read.refresh();
      },
      pc.success,
    );
  }
  async function chooseCollection() {
    setLoadingCollections(true);
    try {
      const value = await readCollections(sid, new AbortController().signal);
      setCollections(value.items);
    } catch {
      write.fail(pc.failed);
    } finally {
      setLoadingCollections(false);
    }
  }
  async function addCollection() {
    if (!collection || locked) return;
    try {
      const current = await readCollection(
        sid,
        collection,
        new AbortController().signal,
      );
      const ids = [
        ...new Set([...current.products.map((p) => p.product_id), ...selected]),
      ];
      await write.run(
        command("PUT", `collections/${collection}/products`, {
          product_ids: ids,
          expected_version: current.version,
        }),
        parseCreated,
        async () => {
          setResults(selected.map((id) => ({ id, status: "" })));
          setSelected([]);
          setCollections(null);
          await read.refresh();
        },
        pc.success,
      );
    } catch {
      write.fail(pc.failed);
    }
  }
  async function duplicate(row: ProductSummary) {
    if (locked) return;
    await write.run(
      command("POST", `products/${row.id}/copy`, {
        expected_version: row.version,
      }),
      parseCreated,
      async (value) => {
        setResults([{ id: value.id, status: "draft" }]);
        await read.refresh();
      },
      pc.success,
    );
  }
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? l.noStore}
      active="products"
      locked={locked}
    >
      <div
        className="orders-page product-admin pe-catalog"
        data-testid="products-page"
      >
        <AdminPageHeader
          locale={locale}
          description={l.subtitle}
          actions={
            <>
              <Link
                className="product-link"
                href={`/${locale}/inventory${sid ? `?store=${sid}` : ""}`}
                data-testid="products-ledger-link"
              >
                {l.inventory}
              </Link>
              {store && (
                <Link
                  className="product-primary"
                  href={`/${locale}/products/new?store=${sid}`}
                  data-testid="product-new"
                >
                  <Icon name="product" size={18} />
                  {l.newProduct}
                </Link>
              )}
            </>
          }
        />
        <TabStrip
          label={l.status}
          previousLabel={catalogPresentationCopy[locale].previousTabs}
          nextLabel={catalogPresentationCopy[locale].nextTabs}
        >
          {productStatuses.map((s) => (
            <button
              key={s}
              type="button"
              disabled={locked}
              data-testid={`products-tab-${s}`}
              aria-pressed={status === s}
              onClick={() => {
                setSelected([]);
                previous.current = [];
                go(sid, q, s, "");
              }}
            >
              {s === "all" ? l.all : c.status[s]}{" "}
              {page ? (s === "all" ? page.total : page.status_counts[s]) : ""}
            </button>
          ))}
        </TabStrip>
        <form className="orders-controls" role="search" onSubmit={search}>
          <label className="product-search">
            {l.search}
            <input
              type="search"
              data-testid="products-search"
              value={draft}
              maxLength={120}
              autoComplete="off"
              onChange={(e) => setDraft(e.target.value)}
            />
          </label>
          <button
            type="submit"
            data-testid="products-search-submit"
            disabled={!store || read.status === "loading"}
          >
            <Icon name="search" size={18} />
            {l.searchButton}
          </button>
          {(q || (status && status !== "all")) && (
            <button
              type="button"
              onClick={() => {
                setDraft("");
                previous.current = [];
                go(sid, "", "all", "");
              }}
            >
              {l.clear}
            </button>
          )}
        </form>
        {(read.status === "loading" || read.status === "hidden") && (
          <p className="orders-message" role="status">
            {l.loading}
          </p>
        )}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>
              {l.retry}
            </button>
          </div>
        )}
        {read.status === "ready" && page && (
          <>
            {selected.length > 0 && (
              <div className="pe-batch" data-testid="product-batch">
                <strong>
                  {pc.selected} {selected.length} / 100
                </strong>
                <button
                  type="button"
                  disabled={locked}
                  onClick={() => void bulkStatus("active")}
                >
                  {pc.publishSelected}
                </button>
                <button
                  type="button"
                  disabled={locked}
                  onClick={() => void bulkStatus("draft")}
                >
                  {pc.unpublish}
                </button>
                <button
                  type="button"
                  disabled={locked}
                  onClick={() => void bulkStatus("archived")}
                >
                  {pc.archive}
                </button>
                <button
                  type="button"
                  disabled={locked || loadingCollections}
                  onClick={() => void chooseCollection()}
                >
                  {pc.addCollection}
                </button>
                <button
                  type="button"
                  disabled={locked}
                  onClick={() => setSelected([])}
                >
                  {pc.clear}
                </button>
                {collections && (
                  <div>
                    <p>{pc.membership}</p>
                    <select
                      aria-label={pc.collections}
                      value={collection}
                      onChange={(e) => setCollection(e.target.value)}
                    >
                      <option value="">{pc.collections}</option>
                      {collections.map((col) => (
                        <option key={col.id} value={col.id}>
                          {col.title}
                        </option>
                      ))}
                    </select>
                    <button
                      type="button"
                      disabled={locked || !collection}
                      onClick={() => void addCollection()}
                    >
                      {pc.apply}
                    </button>
                  </div>
                )}
              </div>
            )}
            {write.message && (
              <div className="orders-message" role="status">
                <p>{write.message.text}</p>
                {write.message.kind === "uncertain" && write.canRetry && (
                  <button
                    type="button"
                    disabled={write.busy}
                    onClick={() => void write.retry()}
                  >
                    {pc.retry}
                  </button>
                )}
              </div>
            )}
            {results.length > 0 && (
              <section
                className="pe-results"
                data-testid="product-batch-results"
              >
                <h2>{pc.results}</h2>
                <ul>
                  {results.map((result) => (
                    <li key={result.id}>
                      {page.items.find((row) => row.id === result.id)?.name ??
                        result.id}
                      :{" "}
                      {result.error
                        ? result.error in pc
                          ? pc[result.error as keyof typeof pc]
                          : pc.failed
                        : pc.success}
                    </li>
                  ))}
                </ul>
              </section>
            )}
            {rows.length > 0 && (
              <TableFrame
                label={l.title}
                scrollHint={catalogPresentationCopy[locale].tableScroll}
              >
                <table
                  className="orders-table product-table"
                  data-testid="products-table"
                >
                  <colgroup>
                    <col className="pe-col-select" />
                    <col className="pe-col-product" />
                    <col className="pe-col-status" />
                    <col className="pe-col-price" />
                    <col className="pe-col-stock" />
                    <col className="pe-col-updated" />
                    <col className="pe-col-actions" />
                  </colgroup>
                  <thead>
                    <tr>
                      <th>
                        <label className="pe-select">
                          <input
                            type="checkbox"
                            aria-label={pc.all}
                            checked={
                              rows.length > 0 &&
                              rows.every((r) => selected.includes(r.id))
                            }
                            disabled={locked}
                            onChange={(e) =>
                              setSelected(
                                e.target.checked
                                  ? rows.slice(0, 100).map((r) => r.id)
                                  : [],
                              )
                            }
                          />
                        </label>
                      </th>
                      <th>{l.product}</th>
                      <th>{l.status}</th>
                      <th>{l.price}</th>
                      <th>{l.stock}</th>
                      <th>{pc.updated}</th>
                      <th>{l.edit}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((row) => (
                      <Row
                        key={row.id}
                        row={row}
                        locale={locale}
                        store={sid}
                        selected={selected.includes(row.id)}
                        locked={locked || !!quick}
                        toggle={() =>
                          setSelected((now) =>
                            now.includes(row.id)
                              ? now.filter((id) => id !== row.id)
                              : [...now, row.id].slice(0, 100),
                          )
                        }
                        quick={(field) =>
                          setQuick({
                            id: row.id,
                            field,
                            inline: row.sku_count === 1,
                            store: sid,
                          })
                        }
                        inlineField={
                          quick?.inline && quick.id === row.id
                            ? quick.field
                            : undefined
                        }
                        inlineEditor={
                          quick?.inline && quick.id === row.id ? (
                            <ProductQuickEdit
                              key={quick.id + quick.field}
                              inline
                              store={sid}
                              id={quick.id}
                              field={quick.field}
                              locale={locale}
                              boundary={read.boundary}
                              refresh={read.refresh}
                              close={() => setQuick(null)}
                            />
                          ) : null
                        }
                        duplicate={() => void duplicate(row)}
                      />
                    ))}
                  </tbody>
                </table>
              </TableFrame>
            )}
            {rows.length === 0 && (
              <p className="orders-message" role="status" aria-live="polite">
                {q || status !== "all" ? l.emptySearch : l.empty}
              </p>
            )}
            <footer className="orders-pager">
              <span>
                {pc.total}: {page.total}
              </span>
              <div>
                <button
                  type="button"
                  data-testid="products-previous"
                  disabled={!previous.current.length && !after}
                  onClick={() =>
                    go(sid, q, status, previous.current.pop() ?? "")
                  }
                >
                  {l.previous}
                </button>
                <button
                  type="button"
                  data-testid="products-next"
                  disabled={!page.next_cursor}
                  onClick={() => {
                    previous.current.push(after);
                    go(sid, q, status, page.next_cursor);
                  }}
                >
                  {l.next}
                </button>
              </div>
            </footer>
          </>
        )}
      </div>
      {quick &&
        quick.store === sid &&
        read.status === "ready" &&
        !quick.inline && (
          <ProductQuickEdit
            key={sid + read.boundary + quick.id + quick.field}
            store={sid}
            id={quick.id}
            field={quick.field}
            locale={locale}
            boundary={read.boundary}
            refresh={read.refresh}
            close={() => setQuick(null)}
          />
        )}
    </WorkspaceFrame>
  );
}

function Row({
  row,
  locale,
  store,
  selected,
  locked,
  toggle,
  duplicate,
  quick,
  inlineField,
  inlineEditor,
}: {
  row: ProductSummary;
  locale: Locale;
  store: string;
  selected: boolean;
  locked: boolean;
  toggle: () => void;
  duplicate: () => void;
  quick: (field: "price" | "stock") => void;
  inlineField?: "price" | "stock";
  inlineEditor?: ReactNode;
}) {
  const c = catalogCopy[locale];
  const pc = productEditorCopy[locale];
  const l = c.list;
  const price =
    row.price_min_minor === null || row.price_max_minor === null
      ? l.noVariants
      : row.price_min_minor === row.price_max_minor
        ? money(locale, row.currency, row.price_min_minor)
        : `${money(locale, row.currency, row.price_min_minor)} – ${money(locale, row.currency, row.price_max_minor)}`;
  return (
    <tr data-testid={`product-row-${row.id}`}>
      <td className="pe-selection-cell">
        <label className="pe-select">
          <input
            type="checkbox"
            aria-label={`${pc.select}: ${row.name}`}
            checked={selected}
            disabled={locked}
            onChange={toggle}
          />
        </label>
      </td>
      <td className="pe-product-cell" data-label={l.product}>
        <Link
          className="product-cell"
          href={editHref(locale, store, row.id)}
          aria-label={`${l.edit}: ${row.name}`}
        >
          <ProductPhoto
            code=""
            name={row.name}
            demo={false}
            imageSrc={
              row.cover_image_id
                ? imageURL(store, row.id, row.cover_image_id)
                : undefined
            }
          />
          <span className="pe-product-identity">
            <strong title={row.name}>{row.name}</strong>
            <small title={`/${row.slug} · ${l.variants(row.sku_count)}`}>
              /{row.slug} · {l.variants(row.sku_count)}
            </small>
            {row.keyword && <span className="pe-keyword">{row.keyword}</span>}
          </span>
        </Link>
      </td>
      <td className="pe-status-cell" data-label={l.status}>
        <Badge
          className="product-status"
          tone={
            row.status === "active"
              ? "success"
              : row.status === "draft"
                ? "warning"
                : "neutral"
          }
        >
          {c.status[row.status]}
        </Badge>
      </td>
      <td className="pe-price-cell" data-label={l.price}>
        <div className="pe-number-edit">
          <span>{price}</span>
          <button
            type="button"
            className="pe-edit-icon"
            data-testid="quick-price"
            disabled={locked || row.sku_count === 0}
            aria-label={`${pc.editPrice}: ${row.name}`}
            onClick={() => quick("price")}
          >
            <EditPencil />
          </button>
        </div>
        {inlineField === "price" && inlineEditor}
      </td>
      <td className="pe-stock-cell" data-label={l.stock}>
        <div className="pe-number-edit">
          <span>
            {row.sku_count === 0 ? (
              "—"
            ) : !row.inventory_tracked ? (
              row.sku_count === 1 ? (
                "∞"
              ) : (
                pc.mixedTracking
              )
            ) : row.available <= 0 ? (
              <span className="product-out">{l.outOfStock}</span>
            ) : (
              l.units(row.available)
            )}
          </span>
          <button
            type="button"
            className="pe-edit-icon"
            data-testid="quick-stock"
            disabled={locked || row.sku_count === 0}
            aria-label={`${pc.editStock}: ${row.name}`}
            onClick={() => quick("stock")}
          >
            <EditPencil />
          </button>
        </div>
        {inlineField === "stock" && inlineEditor}
      </td>
      <td className="pe-updated-cell" data-label={pc.updated}>
        {displayTime(locale, row.updated_at)}
      </td>
      <td className="pe-actions-cell">
        <div className="pe-row-actions">
          <Link href={editHref(locale, store, row.id)}>{l.edit}</Link>
          <button type="button" disabled={locked} onClick={duplicate}>
            {pc.copy}
          </button>
        </div>
      </td>
    </tr>
  );
}

function EditPencil() {
  return (
    <svg
      aria-hidden="true"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.65"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="m15 5 4 4M4 20l4-1 12-12a2.8 2.8 0 0 0-4-4L4 15l-1 5z" />
    </svg>
  );
}
