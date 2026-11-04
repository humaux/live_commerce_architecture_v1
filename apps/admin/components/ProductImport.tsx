// Purpose: Owns product CSV preview, import commit and export controls.
// Depends on: React, data-ops locale namespace, merchant-tools client/model/copy, session boundary and WorkspaceFrame.
// Used by: apps/admin/app/[locale]/products/import/page.tsx
// Invariants: server-only CSV parsing; all-or-nothing writes; uncertain commits keep the selected bytes.
"use client";

// Product CSV import / export (/{locale}/products/import). BFF /api/stores/{store}/tools/products/{export.csv, import/preview, import/commit}
// -> Go internal/httpapi/merchanttools.go -> internal/merchanttools (contract storefront-v2 G2). The wizard is: choose a file, "check" it
// (preview: the server runs the whole import and rolls back, so every row error appears with its row number), then "import" the SAME file
// (all-or-nothing; the same bytes uploaded twice replay and change nothing). The file is read once into a Blob, never stored, and never
// parsed here: Go is the only parser, so the browser and the server cannot disagree. The session fence is lib/merchant-tools-client.
import { dataOpsCopy, dataOpsText } from "../../../packages/i18n/src/data-ops";
import "./imports/data-ops.css";
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { catalogPresentationCopy } from "@/lib/catalog-v2-copy";
import type { Store } from "@/lib/model";
import { sessionBoundary } from "@/lib/settings-client";
import { fetchExport, sendImport } from "@/lib/merchant-tools-client";
import { csvFileProblem, type ImportResult } from "@/lib/merchant-tools-model";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import "./orders.css";
import "./customers.css";
import "./merchant-tools.css";
import "./ProductAdmin.css";

type Phase = "idle" | "checking" | "checked" | "committing" | "done";

