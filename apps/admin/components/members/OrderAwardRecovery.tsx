// Purpose: review and confirm a missed award against the original order.
// Depends on: scoped recovery and ledger reads, the keyed member command, and DS controls.
// Used by: OrderPointsPanel on the original merchant order.
"use client";
import { useEffect, useRef, useState } from "react";
import { Button } from "@live-commerce/ui/ds";
import { decimal } from "@live-commerce/format";
import type { Locale } from "@live-commerce/i18n";
import { MemberWriteError, recoverOrderAward, type AwardRecovery, type Member, type OrderPointEntries } from "@/lib/members-client";
import { isManualReceiptKey } from "@/lib/merchant-tools-model";
import { sessionBoundary } from "@/lib/settings-client";
import { awardRecoveryCopy, orderPointsCopy } from "./order-points-copy";
import { shortOrderNumber } from "../orders/parity-copy";
import s from "./order-points.module.css";

type Props = {
  locale: Locale; store: string; order: string; boundary: string; permitted: boolean;
  data: (OrderPointEntries & { recovery: AwardRecovery | null; balances: Record<string, Member> }) | null;
  onCheck: () => void;
};

export function OrderAwardRecovery({ locale, store, order, boundary, permitted, data, onCheck }: Props) {
  const c = awardRecoveryCopy[locale], common = orderPointsCopy[locale];
  const keySlot = `manual-award-recovery:${store}:${order}:${boundary}`;
  const key = useRef<string | null>(null);
  const active = useRef<AbortController | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const opener = useRef<HTMLButtonElement>(null);
  const [confirm, setConfirm] = useState(false);
  const [status, setStatus] = useState<"loading" | "ready" | "pending" | "unknown" | "refused">("loading");
  const recovery = data?.recovery;
  const recorded = recovery?.state === "already_awarded" && data &&
    (data.entries.some(row => row.id === recovery.entry_id && row.reason === "manual_order_award" &&
      row.member_id === recovery.member_id && row.delta === recovery.points) || data.truncated && !data.entries.some(row => row.reason === "manual_order_award"));
  const eligible = recovery?.state === "eligible" && !data?.entries.some(row => row.delta > 0);
  const member = eligible ? data?.balances[recovery.member_id] : undefined;

  useEffect(() => {
    try {
      const stored = sessionStorage.getItem(keySlot);
      if (isManualReceiptKey(stored)) key.current = stored;
    } catch { /* This tab still retains the key in memory. */ }
    setStatus(key.current ? "unknown" : "ready");
    return () => active.current?.abort();
  }, [keySlot]);
  useEffect(() => {
    if (recorded) {
      key.current = null;
      try { sessionStorage.removeItem(keySlot); } catch { /* The current read already settled the award. */ }
    }
  }, [recorded, keySlot]);
  useEffect(() => {
    if (!confirm || !dialog.current) return;
    const element = dialog.current;
    element.showModal();
    return () => { element.close(); opener.current?.focus(); };
  }, [confirm, eligible]);

  async function submit() {
    if (!confirm || !eligible || !permitted || active.current || status === "unknown") return;
    const controller = new AbortController();
    active.current = controller;
    const intent = key.current ?? crypto.randomUUID();
    setStatus("pending");
    try {
      if (await sessionBoundary() !== boundary || controller.signal.aborted) {
        setStatus("refused");
        return;
      }
      key.current = intent;
      try { sessionStorage.setItem(keySlot, intent); } catch { /* Retain the same key while this tab remains open. */ }
      const receipt = await recoverOrderAward(store, order, intent, controller.signal);
      if (controller.signal.aborted) return;
      setStatus("unknown");
      if (receipt.member_id === recovery.member_id && receipt.points === recovery.points) onCheck();
    } catch (error) {
      if (controller.signal.aborted) return;
      if (error instanceof MemberWriteError && error.outcome !== "unknown") {
        setStatus("refused");
        onCheck();
      } else setStatus("unknown");
    } finally {
      if (!controller.signal.aborted) {
        active.current = null;
        setConfirm(false);
      }
    }
  }

  if (status === "loading") return null;
  if (recorded) return <p role="status" className={s.success} data-testid="order-award-recorded">{c.recorded} · +{decimal(locale, recovery.points, 0)}</p>;
  if (recovery?.state === "not_eligible" && recovery.reason === "award_not_eligible" && !key.current) return null;
  if (!data && status !== "unknown") return null;
  return <div className={s.recovery} data-testid="order-award-recovery">
    {status === "unknown" ? <><p role="status">{common.unknown}</p>
      <Button type="button" variant="secondary" data-testid="order-award-recovery-check" onClick={() => { onCheck(); setStatus("ready"); }}>{c.check}</Button></>
      : eligible ? <>
        <h3>{c.missed}</h3>
        <p>{member?.display_name ?? recovery.member_id} · {common.points} {decimal(locale, recovery.points, 0)}</p>
        {status === "refused" && <p role="alert">{common.refused}</p>}
        {permitted && <Button ref={opener} type="button" variant="primary" data-testid="order-award-recovery-open" onClick={() => setConfirm(true)}>{c.review}</Button>}
        {confirm && <dialog ref={dialog} className={`${s.dialog} ${s.recoveryDialog}`} aria-labelledby="order-award-recovery-title" onCancel={event => {
          event.preventDefault();
          if (!active.current) setConfirm(false);
        }}>
          <section className={s.panel}>
            <h3 id="order-award-recovery-title">{c.review}</h3>
            <p>{c.confirmBody}</p>
            <dl className={s.recoveryFacts}>
              <div><dt>{c.original}</dt><dd>{shortOrderNumber(order)}</dd></div>
              <div><dt>{common.member}</dt><dd>{member?.display_name ?? recovery.member_id}</dd></div>
              <div><dt>{common.points}</dt><dd>{decimal(locale, recovery.points, 0)}</dd></div>
            </dl>
            <div className={s.actions}>
              <Button type="button" variant="secondary" disabledReason={status === "pending" ? common.loading : undefined} onClick={() => setConfirm(false)}>{common.cancel}</Button>
              <Button type="button" variant="primary" data-testid="order-award-recovery-submit" loading={status === "pending"} onClick={() => void submit()}>{c.submit}</Button>
            </div>
          </section>
        </dialog>}
      </> : <p role="status">{recovery?.state === "not_eligible" && recovery.reason === "member_erased" ? common.reasons.member_erased
        : recovery?.state === "not_eligible" && recovery.reason === "limit_reached" ? common.reasons.limit_reached
        : recovery ? c.hold : common.failed}</p>}
  </div>;
}
