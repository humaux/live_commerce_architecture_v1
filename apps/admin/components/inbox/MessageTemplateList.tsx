// Figma local links: https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=5-15002 and https://www.figma.com/design/iJ9lDdFIXgTlKkt5Tvrc0X?node-id=5-15135
// Purpose: signed metadata-only list 5:15002/15135.
// Depends on: useGuardedRead, readLiveSettings, message permissions and shared admin states.
// Used by: templates/page. Invariants: inbox:reply; no body read/edit/write; unknown kinds remain raw metadata; no cached rows.
"use client";
import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import {
  ReadError,
  useGuardedRead,
  type ReadCode,
} from "@/lib/customers-client";
import {
  readLiveSettings,
  LiveSettingsError,
} from "@/lib/live-settings-client";
import { permitted } from "@/src/features/messages/privacy";
import { AdminForbidden } from "../AdminForbidden";
import { AdminStateBlock, adminStateCopy } from "../AdminStateBlock";
import { messageTemplateCopy } from "./MessageTemplateList.copy";
import s from "./MessageTemplateList.module.css";
type Metadata = {
  template_id: string;
  name: string;
  version: number;
  kinds: string[];
  public_safe: boolean;
  created_at: string;
};
/** Only existing projected metadata crosses this boundary; no template body is requested. */
export function MessageTemplateList({
  locale,
  store,
  initialError,
  renderKey,
}: {
  locale: Locale;
  store: Store | null;
  initialError: ReadCode | null;
  renderKey: string;
}) {
  const c = messageTemplateCopy[locale],
    router = useRouter(),
    add = useRef<HTMLButtonElement>(null),
    allowed = permitted(store, "inbox:reply");
  const read = useGuardedRead<{ items: Metadata[] }>(
    `${renderKey}|${locale}|${store?.id ?? ""}|templates`,
    allowed && store
      ? async (signal) => {
          try {
            return await readLiveSettings<{ items: Metadata[] }>(
              store.id,
              "message-templates",
              signal,
            );
          } catch (error) {
            if (error instanceof LiveSettingsError)
              throw new ReadError(
                error.status === 401
                  ? "signed-out"
                  : error.status === 403
                    ? "forbidden"
                    : "unavailable",
              );
            throw error;
          }
        }
      : null,
    initialError ?? (!allowed ? "forbidden" : null),
  );
  useEffect(() => {
    if (
      read.status === "ready" &&
      window.location.hash === "#message-template-add"
    )
      add.current?.focus();
  }, [read.status]);
  if (read.status === "forbidden") return <AdminForbidden locale={locale} />;
  if (read.status !== "ready")
    return (
      <AdminStateBlock
        variant={read.status === "loading" ? "loading" : "error"}
        title={
          read.status === "loading"
            ? adminStateCopy[locale].loading
            : adminStateCopy[locale].errorTitle
        }
      />
    );
  const rows = read.data?.items ?? [];
  const labels: Record<string, string> = {
    dm: c.private,
    private_reply: c.replyList,
    public_reply: c.public,
    recommend: c.recommend,
  };
  const kind = (k: string) => labels[k] ?? k;
  const types = (row: Metadata) =>
    row.kinds.map(kind).join(locale === "en" ? ", " : "、");
  return (
    <section className={s.page} data-figma-node="5:15002 5:15135">
      <h1 className={s.desktop}>{c.title}</h1>
      <h1 className={s.mobile}>{c.mobileTitle}</h1>
      <h2>{c.count.replace("{n}", String(rows.length))}</h2>
      <p>{c.disclaimer}</p>
      <p className={s.note}>{c.unreadable}</p>
      <div className={s.addDock}>
        <button
          ref={add}
          id="message-template-add"
          className={s.primary}
          type="button"
          disabled={!permitted(store, "live:manage")}
          aria-describedby={
            !permitted(store, "live:manage")
              ? "template-manage-required"
              : undefined
          }
          onClick={() =>
            router.push(
              `/${locale}/messages/templates/new?${new URLSearchParams({ store: store!.id })}`,
            )
          }
        >
          {c.add}
        </button>
        {!permitted(store, "live:manage") && (
          <p id="template-manage-required">{c.manageRequired}</p>
        )}
      </div>
      {rows.length === 0 ? (
        <p>{c.empty}</p>
      ) : (
        <>
          <table className={s.table}>
            <thead>
              <tr>
                {[c.name, c.version, c.kinds, c.safe, c.date, c.edit].map(
                  (t) => (
                    <th key={t} scope="col">
                      {t}
                    </th>
                  ),
                )}
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={`${row.template_id}:${row.version}`}>
                  <td>{row.name}</td>
                  <td>v{row.version}</td>
                  <td>{types(row)}</td>
                  <td>{row.public_safe ? c.yes : c.no}</td>
                  <td>{row.created_at.slice(0, 10)}</td>
                  <td className={s.muted}>{c.unavailable}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <ul className={s.cards}>
            {rows.map((row) => (
              <li key={`${row.template_id}:${row.version}`}>
                <h3>
                  {row.name} · v{row.version}
                </h3>
                <p>{types(row)}</p>
                <p className={s.meta}>
                  {c.safe}：{row.public_safe ? c.yes : c.no} · {c.created}：
                  {row.created_at.slice(0, 10)}
                </p>
                <p className={s.meta}>{c.editHint}</p>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}
