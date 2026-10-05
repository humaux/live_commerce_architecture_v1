// Purpose: Product create/edit form — the single writer of the product document (details, media, variants, price, stock, live keyword, visibility) and its save flow.
// Depends on: React/Next; use-product-document and catalog-v2/images clients (admin BFF → Go catalog); product-document draft/money helpers; ProductDocumentVariants, ProductBulkFill, ProductReadiness, useProductEditorLayout and product-editor-copy.
// Used by: ProductEditor (routes /[locale]/products/new and /[locale]/products/[product]).
"use client";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { currencySign } from "@/lib/client";
import { readCollections, readWarehouses } from "@/lib/catalog-v2-client";
import {
  fromMinor,
  toMinor,
  type Collection,
  type OptionAxis,
  type ProductDetail,
} from "@/lib/catalog-v2-model";
import { imageURL, listImages } from "@/lib/images-client";
import {
  newRow,
  emptyDraft,
  rowKey,
  syncMatrix,
  draftFromDetail,
  type DraftRow,
  type ProductDraft,
} from "@/lib/product-document";
import { useProductDocument } from "@/lib/use-product-document";
import type { ProductNavigationState } from "@/lib/use-product-leave-guard";
import { productEditorCopy } from "@/lib/product-editor-copy";
import { catalogCopy } from "@/lib/catalog-v2-copy";
import { ProductDocumentMedia, type DraftPhoto } from "./ProductDocumentMedia";
import { ProductPhotoManager } from "./ProductPhoto";
import { ProductDocumentVariants } from "./ProductDocumentVariants";
import { ProductReadiness } from "./ProductReadiness";
import { useProductEditorLayout } from "./useProductEditorLayout";
function initialDraft(detail: ProductDetail | null): ProductDraft {
  if (!detail) return emptyDraft();
  return draftFromDetail(detail);
}
/** Edits one catalog document; save/image writes remain delegated to the existing catalog command clients. */
export function ProductDocumentForm({
  locale,
  store,
  mode,
  detail,
  boundary,
  onNavigationChange,
}: {
  locale: Locale;
  store: Store;
  mode: "create" | "edit";
  detail: ProductDetail | null;
  boundary: string;
  onNavigationChange: (state: ProductNavigationState) => void;
}) {
  const c = productEditorCopy[locale],
    sign = currencySign(store.currency),
    write = useProductDocument(store.id, store.currency, boundary, c, detail);
  const [draft, setDraft] = useState<ProductDraft>(() => initialDraft(detail)),
    [photos, setPhotos] = useState<DraftPhoto[]>([]),
    [collections, setCollections] = useState<Collection[]>([]),
    [warehouses, setWarehouses] = useState<{ id: string; name: string }[]>([]);
  const [referencesReady, setReferencesReady] = useState(false);
  const [collectionQuery, setCollectionQuery] = useState(""),
    [targetStatus, setTargetStatus] = useState<"draft" | "active" | "archived">(
      detail?.status ?? "draft",
    ),
    [section, setSection] = useState("media"),
    [axisError, setAxisError] = useState("");
  const initial = useRef(JSON.stringify(initialDraft(detail))),
    urlPhotos = useRef<DraftPhoto[]>([]),
    rowArchive = useRef<DraftRow[]>([]);
  const disabled =
    !referencesReady ||
    !write.fenceReady ||
    write.recoveryBlocked ||
    write.busy ||
    write.pending ||
    !!write.done;
  const dirty =
    !write.done &&
    (JSON.stringify(draft) !== initial.current ||
      (!!detail && targetStatus !== (write.savedDetail ?? detail).status) ||
      photos.some((p) => !!p.file) ||
      write.pending);
  const singleVariant = draft.rows.length <= 1 && !draft.axes.length;
  const sections = [
    "media",
    "basics",
    ...(singleVariant ? ["pricing" as const] : []),
    "variants",
    "collections",
    "shipping",
    "seo",
  ] as const;
  const { editor, fields, feedback, focus, noteSaveAttempt } =
    useProductEditorLayout(
      sections,
      setSection,
      JSON.stringify([write.message, write.done?.id, write.recoveryBlocked]),
      write.busy,
    );
  useEffect(() => {
    if (write.savedDetail) {
      const fresh = draftFromDetail(write.savedDetail);
      setDraft(fresh);
      setTargetStatus(write.savedDetail.status);
      initial.current = JSON.stringify(fresh);
      rowArchive.current = [];
    }
  }, [write.savedDetail]);
  const refreshPhotos = () => {
    if (detail)
      void listImages(store.id, detail.id).then((r) => {
        if (r.items)
          setPhotos(
            r.items.map((p) => ({
              key: p.id,
              id: p.id,
              url: imageURL(store.id, detail.id, p.id),
            })),
          );
      });
  };
  useEffect(() => {
    urlPhotos.current = photos;
  }, [photos]);
  useEffect(
    () => () => {
      urlPhotos.current.forEach((p) => {
        if (p.file) URL.revokeObjectURL(p.url);
      });
    },
    [],
  );
  useEffect(() => {
    const abort = new AbortController();
    setReferencesReady(false);
    void Promise.all([
      readCollections(store.id, abort.signal),
      readWarehouses(store.id, abort.signal),
    ])
      .then(([col, wh]) => {
        if (!abort.signal.aborted) {
          setCollections(col.items);
          setWarehouses(wh);
          setReferencesReady(true);
        }
      })
      .catch(() => {
        if (!abort.signal.aborted) write.setMessage(c.failed);
      });
    if (detail)
      void listImages(store.id, detail.id, abort.signal).then((result) => {
        if (!abort.signal.aborted && result.items)
          setPhotos(
            result.items.map((p) => ({
              key: p.id,
              id: p.id,
              url: imageURL(store.id, detail.id, p.id),
            })),
          );
      });
    return () => abort.abort();
  }, [store.id, detail, c.failed]);
  useEffect(() => {
    onNavigationChange({
      dirty,
      locked: write.busy || write.pending || write.recoveryBlocked,
    });
    return () => onNavigationChange({ dirty: false, locked: false });
  }, [
    dirty,
    write.busy,
    write.pending,
    write.recoveryBlocked,
    onNavigationChange,
  ]);
  function change(patch: Partial<ProductDraft>) {
    setDraft((now) => ({ ...now, ...patch }));
    write.setMessage("");
  }
  function setAxes(axes: OptionAxis[]) {
    try {
      const rows = syncMatrix(
        axes.filter((a) => a.name.trim() && a.values.length),
        [...draft.rows, ...rowArchive.current],
      );
      rowArchive.current = [...draft.rows, ...rowArchive.current].filter(
        (r) => !rows.some((n) => rowKey(n) === rowKey(r)),
      );
      change({
        axes,
        rows: detail
          ? rows.map((r) => (r.id ? r : { ...r, quantity: r.quantity || "0" }))
          : rows,
      });
      setAxisError("");
    } catch {
      change({ axes });
      setAxisError(c.matrixLimit);
    }
  }
  const setRow = (patch: Partial<DraftRow>) =>
      change({
        rows: draft.rows.map((r, i) => (i === 0 ? { ...r, ...patch } : r)),
      }),
    row = draft.rows[0] ?? newRow([]);
  const requirements = [
    { key: "media", label: c.images, ok: photos.length > 0 },
    { key: "basics", label: c.name, ok: !!draft.name.trim() },
    {
      key: singleVariant ? "pricing" : "variants",
      label: c.price,
      ok:
        draft.rows.length > 0 &&
        draft.rows.every((r) => toMinor(r.price, store.currency) !== null),
    },
    {
      key: singleVariant ? "pricing" : "variants",
      label: c.stock,
      ok:
        draft.rows.length > 0 &&
        draft.rows.every((r) =>
          r.tracked ? /^\d+$/.test(r.quantity) : /^[1-9]\d{0,2}$/.test(r.max),
        ),
    },
  ];
  const save = (publish: boolean, requestedStatus = targetStatus) => {
    if (disabled) return;
    if (axisError) {
      write.setMessage(axisError);
      noteSaveAttempt();
      return;
    }
    if (
      mode === "create" &&
      draft.rows.some((r) => r.tracked && Number(r.quantity) > 0) &&
      warehouses.length !== 1 &&
      !draft.warehouse
    ) {
      write.setMessage(c.chooseWarehouse);
      focus("shipping");
      noteSaveAttempt();
      return;
    }
    if (
      detail &&
      (rowArchive.current.some((r) => r.id) ||
        draft.rows.some((r) => r.id && !r.active)) &&
      !window.confirm(c.archiveRows)
    )
      return;
    if (
      !publish &&
      requestedStatus === "draft" &&
      (write.savedDetail ?? detail)?.status === "active" &&
      !window.confirm(c.unpublishConfirm)
    )
      return;
    void write.save(
      draft,
      photos,
      publish || (mode === "edit" && requestedStatus === "active"),
      mode === "edit" && requestedStatus !== "archived"
        ? requestedStatus
        : undefined,
    );
    noteSaveAttempt();
  };
  return (
    <form
      ref={editor}
      className="pe-document"
      data-testid={mode === "create" ? "product-create-form" : "product-form"}
      onSubmit={(e) => {
        e.preventDefault();
        save(false);
      }}
    >
      <aside className="pe-index">
        <nav aria-label={c.progress}>
          {sections.map((id) => (
            <button
              type="button"
              aria-current={section === id ? "location" : undefined}
              key={id}
              aria-controls={id}
              onClick={() => focus(id)}
            >
              {c[id]}
            </button>
          ))}
        </nav>
        <section>
          <h2>{c.progress}</h2>
          <h3>{c.required}</h3>
          <ProductReadiness c={c} items={requirements} focus={focus} />
          <p>
            {c.missing}: {requirements.filter((r) => !r.ok).length}
          </p>
          <h3>{c.recommended}</h3>
          <ProductReadiness
            c={c}
            focus={focus}
            items={[
              { key: "media", label: c.recommendedImages, ok: photos.length >= 3 },
              { key: "basics", label: c.description, ok: !!draft.description },
              {
                key: "collections",
                label: c.collections,
                ok: !!draft.collections.length,
              },
              {
                key: singleVariant ? "basics" : "variants",
                label: c.keyword,
                ok: draft.rows.some((r) => !!r.keyword),
              },
              { key: "seo", label: c.seo, ok: !!draft.seo_title },
            ]}
          />
        </section>
      </aside>
      <div className="pe-fields" ref={fields} data-testid="product-fields">
        {detail ? (
          <section id="media" className="product-section pe-media">
            <ProductPhotoManager
              locale={locale}
              store={store.id}
              productID={detail.id}
              productName={draft.name}
              code={draft.rows[0]?.code ?? ""}
              disabled={disabled}
              onChanged={refreshPhotos}
            />
          </section>
        ) : (
          <ProductDocumentMedia
            photos={photos}
            setPhotos={setPhotos}
            disabled={disabled}
            c={c}
            fail={write.setMessage}
          />
        )}
        <fieldset disabled={disabled} className="pe-fieldset">
          <section id="basics" className="product-section product-form">
            <h2>{c.basics}</h2>
            {detail && (
              <label>
                {c.visibility}
                <select
                  data-testid="product-status"
                  aria-describedby="product-status-help"
                  value={targetStatus}
                  onChange={(e) =>
                    setTargetStatus(e.target.value as "draft" | "active")
                  }
                >
                  <option value="draft">{c.draftStatus}</option>
                  <option value="active">{c.activeStatus}</option>
                  {targetStatus === "archived" && (
                    <option value="archived" disabled>
                      {c.archive}
                    </option>
                  )}
                </select>
                <span
                  id="product-status-help"
                  data-testid="product-status-help"
                  className="hint"
                >
                  {catalogCopy[locale].statusHelp[targetStatus]}
                </span>
              </label>
            )}
            <label>
              {c.name}
              <input
                data-testid="product-name"
                value={draft.name}
                maxLength={120}
                onChange={(e) => change({ name: e.target.value })}
              />
              <small>{Array.from(draft.name).length} / 120</small>
            </label>
            <label>
              {c.description}
              <textarea
                data-testid="product-description"
                rows={3}
                value={draft.description}
                maxLength={8000}
                onChange={(e) => change({ description: e.target.value })}
              />
            </label>
            {draft.rows.length <= 1 && !draft.axes.length && (
              <label>
                {c.keyword}
                <input
                  data-testid="product-keyword"
                  value={row.keyword}
                  maxLength={16}
                  onChange={(e) =>
                    setRow({ keyword: e.target.value.toUpperCase() })
                  }
                />
                <small>{c.keywordHelp}</small>
              </label>
            )}
          </section>
          {draft.rows.length <= 1 && !draft.axes.length && (
            <section id="pricing" className="product-section product-form">
              <h2>{c.pricing}</h2>
              <div className="pe-two">
                <label>
                  {c.price} ({sign})
                  <input
                    data-testid="product-price"
                    inputMode="decimal"
                    value={row.price}
                    onChange={(e) => setRow({ price: e.target.value })}
                  />
                </label>
                <label>
                  {c.compare} ({sign})
                  <input
                    data-testid="product-compare"
                    inputMode="decimal"
                    value={row.compare}
                    onChange={(e) => setRow({ compare: e.target.value })}
                  />
                </label>
              </div>
              <div className="pe-stock-mode">
                <label className="pe-check">
                  <input
                    type="radio"
                    name="stock-mode"
                    checked={row.tracked}
                    onChange={() => setRow({ tracked: true })}
                  />
                  {c.tracked}
                </label>
                <label className="pe-check">
                  <input
                    data-testid="product-untracked"
                    type="radio"
                    name="stock-mode"
                    checked={!row.tracked}
                    onChange={() => setRow({ tracked: false })}
                  />
                  {c.untracked}
                </label>
              </div>
              <label>
                {row.tracked ? (detail ? c.targetQty : c.quantity) : c.max}
                <input
                  data-testid={row.tracked ? "product-quantity" : "product-max"}
                  disabled={!!detail && row.tracked && !detail.warehouse_id}
                  inputMode="numeric"
                  value={row.tracked ? row.quantity : row.max}
                  onChange={(e) =>
                    setRow(
                      row.tracked
                        ? { quantity: e.target.value }
                        : { max: e.target.value },
                    )
                  }
                />
              </label>
              {detail && !detail.warehouse_id && (
                <p className="pe-hint">
                  {c.warehouse_required}{" "}
                  <Link href={`/${locale}/inventory?store=${store.id}`}>
                    {c.inventoryLink}
                  </Link>
                </p>
              )}
              <details>
                <summary>{c.advanced}</summary>
                <label>
                  {c.code}
                  <input
                    value={row.code}
                    disabled={!!row.id}
                    maxLength={64}
                    placeholder={c.generated}
                    onChange={(e) => setRow({ code: e.target.value })}
                  />
                </label>
              </details>
            </section>
          )}
          <ProductDocumentVariants
            c={c}
            sign={sign}
            axes={draft.axes}
            rows={draft.rows}
            setAxes={setAxes}
            setRows={(rows) => change({ rows })}
            disabled={disabled}
            inventoryDisabled={!!detail && !detail.warehouse_id}
            onInvalidValues={() => setAxisError(c.matrixLimit)}
          />
          {axisError && <p role="alert">{axisError}</p>}
          {rowArchive.current.filter((r) => r.id).length > 0 && (
            <p role="status">
              {c.archiveRows}: {rowArchive.current.filter((r) => r.id).length}
            </p>
          )}
          {detail && draft.axes.length > 0 && !detail.warehouse_id && (
            <p>
              {c.warehouse_required}{" "}
              <Link href={`/${locale}/inventory?store=${store.id}`}>
                {c.inventoryLink}
              </Link>
            </p>
          )}
          <section id="collections" className="product-section product-form">
            <h2>{c.collections}</h2>
            <label>
              {c.searchCollections}
              <input
                type="search"
                value={collectionQuery}
                onChange={(e) => setCollectionQuery(e.target.value)}
              />
            </label>
            <div className="pe-collections">
              {collections
                .filter((col) =>
                  col.title
                    .toLocaleLowerCase()
                    .includes(collectionQuery.toLocaleLowerCase()),
                )
                .map((col) => (
                  <label className="pe-check" key={col.id}>
                    <input
                      type="checkbox"
                      checked={draft.collections.includes(col.id)}
                      onChange={(e) =>
                        change({
                          collections: e.target.checked
                            ? [...draft.collections, col.id]
                            : draft.collections.filter((id) => id !== col.id),
                        })
                      }
                    />
                    {col.title}
                  </label>
                ))}
            </div>
            {!collections.length && <p>{c.noCollections}</p>}
            <Link href={`/${locale}/collections?store=${store.id}`}>
              {c.manageCollections}
            </Link>
          </section>
          <details id="shipping" className="product-section">
            <summary>{c.shipping}</summary>
            <p className="pe-hint">{c.shippingHelp}</p>
            {detail ? (
              draft.rows.map((sku, i) => (
                <div key={sku.id ?? i} className="product-form pe-two">
                  <h3>{sku.values.join(" / ") || sku.code}</h3>
                  {(["weight", "length", "width", "height"] as const).map(
                    (key) => (
                      <label key={key}>
                        {c[key]}
                        <input
                          data-testid={`shipping-${key}-${i}`}
                          inputMode="decimal"
                          value={sku[key] ?? ""}
                          onChange={(e) =>
                            change({
                              rows: draft.rows.map((r, n) =>
                                n === i ? { ...r, [key]: e.target.value } : r,
                              ),
                            })
                          }
                        />
                      </label>
                    ),
                  )}
                </div>
              ))
            ) : (
              <div className="product-form pe-two">
                {(["weight", "length", "width", "height"] as const).map(
                  (key) => (
                    <label key={key}>
                      {c[key]}
                      <input
                        value={draft[key]}
                        inputMode="decimal"
                        onChange={(e) => change({ [key]: e.target.value })}
                      />
                    </label>
                  ),
                )}
                {warehouses.length > 1 && (
                  <label>
                    {c.warehouse}
                    <select
                      data-testid="product-warehouse"
                      value={draft.warehouse}
                      onChange={(e) => change({ warehouse: e.target.value })}
                    >
                      <option value="">{c.chooseWarehouse}</option>
                      {warehouses.map((w) => (
                        <option key={w.id} value={w.id}>
                          {w.name}
                        </option>
                      ))}
                    </select>
                  </label>
                )}
              </div>
            )}
          </details>
          <details id="seo" className="product-section product-seo">
            <summary>{c.seo}</summary>
            <div className="product-form">
              <label>
                {c.slug}
                <input
                  data-testid="product-slug"
                  value={draft.slug}
                  maxLength={80}
                  placeholder={c.generated}
                  onChange={(e) => change({ slug: e.target.value })}
                />
              </label>
              <label>
                {c.seoTitle}
                <input
                  data-testid="product-seo-title"
                  value={draft.seo_title}
                  maxLength={70}
                  onChange={(e) => change({ seo_title: e.target.value })}
                />
                <small className="product-count">
                  {Array.from(draft.seo_title).length} / 70
                </small>
              </label>
              <label>
                {c.seoDescription}
                <textarea
                  rows={3}
                  data-testid="product-seo-description"
                  value={draft.seo_description}
                  maxLength={160}
                  onChange={(e) => change({ seo_description: e.target.value })}
                />
                <small className="product-count">
                  {Array.from(draft.seo_description).length} / 160
                </small>
              </label>
            </div>
          </details>
        </fieldset>
        {(write.message || write.done) && (
          <div className="pe-feedback" ref={feedback} tabIndex={-1}>
            {write.message && (
              <div
                className="orders-message"
                role={write.done ? "status" : "alert"}
                data-testid="product-message"
              >
                <p>{write.message}</p>
                {write.pending && !write.busy && !write.recoveryBlocked && (
                  <button
                    type="button"
                    data-testid="product-retry"
                    onClick={() => void write.retry()}
                  >
                    {c.retry}
                  </button>
                )}
              </div>
            )}
            {write.done && (
              <section
                className="product-section"
                data-testid="product-save-result"
              >
                <h2>{write.done.name}</h2>
                <p>{c.storeUnpublished}</p>
                <div className="pe-actions">
                  <Link href={`/${locale}/settings?store=${store.id}`}>
                    {c.settings}
                  </Link>
                  <Link
                    href={`/${locale}/products/${write.done.id}?store=${store.id}`}
                  >
                    {c.save}
                  </Link>
                  <Link href={`/${locale}/products?store=${store.id}`}>
                    {c.back}
                  </Link>
                  <button
                    type="button"
                    onClick={() => {
                      write.reset();
                      setDraft(emptyDraft());
                      setPhotos([]);
                      photos.forEach((p) => {
                        if (p.file) URL.revokeObjectURL(p.url);
                      });
                      initial.current = JSON.stringify(emptyDraft());
                    }}
                  >
                    {c.another}
                  </button>
                  {draft.rows.some((r) => !!r.keyword) && (
                    <button
                      type="button"
                      onClick={() =>
                        void navigator.clipboard
                          .writeText(
                            draft.rows
                              .filter((r) => r.keyword)
                              .map((r) => r.keyword)
                              .join("\n"),
                          )
                          .then(() => write.setMessage(c.copied))
                          .catch(() => write.setMessage(c.failed))
                      }
                    >
                      {c.copyKeyword}
                    </button>
                  )}
                </div>
              </section>
            )}
          </div>
        )}
      </div>
      <footer className="pe-savebar">
        <span>
          {write.busy ? c.saving : dirty ? c.dirty : write.done ? c.saved : ""}
        </span>
        <div>
          <button
            type="submit"
            data-testid={mode === "create" ? "product-create" : "product-save"}
            disabled={disabled}
          >
            {mode === "create" ? c.saveDraft : c.save}
          </button>
          {(write.savedDetail ?? detail)?.status === "active" && (
            <button
              type="button"
              disabled={disabled}
              data-testid="product-unpublish"
              onClick={() => save(false, "draft")}
            >
              {c.unpublish}
            </button>
          )}
          <button
            className="product-primary"
            type="button"
            data-testid="product-publish"
            disabled={disabled}
            onClick={() => save(true)}
          >
            {c.publish}
          </button>
        </div>
      </footer>
    </form>
  );
}
