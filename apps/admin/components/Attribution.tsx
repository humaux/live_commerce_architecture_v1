"use client";
// D7/D9 readonly page. BFF GET /api/stores/{store}/ads/attribution -> Go GET /v1/admin/stores/{store}/ads/attribution.
// URL holds only report filters. PG/Go own amounts and order facts; audience-read queues guarded Meta GETs only.
// D7/D9 readonly page. BFF GET /api/stores/{store}/ads/attribution -> Go GET /v1/admin/stores/{store}/ads/attribution.
// URL holds only report filters. PG/Go own amounts and order facts; audience-read queues guarded Meta GETs only.
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { Field, FormRow, DateControl } from "@live-commerce/ui";
import { presentationCopy } from "@/lib/presentation-copy";
import { AdminPageHeader } from "./AdminPageHeader";

import type { Store } from "@/lib/model";
import { validAdsQuery } from "@/lib/ads-request";
import {
  readAttribution,
  type AttributionError,
} from "@/lib/attribution-client";
import { attributionCopy } from "@/lib/attribution-copy";


import type { AttributionReport } from "@/lib/attribution-model";
import { WorkspaceFrame } from "./WorkspaceFrame";

import "./attribution.css";
import { DraftPanel, SessionPanel } from "./AttributionPanels";

