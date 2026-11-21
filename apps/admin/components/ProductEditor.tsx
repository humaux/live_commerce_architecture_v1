// Purpose: Loads the selected catalog product and hosts its document editor.
// Depends on: next/link, react, @live-commerce/i18n, @/lib/model, @/lib/customers-client, @/lib/catalog-v2-client, @/lib/catalog-v2-copy, @/lib/product-editor-copy, @/lib/use-product-leave-guard, ./WorkspaceFrame, ./AdminPageHeader, ./ProductDocumentForm, ./CatalogSimplifiedEditor, ./orders.css, ./ProductAdmin.css, ./ProductDocument.css
// Used by: apps/admin/app/[locale]/products/[product]/page.tsx
// Invariants: server product scope; explicit view selection; dirty navigation and uncertain-write fences remain active.
"use client";
// Approved 03: one document form. The old PATCH/ProductVariants writer is retired.
import Link from "next/link";
import { useEffect, useState } from "react";
import { catalogTaskCopy, type Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { get, ReadError, useGuardedRead, type ReadCode } from "@/lib/customers-client";
import { readProduct } from "@/lib/catalog-v2-client";
import { catalogCopy } from "@/lib/catalog-v2-copy";
import { productEditorCopy } from "@/lib/product-editor-copy";
import {
  useProductLeaveGuard,
  type ProductNavigationState,
} from "@/lib/use-product-leave-guard";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { ProductDocumentForm } from "./ProductDocumentForm";
import { CatalogSimplifiedEditor } from "./CatalogSimplifiedEditor";
import "./orders.css";
import "./ProductAdmin.css";
import "./ProductDocument.css";
/** Loads the selected catalog product and hosts its document editor. Loads the product through catalog-v2-client; ProductDocumentForm owns document writes. */
export function ProductEditor({
  locale,
  store,
  productID,
  initialError,
  renderKey,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  productID: string;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = catalogCopy[locale],
    creating = productID === "new";
  // An explicit merchant choice, scoped to this store/session; never inferred from fixtures or locale.
  const preferenceKey = `catalog-advanced:${store?.id ?? ''}`;
  const [advanced, setAdvanced] = useState(false), [preferenceReady, setPreferenceReady] = useState(false);
  useEffect(() => { try { setAdvanced(sessionStorage.getItem(preferenceKey) === 'true'); } catch { setAdvanced(false); } setPreferenceReady(true); }, [preferenceKey]);
  function switchEditor() {
    if (!beforeNavigate()) return;
    const next = !advanced;
    try { sessionStorage.setItem(preferenceKey, String(next)); } catch { /* the choice still works for this visit */ }
    setAdvanced(next);
  }
  const [navigation, setNavigation] = useState<ProductNavigationState>({
    dirty: false,
    locked: false,
  });
  const [leaveBlocked, setLeaveBlocked] = useState(false);
  const beforeNavigate = useProductLeaveGuard(
    navigation,
    productEditorCopy[locale].leave,
    () => setLeaveBlocked(true),
  );
  // The notice belongs to one refused click; it goes away once the lock is released.
  useEffect(() => {
    if (!navigation.locked) setLeaveBlocked(false);
  }, [navigation.locked]);
  const scope = `${renderKey}|${locale}|${store?.id ?? ""}|${productID}`;
  const read = useGuardedRead(
    scope,
    store
      ? creating
        ? (signal) => get("/api/stores", (value) => {
          // The authenticated stores endpoint validates the store DTO and supplies effective permissions.
          const items = (value as { items?: Store[] } | null)?.items;
          if (!Array.isArray(items)) throw new ReadError("unavailable");
          const permissions = items.find((item) => item?.id === store.id)?.permissions;
          if (!["catalog:read", "catalog:write", "inventory:read"].every(
            (permission) => permissions?.includes(permission),
          )) throw new ReadError("forbidden");
          return null;
        }, signal)
        : (signal) => readProduct(store.id, productID, signal)
      : null,
    initialError,
  );
  const [creation, setCreation] = useState<{ scope: string; boundary: string } | null>(null);
  const ready = read.status === "ready";
  const suspended = read.status === "loading" || read.status === "hidden";
  // Keep only this creation's in-memory state across tab concealment. A fresh read must
  // succeed under the same session fence to reveal it; any other outcome discards it.
  const creationBoundary = !creating ? "" : ready ? read.boundary
    : suspended && creation?.scope === scope ? creation.boundary : "";
  if (creationBoundary && ready && (creation?.scope !== scope || creation.boundary !== creationBoundary)) {
    setCreation({ scope, boundary: creationBoundary });
  } else if (!creationBoundary && creation) {
    setCreation(null);
  }
  const failure =
    read.status === "signed-out"
      ? c.list.signedOut
      : read.status === "forbidden"
        ? c.list.forbidden
        : read.status === "not-found"
          ? c.edit.notFound
          : read.status === "unavailable"
            ? c.list.unavailable
            : "";
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.list.noStore}
      active="products"
      locked={navigation.locked || (!creating && !advanced && navigation.dirty)}
      onBeforeNavigate={beforeNavigate}
    >
      <div
        className="orders-page product-admin product-editor pe-page"
        data-testid="product-editor"
      >
        <AdminPageHeader
          locale={locale}
          title={!creating && !advanced && read.data ? `${catalogTaskCopy[locale].editorTitle} · ${catalogTaskCopy[locale][read.data.status]}` : undefined}
          description={advanced && !creating ? read.data?.name : undefined}
          actions={
            <>
            {!creating && <button type="button" data-testid="catalog-advanced-toggle" disabled={navigation.locked} onClick={switchEditor}>
              {advanced ? catalogTaskCopy[locale].standard : catalogTaskCopy[locale].advanced}
            </button>}
            <Link
              className="product-back"
              href={`/${locale}/products${store ? `?store=${store.id}` : ""}`}
              data-testid="product-back"
            >
              {creating || advanced ? c.edit.back : <svg aria-label={c.edit.back} role="img" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="m14 5-7 7 7 7" /></svg>}
            </Link>
            </>
          }
        />
        {leaveBlocked && navigation.locked && (
          <p role="alert" className="pe-leave-blocked" data-testid="product-leave-blocked">
            {productEditorCopy[locale].leaveBlocked}
          </p>
        )}
        {suspended && (
          <p role="status">{c.edit.loading}</p>
        )}
        {failure && (
          <div role="alert">
            <p>{failure}</p>
            <button type="button" onClick={read.reload}>
              {c.list.retry}
            </button>
          </div>
        )}
        {store && preferenceReady && creationBoundary && <ProductDocumentForm
          key={`${scope}:${creationBoundary}`}
          locale={locale}
          store={store}
          mode="create"
          detail={null}
          boundary={creationBoundary}
          visible={ready}
          onNavigationChange={setNavigation}
        />}
        {!creating && ready && store && preferenceReady && (
          !advanced && read.data ? <CatalogSimplifiedEditor
            key={`${store.id}:${productID}:${read.boundary}:simple`}
            locale={locale} store={store} detail={read.data} boundary={read.boundary} onNavigationChange={setNavigation}
          /> : <ProductDocumentForm
            key={`${store.id}:${productID}:${read.boundary}`}
            locale={locale}
            store={store}
            mode="edit"
            detail={read.data ?? null}
            boundary={read.boundary}
            onNavigationChange={setNavigation}
          />
        )}
      </div>
    </WorkspaceFrame>
  );
}
