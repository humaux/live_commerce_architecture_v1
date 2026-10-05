"use client";
// Read-only attribution panels: frozen BFF aggregates -> Go /v1/admin/stores/{store}/ads/attribution.
// Parent owns report navigation/loading; this module owns the complete report tables and their empty states.
import type { ReactNode } from "react";
import Link from "next/link";
import type { Locale } from "@live-commerce/i18n";
import { TableFrame } from "@live-commerce/ui";
import { money, displayTime } from "@live-commerce/format";
import type { AttributionCopy } from "@/lib/attribution-copy";
import { formatROAS } from "@/lib/attribution-format";
import { matchRoute } from "@/src/routes";
import type {
  Buyers,
  DraftAttribution,
  SessionAttribution,
} from "@/lib/attribution-model";
import { AttributionAudienceRead } from "./AttributionAudienceRead";

function Table({
  c,
  headers,
  children,
  empty,
  testID,
  emptyText,
}: {
  c: AttributionCopy;
  headers: string[];
  children: ReactNode;
  empty: boolean;
  testID?: string;
  emptyText?: string;
}) {
  if (empty) return <p className="attribution-note">{emptyText ?? c.noRows}</p>;
  return (
    <TableFrame
      className="attribution-scroll"
      label={headers.join(" · ")}
      scrollHint={c.scrollHint}
    >
      <table data-testid={testID}>
        <thead>
          <tr>
            {headers.map((h, i) => (
              <th scope="col" key={i}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </TableFrame>
  );
}
const number = (locale: Locale, n: number | null, c: AttributionCopy) =>
  n === null ? c.unknown : String(n);
const amount = (
  locale: Locale,
  currency: string,
  n: number | null,
  c: AttributionCopy,
) => (n === null ? c.unknown : money(locale, currency, n));

export function DraftPanel({
  c,
  locale,
  draft: d,
}: {
  c: AttributionCopy;
  locale: Locale;
  draft: DraftAttribution;
}) {
  const dimension = {
    age_gender: c.ageGender,
    region: c.region,
    placement: c.placement,
    device: c.device,
    hourly: c.hourly,
  };
  return (
    <section
      className="attribution-section"
      aria-labelledby="attribution-draft-title"
      data-testid="attribution-draft-panel"
    >
      <h2 id="attribution-draft-title">{c.drafts}</h2>
      <p>
        {c.source}: {d.source_ref || d.draft_id}
      </p>
      <p className="attribution-note" data-testid="attribution-meta-zone">
        {c.metaZone}: {d.meta_account_timezone ?? c.unknown}
      </p>
      {d.provisional && (
        <p role="status" data-testid="attribution-provisional">
          {c.provisional}
        </p>
      )}
      <dl className="attribution-facts">
        <div>
          <dt>{c.spend}</dt>
          <dd>
            {d.spend_minor === null
              ? "—"
              : money(locale, d.currency, d.spend_minor)}
          </dd>
        </div>
        <div>
          <dt>{c.roas}</dt>
          <dd>{formatROAS(locale, d.roas, "—")}</dd>
        </div>
      </dl>
      <div className="attribution-pair">
        <div data-testid="attribution-ours">
          <h3>{c.ordersTitle}</h3>
          <p className="attribution-note">{c.boostedNote}</p>
          <Table
            c={c}
            testID="attribution-order-paths"
            headers={[c.path, c.orders, c.net, c.pending, c.pendingValue]}
            empty={!d.orders.length}
          >
            {d.orders.map((o) => (
              <tr key={o.path} data-testid={`attribution-path-${o.path}`}>
                <th scope="row">
                  {o.path === "ad_click" ? c.adClick : c.boosted}
                </th>
                <td>{number(locale, o.orders, c)}</td>
                <td>{money(locale, d.currency, o.net_minor)}</td>
                <td>{number(locale, o.pending_orders, c)}</td>
                <td>{money(locale, d.currency, o.pending_minor)}</td>
              </tr>
            ))}
          </Table>
        </div>
        <div data-testid="attribution-meta">
          <h3>{c.metaTitle}</h3>
          <dl className="attribution-facts">
            <div>
              <dt>{c.purchases}</dt>
              <dd>{number(locale, d.meta.purchases, c)}</dd>
            </div>
            <div>
              <dt>{c.purchaseValue}</dt>
              <dd>
                {amount(locale, d.currency, d.meta.purchase_value_minor, c)}
              </dd>
            </div>
          </dl>
        </div>
      </div>
      <h3>{c.audience}</h3>
      {!d.breakdowns.length && <p>{c.noAudience}</p>}
      {d.breakdowns_unavailable.map((unavailable, i) => (
        <p
          className="attribution-note"
          data-testid="attribution-breakdowns-unavailable"
          key={i}
        >
          {unavailable.day} · {c.breakdownsUnavailable}:{" "}
          {unavailable.dimensions
            .map((name) =>
              Object.hasOwn(dimension, name)
                ? dimension[name as keyof typeof dimension]
                : name,
            )
            .join(", ")}
        </p>
      ))}
      <Table
        c={c}
        testID="attribution-breakdowns"
        headers={[
          c.day,
          c.metaZone,
          c.dimension,
          c.bucket,
          c.at,
          c.spend,
          c.reach,
          c.impressions,
          c.clicks,
          c.engagements,
          c.comments,
          c.purchases,
          c.purchaseValue,
        ]}
        empty={!d.breakdowns.length}
      >
        {d.breakdowns.map((b, i) => (
          <tr key={i}>
            <td>{b.day}</td>
            <td>{b.timezone_name}</td>
            <td>{dimension[b.dimension]}</td>
            <th scope="row">{b.bucket}</th>
            <td>{b.hour_start ? displayTime(locale, b.hour_start) : "—"}</td>
            <td>{amount(locale, d.currency, b.spend_minor, c)}</td>
            <td>{number(locale, b.reach, c)}</td>
            <td>{number(locale, b.impressions, c)}</td>
            <td>{number(locale, b.clicks, c)}</td>
            <td>{number(locale, b.engagements, c)}</td>
            <td>{number(locale, b.comments, c)}</td>
            <td>{number(locale, b.purchases, c)}</td>
            <td>{amount(locale, d.currency, b.purchase_value_minor, c)}</td>
          </tr>
        ))}
      </Table>
      <BuyerPanel
        c={c}
        locale={locale}
        currency={d.currency}
        buyers={d.buyers}
      />
    </section>
  );
}

function BuyerPanel({
  c,
  locale,
  currency,
  buyers: b,
}: {
  c: AttributionCopy;
  locale: Locale;
  currency: string;
  buyers: Buyers;
}) {
  return (
    <div data-testid="attribution-buyers">
      <h3>{c.buyers}</h3>
      <dl className="attribution-facts">
        <div>
          <dt>{c.newBuyers}</dt>
          <dd>{number(locale, b.new_buyers, c)}</dd>
        </div>
        <div>
          <dt>{c.returning}</dt>
          <dd>{number(locale, b.returning_buyers, c)}</dd>
        </div>
        <div>
          <dt>{c.average}</dt>
          <dd>{amount(locale, currency, b.average_order_minor, c)}</dd>
        </div>
      </dl>
      {!b.counties.length &&
      !b.top_products.length &&
      !b.orders_per_minute.length ? (
        <p className="attribution-note">{c.noBuyers}</p>
      ) : (
        <>
          <Table
            c={c}
            testID="attribution-counties"
            headers={[c.county, c.orders, c.net]}
            empty={!b.counties.length}
          >
            {b.counties.map((row, i) => (
              <tr key={i}>
                <th scope="row">{row.name}</th>
                <td>{number(locale, row.orders, c)}</td>
                <td>{money(locale, currency, row.net_minor)}</td>
              </tr>
            ))}
          </Table>
          <h4>{c.products}</h4>
          <Table
            c={c}
            testID="attribution-products"
            headers={[c.product, c.quantity]}
            empty={!b.top_products.length}
          >
            {b.top_products.map((p, i) => (
              <tr key={i}>
                <th scope="row">{p.name}</th>
                <td>{number(locale, p.quantity, c)}</td>
              </tr>
            ))}
          </Table>
          <h4>{c.minuteOrders}</h4>
          <Table
            c={c}
            headers={[c.at, c.orders, c.net]}
            empty={!b.orders_per_minute.length}
          >
            {b.orders_per_minute.map((o, i) => (
              <tr key={i}>
                <th scope="row">{displayTime(locale, o.at)}</th>
                <td>{number(locale, o.orders, c)}</td>
                <td>{money(locale, currency, o.net_minor)}</td>
              </tr>
            ))}
          </Table>
        </>
      )}
    </div>
  );
}

export function SessionPanel({
  c,
  locale,
  session: s,
  store,
}: {
  c: AttributionCopy;
  locale: Locale;
  session: SessionAttribution;
  store: string;
}) {
  const audience = s.live_audience;
  return (
    <section
      className="attribution-section"
      aria-labelledby="attribution-session-title"
      data-testid="attribution-session-panel"
    >
      <h2 id="attribution-session-title">
        {c.sessions}: {s.title}
      </h2>
      <p className="attribution-note">
        {c.start}: {displayTime(locale, s.starts_at)} · {c.end}:{" "}
        {s.ends_at ? displayTime(locale, s.ends_at) : c.unknown}
      </p>
      <p className="attribution-note">
        {c.post}: {s.post_ids.join(", ") || c.unknown}
      </p>
      {s.draft_ids.length > 0 && (
        <details
          className="attribution-linked-drafts"
          data-testid="attribution-linked-drafts"
        >
          <summary>
            {c.draftIds} ({s.draft_ids.length})
          </summary>
          <ul>
            {s.draft_ids.map((id) => (
              <li key={id}>{id}</li>
            ))}
          </ul>
        </details>
      )}
      <dl className="attribution-facts" data-testid="attribution-session-facts">
        <div>
          <dt>{c.sessionSpend}</dt>
          <dd>
            {s.spend_minor === null
              ? "—"
              : money(locale, s.currency, s.spend_minor)}
          </dd>
        </div>
        <div>
          <dt>{c.orders}</dt>
          <dd>{number(locale, s.orders, c)}</dd>
        </div>
        <div>
          <dt>{c.net}</dt>
          <dd>{money(locale, s.currency, s.net_minor)}</dd>
        </div>
        <div>
          <dt>{c.pending}</dt>
          <dd>{number(locale, s.pending_orders, c)}</dd>
        </div>
        <div>
          <dt>{c.pendingValue}</dt>
          <dd>{money(locale, s.currency, s.pending_minor)}</dd>
        </div>
        <div>
          <dt>{c.roas}</dt>
          <dd>
            {formatROAS(
              locale,
              s.spend_minor === null || s.spend_minor === 0
                ? null
                : s.net_minor / s.spend_minor,
              "—",
            )}
          </dd>
        </div>
      </dl>
      {s.ambiguous_orders > 0 && (
        <p data-testid="attribution-ambiguous">
          {c.ambiguous}: {number(locale, s.ambiguous_orders, c)}
        </p>
      )}
      <h3>{c.funnel}</h3>
      <Table
        c={c}
        headers={[c.comments, c.claims, c.checkoutLinks, c.paid]}
        empty={false}
        testID="attribution-funnel"
      >
        <tr>
          <td>{s.funnel.comments}</td>
          <td>{s.funnel.claims}</td>
          <td>{s.funnel.checkout_links}</td>
          <td>{s.funnel.paid_orders}</td>
        </tr>
      </Table>
      <h3>{c.timeline}</h3>
      <p className="attribution-note">{c.timelineNote}</p>
      <Table
        c={c}
        emptyText={c.noTimeline}
        headers={[
          c.at,
          c.spend,
          c.viewers,
          c.comments,
          c.claims,
          c.orders,
          c.net,
        ]}
        empty={!s.timeline.length}
        testID="attribution-timeline"
      >
        {s.timeline.map((t, i) => (
          <tr key={i}>
            <th scope="row">{displayTime(locale, t.at)}</th>
            <td>{amount(locale, s.currency, t.spend_minor, c)}</td>
            <td>{number(locale, t.viewers, c)}</td>
            <td>{number(locale, t.comments, c)}</td>
            <td>{number(locale, t.claims, c)}</td>
            <td>{number(locale, t.orders, c)}</td>
            <td>{money(locale, s.currency, t.net_minor)}</td>
          </tr>
        ))}
      </Table>
      <div className="attribution-pair">
        <div data-testid="attribution-live-audience">
          <h3>{c.liveAudience}</h3>
          {audience.status !== "not_authorized" && (
            <AttributionAudienceRead
              key={`${store}:${s.session_id}`}
              store={store}
              session={s.session_id}
              c={c}
            />
          )}
          {audience.status === "not_authorized" ? (
            <>
              <p data-testid="attribution-not-authorized">{c.notAuthorized}</p>
              <Link
                data-testid="attribution-reconnect"
                href={`/${locale}${matchRoute("/settings")!.path}?store=${encodeURIComponent(store)}`}
              >
                {c.reconnect}
              </Link>
            </>
          ) : audience.status === "not_read" ? (
            <p data-testid="attribution-not-read">{c.notRead}</p>
          ) : (
            <>
              <dl className="attribution-facts">
                <div>
                  <dt>{c.views}</dt>
                  <dd>{number(locale, audience.views, c)}</dd>
                </div>
                <div>
                  <dt>{c.peak}</dt>
                  <dd>{number(locale, audience.peak_concurrent, c)}</dd>
                </div>
                <div>
                  <dt>{c.totalTime}</dt>
                  <dd>{number(locale, audience.total_view_time_ms, c)}</dd>
                </div>
              </dl>
              {audience.status === "insufficient" ||
              (!audience.age_gender.length && !audience.regions.length) ? (
                <p data-testid="attribution-insufficient">{c.insufficient}</p>
              ) : (
                <>
                  <Table
                    c={c}
                    headers={[c.ageGender, c.viewTime]}
                    empty={!audience.age_gender.length}
                  >
                    {audience.age_gender.map((b, i) => (
                      <tr key={i}>
                        <th scope="row">{b.bucket}</th>
                        <td>{number(locale, b.view_time_ms, c)}</td>
                      </tr>
                    ))}
                  </Table>
                  <Table
                    c={c}
                    headers={[c.region, c.viewTime]}
                    empty={!audience.regions.length}
                  >
                    {audience.regions.map((b, i) => (
                      <tr key={i}>
                        <th scope="row">{b.bucket}</th>
                        <td>{number(locale, b.view_time_ms, c)}</td>
                      </tr>
                    ))}
                  </Table>
                </>
              )}
            </>
          )}
        </div>
        <BuyerPanel
          c={c}
          locale={locale}
          currency={s.currency}
          buyers={s.buyers}
        />
      </div>
    </section>
  );
}
