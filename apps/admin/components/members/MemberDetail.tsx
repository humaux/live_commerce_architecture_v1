// Purpose: authoritative member balance/info and append-only ledger read view.
// Depends on: members-client, existing WorkspaceFrame/DS, shared format and scoped native links.
// Used by: members/[member]/page.tsx; actor fallback uses actual principal ID.
"use client";
import Link from "next/link";
import { decimal, displayTime } from "@live-commerce/format";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  PageHeader,
} from "@live-commerce/ui/ds";
import {
  appendEntries,
  readMember,
  useMemberPages,
} from "@/lib/members-client";
import { WorkspaceFrame } from "../WorkspaceFrame";
import { MemberReadState, type MembersProps } from "./MemberList";
import { membersCopy } from "./copy";
import s from "./members.module.css";
export function MemberDetail({
  locale,
  store,
  initialError,
  renderKey,
  member,
}: MembersProps & { member: string }) {
  const c = membersCopy[locale];
  const read = useMemberPages(
    `${renderKey}|${locale}|${store?.id}|${member}`,
    store
      ? (after, signal) => readMember(store.id, member, after, signal)
      : null,
    initialError ??
      (store && !store.permissions?.includes("customers:read")
        ? "forbidden"
        : null),
    (p) => p.next_ledger_cursor,
    appendEntries,
  );
  const data = read.data;
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.noStore}
      active="members"
    >
      <main className={s.page} data-testid="member-detail">
        <Link
          data-testid="member-back"
          prefetch={false}
          href={`/${locale}/members${store ? `?store=${store.id}` : ""}`}
        >
          {c.back}
        </Link>
        <PageHeader
          breadcrumbLabel={c.title}
          title={data?.member.display_name ?? c.detail}
          status={<Badge>{c.readOnly}</Badge>}
        />
        <MemberReadState
          status={read.status}
          kind={read.kind}
          c={c}
          locale={locale}
          retry={read.reload}
        />
        {data && (
          <>
            <Card title={c.info}>
              <div className={s.summary}>
                <div>
                  <p>{c.balance}</p>
                  <strong className={s.balance}>
                    {decimal(locale, data.member.balance, 0)}
                  </strong>
                </div>
                <dl className={s.info}>
                  <dt>{c.name}</dt>
                  <dd>{data.member.display_name}</dd>
                  <dt>{c.contact}</dt>
                  <dd>{data.member.contact_note ?? c.dash}</dd>
                  <dt>{c.notes}</dt>
                  <dd>{data.member.notes ?? c.dash}</dd>
                  <dt>{c.updated}</dt>
                  <dd>{displayTime(locale, data.member.updated_at)}</dd>
                </dl>
              </div>
              {data.member.erased_at && <Badge>{c.erased}</Badge>}
            </Card>
            <section data-testid="member-ledger" aria-label={c.ledger}>
              <Card title={c.ledger} flush>
                <div className={`${s.tableWrap} ${s.ledgerTable}`}>
                  <DataTable
                    caption={c.ledger}
                    rows={data.entries}
                    rowId={(entry) => entry.id}
                    empty={<EmptyState title={c.noEntries} />}
                    error={null}
                    loadingLabel={c.loading}
                    columns={[
                      {
                        id: "date",
                        header: c.date,
                        cell: (entry) => (
                          <div data-label={c.date}>
                            <time dateTime={entry.created_at}>
                              {displayTime(locale, entry.created_at)}
                            </time>
                          </div>
                        ),
                      },
                      {
                        id: "delta",
                        header: c.delta,
                        cell: (entry) => (
                          <div data-label={c.delta}>
                            {entry.delta > 0 ? "+" : ""}
                            {decimal(locale, entry.delta, 0)}
                          </div>
                        ),
                      },
                      {
                        id: "reason",
                        header: c.reason,
                        wrap: true,
                        cell: (entry) => (
                          <div data-label={c.reason}>
                            {entry.reason === "manual_order_award"
                              ? c.award
                              : c.adjustment}
                          </div>
                        ),
                      },
                      {
                        id: "order",
                        header: c.order,
                        wrap: true,
                        cell: (entry) => (
                          <div data-label={c.order}>
                            {entry.order_id ? (
                              <Link
                                prefetch={false}
                                href={`/${locale}/orders/${entry.order_id}?store=${store!.id}`}
                              >
                                {entry.order_id}
                              </Link>
                            ) : (
                              c.dash
                            )}
                          </div>
                        ),
                      },
                      {
                        id: "note",
                        header: c.notes,
                        wrap: true,
                        cell: (entry) => (
                          <div data-label={c.notes}>{entry.note ?? c.dash}</div>
                        ),
                      },
                      {
                        id: "actor",
                        header: c.actor,
                        wrap: true,
                        cell: (entry) => (
                          <div data-label={c.actor}>
                            {entry.actor.display_name ??
                              `${c.staffID}: ${entry.actor.principal_id}`}
                          </div>
                        ),
                      },
                    ]}
                  />
                </div>
                <footer className={s.footer} aria-live="polite">
                  <span>
                    {c.loaded}: {data.entries.length}
                  </span>
                  {read.pageError && <p role="alert">{c[read.pageError]}</p>}
                  {data.next_ledger_cursor !== null ? (
                    <Button
                      data-testid="member-ledger-load-more"
                      disabledReason={read.paging ? c.loading : undefined}
                      onClick={() => void read.more()}
                    >
                      {read.paging
                        ? c.loading
                        : read.pageError
                          ? c.retry
                          : c.more}
                    </Button>
                  ) : (
                    <span>{c.noMore}</span>
                  )}
                </footer>
              </Card>
            </section>
          </>
        )}
      </main>
    </WorkspaceFrame>
  );
}
