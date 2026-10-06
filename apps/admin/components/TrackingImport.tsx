"use client";

// Purpose: protected-focus bulk tracking workflow, preserving the exact checked CSV until confirmation/recovery.
// Depends on: tracking client/model/copy, CSRF session fence, native dialog and incumbent orders tokens.
// Used by: MerchantOrders; BFF tools/shipments/tracking-import -> Go internal/httpapi/merchanttools.go.
// Direction: Operate; extend the existing ledger. Input -> failure-first rows -> explicit shipment confirmation.
// Invariants: no CSV/PII in storage; only scoped hash/count recovery metadata; no automatic shipping request.
import { useEffect, useRef, useState, type CSSProperties, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { sendTracking, trackingResultHref } from "@/lib/tracking-import-client";
import { pasteTrackingCSV, TRACKING_MAX_BYTES, trackingOrderNumber, trackingRows, trackingTemplate, type TrackingCommit, type TrackingPreview } from "@/lib/tracking-import-model";
import { trackingCopy } from "@/lib/tracking-import-copy";
import "./tracking-import.css";

type Attempt = { v: 1; boundary: string; hash: string; expected: number };
const attemptKey = (store: string) => `commerce-tracking-import-v1:${store}`;
function readAttempt(store: string, boundary: string): Attempt | null {
  const raw = sessionStorage.getItem(attemptKey(store));
  if (!raw) return null;
  const value: unknown = JSON.parse(raw);
  if (!value || typeof value !== "object") throw new Error("storage_unavailable");
  const a = value as Attempt;
  if (Object.keys(a).sort().join() !== "boundary,expected,hash,v" || a.v !== 1 || !/^[0-9a-f]{64}$/.test(a.hash) || !Number.isInteger(a.expected) || a.expected < 1 || a.expected > 500) throw new Error("storage_unavailable");
  // Metadata from another sign-in is not a recovery capability for this actor.
  return a.boundary === boundary ? a : null;
}
function saveAttempt(store: string, value: Attempt) {
  const raw = JSON.stringify(value);
  sessionStorage.setItem(attemptKey(store), raw);
  if (sessionStorage.getItem(attemptKey(store)) !== raw) throw new Error("storage_unavailable");
}

/** Open a tracking import for one authenticated store; actual writes occur only after explicit confirmation. */
export function TrackingImport({ locale, store, onComplete }: { locale: Locale; store: { id: string; name: string }; onComplete: () => void }) {
  const c = trackingCopy[locale];
  const [open, setOpen] = useState(false), [mode, setMode] = useState<"upload" | "paste">("upload");
  const [boundary, setBoundary] = useState(""), [blob, setBlob] = useState<Blob | null>(null), [hash, setHash] = useState("");
  const [pasted, setPasted] = useState(""), [busy, setBusy] = useState(false), [reading, setReading] = useState(false);
  const [preview, setPreview] = useState<TrackingPreview | null>(null), [result, setResult] = useState<TrackingCommit | null>(null);
  const [attempt, setAttempt] = useState<Attempt | null>(null), [problem, setProblem] = useState(""), [stale, setStale] = useState(false);
  const [theme, setTheme] = useState<CSSProperties>({});
  const dialog = useRef<HTMLDialogElement>(null), file = useRef<HTMLInputElement>(null), working = useRef(false), live = useRef(true), generation = useRef(0), changed = useRef(false);
  const recoverablePaste = mode === "paste" && pasted.trim().length > 0;
  const error = (code: string) => c.errors[code] ?? c.errors.retry_later;
  useEffect(() => { live.current = true; return () => { live.current = false; generation.current++; }; }, []);
  useEffect(() => {
    if (!open) return;
    dialog.current?.showModal();
    let active = true;
    sessionBoundary().then(b => {
      if (!active) return;
      setBoundary(b);
      try { setAttempt(readAttempt(store.id, b)); } catch { setBoundary(""); setProblem(error("storage_unavailable")); }
    }, () => active && setProblem(error("unauthorized")));
    return () => { active = false; };
  }, [open, store.id]);

  async function selectData(data: Uint8Array<ArrayBuffer>) {
    if (data.byteLength === 0) throw new Error("empty");
    if (data.byteLength > TRACKING_MAX_BYTES) throw new Error("too_large");
    try { new TextDecoder("utf-8", { fatal: true }).decode(data); } catch { throw new Error("encoding_not_utf8"); }
    const turn = ++generation.current;
    setReading(true); setBlob(null); setHash(""); setPreview(null); setResult(null); setStale(false); setProblem("");
    try {
      const digest = [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map(b => b.toString(16).padStart(2, "0")).join("");
      if (!live.current || turn !== generation.current) return;
      if (attempt && attempt.hash !== digest) throw new Error("different_file");
      setBlob(new Blob([data], { type: "text/csv;charset=utf-8" })); setHash(digest);
    } finally { if (live.current && turn === generation.current) setReading(false); }
  }
  async function pick(value: File | null) {
    if (!value || working.current) return;
    setBlob(null); setPreview(null); setResult(null); setProblem("");
    try {
      if (value.size > TRACKING_MAX_BYTES) throw new Error("too_large");
      await selectData(new Uint8Array(await value.arrayBuffer()));
    } catch (e) { if (live.current) setProblem(error(e instanceof Error ? e.message : "invalid_csv")); }
  }
  function forget() {
    if (attempt || working.current) return;
    generation.current++; setBlob(null); setHash(""); setPreview(null); setResult(null); setPasted(""); setProblem(""); setStale(false);
    if (file.current) file.current.value = "";
  }
  async function run(mode: "preview" | "commit") {
    if (working.current || !boundary || (mode === "commit" && !blob && !recoverablePaste)) return;
    working.current = true; setBusy(true); setProblem("");
    const turn = generation.current;
    const recovering = attempt !== null;
    let data = blob, currentHash = hash;
    try {
      // Reload retains only hash/count. Explicit recovery rebuilds the pasted bytes, never re-previews.
      if (!data) {
        const bytes = pasteTrackingCSV(pasted);
        currentHash = [...new Uint8Array(await crypto.subtle.digest("SHA-256", bytes))].map(b => b.toString(16).padStart(2, "0")).join("");
        // Keep recovery inputs editable when the pasted bytes do not match the pending request.
        if (attempt && attempt.hash !== currentHash) throw new Error("different_file");
        data = new Blob([bytes], { type: "text/csv;charset=utf-8" }); setBlob(data); setHash(currentHash);
      }
      if (!data) throw new Error("empty");
      if (mode === "preview") { setPreview(null); setStale(false); }
      const expected = attempt?.expected ?? preview?.apply_rows;
      if (mode === "commit") {
        if (!expected || attempt && attempt.hash !== currentHash) throw new Error("different_file");
        const next: Attempt = { v: 1, boundary, hash: currentHash, expected };
        // Persist no CSV, tracking number or order reference: only the exact recovery binding.
        try { saveAttempt(store.id, next); } catch { throw new Error("storage_unavailable"); }
        setAttempt(next);
      }
      const answer = await sendTracking(store.id, mode, data, boundary, expected);
      if (!live.current || turn !== generation.current) return;
      if (answer.kind === "error") {
        setProblem(error(answer.code));
        // A denial of a retry does not prove the earlier unknown request did not commit.
        if (mode === "commit" && !answer.uncertain && !recovering) { sessionStorage.removeItem(attemptKey(store.id)); setAttempt(null); }
      } else if (answer.kind === "committed") {
        if (preview && answer.value.applied + answer.value.unchanged + answer.value.failed !== preview.rows_total) throw new Error("invalid_response");
        changed.current = true;
        setResult(answer.value); setPreview(null); setStale(false); setAttempt(null);
        // A confirmed receipt remains confirmed even if browser cleanup is unavailable; replay is still safe.
        try { sessionStorage.removeItem(attemptKey(store.id)); } catch { /* no shipping ambiguity introduced */ }
      } else {
        if (answer.value.file_sha256 !== currentHash) throw new Error("invalid_response");
        if (answer.kind === "stale") { sessionStorage.removeItem(attemptKey(store.id)); setAttempt(null); }
        setPreview(answer.value); setStale(answer.kind === "stale");
      }
    } catch (e) { if (live.current) setProblem(error(e instanceof Error ? e.message : "retry_later")); }
    finally { working.current = false; if (live.current) setBusy(false); }
  }
  function close() {
    if (working.current || reading) return;
    dialog.current?.close(); setOpen(false);
    if (changed.current) { changed.current = false; onComplete(); }
  }
  function template() {
    const url = URL.createObjectURL(trackingTemplate());
    const anchor = document.createElement("a"); anchor.href = url; anchor.download = "tracking-template.csv"; anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
  const locked = busy || reading || !!attempt && !!blob;
  function tabKeys(event: KeyboardEvent<HTMLButtonElement>) {
    if (locked || !["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
    event.preventDefault();
    const next = event.key === "Home" ? "upload" : event.key === "End" ? "paste" : mode === "upload" ? "paste" : "upload";
    forget(); setMode(next); document.getElementById(`tracking-${next}-tab`)?.focus();
  }
  return <>
    <button type="button" data-testid="orders-tracking-backfill" onClick={event => {
      forget();
      // The native portal lives outside AppShell, so carry its measured incumbent colors/type with it.
      const css = getComputedStyle(event.currentTarget);
      setTheme({ fontFamily: css.fontFamily, ...Object.fromEntries(["--ink", "--muted", "--line", "--teal", "--focus", "--orders-muted"].map(key => [key, css.getPropertyValue(key)]).filter(([, value]) => value)) } as CSSProperties);
      setOpen(true);
    }}>{c.action}</button>
    {open && createPortal(<dialog ref={dialog} className="trk-dialog" style={theme} aria-labelledby="tracking-title" onCancel={event => { event.preventDefault(); close(); }}>
      <header className="trk-heading"><div><h2 id="tracking-title">{c.action}</h2><p>{store.name}</p></div><button type="button" disabled={busy || reading} onClick={close}>{c.close}</button></header>
      <div className="trk-body">
        {result ? <section aria-live="polite" data-testid="tracking-result"><h3>{c.result}</h3><p>{c.completed(result.applied, result.unchanged, result.failed)}</p>{result.replayed && <p>{c.replayed}</p>}
          {result.failed > 0 && <><a className="trk-button" href={trackingResultHref(store.id, result.batch_id)} download>{c.failed}</a><p className="trk-note">{c.failureHint}</p></>}
          <button type="button" onClick={forget}>{c.change}</button></section> : <>
          <p>{c.intro}</p><p className="trk-note">{c.columns}</p><p className="trk-note">{c.limits}</p>
          {!!attempt && <p className="trk-notice" role="status" data-testid="tracking-uncertain">{blob ? c.uncertain : c.recover}</p>}
          <div className="trk-tabs" role="tablist" aria-label={c.input}>
            <button type="button" role="tab" id="tracking-upload-tab" aria-controls="tracking-upload" aria-selected={mode === "upload"} tabIndex={mode === "upload" ? 0 : -1} onKeyDown={tabKeys} disabled={locked} onClick={() => { forget(); setMode("upload"); }}>{c.upload}</button>
            <button type="button" role="tab" id="tracking-paste-tab" aria-controls="tracking-paste" aria-selected={mode === "paste"} tabIndex={mode === "paste" ? 0 : -1} onKeyDown={tabKeys} disabled={locked} onClick={() => { forget(); setMode("paste"); }}>{c.paste}</button>
          </div>
          <div role="tabpanel" id="tracking-upload" aria-labelledby="tracking-upload-tab" hidden={mode !== "upload"}><label htmlFor="tracking-file">{c.upload}</label><input ref={file} id="tracking-file" type="file" accept=".csv,text/csv" disabled={locked} onChange={event => void pick(event.target.files?.[0] ?? null)} /></div>
          <div role="tabpanel" id="tracking-paste" aria-labelledby="tracking-paste-tab" hidden={mode !== "paste"}><label htmlFor="tracking-pasted">{c.input}</label><textarea id="tracking-pasted" rows={6} value={pasted} disabled={locked} onChange={event => { forget(); setPasted(event.target.value); }} /><p className="trk-note">{c.pasteHint}</p></div>
          <button type="button" className="trk-link" onClick={template}>{c.template}</button>
          {preview && <section data-testid="tracking-preview" aria-live="polite"><h3>{c.summary(preview.apply_rows, preview.unchanged_rows, preview.failed_rows)}</h3>
            {preview.apply_rows > 0 ? <p className="trk-note">{c.mail(preview.mail_eta_hours)}</p> : <p>{c.noChanges}</p>}
            {stale && <p className="trk-notice" role="alert" data-testid="tracking-stale">{c.stale}</p>}
            <table className="trk-table"><thead><tr>{[c.row, c.order, c.carrier, c.tracking, c.status].map(label => <th key={label} scope="col">{label}</th>)}</tr></thead><tbody>
              {trackingRows(preview).map(row => <tr key={row.row} data-testid="tracking-row" data-outcome={row.outcome}><td data-label={c.row}>{row.row}</td><td data-label={c.order}>{trackingOrderNumber(row.order_number).slice(0, 128)}</td>
                <td data-label={c.carrier}>{c.carriers[row.carrier_code as keyof typeof c.carriers] ?? c.unknownCarrier}</td><td data-label={c.tracking}>{row.tracking_number.slice(0, 128)}</td>
                <td data-label={c.status}><span className={`trk-outcome trk-${row.outcome}`}>{c.outcomes[row.outcome]}</span>{row.code && <p className="trk-row-issue">{c.errors[row.code] ?? c.genericRow}</p>}</td></tr>)}
            </tbody></table>
          </section>}
          {problem && <p className="trk-error" role="alert">{problem}</p>}
          <footer className="trk-actions">
            {attempt ? <button type="button" className="trk-primary" disabled={busy || reading || !boundary || !blob && !recoverablePaste || !!blob && hash !== attempt.hash} onClick={() => void run("commit")}>{busy ? c.committing : c.retry}</button> : <>
              <button type="button" disabled={busy || reading || !boundary || !blob && !pasted.trim()} onClick={() => void run("preview")}>{busy ? c.checking : reading ? c.reading : c.preview}</button>
              {preview && <><button type="button" className="trk-primary" disabled={busy || !preview.apply_rows || !boundary} onClick={() => void run("commit")}>{busy ? c.committing : c.confirm(preview.apply_rows)}</button><button type="button" disabled={busy} onClick={forget}>{c.change}</button></>}
            </>}
          </footer>
        </>}
      </div>
    </dialog>, document.body)}
  </>;
}
