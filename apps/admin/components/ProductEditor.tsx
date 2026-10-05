"use client";
// Approved 03: one document form. The old PATCH/ProductVariants writer is retired.
import Link from "next/link";
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import { useGuardedRead, type ReadCode } from "@/lib/customers-client";
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
import "./orders.css";
import "./ProductAdmin.css";
import "./ProductDocument.css";
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
  const [navigation, setNavigation] = useState<ProductNavigationState>({
    dirty: false,
    locked: false,
  });
  const beforeNavigate = useProductLeaveGuard(
    navigation,
    productEditorCopy[locale].leave,
  );
  const read = useGuardedRead(
    `${renderKey}|${locale}|${store?.id ?? ""}|${productID}`,
    store
      ? creating
        ? async () => null
        : (signal) => readProduct(store.id, productID, signal)
      : null,
    initialError,
  );
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
      locked={navigation.locked}
      onBeforeNavigate={beforeNavigate}
    >
      <div
        className="orders-page product-admin product-editor pe-page"
        data-testid="product-editor"
      >
        <AdminPageHeader
          locale={locale}
          description={creating ? undefined : read.data?.name}
          actions={
            <Link
              className="product-back"
              href={`/${locale}/products${store ? `?store=${store.id}` : ""}`}
              data-testid="product-back"
            >
              {c.edit.back}
            </Link>
          }
        />
        {(read.status === "loading" || read.status === "hidden") && (
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
        {read.status === "ready" && store && (
          <ProductDocumentForm
            key={`${store.id}:${productID}:${read.boundary}`}
            locale={locale}
            store={store}
            mode={creating ? "create" : "edit"}
            detail={read.data ?? null}
            boundary={read.boundary}
            onNavigationChange={setNavigation}
          />
        )}
      </div>
    </WorkspaceFrame>
  );
}
