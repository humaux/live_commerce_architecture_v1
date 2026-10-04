"use client";

// Bank-transfer section of the merchant order detail row (contracts/storefront-v2.md §C), next to the refund / CVS / shipment sections.
// BFF routes (lib/logistics-client.ts) -> Go internal/httpapi/offline.go:
//   GET  /api/stores/{store}/orders/{id}/bank-transfer                                   (orders:read)
//   POST .../bank-transfer/confirm | reject {reason} | refund-offline {restock}  (Idempotency-Key)  (payments:refund)
// Nothing here is optimistic: every write re-GETs the transfer and re-reads the order (onChanged). One Idempotency-Key per dialog open,
// reused only for a byte-identical retry after an unknown outcome. Confirm is the merchant's own act after checking their own bank account:
// the amount is the server order total, and the platform never confirms anything by itself. Reject rejects the buyer's submission, not the
// order; the offline refund records money the merchant returned outside the platform.

import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { money } from "@/lib/client";
import { postTransferDecision, readTransfer, type WriteResult } from "@/lib/logistics-client";
import { displayTime, type OrderDetail } from "@/lib/orders-model";
import {
  canConfirm,
  canRefundOffline,
  canReject,
  confirmBody,
  refundBody,
  rejectBody,
  type TransferAction,
  type TransferDetail,
} from "@/lib/transfer-model";
import { transferCopy, transferError } from "@/lib/transfer-copy";
import "./order-actions.css";

