// Purpose: Owns the merchant inventory ledger page and product inspection.
// Depends on: React/Next, InventoryView, scoped catalog balances, command journal, purchase-entry read, ProductPhoto and WorkspaceFrame.
// Used by: apps/admin/app/[locale]/inventory/page.tsx
// Invariants: server balances/version authorize writes; durable journals retain the original key and bytes.
"use client";

// Stock ledger (/[locale]/inventory). BFF routes used (all -> Go internal/httpapi, scope from the merchant session):
//   GET catalog-ledger (server page), GET products/{id}/purchase-entry, POST inventory/adjustments (journaled command with the
//   row's own expected_version). Product editing (name, price, photos, archive, creation) lives only on /[locale]/products
//   (product-editor §c9); this tray keeps stock info, the explicit stock adjustment, a read-only price and a link there.
//   Never trusts a client tenant/store id.

import { InventoryView, type InventoryState } from "./inventory/InventoryView";
import "./imports/data-ops.css";
import { useEffect, useRef, useState, type FormEvent } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { type Locale } from "@live-commerce/i18n";
import { Badge } from "@live-commerce/ui";
import { copy, type Copy } from "@/lib/copy";
import type { APIError, PurchaseEntry, WorkspaceData } from "@/lib/model";
import { money, sendCommand, validJournalCommand, type PendingCommand } from "@/lib/client";
import { imageURL } from "@/lib/images-client";
import { ProductPhoto } from "./ProductPhoto";
import { Icon } from "./Icon";
import { WorkspaceFrame } from "./WorkspaceFrame";

function errorText(error: APIError, c: Copy) {
  if (error.code === "unauthorized") return c.noSession;
  if (error.code === "forbidden") return c.forbidden;
  if (error.code === "command_storage_unavailable") return c.storageUnavailable;
  if (error.code === "conflict" || error.code === "insufficient_inventory")
    return c.conflict;
  if (error.code === "invalid_request" || error.code === "invalid_json")
    return c.validation;
  return c.failed;
}

function purchaseEntryValid(
  value: unknown,
  productID: string,
  locale: Locale,
): value is PurchaseEntry {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const row = value as Record<string, unknown>;
  if (
    Object.keys(row).sort().join(",") !== "locale,product_id,state,url" ||
    row.product_id !== productID ||
    row.locale !== locale ||
    typeof row.url !== "string"
  )
    return false;
  if (row.state !== "configured")
    return (
      row.url === "" &&
      [
        "product_inactive",
        "no_active_sku",
        "storefront_unavailable",
        "domain_selection_required",
      ].includes(String(row.state))
    );
  try {
    const url = new URL(row.url);
    return (
      url.protocol === "https:" &&
      !!url.hostname &&
      !url.port &&
      !url.username &&
      !url.password &&
      !url.search &&
      !url.hash &&
      url.pathname === `/${locale}/products/${productID}` &&
      url.toString() === row.url
    );
  } catch {
    return false;
  }
}

async function readPurchaseEntry(
  store: string,
  product: string,
  locale: Locale,
  signal: AbortSignal,
) {
  const response = await fetch(
    `/api/stores/${store}/products/${product}/purchase-entry?locale=${locale}`,
    { cache: "no-store", signal },
  );
  if (!response.ok) throw response.status;
  const value: unknown = await response.json();
  if (!purchaseEntryValid(value, product, locale)) throw 503;
  return value;
}

function purchaseReadError(error: unknown, c: Copy) {
  if (error === 401) return c.noSession;
  if (error === 403) return c.forbidden;
  if (error === 404) return c.purchaseNotFound;
  return c.purchaseFailed;
}

