"use client";

// Product editor page body (/{locale}/products/{id}, and /{locale}/products/new): title, description, visibility status,
// page handle (slug) and search listing (SEO), then the photo manager and the variants block (ProductVariants.tsx).
// BFF: GET /api/stores/{store}/products/{id} (-> Go GET products/{id}, internal/catalog.GetProductDetail),
// POST /api/stores/{store}/products (create, a new product is a draft) and PATCH products/{id} with expected_version
// (-> Go catalog.CreateProduct / PatchProduct, catalog:write). Photos: components/ProductPhoto.tsx (CM3 routes).
// Owns: the form state and the changed-fields-only PATCH. Never decides validity (the server does; the checks here only
// save a round trip) and never shows a state the server did not confirm: after every write the product is re-read.
import { useCallback, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { command, readProduct } from "@/lib/catalog-v2-client";
import { limits, parseCreated, slugPattern, type ProductDetail } from "@/lib/catalog-v2-model";
import { catalogCopy, errorText } from "@/lib/catalog-v2-copy";
import { useWrite } from "@/lib/catalog-v2-write";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { ProductPhotoManager } from "./ProductPhoto";
import { ProductVariants } from "./ProductVariants";
import "./orders.css";
import "./ProductAdmin.css";

const listHref = (locale: Locale, store: string) => `/${locale}/products${store ? `?store=${store}` : ""}`;

export function ProductEditor({
  locale, stores, store, productID, initialError, renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  productID: string;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = catalogCopy[locale];
  const creating = productID === "new";
  const read = useGuardedRead<ProductDetail | null>(
    `${renderKey}|${locale}|${store?.id ?? ""}|${productID}`,
    // Creating needs no read; the guarded read still supplies the session fence the write must carry.
    store ? (creating ? async () => null : (signal) => readProduct(store.id, productID, signal)) : null,
    initialError,
  );
  const failure =
    read.status === "signed-out" ? c.list.signedOut
    : read.status === "forbidden" ? c.list.forbidden
    : read.status === "not-found" ? (store ? c.edit.notFound : c.list.noStore)
    : read.status === "unavailable" ? c.list.unavailable
    : "";
  const sid = store?.id ?? "";
  const detail = read.data;
  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.list.noStore} active="products">
      <div className="orders-page product-admin product-editor" data-testid="product-editor">
        <Link className="product-back" href={listHref(locale, sid)} data-testid="product-back">← {c.edit.back}</Link>
        <header className="orders-heading">
          <h1>{creating ? c.edit.newTitle : (detail?.name ?? c.edit.loading)}</h1>
          {stores.length > 1 && store && <p>{store.name}</p>}
        </header>
        {(read.status === "loading" || read.status === "hidden") && <p className="orders-message" role="status">{c.edit.loading}</p>}
        {failure && (
          <div className="orders-message" role="status">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>{c.list.retry}</button>
          </div>
        )}
        {read.status === "ready" && store && creating && <CreateForm locale={locale} store={store} boundary={read.boundary} />}
        {read.status === "ready" && store && !creating && detail && (
          <>
            <Basics key={detail.id} locale={locale} store={store} detail={detail} boundary={read.boundary} refresh={read.refresh} />
            <section className="product-section">
              <ProductPhotoManager locale={locale} store={store.id} productID={detail.id} productName={detail.name} code="" disabled={false} onChanged={() => void read.refresh()} />
            </section>
            <ProductVariants locale={locale} store={store} detail={detail} boundary={read.boundary} refresh={read.refresh} />
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}

function Messages({ write, locale }: { write: ReturnType<typeof useWrite>; locale: Locale }) {
  const c = catalogCopy[locale].edit;
  const m = write.message;
  if (!m) return null;
  return (
    <div className={`message ${m.kind === "success" ? "success" : "error"}`} role={m.kind === "success" ? "status" : "alert"} data-testid="product-message">
      <span>{m.text}</span>
      {m.kind === "uncertain" && <button type="button" data-testid="product-retry" disabled={write.busy} onClick={() => void write.retry()}>{c.retry}</button>}
      {m.kind !== "success" && <button type="button" onClick={write.dismiss}>{c.dismiss}</button>}
    </div>
  );
}

function Counter({ text, max, c }: { text: string; max: number; c: (n: number, max: number) => string }) {
  return <small className="product-count">{c(Array.from(text).length, max)}</small>;
}

function useCopyWrite(locale: Locale, store: string, boundary: string) {
  const c = catalogCopy[locale];
  const errors = useCallback((code: string) => errorText(c, code), [c]);
  return useWrite(store, boundary, errors, c.edit.uncertain);
}

function CreateForm({ locale, store, boundary }: { locale: Locale; store: Store; boundary: string }) {
  const c = catalogCopy[locale].edit;
  const router = useRouter();
  const write = useCopyWrite(locale, store.id, boundary);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [slug, setSlug] = useState("");
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!name.trim() || Array.from(name).length > limits.name) return write.fail(c.invalidName);
    if (slug && !slugPattern.test(slug)) return write.fail(c.invalidSlug);
    // Status is omitted on purpose: the server default for a new product is draft (contract storefront-v2 A).
    const body: Record<string, string> = { name: name.trim(), description };
    if (slug) body.slug = slug;
    void write.run(command("POST", "products", body), parseCreated, (created) => router.replace(`/${locale}/products/${created.id}?store=${store.id}`), c.saved);
  }
  return (
    <form className="product-section product-form" onSubmit={submit} data-testid="product-create-form">
      <p className="audit-hint">{c.createdHint}</p>
      <label>{c.name}<input data-testid="product-name" value={name} maxLength={limits.name} onChange={(e) => setName(e.target.value)} required /></label>
      <label>{c.description}<textarea data-testid="product-description" value={description} maxLength={limits.description} rows={5} onChange={(e) => setDescription(e.target.value)} /></label>
      <label>{c.slug}<input data-testid="product-slug" value={slug} maxLength={limits.slug} onChange={(e) => setSlug(e.target.value)} aria-describedby="slug-hint" /><small id="slug-hint">{c.slugHint}</small></label>
      <Messages write={write} locale={locale} />
      <button className="product-primary" type="submit" data-testid="product-create" disabled={write.busy}>{c.create}</button>
    </form>
  );
}

function Basics({ locale, store, detail, boundary, refresh }: {
  locale: Locale; store: Store; detail: ProductDetail; boundary: string; refresh: () => Promise<boolean>;
}) {
  const cc = catalogCopy[locale];
  const c = cc.edit;
  const write = useCopyWrite(locale, store.id, boundary);
  const [name, setName] = useState(detail.name);
  const [description, setDescription] = useState(detail.description);
  const [slug, setSlug] = useState(detail.slug);
  const [status, setStatus] = useState(detail.status);
  const [seoTitle, setSeoTitle] = useState(detail.seo_title);
  const [seoDescription, setSeoDescription] = useState(detail.seo_description);
  function submit(event: FormEvent) {
    event.preventDefault();
    if (!name.trim() || Array.from(name).length > limits.name) return write.fail(c.invalidName);
    if (!slugPattern.test(slug) || slug.length > limits.slug) return write.fail(c.invalidSlug);
    // Only changed fields travel (PATCH is partial); expected_version makes a stale screen a 409, never a silent overwrite.
    const body: Record<string, unknown> = { expected_version: detail.version };
    if (name.trim() !== detail.name) body.name = name.trim();
    if (description !== detail.description) body.description = description;
    if (slug !== detail.slug) body.slug = slug;
    if (status !== detail.status) body.status = status;
    if (seoTitle !== detail.seo_title) body.seo_title = seoTitle;
    if (seoDescription !== detail.seo_description) body.seo_description = seoDescription;
    if (Object.keys(body).length === 1) return; // nothing changed: no request, no version bump
    void write.run(command("PATCH", `products/${detail.id}`, body), parseCreated, () => void refresh(), c.saved);
  }
  return (
    <form className="product-section product-form" onSubmit={submit} data-testid="product-form">
      <h2>{c.basics}</h2>
      <label>{c.name}<input data-testid="product-name" value={name} maxLength={limits.name} onChange={(e) => setName(e.target.value)} required /></label>
      <label>{c.description}<textarea data-testid="product-description" value={description} maxLength={limits.description} rows={6} onChange={(e) => setDescription(e.target.value)} /></label>
      <label>{c.slug}<input data-testid="product-slug" value={slug} maxLength={limits.slug} onChange={(e) => setSlug(e.target.value)} aria-describedby="slug-hint" /><small id="slug-hint">{c.slugHint}</small></label>
      <label>
        {c.statusLabel}
        <select data-testid="product-status" value={status} onChange={(e) => setStatus(e.target.value as ProductDetail["status"])}>
          {(["draft", "active", "archived"] as const).map((s) => <option key={s} value={s}>{cc.status[s]}</option>)}
        </select>
        <small data-testid="product-status-help">{cc.statusHelp[status]}</small>
      </label>
      <fieldset className="product-seo">
        <legend>{c.seo}</legend>
        <p className="audit-hint">{c.seoHint}</p>
        <label>{c.seoTitle}<input data-testid="product-seo-title" value={seoTitle} maxLength={limits.seoTitle} onChange={(e) => setSeoTitle(e.target.value)} /><Counter text={seoTitle} max={limits.seoTitle} c={c.count} /></label>
        <label>{c.seoDescription}<textarea data-testid="product-seo-description" value={seoDescription} maxLength={limits.seoDescription} rows={3} onChange={(e) => setSeoDescription(e.target.value)} /><Counter text={seoDescription} max={limits.seoDescription} c={c.count} /></label>
      </fieldset>
      <Messages write={write} locale={locale} />
      <button className="product-primary" type="submit" data-testid="product-save" disabled={write.busy}>{c.save}</button>
    </form>
  );
}
