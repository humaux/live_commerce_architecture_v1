"use client";

// Home-page tab of the Design editor: add, remove, reorder and edit the five section types (hero, featured collection,
// product grid, text block, image with text; <= 20). Text blocks use the restricted markdown of packages/markdown-lite
// (validated again by Go). Data: `home.sections` of the design document, saved by Design.tsx via PUT
// /api/stores/{store}/design/draft -> Go internal/httpapi/design.go. Image pickers read the library through DesignMedia.
import { useState } from "react";
import { markdownProblem } from "@live-commerce/markdown-lite";
import type { DesignCopy } from "@/lib/design-copy";
import { blankSection, MAX, moved, SECTION_TYPES, type DesignDocument, type MediaItem, type Section, type SectionType } from "@/lib/design-model";
import { ImagePicker, type MediaOps } from "./DesignMedia";
import { Field } from "./DesignField";

const text = (v: string | null) => v ?? "";

function SectionEditor({
  store, index, section, pages, media, ops, c, issue, onChange,
}: {
  store: string; index: number; section: Section; pages: DesignDocument["pages"]; media: MediaItem[]; ops: MediaOps; c: DesignCopy;
  issue: (path: string) => string; onChange: (next: Section) => void;
}) {
  const h = c.home;
  const path = `home.sections[${index}]`;
  const heading = (
    <Field label={h.heading} error={issue(`${path}.heading`)}>
      <input value={text(section.heading)} maxLength={80} onChange={(e) => onChange({ ...section, heading: e.target.value })} />
    </Field>
  );
  const body = (value: string, max: number, set: (v: string) => void) => {
    const problem = markdownProblem(value);
    return (
      <Field label={h.body} error={problem ? c.pages.problem + problem : issue(`${path}.body`)}>
        <textarea rows={5} value={value} maxLength={max} onChange={(e) => set(e.target.value)} />
      </Field>
    );
  };
  switch (section.type) {
    case "hero":
      return (
        <>
          <ImagePicker store={store} label={h.image} value={section.image_id} media={media} ops={ops} c={c} testId={`design-section-${index}-image`}
            error={issue(`${path}.image_id`)} onChange={(id) => onChange({ ...section, image_id: id })} />
          {heading}
          <Field label={h.subheading}><input value={text(section.subheading)} maxLength={160} onChange={(e) => onChange({ ...section, subheading: e.target.value })} /></Field>
          <div className="design-two">
            <Field label={h.ctaLabel}><input value={text(section.cta_label)} maxLength={24} onChange={(e) => onChange({ ...section, cta_label: e.target.value })} /></Field>
            <Field label={h.ctaKind}>
              <select value={section.cta_kind ?? ""} onChange={(e) => onChange({ ...section, cta_kind: (e.target.value || null) as typeof section.cta_kind, cta_target: null })}>
                <option value="">{h.ctaNone}</option>
                <option value="all_products">{c.nav.kinds.all_products}</option>
                <option value="collection">{c.nav.kinds.collection}</option>
                <option value="page">{c.nav.kinds.page}</option>
              </select>
            </Field>
          </div>
          {section.cta_kind === "collection" && (
            <Field label={h.collectionSlug} error={issue(`${path}.cta_target`)}><input value={text(section.cta_target)} maxLength={80} onChange={(e) => onChange({ ...section, cta_target: e.target.value })} /></Field>
          )}
          {section.cta_kind === "page" && (
            <Field label={h.ctaTarget} error={issue(`${path}.cta_target`)}>
              <select value={text(section.cta_target)} onChange={(e) => onChange({ ...section, cta_target: e.target.value || null })}>
                <option value="">{c.nav.choosePage}</option>
                {pages.filter((p) => p.slug).map((p) => <option key={p.slug} value={p.slug}>{p.title || p.slug}</option>)}
              </select>
            </Field>
          )}
        </>
      );
    case "featured_collection":
      return (
        <>
          {heading}
          <div className="design-two">
            <Field label={h.collectionSlug} error={issue(`${path}.collection_slug`)}><input value={section.collection_slug} maxLength={80} onChange={(e) => onChange({ ...section, collection_slug: e.target.value })} /></Field>
            <Field label={h.limit}><input type="number" min={4} max={24} value={section.limit} onChange={(e) => onChange({ ...section, limit: Number(e.target.value) })} /></Field>
          </div>
        </>
      );
    case "product_grid":
      return (
        <>
          {heading}
          <div className="design-two">
            <Field label={h.sort}>
              <select value={section.sort} onChange={(e) => onChange({ ...section, sort: e.target.value as typeof section.sort })}>
                {Object.entries(h.sorts).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
            </Field>
            <Field label={h.limit}><input type="number" min={4} max={48} value={section.limit} onChange={(e) => onChange({ ...section, limit: Number(e.target.value) })} /></Field>
          </div>
        </>
      );
    case "rich_text":
      return <>{heading}{body(section.body, 4000, (v) => onChange({ ...section, body: v }))}</>;
    case "image_text":
      return (
        <>
          <ImagePicker store={store} label={h.image} value={section.image_id} media={media} ops={ops} c={c} testId={`design-section-${index}-image`}
            error={issue(`${path}.image_id`)} onChange={(id) => onChange({ ...section, image_id: id })} />
          {heading}
          {body(section.body, 2000, (v) => onChange({ ...section, body: v }))}
          <Field label={h.imageSide}>
            <select value={section.image_side} onChange={(e) => onChange({ ...section, image_side: e.target.value as "left" | "right" })}>
              <option value="left">{h.sides.left}</option>
              <option value="right">{h.sides.right}</option>
            </select>
          </Field>
        </>
      );
  }
}

export function DesignSections({
  store, sections, pages, media, ops, c, issue, onChange,
}: {
  store: string; sections: Section[]; pages: DesignDocument["pages"]; media: MediaItem[]; ops: MediaOps; c: DesignCopy;
  issue: (path: string) => string; onChange: (next: Section[]) => void;
}) {
  const [type, setType] = useState<SectionType>("hero");
  const full = sections.length >= MAX.sections;
  return (
    <div className="design-panel" data-testid="design-home">
      {sections.length === 0 && <p className="design-muted">{c.home.empty}</p>}
      {sections.map((section, i) => (
        <section className="design-block" key={i} data-testid="design-section">
          <div className="design-block-head">
            <h3>{c.home.types[section.type]}</h3>
            <div className="design-row-actions">
              <button type="button" aria-label={c.nav.up} title={c.nav.up} disabled={i === 0} onClick={() => onChange(moved(sections, i, -1))}>↑</button>
              <button type="button" aria-label={c.nav.down} title={c.nav.down} disabled={i === sections.length - 1} onClick={() => onChange(moved(sections, i, 1))}>↓</button>
              <button type="button" aria-label={c.nav.remove} title={c.nav.remove} onClick={() => onChange(sections.filter((_, j) => j !== i))}>×</button>
            </div>
          </div>
          <SectionEditor store={store} index={i} section={section} pages={pages} media={media} ops={ops} c={c} issue={issue}
            onChange={(next) => onChange(sections.map((s, j) => (j === i ? next : s)))} />
        </section>
      ))}
      <div className="design-add">
        <select aria-label={c.home.add} value={type} onChange={(e) => setType(e.target.value as SectionType)} data-testid="design-section-type">
          {SECTION_TYPES.map((t) => <option key={t} value={t}>{c.home.types[t]}</option>)}
        </select>
        <button type="button" disabled={full} data-testid="design-section-add"
          onClick={() => onChange([...sections, blankSection(type, media[0]?.id ?? null)])}>
          {full ? c.home.limit_reached : c.home.add}
        </button>
      </div>
    </div>
  );
}
