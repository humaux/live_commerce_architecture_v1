// Purpose: M14 queue, order-line display and exact readback for buyer return decisions.
// Depends on: existing returns and order reads, session boundary and design system.
// Used by: ReturnsList. The same store and session fence apply to all selected-case data.
"use client";

import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { Badge, Button, ButtonLink, TextInput } from "@live-commerce/ui/ds";
import { ReadError, useGuardedRead, type ReadCode } from "@/lib/customers-client";
import type { Store } from "@/lib/model";
import { postBuyerReturnDecision, readBuyerReturnRequest, readBuyerReturnRequests, readReturnOrderLines } from "@/lib/returns-client";
import { buyerRequestCopy, returnsCopy, returnsError } from "@/lib/returns-copy";
import { closeBody, rejectRequestBody, type BuyerReturnRequest } from "@/lib/returns-model";
import { sessionBoundary } from "@/lib/settings-client";
import { shortOrderNumber } from "./orders/parity-copy";
import styles from "./buyer-return-workbench.module.css";

type Queue = "pending" | "closed";
type Review = {
  application: BuyerReturnRequest;
  action: "approve" | "reject";
  boundary: string;
  check?: "certain" | "uncertain";
};

const caseNumber = (request: BuyerReturnRequest) => `R-${request.number.slice(-8).toUpperCase()}`;
const queueOf = (request: BuyerReturnRequest): Queue => request.status === "requested" ? "pending" : "closed";
const count = (request: BuyerReturnRequest) => request.lines.reduce((total, line) => total + line.quantity, 0);

