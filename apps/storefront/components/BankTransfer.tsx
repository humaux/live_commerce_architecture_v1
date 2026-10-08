"use client";

// Purpose: Order-page panel of the bank_transfer payment mode (contracts/storefront-v2.md §C): the shop's bank details (shown only on the buyer's own
// order), the amount due, a countdown to the deadline, and the "I have transferred" form (last 5 digits, amount, date/time).
// BFF routes: GET /api/buyer/orders/{id}/bank-transfer and PUT /api/buyer/orders/{id}/bank-transfer/proof (keyed) -> Go
// internal/buyerhttp/transfer.go (checkout.read_bank_transfer_buyer / checkout.submit_transfer_proof).
// The panel never confirms anything: the order becomes CONFIRMED only when the shop confirms the transfer (a merchant act), and the amount to
// transfer is the server order total from the view, never a value the buyer or this component computed.
// Depends on: buyer transfer view, bank transfer contract/copy, and shared Taipei time formatting for server instants.
// Used by: buyer order detail; buyer-entered datetime-local remains in the buyer's own zone.
import { useCallback, useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { BuyerClientError, buyerRequest } from "../lib/buyer-client";
import { assertPurchaseContext, readPurchase } from "../lib/purchase";
import { bankTransferCopy } from "../lib/bank-transfer-copy";
import { browseCopy } from "../lib/browse-copy";
import { displayTime } from "../../../packages/format/src/index";
import {
  isTransferErrorCode,
  minorFromText,
  transferCountdown,
  validProofBody,
  validProofResult,
  validTransferView,
  type ProofBody,
  type TransferErrorCode,
  type TransferState,
  type TransferView,
} from "../lib/bank-transfer-contract";

type Money = (amount: number, currency: string) => string;
// States in which the shop may still act and the buyer may still send details.
const open = (state: TransferView["state"]) => state === "AWAITING" || state === "SUBMITTED" || state === "REJECTED";

// A refusal body {code} from the BFF; only codes the contract lists are shown, everything else is generic.
async function refusalCode(response: Response): Promise<TransferErrorCode | null> {
  try {
    const body: unknown = await response.clone().json();
    const code = (body as { code?: unknown })?.code;
    return isTransferErrorCode(code) ? code : null;
  } catch {
    return null;
  }
}

// <input type="datetime-local"> value for "now" in the buyer's own zone, to the minute.
const localInput = (date: Date) => {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
};

/** Show the buyer-owned bank transfer view and submit a reported payment instant. */
export default function BankTransfer({
  context,
  orderID,
  locale,
  money,
  refreshToken,
  onState,
}: {
  context: string;
  orderID: string;
  locale: Locale;
  money: Money;
  refreshToken: number;
  // The latest transfer state read, so the order heading above can follow a confirmation or expiry (D06; settledCommercialState).
  onState?: (state: TransferState) => void;
}) {
  const copy = bankTransferCopy[locale];
  // The proof is entered as device-local wall time; its saved echo is explicitly store time.
  const inputTimeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const inputTimeZoneID = `transfer-paid-at-zone-${orderID}`;
  const [view, setView] = useState<TransferView | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  const [last5, setLast5] = useState("");
  const [amount, setAmount] = useState("");
  const [paidAt, setPaidAt] = useState(() => localInput(new Date()));
  const [sending, setSending] = useState(false);
  const [message, setMessage] = useState<"" | "sent" | "invalid" | "failed">("");
  const [refusal, setRefusal] = useState<TransferErrorCode | null>(null);
  const attempt = useRef<{ key: string; body: string } | null>(null);
  const live = useRef(0);

  const load = useCallback(async () => {
    const version = ++live.current;
    try {
      const next = await readPurchase(`orders/${orderID}/bank-transfer`, context, (v): v is TransferView =>
        validTransferView(v, orderID),
      );
      if (version !== live.current) return;
      setView(next);
      setLoadFailed(false);
    } catch {
      if (version === live.current) setLoadFailed(true);
    }
  }, [context, orderID]);

  useEffect(() => {
    void load();
    return () => {
      live.current++;
    };
  }, [load, refreshToken]);

  useEffect(() => {
    if (view) onState?.(view.state);
  }, [view?.state]);

  // The countdown ticks locally; the server stays the authority on when the window ends (expiry releases the stock).
  useEffect(() => {
    const tick = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(tick);
  }, []);
  // While the shop may still act, re-read the state once a minute so a confirmation shows up without a manual refresh.
  useEffect(() => {
    if (!view || !open(view.state)) return;
    const poll = window.setInterval(() => void load(), 60_000);
    return () => window.clearInterval(poll);
  }, [view?.state, load]);
  // Pre-fill the amount with the server total (major units); the buyer may correct it to what they really sent.
  useEffect(() => {
    if (view && amount === "") setAmount(String(view.amount_minor / 100));
  }, [view?.amount_minor]);

  async function submit() {
    if (!view) return;
    const minor = minorFromText(amount);
    const iso = paidAt ? new Date(paidAt).toISOString() : "";
    const body: ProofBody = { last5: last5.trim(), amount_minor: minor ?? 0, paid_at: iso };
    setRefusal(null);
    if (minor === null || !paidAt || Number.isNaN(Date.parse(paidAt)) || !validProofBody(body)) {
      setMessage("invalid");
      return;
    }
    const text = JSON.stringify(body);
    // One idempotency key per distinct body: a lost response is retried with the same key (exact replay), an edit gets a new one.
    if (attempt.current?.body !== text) attempt.current = { key: crypto.randomUUID(), body: text };
    setSending(true);
    setMessage("");
    try {
      const response = await buyerRequest("PUT", `orders/${orderID}/bank-transfer/proof`, context, body, attempt.current.key);
      if (!response.ok) {
        const code = await refusalCode(response);
        if (code) {
          attempt.current = null;
          setRefusal(code);
          void load();
        } else setMessage("failed");
        return;
      }
      const data: unknown = await response.json();
      if (!validProofResult(data, orderID)) throw new BuyerClientError("invalid_response");
      await assertPurchaseContext(context);
      attempt.current = null;
      setMessage("sent");
      await load();
    } catch {
      setMessage("failed");
    } finally {
      setSending(false);
    }
  }

  if (!view)
    return (
      <section data-testid="bank-transfer" aria-labelledby="bank-transfer-title">
        <h2 id="bank-transfer-title">{copy.title}</h2>
        <p role={loadFailed ? "alert" : "status"}>{loadFailed ? copy.loadFailed : "…"}</p>
        {loadFailed && (
          <button type="button" className="text-button" onClick={() => void load()}>
            {copy.refresh}
          </button>
        )}
      </section>
    );

  const clock = transferCountdown(view.deadline_at, now);
  const proof = view.proof;
  return (
    <section data-testid="bank-transfer" data-state={view.state} aria-labelledby="bank-transfer-title">
      <h2 id="bank-transfer-title">{copy.title}</h2>
      <p className="order-note sf-bank-fraud">{browseCopy[locale].fraudHint} <a href={`/${locale}/legal/anti-fraud`}>{browseCopy[locale].fraud}</a></p>
      <p role="status" data-testid="transfer-state">
        {copy.stateLabel}: <strong>{copy.states[view.state]}</strong>
      </p>
      {/* The shop's account and the amount due only while the buyer may still transfer: a confirmed, cancelled or expired order never asks again. */}
      {view.bank && open(view.state) && (
        <>
          <p data-testid="transfer-amount">
            {copy.amountDue}: <strong>{money(view.amount_minor, view.currency)}</strong>
          </p>
          <dl className="order-breakdown" data-testid="transfer-bank">
            <h3>{copy.bankHeading}</h3>
            <div>
              <dt>{copy.bankName}</dt>
              <dd>{view.bank.bank_name}</dd>
            </div>
            {view.bank.branch && (
              <div>
                <dt>{copy.branch}</dt>
                <dd>{view.bank.branch}</dd>
              </div>
            )}
            <div>
              <dt>{copy.accountName}</dt>
              <dd>{view.bank.account_name}</dd>
            </div>
            <div>
              <dt>{copy.accountNumber}</dt>
              <dd data-testid="transfer-account-number">{view.bank.account_number}</dd>
            </div>
          </dl>
        </>
      )}
      {open(view.state) && (
        <p data-testid="transfer-deadline">
          {copy.deadline} {displayTime(locale, view.deadline_at)} ({copy.storeTime}) ·{" "}
          {clock.expired ? copy.windowEnded : copy.timeLeft(clock.hours, clock.minutes)}
        </p>
      )}
      {open(view.state) && <p className="order-note">{copy.transferNote}</p>}
      {view.state === "REJECTED" && view.reject_reason && (
        <p role="alert" data-testid="transfer-rejected">
          {copy.rejectedPrefix}: {view.reject_reason} · {copy.rejectedHelp}
        </p>
      )}
      {view.state === "CONFIRMED" && <p role="status">{copy.confirmed}</p>}
      {view.state === "EXPIRED" && <p role="status">{copy.expired}</p>}
      {view.state === "REFUNDED_OFFLINE" && <p role="status">{copy.refunded}</p>}
      {proof && (
        <p data-testid="transfer-proof">
          {copy.yourDetails}: ···{proof.last5} · {money(proof.amount_minor, view.currency)} · {displayTime(locale, proof.paid_at)} · {copy.storeTime}
        </p>
      )}
      {open(view.state) && !clock.expired && (
        <form
          data-testid="transfer-form"
          onSubmit={(event) => {
            event.preventDefault();
            void submit();
          }}
        >
          <h3>{proof ? copy.proofEdit : copy.proofHeading}</h3>
          <label>
            <span>{copy.last5}</span>
            <input
              name="last5"
              inputMode="numeric"
              autoComplete="off"
              maxLength={5}
              pattern="[0-9]{5}"
              required
              value={last5}
              onChange={(event) => setLast5(event.target.value.replace(/[^0-9]/g, ""))}
            />
          </label>
          <label>
            <span>
              {copy.amountSent} ({view.currency})
            </span>
            <input
              name="amount"
              inputMode="decimal"
              autoComplete="off"
              required
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
            />
          </label>
          <label>
            <span>{copy.paidAt}</span>
            <input name="paid_at" type="datetime-local" aria-describedby={inputTimeZoneID} required value={paidAt} onChange={(event) => setPaidAt(event.target.value)} />
          </label>
          <p id={inputTimeZoneID}>{copy.paidAtZone(inputTimeZone)}</p>
          <button type="submit" data-testid="transfer-send" disabled={sending}>
            {sending ? copy.sending : copy.send}
          </button>
          {message === "sent" && <p role="status">{copy.sent}</p>}
          {message === "invalid" && <p role="alert">{copy.invalidInput}</p>}
          {message === "failed" && <p role="alert">{copy.sendFailed}</p>}
          {refusal && <p role="alert">{copy.errors[refusal]}</p>}
        </form>
      )}
      <button type="button" className="text-button" data-testid="transfer-refresh" onClick={() => void load()}>
        {copy.refresh}
      </button>
    </section>
  );
}
