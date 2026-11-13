// Purpose: capability-fenced buyer apply/read/withdraw controller for the reviewed eight-frame slice.
// Depends on: existing buyerRequest/session, Order, closed return grammar, StorefrontState and Detail.
// Called by: localized orders/[orderID]/return. Invariants: one immutable keyed attempt, no POST retry,
// no protected facts before active context, persisted GET only for detail, UNKNOWN blocks new commands while mounted.
"use client";
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import { buyerRequest, BuyerClientError, definiteError, readBuyerSession } from "../../lib/buyer-client";
import { readOrder, type Order } from "../../lib/purchase";
import { RETURN_REQUEST_CODES, validReturnRequestWithdraw } from "../../lib/return-request-contract";
import { StorefrontState } from "../state/StorefrontState";
import ReturnRequestDetail from "./ReturnRequestDetail";
import { inputProjection, requestProjection, requestListProjection, canWithdraw, resolvesAttempt, type ReturnRequest } from "./return-request-state";
import { returnRequestCopy } from "./return-request-copy";
import "../../../../packages/ui/src/tokens.css";
import styles from "./ReturnRequest.module.css";
type Attempt = { key: string; body: string; context: string; suffix: string; requestID?: string };
type View = { context: string; order: Order; detail: ReturnRequest | null };
export default function ReturnRequestForm({ locale, orderID, requestID }: { locale: Locale; orderID: string; requestID?: string }) {
  const c = returnRequestCopy[locale];
  const [view, setView] = useState<View | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "failed" | "forbidden">("loading");
  const [busy, setBusy] = useState(false);
  const [unknown, setUnknown] = useState(false);
  const [error, setError] = useState("");
  const [choices, setChoices] = useState<ReturnRequest[] | null>(null);
  const [selected, setSelected] = useState<Record<string, string>>({});
  const [note, setNote] = useState("");
  const epoch = useRef(0);
  const working = useRef(false);
  const attempt = useRef<Attempt | null>(null);
  const selectedID = useRef(requestID);
  const mounted = useRef(false);
  const errorBox = useRef<HTMLParagraphElement>(null);
  function denied(reason: unknown) {
    return reason instanceof BuyerClientError && ([401, 403, 404].includes(reason.status ?? 0) || reason.code === "requires_reset" || reason.code === "context_changed");
  }
  async function current(context: string) {
    const session = await readBuyerSession();
    if (session.state !== "active" || session.context !== context) throw new BuyerClientError("context_changed");
  }
  async function load(id = selectedID.current) {
    if (working.current) return;
    working.current = true;
    const mine = ++epoch.current;
    setBusy(true); setView(null); setState("loading"); setError(""); setChoices(null);
    try {
      const session = await readBuyerSession();
      if (session.state !== "active" || !session.context) throw new BuyerClientError("requires_reset");
      if (attempt.current && attempt.current.context !== session.context) throw new BuyerClientError("context_changed");
      const order = await readOrder(session.context, orderID);
      let detail: ReturnRequest | null = null;
      let items: ReturnRequest[] | null = null;
      const exact = id ?? attempt.current?.requestID;
      const response = exact
        ? await buyerRequest("GET", `return-requests/${exact}`, session.context)
        : await buyerRequest("GET", "return-requests", session.context);
      if (response) {
        if (response.status !== 200) throw new BuyerClientError("request_failed", response.status);
        const data: unknown = await response.json();
        if (exact) {
          detail = requestProjection(data, orderID, exact);
          if (!detail) throw new BuyerClientError("invalid_response");
        } else {
          items = requestListProjection(data, orderID);
          if (!items) throw new BuyerClientError("invalid_response");
        }
      }
      await current(session.context);
      if (!mounted.current || mine !== epoch.current) return;
      // A list or selected request cannot prove which create attempt committed after ACK loss.
      if (detail && attempt.current?.requestID === detail.id && resolvesAttempt(attempt.current.suffix.endsWith("/withdraw") ? "withdraw" : "create", detail)) {
        attempt.current = null; setUnknown(false);
      }
      selectedID.current = detail?.id ?? id;
      setView({ context: session.context, order, detail }); setChoices(attempt.current ? items : null); setState("ready");
    } catch (reason) {
      if (mounted.current && mine === epoch.current) {
        setView(null); setChoices(null); setState(denied(reason) ? "forbidden" : "failed");
        if (denied(reason)) { setNote(""); setSelected({}); }
      }
    } finally {
      working.current = false;
      if (mounted.current && mine === epoch.current) setBusy(false);
      else if (mounted.current && !document.hidden) void load();
    }
  }
  useEffect(() => {
    mounted.current = true; document.documentElement.lang = locale; void load();
    // Invalidate visible facts immediately when leaving this context. Focus re-reads capability.
    const hide = () => { epoch.current++; setView(null); setChoices(null); setState("loading"); };
    const focus = () => { if (!working.current) void load(); };
    window.addEventListener("blur", hide); window.addEventListener("focus", focus);
    const visibility = () => { if (document.hidden) hide(); else focus(); };
    const storageChanged = (event: StorageEvent) => {
      if (event.key === null || event.key === "commerce-buyer-pending-v1") { hide(); focus(); }
    };
    window.addEventListener("storage", storageChanged);
    document.addEventListener("visibilitychange", visibility);
    return () => { mounted.current = false; epoch.current++; window.removeEventListener("blur", hide); window.removeEventListener("focus", focus); window.removeEventListener("storage", storageChanged); document.removeEventListener("visibilitychange", visibility); };
  }, [orderID, locale]);
  useEffect(() => { if (error) errorBox.current?.focus(); }, [error]);
  async function mutate(body: unknown, suffix: string, expectedStatus: number, exactID?: string) {
    if (!view || working.current || attempt.current) return;
    const context = view.context;
    const mine = epoch.current;
    working.current = true; setBusy(true); setError("");
    let refresh = false;
    try {
      await current(context);
      if (!mounted.current || mine !== epoch.current) return;
      const frozen: Attempt = Object.freeze({ key: crypto.randomUUID(), body: JSON.stringify(body), context, suffix, requestID: exactID });
      attempt.current = frozen; setUnknown(true);
      const response = await buyerRequest("POST", frozen.suffix, frozen.context, JSON.parse(frozen.body), frozen.key);
      if (response.status !== expectedStatus) {
        if ([401, 403, 404].includes(response.status)) throw new BuyerClientError("request_failed", response.status);
        const definitive = response.status >= 400 && response.status < 500 && await definiteError(response.clone());
        if (definitive) {
          const envelope = await response.json() as { code: string };
          const code = envelope.code;
          const status = ["window_closed", "request_open", "version_changed", "invalid_state", "not_shipped", "not_returnable", "conflict"].includes(code) ? 409
            : ["unknown_line", "ambiguous_line", "exceeds_shipped", "invalid_quantities", "invalid_request"].includes(code) ? 422
            : code === "invalid_json" ? 400 : code === "json_required" ? 415 : code === "method_not_allowed" ? 405 : 0;
          if (status === response.status && (RETURN_REQUEST_CODES.includes(code) || Object.hasOwn(c.errors, code))) {
            await current(context);
            if (!mounted.current || mine !== epoch.current) return;
            attempt.current = null; setUnknown(false); setError(c.errors[code] ?? c.failed);
            refresh = code === "version_changed" || code === "invalid_state";
            return;
          }
        }
        throw new BuyerClientError("uncertain");
      }
      const saved = requestProjection(await response.json(), orderID, exactID);
      if (!saved || saved.status !== (exactID ? "withdrawn" : "requested")) throw new BuyerClientError("invalid_response");
      await current(context);
      if (!mounted.current || mine !== epoch.current) return;
      // A validated command ACK supplies the ID; only a subsequent persisted GET supplies displayed facts.
      attempt.current = Object.freeze({ ...frozen, requestID: saved.id });
      selectedID.current = saved.id; refresh = true;
    } catch (reason) {
      if (mounted.current && mine === epoch.current) {
        if (denied(reason)) { setView(null); setChoices(null); setNote(""); setSelected({}); setState("forbidden"); }
        else if (!attempt.current) setError(c.failed);
        else setUnknown(true);
      }
    } finally {
      working.current = false;
      if (mounted.current) {
        setBusy(false);
        if (refresh || mine !== epoch.current) void load();
      }
    }
  }
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const body = inputProjection({ reason: "damaged", note, lines: Object.entries(selected).map(([sku_id, quantity]) => ({ sku_id, quantity: /^\d+$/.test(quantity) ? Number(quantity) : NaN })) });
    if (!body) { setError(c.invalid); return; }
    void mutate(body, `orders/${orderID}/return-requests`, 201);
  }
  const locked = busy || unknown;
  return <main className={styles.root} lang={locale} aria-busy={busy}>
    <div className={view?.detail ? styles.detail : styles.apply}>
      <a className={styles.back} href={`/${locale}/orders/${orderID}`}>{c.back}</a>
      {state === "loading" && <p role="status">{c.loading}</p>}
      {(state === "failed" || state === "forbidden") && <StorefrontState locale={locale} variant={state === "forbidden" ? "forbidden" : "offline"} onRead={state === "failed" ? () => void load() : undefined} />}
      {error && <p className={styles.notice} role="alert" tabIndex={-1} ref={errorBox}>{error}</p>}
      {state === "ready" && view && <>
        {unknown && <><p className={styles.notice} role="status">{c.unknown}</p><button type="button" className={styles.secondary} disabled={busy} onClick={() => void load()}>{c.check}</button></>}
        {choices && <section className={styles.card}><h2>{c.choose}</h2>{choices.length === 0 && <p>{c.noMatches}</p>}{choices.map(item => <button type="button" className={styles.secondary} key={item.id} disabled={busy} onClick={() => void load(item.id)}>{item.number}</button>)}</section>}
        {view.detail ? <ReturnRequestDetail locale={locale} request={view.detail} order={view.order} disabled={locked}
          onWithdraw={() => { const body = { expected_version: 1 }; if (view.detail && canWithdraw(view.detail) && validReturnRequestWithdraw(body)) void mutate(body, `return-requests/${view.detail.id}/withdraw`, 200, view.detail.id); }} />
          : <><h1>{c.title}</h1><form className={styles.card} onSubmit={submit}>
            <fieldset disabled={locked}><legend>{c.select}</legend>
              {view.order.snapshot.quote.lines.map(line => <div className={styles.line} key={line.sku_id}>
                <label className={styles.selection}><input type="checkbox" checked={Object.hasOwn(selected, line.sku_id)} onChange={event => setSelected(previous => { const next = { ...previous }; if (event.target.checked) next[line.sku_id] = "1"; else delete next[line.sku_id]; return next; })} />{line.name} · {line.code} × {line.quantity}</label>
                {Object.hasOwn(selected, line.sku_id) && <label>{c.quantity}<input type="number" min="1" step="1" value={selected[line.sku_id]} onChange={event => setSelected(previous => ({ ...previous, [line.sku_id]: event.target.value }))} required /></label>}
              </div>)}
              <label>{c.reason}<select defaultValue="damaged"><option value="damaged">{c.damaged}</option></select></label>
              <label>{c.note}<textarea value={note} onChange={event => setNote(event.target.value)} aria-describedby="return-photos" /></label>
              <p id="return-photos">{c.photos}</p><p className={styles.notice}>{c.disclaimer}</p>
              <button className={styles.primary} type="submit">{c.submit}</button>
            </fieldset>
          </form></>}
      </>}
    </div>
  </main>;
}