export function OrderBankTransfer({
  store,
  detail,
  locale,
  canDecide,
  boundary,
  onChanged,
}: {
  store: string;
  detail: OrderDetail;
  locale: Locale;
  canDecide: boolean;
  boundary: string;
  onChanged: () => Promise<boolean>;
}) {
  const tc = transferCopy[locale];
  const orderID = detail.order_id;
  const [view, setView] = useState<TransferDetail | null>(null);
  const [status, setStatus] = useState<"loading" | "ready" | "error">("loading");
  const [tick, setTick] = useState(0);
  const [open, setOpen] = useState<TransferAction | null>(null);
  const [reason, setReason] = useState("");
  const [restock, setRestock] = useState(false);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [uncertain, setUncertain] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  // One key per dialog open; a changed body (the reject reason) starts a new key, an identical retry replays the old one.
  const pending = useRef<{ key: string; body: string | null } | null>(null);

  useEffect(() => {
    const active = new AbortController();
    readTransfer(store, orderID, active.signal).then(
      (value) => {
        setView(value);
        setStatus("ready");
      },
      () => {
        if (active.signal.aborted) return;
        setView(null); // never act on a stale state when the re-GET failed
        setStatus("error");
      },
    );
    return () => active.abort();
  }, [store, orderID, tick, detail.updated_at]);

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) element.showModal();
    if (!open && element.open) element.close();
  }, [open]);

  const m = (value: number, currency: string) => money(locale, currency, value);
  function begin(action: TransferAction) {
    pending.current = { key: `xfer-${crypto.randomUUID()}`, body: null };
    setReason("");
    setRestock(false);
    setProblem("");
    setUncertain(false);
    setNotice("");
    setOpen(action);
  }
  function finish() {
    setOpen(null);
    // Closing after an unknown outcome still re-reads the server state.
    if (uncertain) {
      setTick((value) => value + 1);
      void onChanged();
    }
    setUncertain(false);
  }
  async function send() {
    if (!open || busy || !pending.current) return;
    const body = open === "reject" ? rejectBody(reason) : open === "confirm" ? confirmBody() : refundBody(restock);
    if (body === null) {
      setProblem(tc.errors.invalid_reason);
      return;
    }
    if (pending.current.body !== null && pending.current.body !== body)
      pending.current = { key: `xfer-${crypto.randomUUID()}`, body };
    else pending.current.body = body;
    const { key } = pending.current;
    setBusy(true);
    setProblem("");
    const result: WriteResult = await postTransferDecision(store, orderID, open, key, body, boundary);
    setBusy(false);
    if (result.ok) {
      pending.current = null;
      setUncertain(false);
      setOpen(null);
      setNotice(tc.done);
      setTick((value) => value + 1);
      void onChanged();
      return;
    }
    if (result.uncertain) {
      setUncertain(true);
      setProblem(tc.uncertain);
      return;
    }
    pending.current = null;
    setProblem(transferError(tc, result.code));
    // A refusal that means "the state moved" (decided meanwhile, window ended) re-reads both the transfer and the order.
    if (["already_confirmed", "already_refunded", "transfer_not_open", "transfer_window_closed", "transfer_not_submitted", "transfer_not_confirmed", "already_shipped"].includes(result.code)) {
      setTick((value) => value + 1);
      void onChanged();
    }
  }

  const proof = view?.proof ?? null;
  // Hint only (the definer decides): anything past MANUAL_UNASSIGNED has been handed to fulfilment.
  const shipped = detail.fulfillment_state !== "MANUAL_UNASSIGNED";
  return (
    <section className="orders-section" data-testid="order-transfer" aria-label={tc.secTitle}>
      <h2>{tc.secTitle}</h2>
      {status === "loading" && <p role="status">{tc.loadingOrder}</p>}
      {status === "error" && (
        <div role="status">
          <p>{tc.secLoadFailed}</p>
          <button type="button" onClick={() => setTick((value) => value + 1)}>{tc.retry}</button>
        </div>
      )}
      {view && (
        <>
          <dl className="orders-facts" data-testid="transfer-facts">
            <div>
              <dt>{tc.state}</dt>
              <dd>
                <span className="orders-badge" data-state={view.state} data-testid="transfer-state">{tc.states[view.state]}</span>
              </dd>
            </div>
            <div>
              <dt>{tc.amount}</dt>
              <dd data-testid="transfer-amount">{m(view.amount_minor, view.currency)}</dd>
            </div>
            <div>
              <dt>{tc.deadline}</dt>
              <dd>{displayTime(locale, view.deadline_at)}</dd>
            </div>
            {view.confirmed_at && (
              <div>
                <dt>{tc.confirmedAt}</dt>
                <dd>{displayTime(locale, view.confirmed_at)}</dd>
              </div>
            )}
            {view.refunded_at && (
              <div>
                <dt>{tc.refundedAt}</dt>
                <dd>{displayTime(locale, view.refunded_at)}</dd>
              </div>
            )}
          </dl>
          <h3>{tc.bankHeading}</h3>
          <dl className="orders-facts" data-testid="transfer-bank">
            <div><dt>{tc.setBank}</dt><dd>{view.bank.bank_name}{view.bank.branch ? ` · ${view.bank.branch}` : ""}</dd></div>
            <div><dt>{tc.setAccountName}</dt><dd>{view.bank.account_name}</dd></div>
            <div><dt>{tc.setAccountNumber}</dt><dd className="orders-mono">{view.bank.account_number}</dd></div>
          </dl>
          <h3>{tc.proofHeading}</h3>
          {proof ? (
            <dl className="orders-facts" data-testid="transfer-proof">
              <div><dt>{tc.last5}</dt><dd className="orders-mono" data-testid="transfer-last5">{proof.last5}</dd></div>
              <div><dt>{tc.amountSent}</dt><dd>{m(proof.amount_minor, view.currency)}</dd></div>
              <div><dt>{tc.paidAt}</dt><dd>{displayTime(locale, proof.paid_at)}</dd></div>
              <div><dt>{tc.submittedAt}</dt><dd>{displayTime(locale, proof.submitted_at)}</dd></div>
              <div><dt>{tc.attempts}</dt><dd>{proof.count}</dd></div>
            </dl>
          ) : (
            <p className="orders-hint">{tc.noProof}</p>
          )}
          {proof && proof.amount_minor !== view.amount_minor && (
            <p className="orders-bad" role="status" data-testid="transfer-mismatch">{tc.mismatch}</p>
          )}
          {view.reject_reason && (
            <p className="orders-hint" data-testid="transfer-reject-reason">{tc.rejectReason}: {view.reject_reason}</p>
          )}
          {canDecide && (
            <div className="orders-form-actions">
              {canConfirm(view.state) && (
                <button type="button" className="primary" data-testid="transfer-confirm" onClick={() => begin("confirm")}>{tc.confirm}</button>
              )}
              {canReject(view.state) && (
                <button type="button" data-testid="transfer-reject" onClick={() => begin("reject")}>{tc.reject}</button>
              )}
              {canRefundOffline(view.state) && (
                <button type="button" data-testid="transfer-refund" onClick={() => begin("refund-offline")}>{tc.refund}</button>
              )}
            </div>
          )}
        </>
      )}
      {notice && <p className="orders-notice" role="status" data-testid="transfer-notice">{notice}</p>}
      <dialog
        ref={dialog}
        className="orders-dialog"
        aria-labelledby={`transfer-title-${orderID}`}
        data-testid="transfer-dialog"
        onClose={() => {
          if (open) finish();
        }}
      >
        {open && view && (
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void send();
            }}
          >
            <h2 id={`transfer-title-${orderID}`}>
              {open === "confirm" ? tc.confirmTitle : open === "reject" ? tc.rejectTitle : tc.refundTitle}
            </h2>
            <p data-testid="transfer-dialog-text">
              {open === "confirm" ? tc.confirmText(m(view.amount_minor, view.currency)) : open === "reject" ? tc.rejectText : tc.refundText}
            </p>
            {open === "confirm" && !proof && (
              <p className="orders-bad" role="note" data-testid="transfer-no-proof-warning">{tc.confirmNoProofWarning}</p>
            )}
            {open === "refund-offline" && (
              <label className="orders-check">
                <input
                  type="checkbox"
                  data-testid="transfer-restock"
                  checked={restock && !shipped}
                  disabled={busy || uncertain || shipped}
                  onChange={(event) => setRestock(event.target.checked)}
                />
                <span>{tc.refundRestockLabel}</span>
                <small className="orders-hint">{shipped ? tc.refundRestockShipped : tc.refundRestockHint}</small>
              </label>
            )}
            {open === "reject" && (
              <label>
                <span>{tc.rejectReasonLabel}</span>
                <textarea
                  data-testid="transfer-reason"
                  required
                  maxLength={200}
                  rows={3}
                  value={reason}
                  disabled={busy || uncertain}
                  onChange={(event) => setReason(event.target.value)}
                />
              </label>
            )}
            {problem && <p className="orders-bad" role="alert" data-testid="transfer-problem">{problem}</p>}
            <div className="orders-dialog-actions">
              <button type="button" onClick={finish}>{uncertain ? tc.close : tc.cancel}</button>
              <button type="submit" className="primary" data-testid="transfer-submit" disabled={busy || (open === "reject" && rejectBody(reason) === null && !uncertain)}>
                {busy ? tc.working : uncertain ? tc.retrySame : open === "confirm" ? tc.submitConfirm : open === "reject" ? tc.submitReject : tc.submitRefund}
              </button>
            </div>
          </form>
        )}
      </dialog>
    </section>
  );
}