/** Owns product CSV preview, import commit and export controls. User actions request server CSV preview, commit or export through merchant-tools-client. */
export function ProductImport({
  locale,
  stores,
  store,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
}) {
  const c = toolsCopy[locale].importer;
  const d = dataOpsCopy(locale);
  const [boundary, setBoundary] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [phase, setPhase] = useState<Phase>("idle");
  const [result, setResult] = useState<ImportResult | null>(null);
  const [problem, setProblem] = useState("");
  const [unknown, setUnknown] = useState(false);
  const [exporting, setExporting] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    let live = true;
    sessionBoundary().then(
      (value) => live && setBoundary(value),
      () => live && setBoundary(""),
    );
    return () => {
      live = false;
    };
  }, []);

  const errorFor = (code: string) =>
    code === "unauthorized"
      ? c.signedOut
      : code === "forbidden"
        ? c.forbidden
        : c.failed;
  async function run(mode: "preview" | "commit") {
    if (!store || !file || !boundary) return;
    setProblem("");
    setPhase(mode === "preview" ? "checking" : "committing");
    const outcome = await sendImport(store.id, mode, file, boundary);
    if (!outcome.ok) {
      if (mode === "commit" && outcome.uncertain) { setUnknown(true); setProblem(d.pendingHelp); }
      else setProblem(errorFor(outcome.code));
      setPhase(mode === "preview" ? "idle" : "checked");
      return;
    }
    setUnknown(false);
    setResult(outcome.value);
    setPhase(mode === "commit" && outcome.value.committed ? "done" : "checked");
  }
  function pick(next: File | null) {
    setResult(null);
    setProblem("");
    setPhase("idle");
    if (!next) return setFile(null);
    const issue = csvFileProblem(next.name, next.size);
    if (issue) {
      setFile(null);
      setProblem(
        issue === "not_csv"
          ? c.notCsv
          : issue === "empty"
            ? c.empty
            : c.tooLarge,
      );
      if (input.current) input.current.value = "";
      return;
    }
    setFile(next);
  }
  async function download() {
    if (!store) return;
    setExporting(true);
    setProblem("");
    const out = await fetchExport(store.id);
    setExporting(false);
    if (!out.ok)
      return setProblem(
        out.code === "export_too_large" ? c.exportTooLarge : c.exportFailed,
      );
    const url = URL.createObjectURL(out.blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = out.name;
    document.body.append(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(url);
  }
  const clean = !!result && result.errors.length === 0 && result.rows > 0;
  const p = catalogPresentationCopy[locale];
  const previewReason =
    phase === "checking" || phase === "committing"
      ? p.busy
      : !file
        ? p.chooseFileFirst
        : !boundary
          ? p.sessionUnavailable
          : "";
  const commitReason =
    phase === "committing"
      ? p.busy
      : phase === "done" || result?.committed
        ? p.importedAlready
        : !clean
          ? result?.errors.length
            ? p.fixFileFirst
            : p.checkFileFirst
          : "";
  const storeQuery = store ? `?store=${encodeURIComponent(store.id)}` : "";
  const errors = !!result?.errors.length;
  const title = phase === "done" ? d.productResult : errors ? d.productErrors : phase === "checked" ? d.previewProduct : d.productTitle;
  const reset = () => { setPhase("idle"); setResult(null); setProblem(""); };
  return <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="products">
    <div className="data-ops" data-testid="import-page">
      <header><h1>{title}</h1>{phase === "idle" && <button data-testid="import-export" disabled={!store || exporting} onClick={() => void download()}>{exporting ? c.exporting : d.exportProduct}</button>}</header>
      {stores.length > 1 && <label>{c.store}<select data-testid="store-selector" value={store?.id ?? ""} onChange={event => window.location.assign(`/${locale}/products/import?store=${encodeURIComponent(event.target.value)}`)}>{stores.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label>}
      {!store && <p role="status">{c.noStore}</p>}
      {store && <>
        {(phase === "idle" || phase === "checking") && <section className="ops-card">
          <h2>{d.productUpload}</h2><p className="ops-callout">{d.productLimits}</p>
          <label className="ops-file">{d.productFile}<input ref={input} type="file" accept=".csv,text/csv" data-testid="import-file" disabled={phase === "checking"} onChange={event => pick(event.target.files?.[0] ?? null)} /></label>
          {file && <p>{file.name}</p>}<p>{d.productRule}</p><p>{d.productStock}</p>
          <button className="primary" data-testid="import-preview" disabled={!!previewReason} onClick={() => void run("preview")}>{phase === "checking" ? c.previewing : d.previewFile}</button>
          {previewReason && file && <p className="ops-note">{previewReason}</p>}
        </section>}
        {(phase === "checked" || phase === "committing") && result && <section className="ops-card" data-testid="import-result">
          {errors ? <>
            <p>{dataOpsText(d.productInvalidRows, { n: new Set(result.errors.map(error => error.row)).size })}</p><p className="ops-warning">{d.noBatch}</p><p>{d.repairCSV}</p>
            <div data-testid="import-errors">{result.errors.map((error, index) => <section className="ops-row" key={index}>
              <h2>{dataOpsText(d.row, { n: error.row, field: error.column === "price" ? d.priceField : error.column === "collections" ? d.collectionField : error.column })}</h2>
              <p>{error.code === "required" && error.column === "price" ? d.missingPrice : error.code === "unknown_collection" ? d.missingCollection : c.codes[error.code]}</p>
              {error.column === "price" && <p className="ops-note">{d.priceRepair}</p>}{error.code === "unknown_collection" && <p className="ops-note">{d.collectionRepair}</p>}
            </section>)}</div>
            {result.errors_truncated && <p>{c.truncated}</p>}
            <button onClick={reset}>{d.reupload}</button>
          </> : <>
            <p>{dataOpsText(d.productRows, { file: file?.name ?? "", n: result.rows })}</p><p>{d.productPreviewRule}</p>
            <dl className="ops-stats"><div><dt>{d.newProducts}</dt><dd>{result.created_products}</dd></div><div><dt>{d.updatedProducts}</dt><dd>{result.updated_products}</dd></div><div><dt>{d.stockAdjustments}</dt><dd>{result.stock_adjustments}</dd></div></dl>
            <div className="ops-actions"><button onClick={reset} disabled={unknown || phase === "committing"}>{d.importHome}</button><button className="primary" data-testid="import-commit" disabled={!!commitReason} onClick={() => void run("commit")}>{phase === "committing" ? c.committing : c.commit}</button></div>
          </>}
        </section>}
        {phase === "done" && result && <section className="ops-card" data-testid="import-result">
          <h2>{d.importDone}</h2><dl className="ops-stats"><div><dt>{d.newProducts}</dt><dd>{result.created_products}</dd></div><div><dt>{d.updatedProducts}</dt><dd>{result.updated_products}</dd></div><div><dt>{d.stockAdjustments}</dt><dd>{result.stock_adjustments}</dd></div></dl>
          <p>{dataOpsText(d.productTotals, { n: result.rows, failed: result.errors.length, same: result.unchanged_rows })}</p>
          {result.replayed && <p>{c.replayed}</p>}
          <div className="ops-actions"><button onClick={reset}>{d.importHome}</button><a className="ops-link" href={`/${locale}/products${storeQuery}`}>{d.viewProducts}</a></div>
        </section>}
        <a className="ops-link" href={`/${locale}/customers/import${storeQuery}`}>{d.customerImport}</a>
      </>}
      {problem && <p role="alert">{problem}</p>}
    </div>
  </WorkspaceFrame>;
}
