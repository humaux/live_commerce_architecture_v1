// Purpose: Read-only review of every field in the exact document being published.
// Depends on: design-copy and design-model; no requests or mutations.
// Used by: Design review, publication result and receipt-bound content views.
// Invariants: preserve all four groups, order and empty page facts; never invent optional values.
import type { DesignCopy } from '@/lib/design-copy';
import type { DesignDocument } from '@/lib/design-model';

export function DesignSummary({ doc, c, store }: { doc: DesignDocument; c: DesignCopy; store: string }) {
  const profile = doc.profile;
  return <div className="design-summary" data-testid="design-summary">
    <section data-testid="design-summary-profile"><details name="design-review-groups"><summary>{c.editor.profile}</summary>
      <dl>{Object.entries(profile).filter(([key]) => key !== 'contact').map(([key, value]) => value &&
        <div key={key}><dt>{({name:c.profile.name, accent_color:c.profile.accent, tagline:c.profile.tagline,
          logo_image_id:c.profile.logo, favicon_image_id:c.profile.favicon, announcement:c.profile.announcement} as Record<string,string>)[key]}</dt><dd>{key.endsWith('_image_id') ? <img width={64} height={64} src={`/api/stores/${store}/design/media/${value}`} alt={c.media.alt} /> : String(value)}</dd></div>)}
        {Object.entries(profile.contact).map(([key, value]) => value && <div key={key}>
          <dt>{({email:c.profile.email, phone:c.profile.phone, address:c.profile.address, line_url:c.profile.line,
            facebook_url:c.profile.facebook, instagram_url:c.profile.instagram} as Record<string,string>)[key]}</dt><dd>{value}</dd></div>)}
      </dl>
    </details></section>
    <section data-testid="design-summary-home"><details name="design-review-groups"><summary>{c.editor.home}</summary>
      <ol>{doc.home.sections.map((s,i) => <li key={i}><strong>{s.heading ? `${s.heading} · ${c.home.types[s.type]}` : c.home.types[s.type]}</strong>
        <dl>{Object.entries(s).filter(([key,value]) => !['heading','type'].includes(key) && value !== null).map(([key,value]) => <div key={key}>
          <dt>{({image_id:c.home.image, subheading:c.home.subheading, cta_label:c.home.ctaLabel, cta_kind:c.home.ctaKind,
            cta_target:c.home.ctaTarget, collection_slug:c.home.collectionSlug, limit:c.home.limit, sort:c.home.sort,
            body:c.home.body, image_side:c.home.imageSide} as Record<string,string>)[key]}</dt>
          <dd>{key === 'image_id' ? <img width={64} height={64} src={`/api/stores/${store}/design/media/${value}`} alt={c.media.alt} /> : key === 'sort' ? c.home.sorts[String(value)] : key === 'cta_kind' ? c.nav.kinds[String(value)] : key === 'image_side' ? c.home.sides[String(value)] : String(value)}</dd>
        </div>)}</dl></li>)}</ol>
    </details></section>
    <section data-testid="design-summary-nav"><details name="design-review-groups"><summary>{c.editor.nav}</summary>
      {(['header','footer'] as const).map(name => <div key={name}><h4>{c.nav[name]}</h4><ol>{doc.nav[name].map((link,i) => <li key={i}>
        {link.label} · {c.nav.kinds[link.kind]}{link.target ? ` · ${link.target}` : ''}
      </li>)}</ol></div>)}
    </details></section>
    <section data-testid="design-summary-pages"><details name="design-review-groups"><summary>{c.editor.pages}</summary>
      {doc.pages.map((p,i) => <article key={i}><h4>{p.title}</h4><p>/{p.slug}</p><p className="design-preserve-lines">{p.body || c.editor.empty}</p></article>)}
    </details></section>
  </div>;
}
