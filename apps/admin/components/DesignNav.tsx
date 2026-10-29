// Purpose: Edit header/footer links with one disclosure and secondary actions.
// Depends on: react, design-copy, design-model, DesignField.
// Used by: Design.tsx.
// Invariants: Preserve link order, limits and target validation; no requests.
"use client";

// Navigation tab of the Design editor: header menu (<= 8 links) and footer links (<= 12). A link points at home, all
// products, a collection (free slug until the catalog-core collections list is wired), one of the pages of this same
// document, or an https URL. Data: `nav` of the design document, saved by Design.tsx via PUT /api/stores/{store}/design/draft
// -> Go internal/httpapi/design.go. Pure form: no request here.
import { useState } from "react";
import type { DesignCopy } from "@/lib/design-copy";
import { MAX, moved, type DesignDocument, type NavItem } from "@/lib/design-model";
import { Field } from "./DesignField";

type Nav = DesignDocument["nav"];
const HEADER_KINDS: NavItem["kind"][] = ["home", "all_products", "collection", "page", "url"];
const FOOTER_KINDS: NavItem["kind"][] = ["page", "url", "collection"];

function List({
  name, items, kinds, max, pages, c, issue, onChange, open, more, setOpen, setMore,
}: {
  name: "header" | "footer"; items: NavItem[]; kinds: NavItem["kind"][]; max: number; pages: DesignDocument["pages"]; c: DesignCopy;
  issue: (path: string) => string; onChange: (items: NavItem[]) => void;
  open: string; more: string; setOpen: (v: string) => void; setMore: (v: string) => void;
}) {
  const n = c.nav;
  const edit = (i: number, patch: Partial<NavItem>) => onChange(items.map((item, j) => (j === i ? { ...item, ...patch } : item)));
  return (
    <section className="design-block" data-testid={`design-nav-${name}`}>
      <h3>{name === "header" ? n.header : n.footer} <span className="design-count">{`${items.length}/${max}`}</span></h3>
      {items.length === 0 && <p className="design-muted">{n.empty}</p>}
      {items.map((item, i) => {
        const path = `nav.${name}[${i}]`;
        return (
          <div className="design-row" key={i} data-testid={`design-nav-${name}-row`}>
            <div className="design-block-head">
              <button type="button" data-testid="design-nav-edit" aria-expanded={open === path} onClick={() => { setOpen(open === path ? '' : path); setMore(''); }}>{item.label || n.add}</button>
              <button type="button" data-testid="design-nav-more" aria-label={c.editor.more} aria-expanded={more === path} onClick={() => { setMore(more === path ? '' : path); setOpen(''); }}>⋯</button>
            </div>
            <div hidden={open !== path}>
            <Field label={n.label} error={issue(`${path}.label`)}>
              <input value={item.label} maxLength={30} onChange={(e) => edit(i, { label: e.target.value })} />
            </Field>
            <Field label={n.kind}>
              <select value={item.kind} onChange={(e) => edit(i, { kind: e.target.value as NavItem["kind"], target: null })}>
                {kinds.map((kind) => <option key={kind} value={kind}>{n.kinds[kind]}</option>)}
              </select>
            </Field>
            {item.kind === "collection" && (
              <Field label={n.collectionSlug} error={issue(`${path}.target`)}>
                <input value={item.target ?? ""} maxLength={80} onChange={(e) => edit(i, { target: e.target.value })} />
              </Field>
            )}
            {item.kind === "page" && (
              <Field label={n.target} error={issue(`${path}.target`)}>
                <select value={item.target ?? ""} onChange={(e) => edit(i, { target: e.target.value || null })}>
                  <option value="">{n.choosePage}</option>
                  {pages.filter((p) => p.slug).map((p) => <option key={p.slug} value={p.slug}>{p.title || p.slug}</option>)}
                </select>
              </Field>
            )}
            {item.kind === "url" && (
              <Field label={n.target} error={issue(`${path}.target`)}>
                <input type="url" placeholder={n.url} value={item.target ?? ""} onChange={(e) => edit(i, { target: e.target.value })} />
              </Field>
            )}
            </div>
            <div className="design-row-actions" hidden={more !== path}>
              <button type="button" aria-label={n.up} title={n.up} disabled={i === 0} onClick={() => onChange(moved(items, i, -1))}>↑</button>
              <button type="button" aria-label={n.down} title={n.down} disabled={i === items.length - 1} onClick={() => onChange(moved(items, i, 1))}>↓</button>
              <button type="button" aria-label={n.remove} title={n.remove} onClick={() => onChange(items.filter((_, j) => j !== i))}>×</button>
            </div>
          </div>
        );
      })}
      <button type="button" disabled={items.length >= max} data-testid={`design-nav-${name}-add`}
        onClick={() => { onChange([...items, { label: "", kind: kinds[0], target: null }]); setOpen(`nav.${name}[${items.length}]`); setMore(''); }}>
        {items.length >= max ? n.limit : n.add}
      </button>
    </section>
  );
}

export function DesignNav({
  nav, pages, c, issue, onChange,
}: {
  nav: Nav; pages: DesignDocument["pages"]; c: DesignCopy; issue: (path: string) => string; onChange: (next: Nav) => void;
}) {
  const [open, setOpen] = useState('');
  const [more, setMore] = useState('');
  return (
    <div className="design-panel" data-testid="design-nav">
      <List name="header" items={nav.header} kinds={HEADER_KINDS} max={MAX.header} pages={pages} c={c} issue={issue} open={open} more={more} setOpen={setOpen} setMore={setMore} onChange={(header) => onChange({ ...nav, header })} />
      <List name="footer" items={nav.footer} kinds={FOOTER_KINDS} max={MAX.footer} pages={pages} c={c} issue={issue} open={open} more={more} setOpen={setOpen} setMore={setMore} onChange={(footer) => onChange({ ...nav, footer })} />
    </div>
  );
}
