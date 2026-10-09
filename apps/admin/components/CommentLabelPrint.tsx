// Purpose: browser-only keyword label selection/preview and A3 print facts; no label-content persistence.
// Depends on: existing inboxWrite/CSRF transport, transient stream rows, React portals and native print CSS.
// Used by: CommentStream's toolbar and row entry points; scope/reset/hide remount clears private state.
"use client";
import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { displayTime } from "@live-commerce/format";
import type { Locale } from "@live-commerce/i18n";
import { inboxWrite, InboxError } from "@/lib/inbox-client";
import type { StreamComment } from "@/src/features/live/comment-model";
import "@/app/print.css";

const copy = {
  "zh-TW": { select: "選取標籤", single: "列印標籤", preview: "預覽標籤", title: "留言標籤", paper: "紙張", label: "60 × 40 mm 標籤", a4: "A4・三列", print: "列印", close: "關閉預覽", printed: "已印", pending: "記錄中…", failed: "部分列印記錄未成功，可繼續列印；未確認的記錄不會顯示為已印。", note: "僅記錄列印操作，無法確認紙張是否印出。標籤內容不會上傳。", denied: "需要直播管理權限才能列印。", unnamed: "未提供名稱", session: "場次", taipei: "台北時間" },
  "zh-CN": { select: "选取标签", single: "打印标签", preview: "预览标签", title: "评论标签", paper: "纸张", label: "60 × 40 mm 标签", a4: "A4・三列", print: "打印", close: "关闭预览", printed: "已打印", pending: "记录中…", failed: "部分打印操作未成功记录，可继续打印；未确认的记录不会显示为已打印。", note: "仅记录打印操作，无法确认纸张是否打印成功。标签内容不会上传。", denied: "需要直播管理权限才能打印。", unnamed: "未提供名称", session: "场次", taipei: "台北时间" },
  en: { select: "Select label", single: "Print label", preview: "Preview labels", title: "Comment labels", paper: "Paper", label: "60 × 40 mm labels", a4: "A4 · three columns", print: "Print", close: "Close preview", printed: "Printed", pending: "Recording…", failed: "Some print actions were not recorded. Printing is still available; unconfirmed records are not marked as printed.", note: "Records the print action, not physical printer success. Label content is not uploaded.", denied: "Live management permission is required to print.", unnamed: "Name unavailable", session: "Session", taipei: "Taipei time" },
};
type PrintControls = {
  c: typeof copy.en; allowed: boolean; busy: boolean; selected: string[]; counts: Record<string, number>;
  eligible: StreamComment[]; toggle: (ref: string) => void; preview: (refs: string[]) => void;
};
const Controls = createContext<PrintControls | null>(null);
const printable = (row: StreamComment) => !!row.marks.claim?.keyword && (row.marks.claim.quantity ?? 0) > 0;

