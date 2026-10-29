"use client";

// Purpose: signed buyer CVS discovery sheet with explicit store choice and confirmation.
// Depends on: public/cvs immutable JSON, shared pure matcher, i18n and native dialog.
// Used by: CvsPickup; parent alone validates/persists the selection through the buyer BFF.
// Invariants: no search API, no automatic selection, no raw paste persistence, canonical display.
import { useEffect, useRef, useState } from "react";
import { cvsPickerCopy, type Locale } from "@live-commerce/i18n";
import { matchCvsStores, normalizeCvsText, type CvsBrand, type CvsStore } from "../../../packages/format/src/cvs-match";
import styles from "./CvsStorePicker.module.css";

const brands: CvsBrand[] = ["UNIMART", "FAMI", "HILIFE", "OKMART"];
const directories = new Map<CvsBrand, CvsStore[]>();
function validDirectory(value: unknown): value is CvsStore[] {
  return Array.isArray(value) && value.length > 0 && value.every((row) =>
    row !== null && typeof row === "object" && ["code", "name", "address", "county", "district"].every((key) => typeof row[key] === "string" && row[key].length > 0));
}

// Comparison uses normalized offsets while the rendered name/address always stays verbatim.
function MatchedText({ text, query }: { text: string; query: string }) {
  const needle = normalizeCvsText(query);
  if (!needle || /\n|門市|门市/.test(query)) return <>{text}</>;
  const normalized = normalizeCvsText(text), at = normalized.indexOf(needle);
  if (at < 0) return <>{text}</>;
  let from = 0, to = text.length;
  for (let i = 0; i <= text.length; i++) {
    const length = normalizeCvsText(text.slice(0, i)).length;
    if (length <= at) from = i;
    if (length >= at + needle.length) { to = i; break; }
  }
  return <>{text.slice(0, from)}<mark className={styles.match}>{text.slice(from, to)}</mark>{text.slice(to)}</>;
}

