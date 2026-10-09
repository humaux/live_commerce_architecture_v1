// Purpose: render import header mapping and safe paged row verdicts; no uploaded data cells are rendered.
// Depends on: React, import-copy, import-model safe DTO types and import-view-model ordering.
// Used by: ImportWizard; column names are metadata, all verdict messages come from local copy.
"use client";
import { useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { ImportKind, ImportMapping, ImportPreview } from "../lib/import-model";
import { importCopy } from "../lib/import-copy";
import { orderedImportRows } from "../lib/import-view-model";
import { importViewCopy } from "../lib/import-view-copy";

/** Map each known field to one source header; this form never reads uploaded row values. */
export function ImportColumnMapping({ locale, fields, required, headers, mapping, disabled, onChange }: {
  locale: Locale; fields: readonly string[]; required: readonly string[]; headers: readonly string[];
  mapping: ImportMapping; disabled: boolean; onChange: (field: string, header: string) => void;
}) {
  const c = importCopy[locale];
  return <fieldset className="import-mapping" disabled={disabled}>
    <legend>{c.mapHeader}</legend>
    {fields.map(field => <label key={field}>{c.fieldLabels[field]}{required.includes(field) && <small> · {c.required}</small>}
      <select data-testid={`import-map-${field}`} value={mapping[field] ?? ""} onChange={event => onChange(field, event.target.value)}>
        <option value="">{c.unmapped}</option>
        {headers.map(header => <option key={header} value={header}>{header}</option>)}
      </select>
    </label>)}
  </fieldset>;
}

/** Display server counts with no inferred consent or revenue; kind-specific privacy counters remain visible. */
export function ImportPreviewCounts({ locale, kind, preview }: { locale: Locale; kind: ImportKind; preview: ImportPreview }) {
  const c = importCopy[locale];
  return <dl className="import-counts">
    <div><dt>{c.newRows}</dt><dd>{preview.new_rows}</dd></div>
    <div><dt>{c.updateRows}</dt><dd>{preview.update_rows}</dd></div>
    <div><dt>{c.failedRows}</dt><dd>{preview.failed_rows}</dd></div>
    <div data-testid="import-erased-count"><dt>{c.erasedRows}</dt><dd>{preview.erased_rows}</dd></div>
    {kind === "customers" && <div data-testid="import-consent-count"><dt>{c.consentIgnoredRows}</dt><dd>{preview.consent_ignored_rows ?? 0}</dd></div>}
    {kind === "orders" && <div data-testid="import-city-count"><dt>{c.cityDroppedRows}</dt><dd>{preview.city_dropped_rows ?? 0}</dd></div>}
  </dl>;
}

/** Page fifty safe verdicts at a time, with failures first and stable source row numbers. */
export function ImportVerdictTable({ locale, preview }: { locale: Locale; preview: ImportPreview }) {
  const c = importCopy[locale]; const [page, setPage] = useState(0);
  const rows = orderedImportRows(preview.rows), start = page * 50;
  return <>
    <p className="orders-hint">{c.onlyRowsHint}</p>
    <div className="orders-table-scroll"><table className="orders-table import-verdicts" data-testid="import-rows">
      <thead><tr><th>{c.row}</th><th>{c.outcome}</th><th>{c.code}</th></tr></thead>
      <tbody>{rows.slice(start, start + 50).map(row => <tr key={row.row} data-testid={`import-row-${row.row}`}>
        <td>{row.row}</td><td>{c.outcomes[row.outcome]}</td>
        <td>{row.code && <span>{c.errors[row.code] ?? c.unavailable}</span>}
          {row.consent_ignored && <span className="import-verdict-note">{c.consentIgnoredRows}</span>}
          {row.warning === "city_dropped" && <span className="import-verdict-note">{c.cityDroppedRows}</span>}
        </td>
      </tr>)}</tbody>
    </table></div>
    {rows.length === 0 && <p>{c.empty}</p>}
    <div className="import-actions">
      <button type="button" data-testid="import-row-previous" disabled={page === 0} onClick={() => setPage(value => value - 1)}>{importViewCopy[locale].previous}</button>
      <output aria-live="polite">{rows.length ? start + 1 : 0}–{Math.min(start + 50, rows.length)} / {rows.length}</output>
      <button type="button" data-testid="import-row-next" disabled={start + 50 >= rows.length} onClick={() => setPage(value => value + 1)}>{importViewCopy[locale].next}</button>
    </div>
  </>;
}
