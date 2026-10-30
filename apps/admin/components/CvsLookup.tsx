// Purpose: explicit public-directory CVS selection shared by the merchant tool and manual-order draft.
// Depends on: react, i18n orderOpsCopy, format/cvs-match, admin cvs-directory-client, order-ops.css.
// Used by: orders/cvs-match route and ManualOrder; never calls a carrier or writes buyer data.
// Invariants: codes stay strings; only an explicit click chooses canonical facts; no paste persistence or auto-substitution.
"use client";
import { useEffect, useMemo, useState } from 'react';
import { orderOpsCopy, type Locale } from '@live-commerce/i18n';
import { matchCvsStores, type CvsBrand, type CvsStore } from '../../../packages/format/src/cvs-match';
import { loadCvsDirectory, cvsDirectorySource } from '../lib/cvs-directory-client';
import './order-ops.css';

export function CvsLookup({ locale, brand: fixedBrand, onChoose, onCancel }: {
  locale: Locale; brand?: CvsBrand; onChoose?: (store: CvsStore) => void; onCancel?: () => void;
}) {
  const c = orderOpsCopy[locale];
  const [brand, setBrand] = useState<CvsBrand>(fixedBrand ?? 'UNIMART');
  const [query, setQuery] = useState('');
  const [stores, setStores] = useState<CvsStore[]>([]);
  const [selected, setSelected] = useState<CvsStore | null>(null);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [reload, setReload] = useState(0);
  const [notice, setNotice] = useState('');
  const [copyView, setCopyView] = useState(false);
  useEffect(() => {
    const abort = new AbortController();
    setStores([]); setSelected(null); setNotice(''); setLoading(true); setFailed(false);
    loadCvsDirectory(brand, abort.signal).then((data) => {
      if (!abort.signal.aborted) { setStores(data); setLoading(false); }
    }, () => { if (!abort.signal.aborted) { setFailed(true); setLoading(false); } });
    return () => abort.abort();
  }, [brand, reload]);
  const matches = useMemo(() => matchCvsStores(brand, query, stores), [brand, query, stores]);
  const brandName = brand === 'FAMI' ? c.family : brand === 'HILIFE' ? 'Hi-Life' : brand === 'OKMART' ? 'OK' : '7-ELEVEN';
  async function copy(all: boolean) {
    if (!selected) return;
    try {
      await navigator.clipboard.writeText(all ? `${brandName} · ${selected.name}\n${selected.code}\n${selected.address}` : selected.code);
      setNotice(all ? c.copiedAll : c.copied); setCopyView(true);
    } catch { setNotice(c.copyFailed); }
  }
  return <section className="ops-cvs" aria-label={c.lookup}>
    <h1>{onChoose ? c.pickup : c.lookup}</h1>
    {!selected && <>
      {!fixedBrand && <div className="ops-actions" role="group" aria-label={c.brand}>
        <button type="button" aria-pressed={brand === 'UNIMART'} onClick={() => { setBrand('UNIMART'); setQuery(''); }}>7-ELEVEN</button>
        <button type="button" aria-pressed={brand === 'FAMI'} onClick={() => { setBrand('FAMI'); setQuery(''); }}>{c.family}</button>
      </div>}
      <label className="ops-query">{c.query}<textarea aria-label={c.query} value={query} maxLength={2000} onChange={(e) => setQuery(e.target.value)} /></label>
      {loading ? <p role="status">{c.loading}</p> : failed ? <div role="status"><p>{c.unavailable}</p><button type="button" onClick={() => setReload((n) => n + 1)}>{c.retry}</button></div> : <>
        {matches.brandMismatch && <p role="status">{c.mismatch}</p>}
        {matches.codeMiss && <p role="status">{c.missing}{matches.codeMiss}</p>}
        {!query.trim() ? <p>{c.empty}</p> : matches.candidates.length === 0 && !matches.brandMismatch ? <p>{c.noMatches}</p> : null}
        <ul className="ops-candidates">{matches.candidates.map((item) => <li key={item.code}>
          <button type="button" onClick={() => { setSelected(item); setNotice(''); }}>
            <strong>{brandName} · {item.name} · {item.code}</strong><span>{item.address}</span>
          </button>
        </li>)}</ul>
      </>}
    </>}
    {selected && <>
      <div className="ops-store-card"><strong>{brandName} · {selected.name}</strong><p className="ops-code">{selected.code}</p><p>{selected.address}</p></div>
      {notice && <p role="status">{notice}</p>}
      {onChoose ? <button type="button" className="primary" onClick={() => onChoose(selected)}>{c.returnOrder}</button> : copyView ?
        <button type="button" onClick={() => { setCopyView(false); setNotice(''); }}>{c.back}</button> :
        <div className="ops-actions"><button type="button" onClick={() => void copy(false)}>{c.code}</button><button type="button" onClick={() => void copy(true)}>{c.all}</button></div>}
      <button type="button" className="ops-wide" onClick={() => { setSelected(null); setCopyView(false); setNotice(''); }}>{c.change}</button>
    </>}
    {onCancel && <button type="button" className="ops-wide" onClick={onCancel}>{c.cancel}</button>}
    <p className="ops-source">{c.source} {cvsDirectorySource(brand).date}</p>
  </section>;
}