/** Keep label data in current stream memory; only a validated paper-size preference crosses storage. */
export function CommentLabelPrint({ locale, store, session, rows, allowed, active, onDenied, children }: {
  locale: Locale; store: string; session: string; rows: StreamComment[]; allowed: boolean; active: boolean;
  onDenied: () => void; children: ReactNode;
}) {
  const c = copy[locale], eligible = active ? rows.filter(printable) : [];
  const [selected, setSelected] = useState<string[]>([]), [preview, setPreview] = useState<string[]>([]);
  const [paper, setPaper] = useState<"label" | "a4">("label"), [counts, setCounts] = useState<Record<string, number>>({});
  const [busy, setBusy] = useState(false), [failed, setFailed] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null), opener = useRef<HTMLElement | null>(null);
  const controller = useRef(new AbortController()), running = useRef(false), pending = useRef(new Map<string, string>());
  const current = useRef({ active, allowed, eligible }); current.current = { active, allowed, eligible };
  const selectedRows = eligible.filter((row) => selected.includes(row.ref));
  const labels = preview.map((ref) => eligible.find((row) => row.ref === ref)).filter((row): row is StreamComment => !!row);
  useEffect(() => {
    try { const saved = localStorage.getItem("comment-label-paper"); if (saved === "a4" || saved === "label") setPaper(saved); } catch { /* Storage is optional. */ }
    return () => { controller.current.abort(); pending.current.clear(); };
  }, []);
  useEffect(() => {
    if (!active || !allowed || labels.length === 0) { dialog.current?.close(); return; }
    if (!dialog.current?.open) dialog.current?.showModal();
  }, [active, allowed, labels.length]);
  const close = () => { setPreview([]); setFailed(false); opener.current?.focus(); };
  const show = (refs: string[]) => {
    if (!allowed || !active || running.current) return;
    opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setFailed(false); setPreview(refs.filter((ref) => eligible.some((row) => row.ref === ref)));
  };
  const print = async () => {
    if (running.current || !allowed || !active || !labels.length) return;
    running.current = true; setBusy(true); setFailed(false);
    let failure = false;
    // Only refs survive awaits; every label is still derived from the currently visible stream.
    const refs = labels.map((row) => row.ref), signal = controller.current.signal;
    const valid = () => !signal.aborted && current.current.active && current.current.allowed && document.visibilityState === "visible" && refs.every((ref) => current.current.eligible.some((row) => row.ref === ref));
    for (const ref of refs) {
      if (!valid()) break;
      const key = pending.current.get(ref) ?? crypto.randomUUID(); pending.current.set(ref, key);
      try {
        // live-console-v1 §7.4 A3 stores only a print fact, never label names/text/quantities.
        const result = await inboxWrite<{ print_count: number; last_printed_at: string | null }>(store, `live-sessions/${session}/comments/${ref}/print`, {}, key, signal);
        if (!Number.isSafeInteger(result.print_count) || result.print_count < 1) throw new InboxError("retry_later", 503);
        if (!valid()) break;
        pending.current.delete(ref);
        setCounts((old) => ({ ...old, [ref]: result.print_count }));
      } catch (e) {
        failure = true;
        // A scoped 404 is authority loss too, not a recoverable print-record failure.
        if (e instanceof InboxError && [401, 403, 404].includes(e.status)) { onDenied(); return; }
        // A lost ACK retains its key in memory. A later explicit print never blindly repeats that fact.
        if (e instanceof InboxError && e.status >= 400 && e.status < 500) pending.current.delete(ref);
      }
    }
    if (!valid()) {
      if (!signal.aborted) { setBusy(false); running.current = false; }
      return;
    }
    setFailed(failure); setBusy(false); running.current = false;
    window.print();
  };
  const controls: PrintControls = { c, allowed: allowed && active, busy, selected: selectedRows.map((row) => row.ref), counts, eligible,
    toggle: (ref) => setSelected((old) => old.includes(ref) ? old.filter((id) => id !== ref) : [...old, ref]), preview: show };
  return <Controls.Provider value={controls}>{children}{active && allowed && labels.length > 0 && createPortal(
    <dialog className="comment-label-portal" ref={dialog} aria-labelledby="comment-label-title" onCancel={(event) => { event.preventDefault(); if (!busy) close(); }}>
      <div className="comment-label-tools">
        <h2 id="comment-label-title">{c.title} · {labels.length}</h2>
        <p>{c.note}</p>
        <label>{c.paper}<select data-testid="comment-label-paper" value={paper} disabled={busy} onChange={(event) => {
          const size = event.target.value === "a4" ? "a4" : "label"; setPaper(size);
          try { localStorage.setItem("comment-label-paper", size); } catch { /* Print still works without storage. */ }
        }}><option value="label">{c.label}</option><option value="a4">{c.a4}</option></select></label>
        <div className="comment-label-actions"><button type="button" data-testid="comment-label-print" className="primary" disabled={busy} onClick={() => void print()}>{busy ? c.pending : c.print}</button><button type="button" data-testid="comment-label-close" disabled={busy} onClick={close}>{c.close}</button></div>
        {failed && <p role="status" data-testid="comment-label-status">{c.failed}</p>}
      </div>
      <div className="comment-label-sheet" data-testid="comment-label-sheet" data-paper={paper}>
        {labels.map((row) => <article className="comment-label" data-testid="comment-label" key={row.ref}>
          <strong className="comment-label-author" data-label-ref={row.ref}>{row.author_name ?? c.unnamed}</strong>
          <span className="comment-label-keyword">{row.marks.claim!.keyword} × {row.marks.claim!.quantity}</span>
          <time>{displayTime(locale, row.created_at)} · {c.taipei}</time>
          <span>{c.session} {session.slice(0, 8)}</span>
        </article>)}
      </div>
    </dialog>, document.body)}</Controls.Provider>;
}

/** Entry point next to existing stream filters; no second comment/read/write implementation. */
export function CommentLabelToolbar() {
  const v = useContext(Controls); if (!v || !v.eligible.length) return null;
  return <div className="comment-label-toolbar"><button type="button" data-testid="comment-label-preview" disabled={!v.allowed || v.busy || !v.selected.length} onClick={() => v.preview(v.selected)}>{v.c.preview} ({v.selected.length})</button>{!v.allowed && <span className="live-helper">{v.c.denied}</span>}</div>;
}

/** Keyword-row checkbox, single-print entry and confirmed count; no optimistic badge on failure. */
export function CommentLabelAction({ row }: { row: StreamComment }) {
  const v = useContext(Controls); if (!v || !printable(row)) return null;
  const count = Math.max(row.marks.printed?.count ?? 0, v.counts[row.ref] ?? 0);
  return <div className="comment-label-actions"><label><input type="checkbox" data-testid={`comment-label-select-${row.ref}`} checked={v.selected.includes(row.ref)} disabled={!v.allowed || v.busy} onChange={() => v.toggle(row.ref)} />{v.c.select}</label><button type="button" data-testid={`comment-label-single-${row.ref}`} disabled={!v.allowed || v.busy} onClick={() => v.preview([row.ref])}>{v.c.single}</button>{count > 0 && <span data-testid={`comment-label-count-${row.ref}`}>{v.c.printed} ×{count}</span>}</div>;
}
