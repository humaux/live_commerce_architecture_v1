"use client";

// Variants block of the product editor: option axes, the SKU matrix they generate, per-variant price / compare-at /
// SKU code / archive, and stock adjustment (unit catalog-core, contracts/storefront-v2.md A). BFF -> Go (all catalog:write
// unless noted): PATCH products/{id} {options} (catalog.PatchProduct), POST skus (CreateSKU, option_values aligned to the
// axes), POST skus/{id}/price {price_minor, compare_at_minor|null, expected_version} (SetSKUPrice), PATCH skus/{id} (code
// rename: UpdateSKU is a full replace, so every other field is sent back unchanged), POST skus/{id}/archive, and stock:
// GET warehouses, GET catalog-ledger (balance version) then POST inventory/adjustments (inventory:write, ledger writer).
// Owns: the axis draft state, the matrix of missing combinations and one idempotency key per row action. Never decides
// alignment, uniqueness or compare-at > price: the server answers 409/422 and the screen shows that, not a guess.
import { useCallback, useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { command, readStockVersion, readWarehouses, send } from "@/lib/catalog-v2-client";
import {
  cleanAxes, codePattern, fromMinor, limits, parseCreated, sameValues, toMinor, variantMatrix, variantTitle,
  type OptionAxis, type ProductDetail, type Variant,
} from "@/lib/catalog-v2-model";
import { catalogCopy, errorText } from "@/lib/catalog-v2-copy";
import { useWrite } from "@/lib/catalog-v2-write";

type Props = { locale: Locale; store: Store; detail: ProductDetail; boundary: string; refresh: () => Promise<boolean> };
type AxisDraft = { name: string; values: string };
const toDraft = (axes: OptionAxis[]): AxisDraft[] => axes.map((a) => ({ name: a.name, values: a.values.join(", ") }));
const fromDraft = (draft: AxisDraft[]): OptionAxis[] => draft.map((d) => ({ name: d.name, values: d.values.split(",") }));

export function ProductVariants({ locale, store, detail, boundary, refresh }: Props) {
  const cc = catalogCopy[locale];
  const c = cc.edit;
  const errors = useCallback((code: string) => errorText(cc, code), [cc]);
  const write = useWrite(store.id, boundary, errors, c.uncertain);
  const [draft, setDraft] = useState<AxisDraft[]>(() => toDraft(detail.options));
  const [axisError, setAxisError] = useState("");
  const axesChanged = JSON.stringify(cleanAxes(fromDraft(draft)).axes) !== JSON.stringify(detail.options);

  function saveAxes() {
    const cleaned = cleanAxes(fromDraft(draft));
    if (cleaned.error) return setAxisError(cleaned.error === "duplicate" ? c.optionsDuplicate : c.optionsLimit);
    setAxisError("");
    void write.run(command("PATCH", `products/${detail.id}`, { expected_version: detail.version, options: cleaned.axes }), parseCreated,
      () => void refresh(), c.saved, { conflict: c.optionsLocked });
  }

  const missing = variantMatrix(detail.options).filter((row) => !detail.skus.some((v) => sameValues(v.option_values, row)));
  const full = detail.skus.length >= limits.variants;
  return (
    <section className="product-section" data-testid="product-variants">
      <h2>{c.options}</h2>
      <p className="audit-hint">{c.optionsHint}</p>
      <div className="product-axes">
        {draft.map((axis, i) => (
          <div className="product-axis" key={i}>
            <label>{c.optionName}<input data-testid={`axis-name-${i}`} value={axis.name} maxLength={limits.axisName}
              onChange={(e) => setDraft(draft.map((d, j) => (j === i ? { ...d, name: e.target.value } : d)))} /></label>
            <label>{c.optionValues}<input data-testid={`axis-values-${i}`} value={axis.values}
              onChange={(e) => setDraft(draft.map((d, j) => (j === i ? { ...d, values: e.target.value } : d)))} /></label>
            <button type="button" onClick={() => setDraft(draft.filter((_, j) => j !== i))}>{c.removeOption}</button>
          </div>
        ))}
      </div>
      <div className="product-row-actions">
        <button type="button" data-testid="axis-add" disabled={draft.length >= limits.axes} onClick={() => setDraft([...draft, { name: "", values: "" }])}>{c.addOption}</button>
        <button type="button" className="product-primary" data-testid="axis-save" disabled={!axesChanged || write.busy} onClick={saveAxes}>{c.saveOptions}</button>
      </div>
      {axisError && <p className="message error" role="alert">{axisError}</p>}

      <h2>{c.variants}</h2>
      {write.message && (
        <div className={`message ${write.message.kind === "success" ? "success" : "error"}`} role={write.message.kind === "success" ? "status" : "alert"} data-testid="variants-message">
          <span>{write.message.text}</span>
          {write.message.kind === "uncertain" && <button type="button" disabled={write.busy} onClick={() => void write.retry()}>{c.retry}</button>}
          {write.message.kind !== "success" && <button type="button" onClick={write.dismiss}>{c.dismiss}</button>}
        </div>
      )}
      {detail.skus.length === 0 && missing.length === 0 && detail.options.length > 0 && <p>{c.noVariants}</p>}
      {detail.skus.length > 0 && (
        <div className="orders-table-scroll">
          <table className="orders-table product-variant-table" data-testid="variant-table">
            <thead><tr><th>{c.variant}</th><th>{c.code}</th><th>{c.price}</th><th>{c.compareAt}</th><th>{c.stock}</th></tr></thead>
            <tbody>
              {detail.skus.map((v) => (
                <VariantRow key={`${v.id}:${v.version}`} v={v} locale={locale} store={store} detail={detail} boundary={boundary} refresh={refresh} write={write} />
              ))}
            </tbody>
          </table>
        </div>
      )}
      {full && <p className="audit-hint">{c.variantLimit}</p>}
      {!full && (detail.options.length === 0 ? detail.skus.length === 0 : missing.length > 0) && (
        <NewVariants key={`${detail.version}:${detail.skus.length}`} locale={locale} store={store} detail={detail} rows={detail.options.length === 0 ? [[]] : missing.slice(0, limits.variants - detail.skus.length)} boundary={boundary} refresh={refresh} />
      )}
    </section>
  );
}

// One existing variant: price + compare-at in one command (price route), SKU code rename and archive each their own.
function VariantRow({ v, locale, store, detail, boundary, refresh, write }: {
  v: Variant; locale: Locale; store: Store; detail: ProductDetail; boundary: string; refresh: () => Promise<boolean>; write: ReturnType<typeof useWrite>;
}) {
  const c = catalogCopy[locale].edit;
  const [price, setPrice] = useState(fromMinor(v.price_minor, v.currency));
  const [compare, setCompare] = useState(v.compare_at_minor === null ? "" : fromMinor(v.compare_at_minor, v.currency));
  const [code, setCode] = useState(v.code);
  const [renaming, setRenaming] = useState(false);
  const [adjusting, setAdjusting] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const dirty = price !== fromMinor(v.price_minor, v.currency) || compare !== (v.compare_at_minor === null ? "" : fromMinor(v.compare_at_minor, v.currency));

  function savePrice() {
    const p = toMinor(price, v.currency);
    if (p === null) return write.fail(c.invalidPrice);
    const cmp = compare.trim() === "" ? null : toMinor(compare, v.currency);
    if (compare.trim() !== "" && (cmp === null || cmp <= p)) return write.fail(cmp === null ? c.invalidPrice : c.invalidCompare);
    void write.run(command("POST", `skus/${v.id}/price`, { price_minor: p, compare_at_minor: cmp, expected_version: v.version }), parseCreated, () => void refresh(), c.saved);
  }
  function saveCode() {
    if (!codePattern.test(code)) return write.fail(c.invalidCode);
    // PATCH skus/{id} replaces every field: send the logistics/customs fields and compare-at back exactly as read.
    const body = {
      product_id: detail.id, code, price_minor: v.price_minor, weight_grams: v.weight_grams, length_mm: v.length_mm, width_mm: v.width_mm,
      height_mm: v.height_mm, origin_country: v.origin_country, customs_name: v.customs_name, hs_candidate: v.hs_candidate,
      ...(v.compare_at_minor === null ? {} : { compare_at_minor: v.compare_at_minor }), expected_version: v.version,
    };
    void write.run(command("PATCH", `skus/${v.id}`, body), parseCreated, () => { setRenaming(false); void refresh(); }, c.saved);
  }
  return (
    <>
      <tr data-testid={`variant-row-${v.id}`}>
        <td data-label={c.variant}>{v.title}</td>
        <td data-label={c.code}>
          {renaming ? (
            <span className="product-inline">
              <input aria-label={c.code} data-testid={`variant-code-${v.id}`} value={code} maxLength={limits.code} onChange={(e) => setCode(e.target.value)} />
              <button type="button" disabled={write.busy} onClick={saveCode}>{c.saveVariant}</button>
            </span>
          ) : (
            <button type="button" className="product-textbutton" data-testid={`variant-rename-${v.id}`} onClick={() => setRenaming(true)}>{v.code}</button>
          )}
        </td>
        <td data-label={c.price}><input aria-label={c.price} inputMode="decimal" data-testid={`variant-price-${v.id}`} value={price} onChange={(e) => setPrice(e.target.value)} /> <small>{v.currency}</small></td>
        <td data-label={c.compareAt}><input aria-label={c.compareAt} inputMode="decimal" data-testid={`variant-compare-${v.id}`} value={compare} onChange={(e) => setCompare(e.target.value)} />
          <button type="button" data-testid={`variant-save-${v.id}`} disabled={!dirty || write.busy} onClick={savePrice}>{c.saveVariant}</button></td>
        <td data-label={c.stock}>
          <span data-testid={`variant-stock-${v.id}`}>{c.available(v.available)}</span>
          <button type="button" data-testid={`variant-adjust-${v.id}`} onClick={() => setAdjusting(!adjusting)}>{c.adjust}</button>
          {confirming ? (
            <button type="button" className="danger" disabled={write.busy} onClick={() => void write.run(command("POST", `skus/${v.id}/archive`, { expected_version: v.version }), parseCreated, () => void refresh(), c.saved)}>{c.confirmArchive}</button>
          ) : (
            <button type="button" className="danger" data-testid={`variant-archive-${v.id}`} onClick={() => setConfirming(true)}>{c.archiveVariant}</button>
          )}
        </td>
      </tr>
      {adjusting && (
        <tr><td colSpan={5}><AdjustStock v={v} locale={locale} store={store} boundary={boundary} refresh={refresh} write={write} close={() => setAdjusting(false)} /></td></tr>
      )}
    </>
  );
}

function AdjustStock({ v, locale, store, boundary, refresh, write, close }: {
  v: Variant; locale: Locale; store: Store; boundary: string; refresh: () => Promise<boolean>; write: ReturnType<typeof useWrite>; close: () => void;
}) {
  const c = catalogCopy[locale].edit;
  const [warehouses, setWarehouses] = useState<{ id: string; name: string }[] | null>(null);
  const [warehouse, setWarehouse] = useState("");
  const [delta, setDelta] = useState("");
  const [reason, setReason] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    readWarehouses(store.id, controller.signal).then((list) => { setWarehouses(list); setWarehouse(list[0]?.id ?? ""); }, () => setWarehouses([]));
    return () => controller.abort();
  }, [store.id]);
  async function apply() {
    const n = Number(delta);
    if (!/^[+-]?\d{1,9}$/.test(delta.trim()) || n === 0) return write.fail(c.invalidDelta);
    if (!reason.trim() || !warehouse) return write.fail(catalogCopy[locale].errors.invalid_request);
    // The adjust route takes the balance version as expected_version (0 = no balance yet); read it now, then send ONE command.
    let version: number;
    try {
      version = await readStockVersion(store.id, warehouse, v.code, v.id, AbortSignal.timeout(8000));
    } catch {
      return write.fail(catalogCopy[locale].errors.retry_later);
    }
    const ok = await write.run(command("POST", "inventory/adjustments", { warehouse_id: warehouse, sku_id: v.id, delta: n, expected_version: version, reason: reason.trim() }),
      parseCreated, () => void refresh(), c.stockAdjusted);
    if (ok) close();
  }
  return (
    <div className="product-adjust" data-testid={`adjust-form-${v.id}`}>
      {warehouses && warehouses.length > 1 && (
        <label>{c.warehouse}<select value={warehouse} onChange={(e) => setWarehouse(e.target.value)}>{warehouses.map((w) => <option key={w.id} value={w.id}>{w.name}</option>)}</select></label>
      )}
      <label>{c.adjustDelta}<input inputMode="numeric" data-testid="adjust-delta" value={delta} onChange={(e) => setDelta(e.target.value)} /></label>
      <label>{c.adjustReason}<input data-testid="adjust-reason" value={reason} maxLength={240} onChange={(e) => setReason(e.target.value)} /></label>
      <button type="button" className="product-primary" data-testid="adjust-apply" disabled={write.busy || !warehouse} onClick={() => void apply()}>{c.apply}</button>
      <button type="button" onClick={close}>{c.cancel}</button>
    </div>
  );
}