export default function CvsStorePicker({ open, brand, locale, onClose, onBrand, onConfirm, onEdit, error, coolingDown, availableBrands }: {
  open: boolean; brand: CvsBrand; locale: Locale; onClose: () => void;
  onBrand: (brand: CvsBrand) => void;
  onConfirm: (store: CvsStore) => Promise<"saved" | "rejected" | "retry">;
  error: string | null; coolingDown: boolean;
  availableBrands: CvsBrand[];
  onEdit: () => void;
}) {
  const copy = cvsPickerCopy[locale];
  const dialog = useRef<HTMLDialogElement>(null);
  const [path, setPath] = useState<"region" | "search">("region");
  const [query, setQuery] = useState("");
  const [county, setCounty] = useState("");
  const [district, setDistrict] = useState("");
  const [rows, setRows] = useState<CvsStore[]>([]);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);
  const [reload, setReload] = useState(0);
  const [selected, setSelected] = useState<CvsStore | null>(null);
  const [saving, setSaving] = useState(false);
  const [limit, setLimit] = useState(40);
  const [rejected, setRejected] = useState<Set<string>>(new Set());

  useEffect(() => {
    const node = dialog.current;
    if (!node) return;
    if (!open) { node.close(); setSelected(null); return; }
    if (!node.open) node.showModal();
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.body.style.overflow = overflow; };
  }, [open]);
  useEffect(() => {
    setSelected(null); setCounty(""); setDistrict(""); setRejected(new Set()); setLimit(40);
  }, [brand]);
  useEffect(() => {
    if (!open) return;
    const cached = directories.get(brand);
    if (cached) { setRows(cached); setLoading(false); setFailed(false); return; }
    const controller = new AbortController();
    setRows([]); setLoading(true); setFailed(false);
    void (async () => {
      try {
        const response = await fetch(`/cvs/${brand.toLowerCase()}-20261028.json`, { signal: controller.signal, credentials: "omit" });
        if (!response.ok) throw new Error("directory_unavailable");
        const data: unknown = await response.json();
        if (!validDirectory(data)) throw new Error("invalid_directory");
        if (controller.signal.aborted) return;
        directories.set(brand, data); setRows(data);
      } catch {
        if (!controller.signal.aborted) setFailed(true);
      } finally { if (!controller.signal.aborted) setLoading(false); }
    })();
    return () => controller.abort();
  }, [brand, open, reload]);

  const matched = matchCvsStores(brand, query, rows);
  const counties = [...new Set(rows.map((row) => row.county))].sort();
  const districts = [...new Set(rows.filter((row) => row.county === county).map((row) => row.district))].sort();
  const candidates = (path === "search" ? matched.candidates : county && district
    ? rows.filter((row) => row.county === county && row.district === district) : []).filter((row) => !rejected.has(row.code));
  const ready = path === "search" ? query.trim() !== "" : county !== "" && district !== "";
  const changed = () => { setSelected(null); setLimit(40); onEdit(); };
  const canConfirm = !!selected && !saving && !loading && !coolingDown;
  return (
    <dialog ref={dialog} className={styles.picker} data-testid="cvs-picker" aria-labelledby="cvs-picker-title"
      onCancel={(event) => { event.preventDefault(); if (!saving) onClose(); }} onClose={onClose}>
      <div className={styles.layout}>
        <header className={styles.header}>
          <div className={styles.title}><h2 id="cvs-picker-title">{copy.title}</h2>
            <button type="button" data-testid="cvs-picker-close" aria-label={copy.close} disabled={saving} onClick={onClose}>
              <svg aria-hidden="true" width="20" height="20" viewBox="0 0 20 20"><path d="m4 4 12 12M16 4 4 16" fill="none" stroke="currentColor" strokeWidth="2" /></svg>
            </button>
          </div>
          <div className={styles.tabs}>
            <button type="button" data-testid="cvs-picker-region-tab" aria-pressed={path === "region"} disabled={saving} onClick={() => { setPath("region"); changed(); }}>{copy.region}</button>
            <button type="button" data-testid="cvs-picker-search-tab" aria-pressed={path === "search"} disabled={saving} onClick={() => { setPath("search"); changed(); }}>{copy.search}</button>
          </div>
          <div className={styles.brands}>
            {brands.map((key) => <button type="button" key={key} data-testid={`cvs-brand-${key}`} aria-pressed={key === brand || (key === "UNIMART" && brand === "UNIMARTFREEZE")} disabled={saving || !availableBrands.includes(key)}
              onClick={() => { changed(); onBrand(key); }}>{copy.chains[key]}</button>)}
          </div>
          {path === "search" ? <textarea data-testid="cvs-picker-query" aria-label={copy.query} placeholder={copy.query} value={query} rows={2} maxLength={2000} disabled={saving} autoComplete="off"
            onChange={(event) => { setQuery(event.target.value); changed(); }}
            onPaste={(event) => {
              const text = event.clipboardData.getData("text");
              const lines = text.split(/\r?\n/).flatMap((line) => {
                const storeField = /(?:收件)?[門门]市(?:代[號号]|[編编][號号]|名称|名稱|地址)/.exec(line);
                return storeField ? [line.slice(storeField.index)] : [];
              });
              if (lines.length || /[\r\n]/.test(text)) { event.preventDefault(); setQuery(lines.join("\n").slice(0, 2000)); changed(); }
            }} /> : <div className={styles.region}>
            <select data-testid="cvs-picker-county" aria-label={copy.county} value={county} disabled={saving || loading} onChange={(event) => { setCounty(event.target.value); setDistrict(""); changed(); }}>
              <option value="">{copy.county}</option>{counties.map((value) => <option key={value} value={value}>{value === "其他" ? copy.other : value}</option>)}
            </select>
            <select data-testid="cvs-picker-district" aria-label={copy.district} value={district} disabled={saving || !county || loading} onChange={(event) => { setDistrict(event.target.value); changed(); }}>
              <option value="">{copy.district}</option>{districts.map((value) => <option key={value} value={value}>{value === "其他" ? copy.other : value}</option>)}
            </select>
          </div>}
        </header>
        <div className={styles.results} aria-live="polite" aria-busy={loading || saving}>
          {loading ? <p role="status">{copy.loading}</p> : failed ? <div role="alert"><p>{copy.failed}</p><button type="button" onClick={() => setReload((value) => value + 1)}>{copy.retry}</button></div> : <>
            {path === "search" && matched.codeMiss && <p data-testid="cvs-picker-code-miss">{copy.missing}{matched.codeMiss}</p>}
            {path === "search" && matched.brandMismatch && <p>{copy.mismatch}</p>}
            {ready && !candidates.length && <p data-testid="cvs-picker-empty">{copy.empty}</p>}
            {candidates.slice(0, limit).map((row) => <button type="button" key={row.code} data-testid={`cvs-candidate-${row.code}`} className={styles.row} aria-pressed={selected?.code === row.code} disabled={saving}
              onClick={() => setSelected(row)}>
              <span className={styles.radio} aria-hidden="true" />
              <span className={styles.facts}><strong className={path === "search" && matched.code === row.code ? styles.match : undefined}>{`${row.name} · ${row.code}`}</strong><span><MatchedText text={row.address} query={path === "search" ? query : ""} /></span></span>
            </button>)}
            {candidates.length > limit && <button type="button" disabled={saving} onClick={() => setLimit((value) => value + 40)}>{copy.more}</button>}
          </>}
        </div>
        <footer className={styles.footer}>
          {error && <p role="alert">{error}</p>}
          <small data-testid="cvs-picker-source">{copy.source}</small>
          <button type="button" className="primary" data-testid="cvs-picker-confirm" disabled={!canConfirm}
            title={!selected ? copy.choose : undefined}
            onClick={() => {
              if (!selected || !canConfirm) return;
              const choice = selected;
              setSaving(true);
              void onConfirm(choice).then((result) => {
                if (result === "saved") onClose();
                else if (result === "rejected") { setSelected(null); setRejected((current) => new Set([...current, choice.code])); }
              }).finally(() => setSaving(false));
            }}>{saving ? copy.confirming : copy.confirm}</button>
        </footer>
      </div>
    </dialog>
  );
}