export function Attribution({
  locale,
  store,
  from,
  to,
  draftID,
  sessionID,
  initialError,
}: {
  locale: Locale;
  store: Store | null;
  from: string;
  to: string;
  draftID: string;
  sessionID: string;
  initialError: AttributionError | null;
}) {
  const c = attributionCopy[locale],
    router = useRouter();
  const key = `${store?.id}:${from}:${to}`;
  const [result, setResult] = useState<{
    key: string;
    report: AttributionReport;
  } | null>(null);
  const [error, setError] = useState<AttributionError | null>(initialError),
    [retry, setRetry] = useState(0);
  const [range, setRange] = useState({ from, to }),
    [invalid, setInvalid] = useState(false);
  useEffect(() => {
    setRange({ from, to });
    setInvalid(false);
  }, [from, to]);
  useEffect(() => {
    setResult(null);
    setError(initialError);
    if (!store || initialError) return;
    const abort = new AbortController();
    let current = true;
    const expire = () => {
      current = false;
      abort.abort();
      setResult(null);
      setError("signed-out");
    };
    const onStorage = (event: StorageEvent) => {
      if (event.key === "commerce-session-logout") expire();
    };
    window.addEventListener("commerce-session-logout", expire);
    window.addEventListener("storage", onStorage);
    let channel: BroadcastChannel | null = null;
    try {
      channel = new BroadcastChannel("commerce-session");
      channel.onmessage = (event) => {
        if (event.data?.type === "logout") expire();
      };
    } catch {
      /* same-tab + storage remain */
    }
    void readAttribution(store.id, from, to, abort.signal)
      .then((report) => {
        if (current) setResult({ key, report });
      })
      .catch((cause: unknown) => {
        if (!current || abort.signal.aborted) return;
        const code = cause instanceof Error ? cause.message : "unavailable";
        setError(
          code === "signed-out" || code === "forbidden" ? code : "unavailable",
        );
      });
    return () => {
      current = false;
      abort.abort();
      window.removeEventListener("commerce-session-logout", expire);
      window.removeEventListener("storage", onStorage);
      channel?.close();
    };
  }, [store?.id, from, to, retry, initialError, key]);
  const report = result?.key === key && !error ? result.report : null;
  const selectedDraft =
    report?.drafts.find((d) => d.draft_id === draftID) ??
    (!draftID ? report?.drafts[0] : undefined);
  const selectedSession =
    report?.sessions.find((s) => s.session_id === sessionID) ??
    (!sessionID ? report?.sessions[0] : undefined);
  function navigate(fields: {
    from?: string;
    to?: string;
    draft?: string;
    session?: string;
  }) {
    const q = new URLSearchParams({
      ...(store ? { store: store.id } : {}),
      from,
      to,
    });
    if (draftID) q.set("draft", draftID);
    if (sessionID) q.set("session", sessionID);
    for (const [name, value] of Object.entries(fields))
      value ? q.set(name, value) : q.delete(name);
    router.push(`/${locale}/ads/attribution?${q}`);
  }
  const loading = !error && !!store && !report;
  return (
    <WorkspaceFrame
      locale={locale}
      storeName={store?.name ?? ""}
      active="ads-attribution"
    >
      <div className="attribution-page" data-testid="ads-attribution">
        <AdminPageHeader
          locale={locale}
          description={c.intro}
          actions={
            <Link
            data-testid="attribution-back"
            href={`/${locale}/ads${store ? `?store=${store.id}` : ""}`}
          >
            {c.back}
          </Link>
          }
        />
        <form
          className="attribution-filters"
          onSubmit={(event) => {
            event.preventDefault();
            if (
              !validAdsQuery(
                `https://local.invalid/?from=${range.from}&to=${range.to}`,
                "ads/attribution",
              )
            ) {
              setInvalid(true);
              return;
            }
            setInvalid(false);
            if (range.from === from && range.to === to) setRetry((v) => v + 1);
            else navigate({ ...range, draft: "", session: "" });
          }}
        >
          <FormRow>
            <Field id="attribution-from" label={c.from} width="short">
              <DateControl emptyLabel={presentationCopy[locale].date}
                id="attribution-from"
                lang={locale}
                type="date"
                required
                value={range.from}
                onChange={(e) =>
                setRange((v) => ({ ...v, from: e.target.value }))
              }
                data-testid="attribution-from"
              />
            </Field>
            <Field id="attribution-to" label={c.to} width="short">
              <DateControl emptyLabel={presentationCopy[locale].date}
                id="attribution-to"
                lang={locale}
                type="date"
                required
                value={range.to}
                onChange={(e) => setRange((v) => ({ ...v, to: e.target.value }))}
                data-testid="attribution-to"
              />
            </Field>
          </FormRow>
          <button
            type="submit"
            disabled={
              loading ||
              !store ||
              error === "signed-out" ||
              error === "forbidden"
            }
            data-testid="attribution-apply"
          >
            {c.apply}
          </button>
        </form>
        {invalid && <p role="alert">{c.invalid}</p>}
        {loading && (
          <p role="status" data-testid="attribution-loading">
            {c.loading}
          </p>
        )}
        {error && (
          <div role="alert" data-testid="attribution-error">
            <p>
              {error === "signed-out"
                ? c.signedOut
                : error === "forbidden"
                  ? c.forbidden
                  : c.unavailable}
            </p>
            {error === "signed-out" ? (
              <Link href={`/${locale}/`}>{c.signIn}</Link>
            ) : (
              error === "unavailable" && (
                <button
                  type="button"
                  data-testid="attribution-retry"
                  onClick={() => {
                    if (initialError) router.refresh();
                    else setRetry((v) => v + 1);
                  }}
                >
                  {c.retry}
                </button>
              )
            )}
          </div>
        )}
        {!store && !error && <p>{c.empty}</p>}
        {report && (
          <>
            <p className="attribution-note" data-testid="attribution-window">
              {report.window.from} – {report.window.to} · {c.orderZone}
            </p>
            {report.truncated && (
              <p role="status" data-testid="attribution-truncated">
                {c.truncated}
              </p>
            )}
            {!report.drafts.length && !report.sessions.length && (
              <p data-testid="attribution-empty">{c.empty}</p>
            )}
            <FormRow className="attribution-selection">
              <Field
                id="attribution-draft"
                label={c.selectDraft}
                hint={!report.drafts.length ? c.noDraft : undefined}
                width="long"
              >
                <select
                  id="attribution-draft"
                  aria-describedby="attribution-draft-hint"
                  data-testid="attribution-draft"
                  value={selectedDraft?.draft_id ?? ""}
                  disabled={!report.drafts.length}
                  onChange={(e) => navigate({ draft: e.target.value })}
                >
                  {!selectedDraft && <option value="">{c.noDraft}</option>}
                  {report.drafts.map((d) => (
                    <option key={d.draft_id} value={d.draft_id}>
                      {d.source_ref || d.draft_id}
                    </option>
                  ))}
                </select>
              </Field>
              <Field
                id="attribution-session"
                label={c.selectSession}
                hint={!report.sessions.length ? c.noSession : undefined}
                width="long"
              >
                <select
                  id="attribution-session"
                  aria-describedby="attribution-session-hint"
                  data-testid="attribution-session"
                  value={selectedSession?.session_id ?? ""}
                  disabled={!report.sessions.length}
                  onChange={(e) => navigate({ session: e.target.value })}
                >
                  {!selectedSession && <option value="">{c.noSession}</option>}
                  {report.sessions.map((s) => (
                    <option key={s.session_id} value={s.session_id}>
                      {s.title || s.session_id}
                    </option>
                  ))}
                </select>
              </Field>
            </FormRow>
            {selectedDraft && (
              <DraftPanel c={c} locale={locale} draft={selectedDraft} />
            )}
            {selectedSession && (
              <SessionPanel
                c={c}
                locale={locale}
                session={selectedSession}
                store={store!.id}
              />
            )}
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}
