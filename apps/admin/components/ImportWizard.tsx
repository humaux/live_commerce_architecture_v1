// Purpose: explicit customer/readonly-order CSV import steps, with files kept only in the current visible session scope.
// Depends on: import-client -> exact BFF -> Go migrationimport0152/0156, frozen DTOs/copy and settings session fences.
// Used by: /[locale]/customers/import; commits replay the same immutable file/mapping/count after UNKNOWN.
// Invariants: I01 scope, I06 no automatic retry or repeated confirmed-mismatch replay, I11 no raw cells in display/storage/logs, I05 archive never revenue.
"use client";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "../lib/model";
import type { ReadCode } from "../lib/customers-client";
import { csrfCookie, sessionBoundary } from "../lib/settings-client";
import { importFields, requiredImportFields, type ImportKind, type ImportMapping, type ImportPreview, type ImportReceipt } from "../lib/import-model";
import { readImportHeader, guessImportMapping, sendImport, downloadImportFailures } from "../lib/import-client";
import { importCopy } from "../lib/import-copy";
import { importViewCopy } from "../lib/import-view-copy";
import { safeImportFailureCSV } from "../lib/import-view-model";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { ImportColumnMapping, ImportPreviewCounts, ImportVerdictTable } from "./ImportWizardSteps";
import "./orders.css";
import "./import-wizard.css";

const pageHidden = () => document.visibilityState === "hidden";

type Stage = "select" | "upload" | "mapping" | "preview" | "confirm" | "result";
type Attempt = { kind: ImportKind; file: File; mapping: ImportMapping; expected: number; unknown: boolean };

/** Mount no file-bearing form until its current CSRF/session fence is established; hiding/leaving unmounts it. */
export function ImportWizard({ locale, store, initialError, renderKey }: {
  locale: Locale; stores: Store[]; store: Store | null; initialError: ReadCode | null; renderKey: string;
}) {
  const c = importCopy[locale];
  const [scope, setScope] = useState<{ boundary: string; version: number } | null>(null);
  const [failed, setFailed] = useState(false);
  const [expiredUnknown, setExpiredUnknown] = useState(false);
  useEffect(() => {
    let alive = true, generation = 0;
    async function establish() {
      const current = ++generation;
      setScope(null); setFailed(false);
      if (pageHidden()) return;
      try {
        const boundary = await sessionBoundary();
        if (alive && current === generation && !pageHidden()) setScope({ boundary, version: current });
      } catch { if (alive && current === generation) setFailed(true); }
    }
    const hidden = () => { generation++; setScope(null); };
    const visibility = () => { if (pageHidden()) hidden(); else void establish(); };
    void establish();
    document.addEventListener("visibilitychange", visibility);
    window.addEventListener("pagehide", hidden); window.addEventListener("pageshow", establish);
    return () => { alive = false; generation++; document.removeEventListener("visibilitychange", visibility);
      window.removeEventListener("pagehide", hidden); window.removeEventListener("pageshow", establish); };
  }, [store?.id, renderKey, locale]);
  const denial = initialError === "forbidden" ? c.forbidden : initialError === "unavailable" ? c.unavailable
    : initialError || failed ? c.signedOut : !store ? c.notFound : !store.permissions?.includes("customers:privacy") ? c.forbidden : "";
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? c.title} active="customers">
    <div className="orders-page import-wizard" data-testid="import-wizard">
      <AdminPageHeader locale={locale} description={c.subtitle} />
      <p>{c.consentNotice}</p><p>{c.historyNotice}</p><p className="orders-hint">{importViewCopy[locale].scopeReset}</p>
      {denial ? <p role="status">{denial}</p> : !scope || !store ? <p role="status">{c.loading}</p> :
        <ImportForm key={`${store.id}|${renderKey}|${scope.version}|${scope.boundary}`} locale={locale} store={store.id} boundary={scope.boundary}
          onSignedOut={uncertain => { setScope(null); setFailed(true); setExpiredUnknown(current => current || uncertain); }}
          onUnconfirmed={() => setExpiredUnknown(true)} />}
      {expiredUnknown && <p role="status">{importViewCopy[locale].sessionUnknown}</p>}
    </div>
  </WorkspaceFrame>;
}