/** Owns the merchant inventory ledger page and product inspection. Reads the workspace through the BFF and submits journal commands through lib/client. */
export function Ledger({
  locale,
  initial,
}: {
  locale: Locale;
  initial: WorkspaceData;
}) {
  const [view, setView] = useState<InventoryState>("list");
  const before = useRef<WorkspaceData["rows"]["items"][number] | undefined>(undefined);
  const c = copy[locale],
    router = useRouter(),
    pathname = usePathname(),
    search = useSearchParams();
  const searchQuery = search.get("q") ?? "";
  const [selectedID, select] = useState(
    initial.fixture
      ? (initial.rows.items[1]?.sku_id ?? initial.rows.items[0]?.sku_id ?? "")
      : "",
  );
  const selected = initial.rows.items.find((row) => row.sku_id === selectedID);
  const [entryRefresh, setEntryRefresh] = useState(0);
  const entryProductID = selected?.product_id ?? "";
  const entryScope = `${initial.storeID}:${locale}:${selectedID}:${entryProductID}`;
  const currentEntryScope = useRef(entryScope);
  currentEntryScope.current = entryScope;
  const entryEpoch = useRef(0);
  const entryController = useRef<AbortController | null>(null);
  const [entry, setEntry] = useState<PurchaseEntry | null>(null);
  const [entryLoading, setEntryLoading] = useState(false);
  const [entryError, setEntryError] = useState("");
  const [entryFeedback, setEntryFeedback] = useState("");
  const [query, setQuery] = useState(searchQuery);
  // catalog-core: the home ledger is the Inventory view; Products and Collections are their own pages (WorkspaceFrame).
  const [pending, setPending] = useState<PendingCommand | null>(null),
    [busy, setBusy] = useState(false);
  const [journalReady, setJournalReady] = useState(false);
  const [notice, setNotice] = useState(""),
    [error, setError] = useState<APIError | null>(null);
  const [delta, setDelta] = useState("0"),
    [reason, setReason] = useState("");
  const [warehouseChoices, setWarehouseChoices] = useState(initial.warehouses);
  const [warehouseCursor, setWarehouseCursor] = useState(
    initial.warehouseCursor,
  );
  const [warehouseLoading, setWarehouseLoading] = useState(false);
  const locked = busy || pending !== null || !journalReady;
  const key = `commerce-pending:${initial.storeID}`;

  // A server refresh may replace the search object without changing its query.
  // Preserve text being typed while that refresh completes.
  useEffect(() => {
    setQuery(searchQuery);
  }, [searchQuery]);
  useEffect(() => {
    setWarehouseChoices(initial.warehouses);
    setWarehouseCursor(initial.warehouseCursor);
  }, [initial]);
  useEffect(() => {
    // No credentials are persisted. This tiny command journal survives reloads
    // so a browser failure cannot turn an uncertain write into a duplicate.
    try {
      const saved = localStorage.getItem(key);
      if (saved) {
        const cmd: unknown = JSON.parse(saved);
        // Known kind/resource pairs only (lib/client.ts). A write journaled by an older build (product, sku, edit, price, archive)
        // is still replayed with its original key and bytes so an uncertain write can never be duplicated.
        if (!validJournalCommand(cmd)) throw new Error("Invalid journal");
        setPending(cmd);
        setView("unknown");
      }
      // The inline "quick add" that used this key is gone (product-editor §c9): drop the leftover half-created marker.
      localStorage.removeItem(`${key}:product`);
      if (!navigator.locks) throw new Error("Cross-tab lock unavailable");
      setJournalReady(true);
    } catch {
      setError({
        code: "command_storage_unavailable",
        message: "",
        request_id: "",
        retryable: false,
        details: {},
      });
    }
  }, [key]);
  useEffect(() => {
    if (!locked) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [locked]);

  useEffect(() => {
    const epoch = ++entryEpoch.current;
    entryController.current?.abort();
    setEntry(null);
    setEntryError("");
    setEntryFeedback("");
    if (!entryProductID || !initial.storeID) {
      setEntryLoading(false);
      return;
    }
    const controller = new AbortController();
    entryController.current = controller;
    const timeout = setTimeout(() => controller.abort(), 8000);
    setEntryLoading(true);
    void readPurchaseEntry(
      initial.storeID,
      entryProductID,
      locale,
      controller.signal,
    )
      .then((value) => {
        if (
          epoch === entryEpoch.current &&
          currentEntryScope.current === entryScope
        )
          setEntry(value);
      })
      .catch((failure: unknown) => {
        if (
          epoch === entryEpoch.current &&
          currentEntryScope.current === entryScope
        )
          setEntryError(purchaseReadError(failure, c));
      })
      .finally(() => {
        clearTimeout(timeout);
        if (epoch === entryEpoch.current) setEntryLoading(false);
      });
    return () => {
      controller.abort();
      clearTimeout(timeout);
      if (entryController.current === controller)
        entryController.current = null;
      ++entryEpoch.current;
    };
  }, [
    entryScope,
    entryRefresh,
    initial.rows,
    c,
    entryProductID,
    initial.storeID,
    locale,
  ]);

  async function usePurchaseEntry(action: "copy" | "open") {
    if (!entryProductID || !initial.storeID || entryLoading || locked) return;
    const epoch = ++entryEpoch.current;
    const scope = entryScope;
    entryController.current?.abort();
    const controller = new AbortController();
    entryController.current = controller;
    const timeout = setTimeout(() => controller.abort(), 8000);
    setEntryLoading(true);
    setEntry(null);
    setEntryError("");
    setEntryFeedback("");
    try {
      const fresh = await readPurchaseEntry(
        initial.storeID,
        entryProductID,
        locale,
        controller.signal,
      );
      if (epoch !== entryEpoch.current || currentEntryScope.current !== scope)
        return;
      setEntry(fresh);
      if (fresh.state !== "configured") return;
      if (action === "open") {
        window.location.assign(fresh.url);
      } else {
        try {
          await navigator.clipboard.writeText(fresh.url);
          if (
            epoch === entryEpoch.current &&
            currentEntryScope.current === scope
          )
            setEntryFeedback(c.purchaseCopied);
        } catch {
          if (
            epoch === entryEpoch.current &&
            currentEntryScope.current === scope
          )
            setEntryFeedback(c.purchaseCopyFailed);
        }
      }
    } catch (failure) {
      if (epoch === entryEpoch.current && currentEntryScope.current === scope)
        setEntryError(purchaseReadError(failure, c));
    } finally {
      clearTimeout(timeout);
      if (entryController.current === controller)
        entryController.current = null;
      if (epoch === entryEpoch.current) setEntryLoading(false);
    }
  }

  function navigate(values: Record<string, string>, resetCursor = true) {
    if (locked) return;
    const params = new URLSearchParams(search.toString());
    if (resetCursor) params.delete("cursor");
    for (const [name, value] of Object.entries(values)) {
      if (value) params.set(name, value);
      else params.delete(name);
    }
    select("");
    router.push(`${pathname}?${params}`);
  }
  async function submit(command: PendingCommand) {
    if (busy || !journalReady) return;
    setBusy(true);
    setNotice("");
    setError(null);
    try {
      await navigator.locks.request(key, async () => {
        const existing = localStorage.getItem(key);
        if (existing) {
          const prior = JSON.parse(existing) as PendingCommand;
          if (prior.key !== command.key) {
            setPending(prior);
            setView("unknown");
            return;
          }
          // The persisted bytes, not editable UI fields, are authoritative on retry.
          command = prior;
        }
        const encoded = JSON.stringify(command);
        localStorage.setItem(key, encoded);
        if (localStorage.getItem(key) !== encoded)
          throw new Error("Journal not durable");
        setPending(command);
        const result = await sendCommand(initial.storeID, command);
        if (result.uncertain) {
          setError(result.body);
          setView("unknown");
          return;
        }
        if (result.status >= 400) {
          localStorage.removeItem(key);
          setPending(null);
          setError(result.body);
          setView(result.body.code === "conflict" || result.body.code === "insufficient_inventory" ? "conflict" : "review");
          return;
        }
        setView("result");
        setNotice(c.success);
        localStorage.removeItem(key);
        setPending(null);
        router.refresh();
      });
    } catch {
      setError({
        code: "command_storage_unavailable",
        message: "",
        request_id: "",
        retryable: false,
        details: {},
      });
    } finally {
      setBusy(false);
    }
  }
  function command(
    kind: PendingCommand["kind"],
    resource: string,
    payload: Record<string, unknown>,
  ) {
    if (locked) return;
    void submit({
      kind,
      resource,
      body: JSON.stringify(payload),
      key: crypto.randomUUID(),
    });
  }
  function adjust(event: FormEvent) {
    event.preventDefault();
    const quantity = Number(delta);
    if (
      !selected ||
      !Number.isSafeInteger(quantity) ||
      quantity === 0 ||
      !reason.trim()
    ) {
      setError({
        code: "invalid_request",
        message: "",
        request_id: "",
        retryable: false,
        details: {},
      });
      return;
    }
    before.current = selected;
    command("adjust", "inventory/adjustments", {
      warehouse_id: selected.warehouse_id,
      sku_id: selected.sku_id,
      expected_version: selected.balance_version,
      delta: quantity,
      reason: reason.trim(),
    });
  }
  async function moreWarehouses() {
    setWarehouseLoading(true);
    try {
      const response = await fetch(
        `/api/stores/${initial.storeID}/warehouses?cursor=${encodeURIComponent(warehouseCursor)}`,
      );
      const result = await response.json();
      if (!response.ok) setError(result);
      else {
        setWarehouseChoices((current) => [...current, ...result.items]);
        setWarehouseCursor(result.next_cursor);
      }
    } catch {
      setError({
        code: "retry_later",
        message: "",
        request_id: "",
        retryable: true,
        details: {},
      });
    } finally {
      setWarehouseLoading(false);
    }
  }
  return <WorkspaceFrame locale={locale} storeName={initial.storeName || (initial.fixture ? c.store : c.notConnected)} active="inventory" locked={locked}>
    <InventoryView locale={locale} initial={initial} state={view} setState={setView}
      selected={view === "result" ? before.current ?? selected : selected}
      select={select} delta={delta} setDelta={setDelta} reason={reason} setReason={setReason}
      busy={busy} locked={locked} query={query} setQuery={setQuery} navigate={navigate}
      submit={() => adjust({ preventDefault() {} } as FormEvent)} retry={() => { if (pending) void submit(pending); }}
      reload={() => { setError(null); setView("edit"); setReason(""); router.refresh(); }}
      error={initial.error || error ? errorText(error ?? initial.error!, c) : ""}>
      {view === "list" && initial.fixture && <p className="ops-note">{c.fixture}</p>}
      {view === "list" && !initial.rows.items.length && <div className="empty-state"><h2>{initial.error ? c.noSession : c.empty}</h2><p>{initial.error ? c.noSessionHint : c.emptyHint}</p></div>}
      {view === "edit" && selected && <details className="ops-card" data-testid="inventory-product-details"><summary>{c.product}</summary><label>{c.reason}<input type="text" value={reason} disabled={locked || selected.status !== "active"} maxLength={240} onChange={event => setReason(event.target.value)} /></label>            <section className="inspector" aria-label={c.selected}>
              <button
                className="close-details icon-button"
                aria-label={c.close}
                disabled={locked}
                onClick={() => { select(""); setView("list"); }}
              >
                <Icon name="close" size={17} />
              </button>
              <div className="selected-product">
                <ProductPhoto
                  code={selected.code}
                  name={selected.product_name}
                  demo={initial.fixture}
                  large
                  imageSrc={
                    selected.cover_image_id
                      ? imageURL(initial.storeID, selected.product_id, selected.cover_image_id)
                      : undefined
                  }
                />
                <div>
                  <h2>{selected.product_name}</h2>
                  <p>SKU: {selected.code}</p>
                  {/* Display only: the price is edited on the product page, in whole major units (D02). */}
                  <p data-testid="tray-price">
                    {c.price}: {money(locale, selected.currency, selected.price_minor)}
                  </p>
                  <Badge
                    className={`status ${selected.status}`}
                    tone={selected.status === "active" ? "success" : "neutral"}
                  >
                    {selected.status === "active" ? c.active : c.archived}
                  </Badge>
                  <Link className="tray-edit" href={`/${locale}/products/${selected.product_id}${initial.storeID ? `?store=${initial.storeID}` : ""}`}>
                    {c.editProduct}
                  </Link>
                </div>
              </div>
              <div className="stock-summary">
                <h2>{c.stockInfo}</h2>
                <dl>
                  {[
                    [c.onHand, selected.on_hand],
                    [c.reserved, selected.reserved],
                    [c.available, selected.available],
                  ].map(([label, value]) => (
                    <div key={label}>
                      <dt>{label}</dt>
                      <dd>{value}</dd>
                    </div>
                  ))}
                </dl>
              </div>
            </section>            <section
              className="purchase-entry"
              aria-label={c.purchaseEntry}
              data-testid="purchase-entry"
            >
              <div className="purchase-entry-copy">
                <h2>{c.purchaseEntry}</h2>
                {entryLoading ? (
                  <p role="status">{c.purchaseLoading}</p>
                ) : entryError ? (
                  <p role="alert">{entryError}</p>
                ) : entry ? (
                  <p role="status">
                    {{
                      configured: c.purchaseConfigured,
                      product_inactive: c.purchaseInactive,
                      no_active_sku: c.purchaseNoSKU,
                      storefront_unavailable: c.purchaseUnavailable,
                      domain_selection_required: c.purchaseAmbiguous,
                    }[entry.state]}
                  </p>
                ) : null}
                {entry?.state === "configured" && (
                  <small>{c.purchaseHint}</small>
                )}
                {entryFeedback && <small role="status">{entryFeedback}</small>}
              </div>
              <div className="purchase-entry-controls">
                {entry?.state === "configured" && (
                  <input
                    aria-label={c.purchaseEntry}
                    readOnly
                    value={entry.url}
                    onFocus={(event) => event.currentTarget.select()}
                  />
                )}
                <div className="purchase-entry-actions">
                  <button
                    type="button"
                    disabled={entryLoading || locked}
                    onClick={() => setEntryRefresh((current) => current + 1)}
                  >
                    {c.refresh}
                  </button>
                  {entry?.state === "configured" && (
                    <>
                      <button
                        type="button"
                        disabled={entryLoading || locked}
                        onClick={() => void usePurchaseEntry("copy")}
                      >
                        {c.copyPurchaseLink}
                      </button>
                      <button
                        type="button"
                        disabled={entryLoading || locked}
                        onClick={() => void usePurchaseEntry("open")}
                      >
                        {c.openPurchasePage}
                      </button>
                    </>
                  )}
                </div>
              </div>
            </section></details>}
    </InventoryView>
  </WorkspaceFrame>;
}