export function BuyerReturnWorkbench({ locale, store, initialError, refreshRmas, requestsHref }: {
  locale: Locale;
  store: Store;
  initialError: ReadCode | null;
  refreshRmas: () => Promise<boolean>;
  requestsHref: string;
}) {
  const c = returnsCopy[locale];
  const copy = buyerRequestCopy[locale];
  const list = useGuardedRead(store.id, (signal) => readBuyerReturnRequests(store.id, signal), initialError);
  const [queue, setQueue] = useState<Queue>("pending");
  const [selectedId, setSelectedId] = useState("");
  const [review, setReview] = useState<Review | null>(null);
  const [reason, setReason] = useState("");
  const [notice, setNotice] = useState("");
  const [problem, setProblem] = useState("");
  const [busy, setBusy] = useState(false);
  const sending = useRef(false);
  const focus = useRef<HTMLParagraphElement>(null);
  const reasonLabel = useRef<HTMLLabelElement>(null);
  const detailTitle = useRef<HTMLHeadingElement>(null);
  const queueList = useRef<HTMLUListElement>(null);
  const boundary = list.status === "ready" ? list.boundary : "";
  const active = review?.boundary === boundary ? review : null;
  const canDecide = store.permissions?.includes("fulfillment:write") === true;
  const applications = list.data ?? [];
  const pendingCount = applications.filter((item) => queueOf(item) === "pending").length;
  const rows = applications.filter((item) => queueOf(item) === queue);
  const selected = active
    ? applications.find((item) => item.id === active.application.id) ?? active.application
    : rows.find((item) => item.id === selectedId) ?? rows[0];
  const orderLines = useGuardedRead(
    `${boundary}|${store.id}|${selected?.order_id ?? ""}`,
    boundary && selected ? (signal) => readReturnOrderLines(store.id, selected.order_id, signal) : null,
    null,
  );
  const quantities = new Map<string, number>();
  for (const line of selected?.lines ?? []) quantities.set(line.sku_id, (quantities.get(line.sku_id) ?? 0) + line.quantity);
  const orderLabel = selected ? shortOrderNumber(selected.order_id) : "";
  useEffect(() => {
    if (active && !active.check) (active.action === "reject" ? reasonLabel.current?.querySelector("input") : focus.current)?.focus();
  }, [active]);
  useEffect(() => {
    const scroller = queueList.current;
    const chosen = scroller?.querySelector('[aria-pressed="true"]');
    if (!scroller || !chosen || scroller.scrollHeight <= scroller.clientHeight) return;
    const frame = scroller.getBoundingClientRect();
    const item = chosen.getBoundingClientRect();
    if (item.top < frame.top) scroller.scrollTop += item.top - frame.top;
    else if (item.bottom > frame.bottom) scroller.scrollTop += item.bottom - frame.bottom;
  }, [queue, selected?.id]);

  function selectQueue(next: Queue) {
    if (active) return;
    setQueue(next);
    setSelectedId("");
    setNotice("");
  }

  function selectCase(id: string) {
    if (active) return;
    setSelectedId(id);
    setNotice("");
  }

  function open(application: BuyerReturnRequest, action: Review["action"]) {
    if (!boundary || active || sending.current) return;
    setNotice("");
    setProblem("");
    setReason("");
    setReview({ application, action, boundary });
  }

  async function readDecision(current: Review, changed: boolean) {
    try {
      if (await sessionBoundary() !== current.boundary) return;
      const saved = await readBuyerReturnRequest(store.id, current.application.id, AbortSignal.timeout(8000));
      if (await sessionBoundary() !== current.boundary) return;
      if ((current.check === "uncertain" && saved.status === "requested") || !await list.refresh()) {
        setProblem(copy.pending);
        return;
      }
      if (saved.return_id) void refreshRmas();
      setNotice(`${caseNumber(saved)} · ${copy.states[saved.status]}${changed ? ` · ${copy.changed}` : ""}`);
      setQueue(queueOf(saved));
      setSelectedId(saved.id);
      setReview(null);
      detailTitle.current?.focus();
    } catch (error) {
      if (error instanceof ReadError && (error.code === "signed-out" || error.code === "forbidden")) list.reload();
      else setProblem(copy.pending);
    }
  }

  async function confirm(current: Review) {
    if (sending.current || !canDecide) return;
    const { action, application } = current;
    const body = action === "reject" ? rejectRequestBody(application.version, reason) : closeBody(application.version);
    if (!current.check && !body) { setProblem(copy.invalidReason); return; }
    sending.current = true;
    setBusy(true);
    setProblem("");
    try {
      if (current.check || !body) {
        await readDecision(current, false);
        return;
      }
      const result = await postBuyerReturnDecision(store.id, application.id, action, crypto.randomUUID(), body, current.boundary);
      const changed = !result.ok && ["version_changed", "invalid_state", "conflict"].includes(result.code);
      const uncertain = !result.ok && result.uncertain;
      if (result.ok || uncertain || changed) {
        const pending: Review = { ...current, check: uncertain ? "uncertain" : "certain" };
        setReview(pending);
        await readDecision(pending, changed);
      } else {
        setProblem(returnsError(c, result.code));
        if (result.code === "unauthorized") list.reload();
      }
    } finally {
      sending.current = false;
      setBusy(false);
    }
  }

  const failure = list.status === "signed-out" ? c.signedOut : list.status === "forbidden" ? c.forbidden :
    list.status === "not-found" ? c.noStore : list.status === "unavailable" ? c.unavailable : "";
  return <section className={styles.workbench} data-testid="buyer-return-requests" aria-label={copy.title}>
    <div className={styles.queue} data-testid="buyer-return-queue">
      <h2>{queue === "pending" ? copy.pendingTitle : copy.closedTitle}</h2>
      <div className={styles.tabs} aria-label={copy.queueTitle} role="group">
        <button type="button" aria-pressed={queue === "pending"} aria-disabled={!!active}
          onClick={() => selectQueue("pending")} data-testid="returns-queue-pending">{copy.pendingQueue} {pendingCount}</button>
        <button type="button" aria-pressed={queue === "closed"} aria-disabled={!!active}
          onClick={() => selectQueue("closed")} data-testid="returns-queue-closed">{copy.closedQueue}</button>
      </div>
      {(list.status === "loading" || list.status === "hidden") && <p role="status">{c.loading}</p>}
      {failure && <div role="status"><p>{failure}</p><Button onClick={list.reload}>{c.retry}</Button></div>}
      {list.status === "ready" && rows.length === 0 && <p className={styles.meta}>{copy.empty}</p>}
      {list.status === "ready" && rows.length > 0 && <ul className={styles.cases} ref={queueList}>
        {rows.map((application) => <li key={application.id}>
          <button type="button" className={styles.case} aria-pressed={selected?.id === application.id} aria-disabled={!!active}
            data-testid={`buyer-return-${application.id}`} onClick={() => selectCase(application.id)}>
            <span className={styles.caseLine}><strong>{caseNumber(application)}</strong>
              <span className={styles.meta} data-state={application.status}>{copy.states[application.status]}</span>
            </span>
            <span className={styles.meta}>{copy.quantity(count(application))} · {shortOrderNumber(application.order_id)}</span>
          </button>
        </li>)}
      </ul>}
    </div>

    {boundary && selected && <div className={styles.detailColumn}>
      <article className={styles.detail} data-testid="buyer-return-detail" aria-labelledby="return-case-title">
        <h2 id="return-case-title" tabIndex={-1} ref={detailTitle}>{copy.damageCase} · {copy.quantity(count(selected))}</h2>
        <div className={styles.caseLine}>
          <span><strong data-testid="returns-order-number">{orderLabel}</strong><span className={styles.caseRef}>{caseNumber(selected)}</span></span>
          <span data-state={selected.status}>
            <Badge tone={selected.status === "requested" ? "warning" : selected.status === "approved" ? "success" : "neutral"}>
              {copy.states[selected.status]}
            </Badge>
          </span>
        </div>

        {orderLines.status === "ready" && orderLines.data && <ul className={styles.items} data-testid="returns-order-lines">
          {[...quantities].map(([sku, quantity]) => {
            const line = orderLines.data?.find((item) => item.sku_id === sku);
            return <li key={sku} className={styles.caseLine}>
              <div>
                <strong>{line?.name ?? copy.orderLineUnavailable}</strong>
                {line && <p className={styles.meta}>{line.code} · {copy.ordered(line.quantity)}</p>}
              </div>
              {line && <span className={styles.itemCount}>×{quantity}</span>}
            </li>;
          })}
        </ul>}
        {orderLines.status === "loading" && <p role="status" className={styles.meta}>{c.loading}</p>}
        {orderLines.status !== "ready" && orderLines.status !== "loading" && <div className={styles.caseLine}>
          <p className={styles.meta}>{copy.orderLineUnavailable}</p>
          {orderLines.status === "unavailable" && <Button onClick={orderLines.reload}>{c.retry}</Button>}
        </div>}

        <div className={styles.feedback}>
          <h3>{copy.feedback}</h3>
          <p>{selected.note || copy.noFeedback}</p>
        </div>
        {selected.rejection_reason && <div className={styles.rejection} data-testid="returns-rejection-reason">
          <h3>{copy.reason}</h3><p>{selected.rejection_reason}</p>
        </div>}
        <ButtonLink size="lg" className={styles.original} data-testid="returns-original-order"
          href={`/${locale}/orders?store=${store.id}&state=all&order=${selected.order_id}`}>{copy.openOrder(orderLabel)}</ButtonLink>
        {selected.return_id && selected.return_state && <a href={`${requestsHref}&rma=${selected.return_id}`}>{copy.rma} · {c.states[selected.return_state]}</a>}
      </article>

      {(active || canDecide && selected.status === "requested" || notice) && <section className={styles.detail}
        data-testid="returns-review-panel" aria-label={copy.reviewTitle}>
        <h2>{copy.reviewTitle}</h2>
        {notice && <p role="status">{notice}</p>}
        {canDecide && selected.status === "requested" && !active && <div className={styles.tabs}>
          <Button size="lg" variant="primary" onClick={() => open(selected, "approve")}>{copy.approve}</Button>
          <Button size="lg" onClick={() => open(selected, "reject")}>{copy.reject}</Button>
        </div>}
        {active && <form className={styles.form} onSubmit={(event) => { event.preventDefault(); void confirm(active); }}>
          {!active.check && <p ref={focus} tabIndex={-1}>{active.action === "approve" ? copy.approveHint : copy.rejectHint}</p>}
          {!active.check && active.action === "reject" && <label ref={reasonLabel}>{copy.reason}
            <TextInput value={reason} onChange={(event) => setReason(event.target.value)} required />
          </label>}
          {problem && <p role="alert">{problem}</p>}
          <div className={styles.tabs}>
            {!active.check && <Button size="lg" disabledReason={busy ? c.sending : undefined} onClick={() => {
              setReview(null);
              detailTitle.current?.focus();
            }}>{c.cancel}</Button>}
            <Button size="lg" type="submit" variant="primary" loading={busy} loadingLabel={active.check ? copy.checking : c.sending}>
              {active.check ? copy.check : active.action === "approve" ? copy.approve : copy.reject}
            </Button>
          </div>
        </form>}
      </section>}
    </div>}
  </section>;
}
