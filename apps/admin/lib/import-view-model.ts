// Purpose: arrange safe import verdicts and produce a failure file containing row numbers and codes only.
// Depends on: import-model SafeImportRow (types only); never accepts or reads uploaded CSV bytes.
// Used by: ImportWizardSteps and preview-only failure downloads before a batch exists.
import type { SafeImportRow } from "./import-model";

/** Return a new verdict list with failures first; server row numbers retain their meaning. */
export function orderedImportRows(rows: readonly SafeImportRow[]): SafeImportRow[] {
  return [...rows].sort((a, b) => Number(b.outcome === "failed") - Number(a.outcome === "failed") || a.row - b.row);
}

/** Export safe failed-row metadata, never uploaded cells or successful source identifiers. */
export function safeImportFailureCSV(rows: readonly SafeImportRow[]): string {
  const code = (value: string | undefined) => value && /^[a-z][a-z0-9_]{0,63}$/.test(value) ? value : "invalid_request";
  const lines = orderedImportRows(rows).filter(row => row.outcome === "failed").map(row =>
    `${row.row},failed,${code(row.code)}\r\n`);
  return "\uFEFFrow,outcome,code\r\n" + lines.join("");
}
