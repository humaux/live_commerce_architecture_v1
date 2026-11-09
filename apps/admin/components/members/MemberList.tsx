// Purpose: scoped read-only member search and incremental list, with private query held only in memory.
// Depends on: LP1 members-client, WorkspaceFrame, DS, shared format and native Next links.
// Used by: members/page.tsx; no member mutations or query-bearing page links.
"use client";
import { useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { decimal, displayTime } from "@live-commerce/format";
import type { Locale } from "@live-commerce/i18n";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  ErrorState,
  PageHeader,
  PermissionState,
} from "@live-commerce/ui/ds";
import type { Store } from "@/lib/model";
import type { ReadCode, ReadStatus } from "@/lib/customers-client";
import {
  appendMembers,
  memberQuery,
  searchMembers,
  useMemberPages,
} from "@/lib/members-client";
import { WorkspaceFrame } from "../WorkspaceFrame";
import AdminLoading from "../AdminLoading";
import { membersCopy, type MembersCopy } from "./copy";
import s from "./members.module.css";
export type MembersProps = {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  initialError: ReadCode | null;
  renderKey: string;
};
export function MemberReadState({
  status,
  kind,
  c,
  locale,
  retry,
}: {
  status: ReadStatus;
  kind: "network" | "protocol";
  c: MembersCopy;
  locale: Locale;
  retry: () => void;
}) {
  if (status === "loading" || status === "hidden") return <AdminLoading />;
  if (status === "forbidden")
    return <PermissionState title={c.forbidden} reason={c.permission} />;
  if (status === "signed-out")
    return (
      <ErrorState
        title={c.signedOut}
        actions={<Link href={`/${locale}/`}>{c.signIn}</Link>}
      />
    );
  if (status === "not-found") return <EmptyState title={c.notFound} />;
  if (status === "unavailable")
    return <ErrorState title={c[kind]} onRetry={retry} retryLabel={c.retry} />;
  return null;
}
export function MemberList(props: MembersProps) {
  // Remount on every server scope/render boundary so draft PII cannot survive store/locale navigation.
  return (
    <MemberListScope
      key={`${props.renderKey}|${props.locale}|${props.store?.id}`}
      {...props}
    />
  );
}
function MemberListScope({
  locale,
  store,
  initialError,
  renderKey,
}: MembersProps) {
  const c = membersCopy[locale];
  const [draft, setDraft] = useState(""),
    [q, setQ] = useState("");
  const [queryError, setQueryError] = useState(false),
    [revision, setRevision] = useState(0);
  const read = useMemberPages(
    `${renderKey}|${store?.id}|${q}|${revision}`,
    store ? (after, signal) => searchMembers(store.id, q, after, signal) : null,
    initialError ??
      (store && !store.permissions?.includes("customers:read")
        ? "forbidden"
        : null),
    (p) => p.next_cursor,
    appendMembers,
  );
  useEffect(() => {
    if (read.status === "signed-out") setDraft("");
  }, [read.status]);
  function search(event: FormEvent) {
    event.preventDefault();
    try {
      const next = memberQuery(draft);
      setQueryError(false);
      setQ(next ?? "");
      setRevision((n) => n + 1);
    } catch {
      setQueryError(true);
    }
  }
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? c.noStore}
      active="members"
    >
      <main className={s.page} data-testid="members-list">
        <PageHeader
          breadcrumbLabel={c.title}
          title={c.title}
          description={c.description}
        />
        {read.status !== "signed-out" && read.status !== "hidden" && (
          <form
            role="search"
            data-testid="members-search"
            className={s.search}
            onSubmit={search}
          >
            <label htmlFor="members-query">{c.search}</label>
            <input
              id="members-query"
              type="search"
              autoComplete="off"
              data-testid="members-search-input"
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              aria-invalid={queryError}
              aria-describedby={queryError ? "members-query-error" : undefined}
            />
            <Button
              type="submit"
              data-testid="members-search-submit"
              disabledReason={!store ? c.noStore : undefined}
            >
              {c.submit}
            </Button>
            {queryError && (
              <p id="members-query-error" role="alert">
                {c.invalidQuery}
              </p>
            )}
          </form>
        )}
        <MemberReadState
          status={read.status}
          kind={read.kind}
          c={c}
          locale={locale}
          retry={read.reload}
        />
        {read.data && (
          <Card flush>
            <div className={s.tableWrap}>
              <DataTable
                caption={c.title}
                rows={read.data.members}
                rowId={(row) => row.id}
                empty={<EmptyState title={q ? c.queryEmpty : c.empty} />}
                error={null}
                loadingLabel={c.loading}
                columns={[
                  {
                    id: "name",
                    header: c.name,
                    wrap: true,
                    cell: (row) => (
                      <div data-label={c.name}>
                        <Link
                          prefetch={false}
                          href={`/${locale}/members/${row.id}?store=${store!.id}`}
                        >
                          {row.display_name}
                        </Link>
                        {row.erased && <Badge>{c.erased}</Badge>}
                      </div>
                    ),
                  },
                  {
                    id: "contact",
                    header: c.contact,
                    wrap: true,
                    cell: (row) => (
                      <div data-label={c.contact}>
                        {row.contact_note ?? c.dash}
                      </div>
                    ),
                  },
                  {
                    id: "balance",
                    header: c.balance,
                    cell: (row) => (
                      <div data-label={c.balance}>
                        {decimal(locale, row.balance, 0)}
                      </div>
                    ),
                  },
                  {
                    id: "updated",
                    header: c.updated,
                    cell: (row) => (
                      <div data-label={c.updated}>
                        <time dateTime={row.updated_at}>
                          {displayTime(locale, row.updated_at)}
                        </time>
                      </div>
                    ),
                  },
                ]}
              />
            </div>
            <footer className={s.footer} aria-live="polite">
              <span>
                {c.loaded}: {read.data.members.length}
              </span>
              {read.pageError && <p role="alert">{c[read.pageError]}</p>}
              {read.data.next_cursor !== null ? (
                <Button
                  data-testid="members-load-more"
                  disabledReason={read.paging ? c.loading : undefined}
                  onClick={() => void read.more()}
                >
                  {read.paging ? c.loading : read.pageError ? c.retry : c.more}
                </Button>
              ) : (
                <span>{c.noMore}</span>
              )}
            </footer>
          </Card>
        )}
      </main>
    </WorkspaceFrame>
  );
}