type NewRow = { values: string[]; code: string; price: string; key: string };
const safeCode = (slug: string, values: string[]) =>
  [slug, ...values].join("-").replace(/[^A-Za-z0-9_.-]+/g, "-").replace(/-+/g, "-").replace(/^-|-$/g, "").slice(0, limits.code) || "SKU";

// Rows for option combinations (or the single default variant) that have no SKU yet. Each row owns one idempotency key,
// regenerated whenever the row is edited (same key with different bytes would be a 409); rows are created in order and
// the run stops at the first failure so the screen shows exactly which rows exist after the re-read.
function NewVariants({ locale, store, detail, rows, boundary, refresh }: {
  locale: Locale; store: Store; detail: ProductDetail; rows: string[][]; boundary: string; refresh: () => Promise<boolean>;
}) {
  const cc = catalogCopy[locale];
  const c = cc.edit;
  const [list, setList] = useState<NewRow[]>(() => rows.map((values) => ({ values, code: safeCode(detail.slug, values), price: "", key: crypto.randomUUID() })));
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const running = useRef(false);
  const edit = (i: number, patch: Partial<NewRow>) => setList((rowsNow) => rowsNow.map((r, j) => (j === i ? { ...r, ...patch, key: crypto.randomUUID() } : r)));

  async function create() {
    if (running.current) return;
    for (const r of list) {
      if (toMinor(r.price, store.currency) === null) return setMessage(c.invalidPrice);
      if (!codePattern.test(r.code)) return setMessage(c.invalidCode);
    }
    running.current = true;
    setBusy(true);
    setMessage("");
    let failed = "";
    for (const r of list) {
      const body = { product_id: detail.id, code: r.code, price_minor: toMinor(r.price, store.currency), option_values: r.values };
      const outcome = await send(store.id, { key: r.key, method: "POST", resource: "skus", body: JSON.stringify(body) }, boundary, parseCreated);
      if (!outcome.ok) {
        failed = outcome.uncertain ? c.uncertain : errorText(cc, outcome.code);
        break;
      }
    }
    running.current = false;
    setBusy(false);
    if (failed) setMessage(failed);
    await refresh();
  }
  return (
    <div className="product-new-variants" data-testid="new-variants">
      <h3>{detail.options.length === 0 ? c.defaultVariant : c.newVariants}</h3>
      <p className="audit-hint">{detail.options.length === 0 ? c.defaultVariantHint : c.newVariantsHint}</p>
      <div className="orders-table-scroll">
        <table className="orders-table product-variant-table">
          <thead><tr><th>{c.variant}</th><th>{c.code}</th><th>{c.price}</th></tr></thead>
          <tbody>
            {list.map((r, i) => (
              <tr key={r.values.join("\u0000")} data-testid={`new-variant-${i}`}>
                <td data-label={c.variant}>{variantTitle(r.values)}</td>
                <td data-label={c.code}><input aria-label={c.code} data-testid={`new-code-${i}`} value={r.code} maxLength={limits.code} onChange={(e) => edit(i, { code: e.target.value })} /></td>
                <td data-label={c.price}><input aria-label={c.price} inputMode="decimal" data-testid={`new-price-${i}`} value={r.price} onChange={(e) => edit(i, { price: e.target.value })} /> <small>{store.currency}</small></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {message && <p className="message error" role="alert" data-testid="new-variants-message">{message}</p>}
      <button type="button" className="product-primary" data-testid="new-variants-create" disabled={busy} onClick={() => void create()}>{detail.options.length === 0 ? c.createDefault : c.createVariants}</button>
    </div>
  );
}
