"use client";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import type { ProductDetail } from "@/lib/catalog-v2-model";
import { parseCreated } from "@/lib/catalog-v2-model";
import { command, readProduct } from "@/lib/catalog-v2-client";
import { useWrite } from "@/lib/catalog-v2-write";
import { useProductLeaveGuard } from "@/lib/use-product-leave-guard";
import {
  draftFromDetail,
  editDocument,
  type ProductDraft,
} from "@/lib/product-document";
import { productEditorCopy } from "@/lib/product-editor-copy";
import { currencySign } from "@live-commerce/format";

export function ProductQuickEdit({
  store,
  id,
  locale,
  field,
  boundary,
  close,
  refresh,
  inline = false,
}: {
  store: string;
  id: string;
  locale: Locale;
  field: "price" | "stock";
  boundary: string;
  close: () => void;
  refresh: () => Promise<unknown>;
  inline?: boolean;
}) {
  const c = productEditorCopy[locale],
    dialog = useRef<HTMLDialogElement>(null);
  const [detail, setDetail] = useState<ProductDetail | null>(null),
    [draft, setDraft] = useState<ProductDraft | null>(null),
    [error, setError] = useState(false);
  const [deltas, setDeltas] = useState<Record<string, string>>({});
  const write = useWrite(
    store,
    boundary,
    (code) => (code in c ? c[code as keyof typeof c] : c.failed),
    c.uncertain,
    c.listRecoveryRequired,
  );
  const locked = write.busy || write.message?.kind === "uncertain";
  useProductLeaveGuard(
    {
      locked,
      dirty:
        !!detail &&
        !!draft &&
        JSON.stringify(draft) !== JSON.stringify(draftFromDetail(detail)),
    },
    c.leave,
  );
  useEffect(() => {
    const el = dialog.current;
    const trigger = document.activeElement as HTMLElement;
    if (!inline) el?.showModal();
    return () => {
      el?.close();
      trigger?.focus();
    };
  }, [inline]);
  useEffect(() => {
    const abort = new AbortController();
    void readProduct(store, id, abort.signal)
      .then((v) => {
        if (!abort.signal.aborted) {
          setDetail(v);
          setDraft(draftFromDetail(v));
        }
      })
      .catch(() => {
        if (!abort.signal.aborted) setError(true);
      });
    return () => abort.abort();
  }, [store, id]);
  async function save() {
    if (!detail || !draft || locked) return;
    try {
      if (Object.values(deltas).some((d) => !/^[-+]?\d+$/.test(d)))
        throw new Error("invalid_stock");
      const patch = editDocument(
        draft,
        detail,
        detail.skus[0]?.currency ?? "TWD",
      );
      if (Object.keys(patch).length === 1) {
        write.notice(c.noChanges);
        return;
      }
      await write.run(
        command("PUT", `products/${id}/document`, patch),
        parseCreated,
        async () => {
          await refresh();
          close();
        },
        c.saved,
      );
    } catch (e) {
      write.fail(
        e instanceof Error && e.message in c
          ? c[e.message as keyof typeof c]
          : c.failed,
      );
    }
  }
  return (
    <dialog
      ref={dialog}
      open={inline || undefined}
      className={inline ? "pe-quick pe-quick-inline" : "pe-quick"}
      data-testid="product-quick-edit"
      aria-labelledby="quick-title"
      onKeyDown={(e) => {
        if (inline && e.key === "Escape") {
          e.preventDefault();
          if (!locked) close();
        }
      }}
      onCancel={(e) => {
        e.preventDefault();
        if (!locked) close();
      }}
    >
      <form
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <header>
          <h2 id="quick-title">
            {field === "price" ? c.editPrice : c.editStock} ·{" "}
            {detail?.name ?? c.loading}
          </h2>
          <button
            type="button"
            disabled={locked}
            onClick={close}
            aria-label={c.close}
          >
            ×
          </button>
        </header>
        {!detail && (
          <p role={error ? "alert" : "status"}>
            {error ? c.failed : c.loading}
          </p>
        )}
        {detail &&
          draft &&
          draft.rows.map((row, i) => {
            const sku = detail.skus[i];
            const change = (v: Partial<typeof row>) =>
              setDraft({
                ...draft,
                rows: draft.rows.map((r, n) => (i === n ? { ...r, ...v } : r)),
              });
            return (
              <fieldset
                disabled={locked}
                key={sku.id}
                data-testid={`quick-sku-${sku.id}`}
              >
                <legend>
                  {sku.title} · {sku.code}
                </legend>
                {field === "price" ? (
                  <div className="pe-two">
                    <label>
                      {c.price} ({currencySign(sku.currency)})
                      <input
                        autoFocus={i === 0}
                        data-testid={`quick-price-${i}`}
                        value={row.price}
                        inputMode="decimal"
                        onChange={(e) => change({ price: e.target.value })}
                      />
                    </label>
                    <label>
                      {c.compare}
                      <input
                        data-testid={`quick-compare-${i}`}
                        value={row.compare}
                        inputMode="decimal"
                        onChange={(e) => change({ compare: e.target.value })}
                      />
                    </label>
                  </div>
                ) : (
                  <>
                    <p>
                      {c.onHand}: {sku.on_hand ?? "—"} · {c.committed}:{" "}
                      {sku.committed ?? "—"}
                    </p>
                    <label className="pe-check">
                      <input
                        type="checkbox"
                        data-testid={`quick-untracked-${i}`}
                        checked={!row.tracked}
                        onChange={(e) => change({ tracked: !e.target.checked })}
                      />
                      {c.untracked}
                    </label>
                    {row.tracked ? (
                      <div className="pe-two">
                        <label>
                          {c.delta}
                          <input
                            autoFocus={i === 0}
                            data-testid={`quick-delta-${i}`}
                            inputMode="text"
                            disabled={
                              !detail.warehouse_id || sku.on_hand === null
                            }
                            value={
                              deltas[sku.id] ??
                              (row.quantity !== "" && sku.on_hand !== null
                                ? String(Number(row.quantity) - sku.on_hand)
                                : "")
                            }
                            onChange={(e) => {
                              const s = e.target.value;
                              setDeltas((now) => ({ ...now, [sku.id]: s }));
                              if (/^-?\d+$/.test(s) && sku.on_hand !== null)
                                change({
                                  quantity: String(sku.on_hand + Number(s)),
                                });
                            }}
                          />
                          {sku.on_hand === null && (
                            <small>{c.deltaUnavailable}</small>
                          )}
                        </label>
                        <label>
                          {c.targetQty}
                          <input
                            data-testid={`quick-target-${i}`}
                            inputMode="numeric"
                            disabled={!detail.warehouse_id}
                            value={row.quantity}
                            onChange={(e) => {
                              setDeltas((now) => {
                                const next = { ...now };
                                delete next[sku.id];
                                return next;
                              });
                              change({ quantity: e.target.value });
                            }}
                          />
                        </label>
                      </div>
                    ) : (
                      <label>
                        {c.max}
                        <input
                          data-testid={`quick-max-${i}`}
                          inputMode="numeric"
                          value={row.max}
                          onChange={(e) => change({ max: e.target.value })}
                        />
                      </label>
                    )}
                  </>
                )}
              </fieldset>
            );
          })}
        {field === "stock" && detail && !detail.warehouse_id && (
          <p>
            {c.warehouse_required}{" "}
            <Link href={`/${locale}/inventory?store=${store}`}>
              {c.inventoryLink}
            </Link>
          </p>
        )}
        {write.message && (
          <p role={write.message.kind === "info" ? "status" : "alert"}>
            {write.message.text}
            {write.canRetry && (
              <button type="button" onClick={() => void write.retry()}>
                {c.retry}
              </button>
            )}
          </p>
        )}
        <footer>
          <button type="button" disabled={locked} onClick={close}>
            {c.cancel}
          </button>
          <button
            className="product-primary"
            type="submit"
            data-testid="quick-save"
            disabled={locked || !detail}
          >
            {c.save}
          </button>
        </footer>
      </form>
    </dialog>
  );
}
