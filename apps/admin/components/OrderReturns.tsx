// Purpose: Returns (RMA) section of the merchant order detail: register a return (lines/qty, reason enum + note),
//   receive, inspect (sellable/scrap disposition), close and withdraw, with the §2 state timeline per RMA.
// Depends on: react, @live-commerce/i18n, @/lib/orders-model (OrderDetail, displayTime), @/lib/returns-model,
//   @/lib/returns-client, @/lib/returns-copy, @/src/shell/api (readWorkspace: the caller's own permissions for the
//   inventory:write disposition gate), ./order-actions.css.
//   BFF: GET|POST /api/stores/{store}/orders/{id}/returns, POST /api/stores/{store}/returns/{rma}/{receive|inspect|close|cancel}
//   -> Go internal/httpapi/returns.go (contracts/returns-v1.md §6).
// Used by: apps/admin/components/OrderDetailPanel.tsx (mounted for shipped orders).
// Invariants: returns-v1 §2/§4 — every command carries one Idempotency-Key (reused only for a byte-identical retry
//   after an unknown outcome) and expected_version CAS; no success is shown before the server state is re-read;
//   the disposition buttons are disabled without inventory:write (session hint only — Go re-authorizes).
// Status: MOCK (no provider calls; stock authority is PostgreSQL).
"use client";