function ImportForm({ locale, store, boundary, onSignedOut, onUnconfirmed }: {
  locale: Locale; store: string; boundary: string; onSignedOut: (uncertain: boolean) => void; onUnconfirmed: () => void;
}) {
  const c = importCopy[locale], v = importViewCopy[locale];
  const [kind, setKind] = useState<ImportKind>("customers"); const [stage, setStage] = useState<Stage>("select");
  const [file, setFile] = useState<File | null>(null), [headers, setHeaders] = useState<string[]>([]);
  const [mapping, setMapping] = useState<ImportMapping>({}), [preview, setPreview] = useState<ImportPreview | null>(null);
  const [receipt, setReceipt] = useState<ImportReceipt | null>(null), [customerDone, setCustomerDone] = useState(false);
  const [receiptMismatch, setReceiptMismatch] = useState(false);
  const [busy, setBusy] = useState(false), [unknown, setUnknown] = useState(false), [error, setError] = useState("");
  const [notice, setNotice] = useState(""); const [previewVersion, setPreviewVersion] = useState(0);
  const mounted = useRef(true), revision = useRef(0), active = useRef<AbortController | null>(null), inflight = useRef(false);
  const pending = useRef<Attempt | null>(null);
  useEffect(() => { mounted.current = true;
    return () => {
      // Keep only a boolean notice outside the private form: abort/drop does not prove a dispatched commit had no effect.
      if (pending.current) onUnconfirmed();
      mounted.current = false; revision.current++; active.current?.abort(); pending.current = null;
    }; }, []);
  const locked = busy || unknown;
  const missing = requiredImportFields[kind].some(field => !mapping[field]);
  const selected = Object.values(mapping).filter(Boolean), duplicate = new Set(selected).size !== selected.length;
  function reset(nextKind: ImportKind, nextStage: Stage = "upload") {
    if (inflight.current || pending.current?.unknown) return;
    revision.current++; setKind(nextKind); setStage(nextStage); setFile(null); setHeaders([]); setMapping({});
    setPreview(null); setReceipt(null); setReceiptMismatch(false); setError(""); setNotice(""); pending.current = null;
  }
  async function selectFile(chosen: File | undefined) {
    if (inflight.current || pending.current?.unknown) return;
    const current = ++revision.current;
    setFile(null); setHeaders([]); setMapping({}); setPreview(null); setReceipt(null); setReceiptMismatch(false); setError(""); setNotice("");
    if (!chosen) return;
    setBusy(true); inflight.current = true;
    try {
      const names = await readImportHeader(chosen);
      if (!mounted.current || current !== revision.current || pageHidden()) return;
      setFile(chosen); setHeaders(names); setMapping(guessImportMapping(kind, names)); setStage("mapping");
    } catch (cause) {
      if (mounted.current && current === revision.current) {
        const code = cause instanceof Error ? cause.message : "";
        setError(code === "file_too_large" ? c.fileTooLarge : c.errors[code] ?? c.headersFailure);
      }
    } finally { inflight.current = false; if (mounted.current && current === revision.current) setBusy(false); }
  }
  async function execute(action: "preview" | "commit", replay = false) {
    if (inflight.current || pageHidden()) return;
    if (replay && !pending.current) return;
    if (!replay && pending.current?.unknown) return;
    if (!file || (!replay && (missing || duplicate))) { setError(!file ? c.noFile : c.requiredMapping); return; }
    if (action === "commit" && !replay && (!preview || preview.apply_rows === 0)) return;
    const p: Attempt = replay && pending.current ? pending.current : {
      kind, file, mapping: Object.freeze({ ...mapping }), expected: preview?.apply_rows ?? 0, unknown: false,
    };
    if (action === "commit") pending.current = p;
    const controller = new AbortController(); active.current = controller; inflight.current = true;
    setBusy(true); setError(""); setNotice("");
    try {
      const result = await sendImport({ store, kind: p.kind, action, file: p.file, mapping: p.mapping,
        ...(action === "commit" ? { expectedApplyRows: p.expected } : {}), boundary, signal: controller.signal });
      if (!mounted.current || controller.signal.aborted || pageHidden()) return;
      if (result.kind === "error") {
        if (result.code === "unauthorized") {
          const unconfirmed = result.uncertain || p.unknown;
          if (!unconfirmed) pending.current = null;
          onSignedOut(unconfirmed); return;
        }
        if (action === "commit") { p.unknown ||= result.uncertain; pending.current = p.unknown ? p : null; setUnknown(p.unknown); }
        setError(result.code === "idempotency_conflict" ? c.idempotencyConflict : c.errors[result.code] ?? c.unavailable);
        return;
      }
      if (result.kind === "terminal") {
        // A saved result confirms this command cannot be repaired by sending the same bytes again.
        pending.current = null; setUnknown(false); setReceipt(result.value); setReceiptMismatch(true); setStage("result");
        return;
      }
      if (result.kind === "receipt") {
        pending.current = null; setUnknown(false); setReceipt(result.value); setReceiptMismatch(false); setStage("result");
        if (p.kind === "customers" && result.value.created + result.value.updated > 0) setCustomerDone(true);
        return;
      }
      if (result.kind === "stale" && p.unknown) {
        // A refused retry cannot erase a previous UNKNOWN receipt. Keep the old immutable command until confirmed.
        setUnknown(true); setError(c.staleNotice); return;
      }
      // command.Run hashes the REQUESTED mapping, including omitted/empty fields. Resolved preview metadata must not replace it.
      pending.current = null; setUnknown(false); setPreview(result.value);
      setPreviewVersion(value => value + 1); setStage("preview");
      if (result.kind === "stale") setNotice(c.staleNotice);
    } catch {
      if (mounted.current) {
        if (action === "commit") { p.unknown = true; pending.current = p; setUnknown(true); }
        setError(c.unavailable);
      }
    } finally { inflight.current = false; if (mounted.current) setBusy(false); }
  }
  async function download() {
    if (inflight.current || unknown || !preview) return;
    const controller = new AbortController(); active.current = controller; inflight.current = true;
    const initialCSRF = csrfCookie();
    setBusy(true); setError(""); setNotice("");
    try {
      let outcome: string;
      if (receipt) outcome = await downloadImportFailures({ store, batch: receipt.batch_id, boundary, signal: controller.signal });
      else {
        // No batch exists for an all-failed dry run; this file is only the safe verdict metadata already visible here.
        const csrf = csrfCookie(); const csv = safeImportFailureCSV(preview.rows);
        if (await sessionBoundary(csrf) !== boundary || controller.signal.aborted || pageHidden() || csrfCookie() !== csrf) outcome = "signed-out";
        else {
          const href = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
          try { const link = document.createElement("a"); link.href = href; link.download = "import-failed-rows.csv";
            document.body.append(link); link.click(); link.remove(); } finally { URL.revokeObjectURL(href); }
          outcome = "done";
        }
      }
      if (mounted.current && !controller.signal.aborted) {
        if (outcome === "done") setNotice(c.downloadStarted);
        else if (outcome === "signed-out") onSignedOut(false);
        else setError(outcome === "forbidden" ? c.forbidden : c.downloadUnknown);
      }
    } catch { if (mounted.current) {
      if (!initialCSRF || csrfCookie() !== initialCSRF) onSignedOut(false);
      else setError(c.downloadUnknown);
    } }
    finally { inflight.current = false; if (mounted.current) setBusy(false); }
  }
  const rowFailures = receipt?.failed ?? preview?.failed_rows ?? 0;
  return <div data-stage={stage}>
    <ol className="import-stages">{(["select", "upload", "mapping", "preview", "confirm", "result"] as const).map(step =>
      <li key={step} aria-current={stage === step ? "step" : undefined}>{c.stages[step]}</li>)}</ol>
    <div className="import-type" role="group" aria-label={c.stages.select}>
      <button type="button" data-testid="import-type-customers" disabled={locked} aria-pressed={kind === "customers"} onClick={() => reset("customers")}>{c.customerKind}</button>
      <button type="button" data-testid="import-type-orders" disabled={locked || !customerDone} aria-pressed={kind === "orders"} onClick={() => reset("orders")}>{c.ordersKind}</button>
    </div>
    <p role="status">{customerDone ? v.ready : c.customerFirst}</p>
    {stage === "upload" && <section className="import-panel" aria-label={c.stages.upload}>
      <label>{c.uploadLabel}<input type="file" data-testid="import-file" accept=".csv,text/csv" disabled={locked} onChange={event => void selectFile(event.target.files?.[0])} /></label>
      <p>{c.utf8Hint}</p><p className="orders-hint">{c.limits}</p><p className="orders-hint">{c.storageNotice}</p>
    </section>}
    {stage === "mapping" && <section className="import-panel" aria-label={c.stages.mapping}>
      <ImportColumnMapping locale={locale} fields={importFields[kind]} required={requiredImportFields[kind]} headers={headers} mapping={mapping} disabled={locked}
        onChange={(field, header) => { setMapping(current => ({ ...current, [field]: header })); setPreview(null); setError(""); }} />
      <div className="import-actions"><button type="button" data-testid="import-guess" disabled={locked} onClick={() => setMapping(guessImportMapping(kind, headers))}>{c.guess}</button>
        <button type="button" data-testid="import-preview" disabled={locked || missing || duplicate} onClick={() => void execute("preview")}>{c.preview}</button></div>
      {missing || duplicate ? <p>{c.requiredMapping}</p> : null}
    </section>}
    {(stage === "preview" || stage === "confirm") && preview && <section className="import-panel" aria-label={c.stages[stage]}>
      <ImportPreviewCounts locale={locale} kind={kind} preview={preview} />
      {preview.apply_rows === 0 && <p role="status">{c.errors.nothing_to_apply}</p>}
      {stage === "preview" && <ImportVerdictTable key={previewVersion} locale={locale} preview={preview} />}
      <div className="import-actions">{stage === "preview" ?
        <button type="button" data-testid="import-confirm-next" disabled={locked || preview.apply_rows === 0} onClick={() => setStage("confirm")}>{c.confirmLabel}</button> :
        <button type="button" data-testid="import-confirm" disabled={locked || preview.apply_rows === 0} onClick={() => void execute("commit")}>{c.commit}</button>}
      </div>
    </section>}
    {stage === "result" && receipt && <section className="import-panel" aria-label={c.stages.result} data-testid="import-receipt">
      {receiptMismatch && <p role="alert" data-testid="import-terminal-mismatch">{v.receiptMismatch}</p>}
      <h2>{c.stages.result}</h2><dl className="import-counts">
        <div><dt>{c.created}</dt><dd>{receipt.created}</dd></div><div><dt>{c.updated}</dt><dd>{receipt.updated}</dd></div><div><dt>{c.failed}</dt><dd>{receipt.failed}</dd></div>
      </dl>{receipt.replayed && <p data-testid="import-replayed">{c.replayed}</p>}
      {preview && <><h3>{c.stages.preview}</h3><ImportPreviewCounts locale={locale} kind={kind} preview={preview} /></>}
    </section>}
    {preview && <button type="button" data-testid="import-download-failed" disabled={locked || rowFailures === 0} onClick={() => void download()}>{c.failedDownload}</button>}
    {unknown && <div role="status"><p>{c.unknownCommit}</p><button type="button" data-testid="import-retry-same" disabled={busy} onClick={() => void execute("commit", true)}>{v.retrySame}</button></div>}
    {busy && <p role="status">{c.working}</p>}{error && <p role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
    <div className="import-actions">
      {stage !== "select" && <button type="button" data-testid="import-back" disabled={locked} onClick={() => {
        setError(""); setNotice("");
        if (stage === "confirm") setStage("preview"); else if (stage === "preview") { setPreview(null); setStage("mapping"); }
        else reset(kind, "select");
      }}>{c.back}</button>}
      <button type="button" data-testid="import-restart" disabled={locked || stage === "select"} onClick={() => reset("customers", "select")}>{c.restart}</button>
    </div>
  </div>;
}
