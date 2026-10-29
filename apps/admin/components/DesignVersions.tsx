// Purpose: Read-only publication history and explicit rollback controls.
// Depends on: @live-commerce/i18n, @live-commerce/format, @live-commerce/ui, design-copy, design-model, catalog-v2-copy.
// Used by: Design.tsx and the timezone component harness.
// Invariants: append-only server versions; rollback remains a fenced command owned by Design.
import type { Locale } from '@live-commerce/i18n';
import { displayTime } from '@live-commerce/format';
import { Badge, TableFrame } from '@live-commerce/ui';
import { catalogPresentationCopy } from '@/lib/catalog-v2-copy';
import { fill, type DesignCopy } from '@/lib/design-copy';
import type { VersionList } from '@/lib/design-model';

export function Versions({
  list,
  live,
  busy,
  c,
  locale,
  onRollback,
}: {
  list: VersionList;
  live: number | null;
  busy: boolean;
  c: DesignCopy;
  locale: Locale;
  onRollback: (version: number) => void;
}) {
  return (
    <div className="design-panel" data-testid="design-versions">
      <p className="design-muted">{c.versions.help}</p>
      {list.items.length === 0 && <p>{c.versions.none}</p>}
      {list.items.length > 0 && (
        <TableFrame
          label={c.tabs.versions}
          scrollHint={catalogPresentationCopy[locale].tableScroll}
        >
          <table className="design-table">
            <thead>
              <tr>
                <th>{c.versions.version}</th>
                <th>{c.versions.kind.publish}</th>
                <th>{c.versions.at}</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {list.items.map((v) => (
                <tr key={v.version} data-testid="design-version-row">
                  <td>
                    v{v.version}
                    {v.version === live && (
                      <Badge tone="info" className="design-live">
                        {c.versions.live}
                      </Badge>
                    )}
                  </td>
                  <td>
                    {c.versions.kind[v.kind]}
                    {v.kind === "rollback"
                      ? ` · ${fill(c.versions.source, { n: v.source_version })}`
                      : ""}
                  </td>
                  <td>{displayTime(locale, v.published_at)}</td>
                  <td>
                    {v.version !== live && (
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => onRollback(v.version)}
                        data-testid="design-rollback"
                      >
                        {c.versions.rollback}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </TableFrame>
      )}
    </div>
  );
}
