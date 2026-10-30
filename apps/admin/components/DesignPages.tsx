// Purpose: Edit one information page with escaped Markdown preview.
// Depends on: react, markdown-lite, design-copy, design-model, DesignField.
// Used by: Design.tsx.
// Invariants: Title/slug required, body may be empty; previews escape before rendering.
"use client";

// Pages tab of the Design editor: information pages (shipping, returns, contact…) with a slug, a title and a restricted-
// markdown body. The live preview is packages/markdown-lite renderMarkdown, which escapes every character before adding
// its own whitelisted tags, so the merchant sees exactly what the storefront will render. Data: `pages` of the design
// document, saved by Design.tsx via PUT /api/stores/{store}/design/draft -> Go internal/httpapi/design.go (which validates the
// same grammar and is the authority). Pure form: no request here.
import { useState } from "react";
import { markdownProblem, renderMarkdown } from "@live-commerce/markdown-lite";
import type { DesignCopy } from "@/lib/design-copy";
import { MAX, type Page } from "@/lib/design-model";
import { Field } from "./DesignField";

export function DesignPages({ pages, c, issue, onChange }: { pages: Page[]; c: DesignCopy; issue: (path: string) => string; onChange: (next: Page[]) => void }) {
  const p = c.pages;
  const edit = (i: number, patch: Partial<Page>) => onChange(pages.map((page, j) => (j === i ? { ...page, ...patch } : page)));
  const full = pages.length >= MAX.pages;
  const [open, setOpen] = useState<number | null>(null);
  const [more, setMore] = useState(false);
  return (
    <div className="design-panel" data-testid="design-pages">
      {pages.length === 0 && <p className="design-muted">{p.empty}</p>}
      <div className="design-task-list">{pages.map((page,i) => <button type="button" key={i} data-testid="design-page-open" aria-expanded={open === i} onClick={() => { setOpen(open === i ? null : i); setMore(false); }}>{page.title || page.slug || c.editor.newPage}</button>)}</div>
      {pages.map((page, i) => {
        if (open !== i) return null;
        const problem = markdownProblem(page.body);
        return (
          <section className="design-block" key={i} data-testid="design-page-block">
            <div className="design-block-head">
              <button type="button" data-testid="design-page-more" aria-label={c.editor.more} aria-expanded={more} onClick={() => setMore(!more)}>⋯</button>
              <div hidden={!more}>
              <button type="button" aria-label={p.remove} title={p.remove} onClick={() => onChange(pages.filter((_, j) => j !== i))}>×</button>
              </div>
            </div>
            <div className="design-two">
              <Field label={p.title} error={issue(`pages[${i}].title`)}><input value={page.title} maxLength={80} onChange={(e) => edit(i, { title: e.target.value })} /></Field>
              <Field label={p.slug} error={issue(`pages[${i}].slug`)}><input value={page.slug} maxLength={80} spellCheck={false} onChange={(e) => edit(i, { slug: e.target.value })} /></Field>
            </div>
            <div className="design-two design-md">
              <Field label={p.body} error={problem ? p.problem + problem : issue(`pages[${i}].body`)}>
                <textarea rows={9} value={page.body} maxLength={20000} onChange={(e) => edit(i, { body: e.target.value })} />
              </Field>
              <div className="design-field">
                <span className="design-label">{p.preview}</span>
                {/* Escaped-first output of markdown-lite: only p/ul/li/strong/em/br and https links can appear. */}
                <div className="design-preview" data-testid="design-page-preview" dangerouslySetInnerHTML={{ __html: renderMarkdown(page.body) }} />
              </div>
            </div>
            <details><summary>{p.preview}</summary><small className="design-muted">{p.help}</small></details>
          </section>
        );
      })}
      <button type="button" disabled={full} data-testid="design-page-add" onClick={() => { onChange([...pages, { slug: "", title: "", body: "" }]); setOpen(pages.length); setMore(false); }}>
        {full ? p.limit : p.add}
      </button>
    </div>
  );
}
