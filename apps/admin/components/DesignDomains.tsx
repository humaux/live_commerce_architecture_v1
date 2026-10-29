// Purpose: Show existing system/custom domain responses without adding domain settings.
// Depends on: react, design-copy, storefront-client, settings-client session fence.
// Used by: Design task hub.
// Invariants: never expose verification tokens; only ACTIVE and serving is ready; reads remain store/session scoped.
import { useEffect, useState } from 'react';
import type { Locale } from '@live-commerce/i18n';
import type { DesignCopy } from '@/lib/design-copy';
import { readDomains } from '@/lib/storefront-client';
import { sessionBoundary } from '@/lib/settings-client';

export function DesignDomains({ store, boundary, locale, c }: { store: string; boundary: string; locale: Locale; c: DesignCopy }) {
  const [view, setView] = useState<Awaited<ReturnType<typeof readDomains>> | null>(null);
  const [error, setError] = useState(false);
  useEffect(() => {
    let current = true;
    const controller = new AbortController();
    void (async () => {
      try {
        const result = await readDomains(store, controller.signal);
        if (current && await sessionBoundary() === boundary) setView(result);
      } catch { if (current) setError(true); }
    })();
    return () => { current = false; controller.abort(); };
  }, [store,boundary]);
  return <section className="design-panel" data-testid="design-domains">
    {!view && <p role="status">{error ? c.unavailable : c.loading}</p>}
    {view?.domains.map(d => <article key={d.origin} data-testid={`design-domain-${d.kind}`} data-state={d.state}>
      <h3>{d.kind === 'platform' ? c.editor.platform : c.editor.custom}</h3><p>{d.origin}</p>
      <p>{d.state === 'ACTIVE' && d.serving ? c.editor.active : ['DETACHED','SUSPENDED'].includes(d.state) ? c.editor.unavailable : c.editor.waiting}</p>
    </article>)}
    {view?.domains.length === 0 && <p>{c.editor.noAddress}</p>}
    <a href={`/${locale}/settings?store=${store}`} data-testid="design-domain-settings">{c.editor.settings}</a>
  </section>;
}