import { Fragment, useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { displayTime, type OrderDetail } from "@/lib/orders-model";
import {
  cancelRmaBody,
  closeBody,
  inspectBody,
  receiveBody,
  registerBody,
  registerReasonText,
  returnReasons,
  type ReturnReason,
  type Rma,
} from "@/lib/returns-model";
import { postRegisterReturn, postRmaStep, readOrderReturns } from "@/lib/returns-client";
import { returnsCopy, returnsError, type ReturnsCopy } from "@/lib/returns-copy";
import { readWorkspace } from "@/src/shell/api";
import "./order-actions.css";

type Dialog =
  | { kind: "register" }
  | { kind: "receive" | "inspect" | "close" | "cancel-rma"; rma: Rma };

const tone = (state: Rma["state"]) =>
  state === "CLOSED" ? "success" : state === "CANCELLED" ? "neutral" : "warning";

/** Non-negative integer typed into a quantity field; null = not a whole number. */
function qtyValue(text: string): number | null {
  if (!/^\d{1,9}$/.test(text.trim())) return null;
  const value = Number(text.trim());
  return Number.isSafeInteger(value) ? value : null;
}

/** Returns section of one order detail row. All server state is re-read after every command. */
export function OrderReturns({
  store,
  detail,
  locale,
  canWrite,
  boundary,
  onChanged,
}: {
  store: string;
  detail: OrderDetail;
  locale: Locale;
  canWrite: boolean; // fulfillment:write (order-actions probe)
  boundary: string;
  onChanged: () => Promise<boolean>;
}) {
  const c = returnsCopy[locale];
  const orderID = detail.order_id;
  const [list, setList] = useState<Rma[] | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [tick, setTick] = useState(0);
  const [canInventory, setCanInventory] = useState(true); // unknown session permissions -> enabled; Go re-authorizes
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [notice, setNotice] = useState("");
  const [problem, setProblem] = useState("");
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  // register form
  const [quantities, setQuantities] = useState<Record<string, string>>({});
  const [reason, setReason] = useState<ReturnReason>("buyer_request");
  const [note, setNote] = useState("");
  // receive form: sku -> qty_received text; inspect form: sku -> restock/scrap text
  const [received, setReceived] = useState<Record<string, string>>({});
  const [restock, setRestock] = useState<Record<string, string>>({});
  const [scrap, setScrap] = useState<Record<string, string>>({});
  const dialogRef = useRef<HTMLDialogElement>(null);
  const firstField = useRef<HTMLSelectElement | HTMLInputElement | null>(null);
  const confirmText = useRef<HTMLParagraphElement>(null);
  // One key per dialog submission, reused only for a byte-identical retry of that submission (§4).
  const pending = useRef<{ key: string; body: string } | null>(null);

  useEffect(() => {
    const active = new AbortController();
    readOrderReturns(store, orderID, active.signal).then(
      (value) => {
        setList(value);
        setStatus("ready");
      },
      () => {
        if (active.signal.aborted) return;
        setList(null);
        setStatus("error");
      },
    );
    return () => active.abort();
  }, [store, orderID, tick]);

  // The caller's own store permissions (0089 session store list) gate the stock-writing steps; the server decides.
  useEffect(() => {
    const active = new AbortController();
    readWorkspace(active.signal).then(
      (stores) => {
        const mine = stores.find((item) => item.id === store);
        if (mine?.permissions) setCanInventory(mine.permissions.includes("inventory:write"));
      },
      () => undefined, // unknown -> leave enabled; a real refusal surfaces as 403 copy
    );
    return () => active.abort();
  }, [store]);

  useEffect(() => {
    const element = dialogRef.current;
    if (!element) return;
    if (dialog && !element.open) element.showModal();
    if (!dialog && element.open) element.close();
  }, [dialog]);

  // Focus parking: a confirmation step never leaves focus on the submit button (same rule as OrderRefunds).
  useEffect(() => {
    if (!dialog) return;
    if (dialog.kind === "close" || dialog.kind === "cancel-rma") confirmText.current?.focus();
    else firstField.current?.focus();
  }, [dialog]);

  function open(next: Dialog) {
    pending.current = null;
    setProblem("");
    setUncertain(false);
    setNotice("");
    if (next.kind === "register") {
      setQuantities({});
      setReason("buyer_request");
      setNote("");
    } else if (next.kind === "receive") {
      setReceived(Object.fromEntries(next.rma.lines.map((l) => [l.sku_id, String(l.qty_registered)])));
    } else if (next.kind === "inspect") {
      setRestock(Object.fromEntries(next.rma.lines.map((l) => [l.sku_id, String(l.qty_received ?? 0)])));
      setScrap(Object.fromEntries(next.rma.lines.map((l) => [l.sku_id, "0"])));
    }
    setDialog(next);
  }
  function finish() {
    setDialog(null);
    if (uncertain) setTick((value) => value + 1);
    setUncertain(false);
  }

  async function run(body: string, send: (key: string) => Promise<{ ok: true } | { ok: false; code: string; uncertain: boolean }>, done: string) {
    if (busy) return;
    if (pending.current?.body !== body) pending.current = { key: `returns-${crypto.randomUUID()}`, body };
    setBusy(true);
    setProblem("");
    const result = await send(pending.current.key);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setDialog(null);
      setUncertain(false);
      setNotice(done);
      setTick((value) => value + 1);
      void onChanged();
      return;
    }
    if (result.uncertain) {
      setUncertain(true);
      setProblem(c.uncertain);
      return;
    }
    pending.current = null;
    setProblem(returnsError(c, result.code));
    // The dialog stays open on a refusal so the merchant reads the code next to the form they typed.
    if (result.code === "version_changed" || result.code === "invalid_state" || result.code === "exceeds_shipped")
      setTick((value) => value + 1); // stale view: re-read before the merchant retries
  }

  const itemName = (sku: string) => {
    const item = detail.items.find((row) => row.sku_id === sku);
    return item ? `${item.name} (${item.code})` : sku;
  };

  // ---- per-dialog validation + submission ---------------------------------------------------------------------------
  const registerLines = detail.items
    .map((item) => ({ sku_id: item.sku_id, quantity: qtyValue(quantities[item.sku_id] ?? "0") ?? 0 }))
    .filter((line) => line.quantity > 0);
  const registerReasonOK = registerReasonText(reason, note) !== null;
  const registerOK =
    registerReasonOK &&
    registerLines.length >= 1 &&
    detail.items.every((item) => {
      const qty = qtyValue(quantities[item.sku_id] ?? "0");
      return qty !== null && qty <= item.quantity;
    });
  function submitRegister() {
    const text = registerReasonText(reason, note);
    const body = text && registerBody(text, registerLines);
    if (!body) return setProblem(c.registerInvalid);
    void run(body, (key) => postRegisterReturn(store, orderID, key, body, boundary), c.registerDone);
  }

  const receiveRma = dialog?.kind === "receive" ? dialog.rma : null;
  const receiveLines =
    receiveRma?.lines.map((line) => ({
      sku_id: line.sku_id,
      qty_received: qtyValue(received[line.sku_id] ?? "") ?? -1,
    })) ?? [];
  const receiveOK =
    !!receiveRma &&
    receiveRma.lines.every((line) => {
      const qty = qtyValue(received[line.sku_id] ?? "");
      return qty !== null && qty <= line.qty_registered;
    }) &&
    receiveLines.some((line) => line.qty_received > 0);
  function submitReceive() {
    if (!receiveRma) return;
    const body = receiveBody(receiveRma.version, receiveLines.filter((l) => l.qty_received >= 0));
    if (!body) return setProblem(c.receiveInvalid);
    void run(body, (key) => postRmaStep(store, receiveRma.id, "receive", key, body, boundary), c.receiveDone);
  }

  const inspectRma = dialog?.kind === "inspect" ? dialog.rma : null;
  const inspectLines =
    inspectRma?.lines.map((line) => ({
      sku_id: line.sku_id,
      qty_received: line.qty_received ?? 0,
      qty_restock: qtyValue(restock[line.sku_id] ?? "") ?? -1,
      qty_scrap: qtyValue(scrap[line.sku_id] ?? "") ?? -1,
    })) ?? [];
  const inspectOK =
    !!inspectRma &&
    inspectLines.every((line) => line.qty_restock >= 0 && line.qty_scrap >= 0 &&
      line.qty_restock + line.qty_scrap === line.qty_received);
  function submitInspect() {
    if (!inspectRma) return;
    const body = inspectBody(inspectRma.version, inspectLines);
    if (!body) return setProblem(c.inspectInvalid);
    void run(body, (key) => postRmaStep(store, inspectRma.id, "inspect", key, body, boundary), c.inspectDone);
  }

  const closeRma = dialog?.kind === "close" ? dialog.rma : null;
  const closeUnits = closeRma?.lines.reduce((sum, line) => sum + (line.qty_restock ?? 0), 0) ?? 0;
  function submitClose() {
    if (!closeRma) return;
    const body = closeBody(closeRma.version);
    if (!body) return;
    void run(body, (key) => postRmaStep(store, closeRma.id, "close", key, body, boundary), c.closeDone);
  }

  const withdrawRma = dialog?.kind === "cancel-rma" ? dialog.rma : null;
  function submitWithdraw() {
    if (!withdrawRma) return;
    const body = cancelRmaBody(withdrawRma.version);
    if (!body) return;
    void run(body, (key) => postRmaStep(store, withdrawRma.id, "cancel", key, body, boundary), c.cancelRmaDone);
  }

  const canStock = canWrite && canInventory;

  return (
    <section className="orders-section" data-testid="order-returns" aria-label={c.sectionTitle}>
      <h2>{c.sectionTitle}</h2>
      {status === "loading" && !list && <p className="orders-empty" role="status">{c.loading}</p>}
      {status === "error" && (
        <div role="status">
          <p>{c.unavailable}</p>
          <button type="button" onClick={() => setTick((value) => value + 1)}>{c.retry}</button>
        </div>
      )}
      {list && (
        <>
          {list.length === 0 ? (
            <p className="orders-empty">{c.none}</p>
          ) : (
            list.map((rma) => (
              <article key={rma.id} className="orders-rma" data-testid={`rma-${rma.id}`} data-state={rma.state}>
                <header className="orders-rma-head">
                  <span className={`orders-badge orders-tone-${tone(rma.state)}`} data-state={rma.state}>
                    {c.states[rma.state]}
                  </span>
                  <small>{c.colCreated}: {displayTime(locale, rma.created_at)}</small>
                  <small>{c.colUpdated}: {displayTime(locale, rma.updated_at)}</small>
                </header>
                <p className="orders-rma-reason">
                  {c.colReason}: {c.reasons[rma.reason.split(":")[0] as ReturnReason] ?? rma.reason}
                  {rma.reason.includes(":") && ` — ${rma.reason.slice(rma.reason.indexOf(":") + 1).trim()}`}
                </p>
                <div className="orders-actions-scroll">
                  <table className="orders-actions-table">
                    <thead>
                      <tr>
                        <th>{c.colLines}</th>
                        <th>{c.qtyToReturn}</th>
                        <th>{c.qtyReceived}</th>
                        <th>{c.qtyRestock}</th>
                        <th>{c.qtyScrap}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {rma.lines.map((line) => (
                        <tr key={`${line.warehouse_id}|${line.sku_id}`}>
                          <td>{itemName(line.sku_id)}</td>
                          <td>{line.qty_registered}</td>
                          <td>{line.qty_received ?? "—"}</td>
                          <td>{line.qty_restock ?? "—"}</td>
                          <td>{line.qty_scrap ?? "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                {(rma.state === "INSPECTED" || rma.state === "CLOSED") && (
                  <p className="orders-notice">
                    {rma.refund_id ? c.refundLinked : (
                      <>
                        {c.refundHint}{" "}
                        {["CAPTURED", "PARTIALLY_REFUNDED", "REFUNDED", "REVIEW_REQUIRED"].includes(detail.payment_state) && (
                          <a href="#order-refunds">{c.refundLink}</a>
                        )}
                      </>
                    )}
                  </p>
                )}
                <div className="orders-dialog-actions">
                  {canWrite && rma.state === "REGISTERED" && (
                    <>
                      <button type="button" className="orders-compact" data-testid={`rma-receive-${rma.id}`} onClick={() => open({ kind: "receive", rma })}>
                        {c.receive}
                      </button>
                      <button type="button" className="orders-compact" data-testid={`rma-withdraw-${rma.id}`} onClick={() => open({ kind: "cancel-rma", rma })}>
                        {c.cancelRma}
                      </button>
                    </>
                  )}
                  {rma.state === "RECEIVED" && (
                    <button type="button" className="orders-compact" data-testid={`rma-inspect-${rma.id}`}
                      disabled={!canStock} title={!canStock ? c.errors.forbidden : undefined}
                      onClick={() => open({ kind: "inspect", rma })}>
                      {c.inspect}
                    </button>
                  )}
                  {rma.state === "INSPECTED" && (
                    <button type="button" className="orders-compact" data-testid={`rma-close-${rma.id}`}
                      disabled={!canStock} title={!canStock ? c.errors.forbidden : undefined}
                      onClick={() => open({ kind: "close", rma })}>
                      {c.close}
                    </button>
                  )}
                </div>
              </article>
            ))
          )}
          {canWrite && (
            <button type="button" className="orders-section-action" data-testid="return-register" onClick={() => open({ kind: "register" })}>
              {c.register}
            </button>
          )}
        </>
      )}
      {notice && <p className="orders-notice" role="status" data-testid="return-notice">{notice}</p>}
      <dialog
        ref={dialogRef}
        className="orders-dialog"
        aria-labelledby={`return-title-${orderID}`}
        data-testid="return-dialog"
        onClose={() => {
          if (dialog) finish();
        }}
      >
        {dialog && (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (dialog.kind === "register") submitRegister();
              else if (dialog.kind === "receive") submitReceive();
              else if (dialog.kind === "inspect") submitInspect();
              else if (dialog.kind === "close") submitClose();
              else submitWithdraw();
            }}
          >
            <h2 id={`return-title-${orderID}`}>
              {dialog.kind === "register" ? c.registerTitle
                : dialog.kind === "receive" ? c.receiveTitle
                : dialog.kind === "inspect" ? c.inspectTitle
                : dialog.kind === "close" ? c.closeTitle
                : c.cancelRmaTitle}
            </h2>
            {dialog.kind === "register" && (
              <>
                <p>{c.registerIntro}</p>
                <label>
                  {c.registerReason}
                  <select
                    ref={(el) => { firstField.current = el; }}
                    data-testid="return-reason"
                    value={reason}
                    onChange={(event) => setReason(event.target.value as ReturnReason)}
                  >
                    {returnReasons.map((value) => (
                      <option key={value} value={value}>{c.reasons[value]}</option>
                    ))}
                  </select>
                </label>
                <label>
                  {c.note}
                  <input
                    data-testid="return-note"
                    value={note}
                    maxLength={60}
                    placeholder={c.notePlaceholder}
                    autoComplete="off"
                    aria-invalid={!registerReasonOK}
                    onChange={(event) => setNote(event.target.value)}
                  />
                </label>
                {detail.items.map((item) => {
                  const value = quantities[item.sku_id] ?? "0";
                  const qty = qtyValue(value);
                  const bad = qty === null || qty > item.quantity;
                  return (
                    <label key={item.sku_id}>
                      {item.name} ({item.code}) — {c.shippedQty(item.quantity)}
                      <input
                        data-testid={`return-qty-${item.sku_id}`}
                        inputMode="numeric"
                        autoComplete="off"
                        value={value}
                        aria-invalid={bad}
                        onChange={(event) => setQuantities((old) => ({ ...old, [item.sku_id]: event.target.value }))}
                      />
                    </label>
                  );
                })}
              </>
            )}
            {receiveRma && (
              <Fragment key="receive">
                <p>{c.receiveIntro}</p>
                {receiveRma.lines.map((line, index) => (
                  <label key={line.sku_id}>
                    {itemName(line.sku_id)} — {c.registeredQty(line.qty_registered)}
                    <input
                      ref={index === 0 ? (el) => { firstField.current = el; } : undefined}
                      data-testid={`receive-qty-${line.sku_id}`}
                      inputMode="numeric"
                      autoComplete="off"
                      value={received[line.sku_id] ?? ""}
                      aria-invalid={!receiveOK}
                      onChange={(event) => setReceived((old) => ({ ...old, [line.sku_id]: event.target.value }))}
                    />
                  </label>
                ))}
              </Fragment>
            )}
            {inspectRma && (
              <Fragment key="inspect">
                <p>{c.inspectIntro}</p>
                {inspectRma.lines.map((line, index) => (
                  <fieldset key={line.sku_id} className="orders-rma-inspect">
                    <legend>{itemName(line.sku_id)} — {c.receivedQty(line.qty_received ?? 0)}</legend>
                    <label>
                      {c.qtyRestock}
                      <input
                        ref={index === 0 ? (el) => { firstField.current = el; } : undefined}
                        data-testid={`inspect-restock-${line.sku_id}`}
                        inputMode="numeric"
                        autoComplete="off"
                        value={restock[line.sku_id] ?? ""}
                        aria-invalid={!inspectOK}
                        onChange={(event) => setRestock((old) => ({ ...old, [line.sku_id]: event.target.value }))}
                      />
                    </label>
                    <label>
                      {c.qtyScrap}
                      <input
                        data-testid={`inspect-scrap-${line.sku_id}`}
                        inputMode="numeric"
                        autoComplete="off"
                        value={scrap[line.sku_id] ?? ""}
                        aria-invalid={!inspectOK}
                        onChange={(event) => setScrap((old) => ({ ...old, [line.sku_id]: event.target.value }))}
                      />
                    </label>
                  </fieldset>
                ))}
              </Fragment>
            )}
            {closeRma && (
              <p ref={confirmText} tabIndex={-1} data-testid="return-close-confirm">{c.closeConfirm(closeUnits)}</p>
            )}
            {withdrawRma && (
              <p ref={confirmText} tabIndex={-1} data-testid="return-withdraw-confirm">{c.cancelRmaConfirm}</p>
            )}
            {problem && <p className="orders-bad" role="alert" data-testid="return-problem">{problem}</p>}
            <div className="orders-dialog-actions">
              <button type="button" disabled={busy} onClick={finish}>
                {uncertain ? c.close2 : c.cancel}
              </button>
              <button
                type="submit"
                className="primary"
                data-testid="return-submit"
                disabled={busy ||
                  (dialog.kind === "register" && !registerOK) ||
                  (dialog.kind === "receive" && !receiveOK) ||
                  (dialog.kind === "inspect" && !inspectOK)}
              >
                {busy ? c.sending : uncertain ? c.retrySame
                  : dialog.kind === "register" ? c.registerSubmit
                  : dialog.kind === "receive" ? c.receiveSubmit
                  : dialog.kind === "inspect" ? c.inspectSubmit
                  : dialog.kind === "close" ? c.closeSubmit
                  : c.cancelRmaSubmit}
              </button>
            </div>
          </form>
        )}
      </dialog>
    </section>
  );
}
