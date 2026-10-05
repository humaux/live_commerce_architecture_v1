"use client";

// Product CSV import / export (/{locale}/products/import). BFF /api/stores/{store}/tools/products/{export.csv, import/preview, import/commit}
// -> Go internal/httpapi/merchanttools.go -> internal/merchanttools (contract storefront-v2 G2). The wizard is: choose a file, "check" it
// (preview: the server runs the whole import and rolls back, so every row error appears with its row number), then "import" the SAME file
// (all-or-nothing; the same bytes uploaded twice replay and change nothing). The file is read once into a Blob, never stored, and never
// parsed here: Go is the only parser, so the browser and the server cannot disagree. The session fence is lib/merchant-tools-client.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { FilePicker } from "@live-commerce/ui";
import { catalogPresentationCopy } from "@/lib/catalog-v2-copy";
import type { Store } from "@/lib/model";
import { sessionBoundary } from "@/lib/settings-client";
import { fetchExport, sendImport } from "@/lib/merchant-tools-client";
import { csvFileProblem, type ImportResult } from "@/lib/merchant-tools-model";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import "./orders.css";
import "./customers.css";
import "./merchant-tools.css";
import "./ProductAdmin.css";

type Phase = "idle" | "checking" | "checked" | "committing" | "done";

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
  const [boundary, setBoundary] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [phase, setPhase] = useState<Phase>("idle");
  const [result, setResult] = useState<ImportResult | null>(null);
  const [problem, setProblem] = useState("");
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
      setProblem(errorFor(outcome.code));
      setPhase(mode === "preview" ? "idle" : "checked");
      return;
    }
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
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.noStore}
      active="products"
    >
      <div
        className="orders-page product-admin product-import-page"
        data-testid="import-page"
      >
        <AdminPageHeader locale={locale} description={c.subtitle} />
        {stores.length > 1 && (
          <div className="orders-controls">
            <label>
              {c.store}
              <select
                data-testid="store-selector"
                value={store?.id ?? ""}
                onChange={(event) =>
                  window.location.assign(
                    `/${locale}/products/import?store=${event.target.value}`,
                  )
                }
              >
                {stores.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
          </div>
        )}
        {!store && (
          <p className="orders-message" role="status">
            {c.noStore}
          </p>
        )}
        {store && (
          <div className="product-import-grid">
            <section className="mt-card" aria-label={c.exportTitle}>
              <h2>{c.exportTitle}</h2>
              <p className="mt-note">{c.exportText}</p>
              <div className="mt-actions" style={{ marginTop: 14 }}>
                <button
                  type="button"
                  data-testid="import-export"
                  disabled={exporting}
                  onClick={() => void download()}
                >
                  {exporting ? c.exporting : c.exportButton}
                </button>
              </div>
            </section>
            <section className="mt-card" aria-label={c.importTitle}>
              <h2>{c.importTitle}</h2>
              <p className="mt-note">{c.importText}</p>
              <ul className="mt-rules">
                {c.rules.map((rule) => (
                  <li key={rule}>{rule}</li>
                ))}
              </ul>
              <p className="mt-note">{c.limits}</p>
              <div className="mt-form" style={{ marginTop: 14 }}>
                <FilePicker
                  label={c.chooseFile}
                  emptyLabel={p.noFile}
                  fileName={file?.name ?? ""}
                  inputRef={input}
                  accept=".csv,text/csv"
                  data-testid="import-file"
                  onChange={(event) => pick(event.target.files?.[0] ?? null)}
                />
                <div className="mt-actions">
                  <button
                    type="button"
                    data-testid="import-preview"
                    aria-describedby={
                      previewReason ? "import-preview-reason" : undefined
                    }
                    disabled={
                      !file ||
                      !boundary ||
                      phase === "checking" ||
                      phase === "committing"
                    }
                    onClick={() => void run("preview")}
                  >
                    {phase === "checking" ? c.previewing : c.preview}
                  </button>
                  <button
                    type="button"
                    className="product-primary"
                    data-testid="import-commit"
                    aria-describedby={
                      commitReason
                        ? commitReason === previewReason
                          ? "import-preview-reason"
                          : "import-commit-reason"
                        : undefined
                    }
                    disabled={
                      !clean ||
                      phase === "committing" ||
                      phase === "done" ||
                      result?.committed
                    }
                    onClick={() => void run("commit")}
                  >
                    {phase === "committing" ? c.committing : c.commit}
                  </button>
                </div>
                {previewReason && (
                  <p
                    id="import-preview-reason"
                    className="product-disabled-reason"
                  >
                    {previewReason}
                  </p>
                )}
                {commitReason && commitReason !== previewReason && (
                  <p
                    id="import-commit-reason"
                    className="product-disabled-reason"
                  >
                    {commitReason}
                  </p>
                )}
              </div>
            </section>
          </div>
        )}
        {problem && (
          <p className="mt-warn" role="alert" data-testid="import-problem">
            {problem}
          </p>
        )}
        {result && (
          <section
            className="mt-card"
            aria-label={c.summary}
            data-testid="import-result"
          >
            <h2>{c.summary}</h2>
            {result.committed && (
              <p className="mt-ok" role="status">
                {result.replayed ? c.replayed : c.committed}
              </p>
            )}
            {!result.committed && clean && (
              <p className="mt-ok" role="status">
                {c.previewOK}
              </p>
            )}
            <dl className="mt-stats">
              {(
                [
                  [c.rows, result.rows],
                  [c.createdProducts, result.created_products],
                  [c.updatedProducts, result.updated_products],
                  [c.createdSkus, result.created_skus],
                  [c.updatedSkus, result.updated_skus],
                  [c.stock, result.stock_adjustments],
                  [c.unchanged, result.unchanged_rows],
                ] as const
              ).map(([label, n]) => (
                <div key={label}>
                  <dt>{label}</dt>
                  <dd>{n}</dd>
                </div>
              ))}
            </dl>
            {result.errors.length > 0 && (
              <>
                <h3>
                  {c.problems} ({result.errors.length}
                  {result.errors_truncated ? "+" : ""})
                </h3>
                <p className="mt-note">{c.fix}</p>
                <div className="mt-table-frame" data-testid="import-errors">
                  <table className="mt-table">
                    <thead>
                      <tr>
                        <th scope="col" className="num">
                          {c.row}
                        </th>
                        <th scope="col">{c.column}</th>
                        <th scope="col">{c.problem}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {result.errors.map((e, index) => (
                        <tr key={`${e.row}|${e.column}|${e.code}|${index}`}>
                          <td className="num">
                            {e.row === 0 ? c.fileLevel : e.row}
                          </td>
                          <td>{e.column || "—"}</td>
                          <td>{c.codes[e.code] ?? e.code}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                {result.errors_truncated && (
                  <p className="mt-note">{c.truncated}</p>
                )}
              </>
            )}
          </section>
        )}
      </div>
    </WorkspaceFrame>
  );
}
