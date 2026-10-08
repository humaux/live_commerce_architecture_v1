// Purpose: Owns merchant ads settings, connection and draft workflows.
// Depends on: @live-commerce/format (Taipei store timestamps), react, next/navigation, @live-commerce/i18n, @live-commerce/ui, ./AdminPageHeader, @/lib/model, @/lib/settings-client, @/lib/ads-client, @/lib/ads-model, @/lib/ads-copy, ./WorkspaceFrame, ./Icon, ./ads.css, ./AdsConnection, ./AdsDraft, ./AdsResults, @/lib/attribution-copy
// Used by: apps/admin/app/[locale]/ads/page.tsx
"use client";

// Merchant ads page: Meta connection, draft list + detail, draft form, report, CAPI setting (contracts/meta-ads-v1.md §2, §5, §7).
// BFF routes, all -> Go internal/httpapi/ads.go under /v1/admin/stores/{store_id}/ads (lib/ads-client.ts is the only caller):
//   GET  /api/stores/{store}/ads/settings | drafts | drafts/{id} | meta/states/{id} | report?from&to   -> GET  ads/…
//   POST /api/stores/{store}/ads/meta/bindings | drafts | drafts/{id}/(approve|publish|pause|end)     -> POST ads/…
//   PUT  /api/stores/{store}/ads/drafts/{id} (If-Match) | capi                                        -> PUT  ads/…
//   POST /api/ads/meta/connect (app/api/ads/meta/connect/route.ts)                                    -> POST ads/meta/connect
//   the return from Meta lands on /api/ads/meta/callback, which 303s back here with ?connect=<state_id>.
// Rules kept here: no token is ever visible (Go holds it sealed); nothing is optimistic (every write is followed by a
// re-GET); one Idempotency-Key per dialog open, reused only for a byte-identical retry after an unknown outcome;
// a paused draft is never resumed (X7: copy into a new draft); spend is never described as a hard stop (§12).
// Merchant ads page: Meta connection, draft list + detail, draft form, report, CAPI setting (contracts/meta-ads-v1.md §2, §5, §7).
// BFF routes, all -> Go internal/httpapi/ads.go under /v1/admin/stores/{store_id}/ads (lib/ads-client.ts is the only caller):
//   GET  /api/stores/{store}/ads/settings | drafts | drafts/{id} | meta/states/{id} | report?from&to   -> GET  ads/…
//   POST /api/stores/{store}/ads/meta/bindings | drafts | drafts/{id}/(approve|publish|pause|end)     -> POST ads/…
//   PUT  /api/stores/{store}/ads/drafts/{id} (If-Match) | capi                                        -> PUT  ads/…
//   POST /api/ads/meta/connect (app/api/ads/meta/connect/route.ts)                                    -> POST ads/meta/connect
//   the return from Meta lands on /api/ads/meta/callback, which 303s back here with ?connect=<state_id>.
// Rules kept here: no token is ever visible (Go holds it sealed); nothing is optimistic (every write is followed by a
// re-GET); one Idempotency-Key per dialog open, reused only for a byte-identical retry after an unknown outcome;
// a paused draft is never resumed (X7: copy into a new draft); spend is never described as a hard stop (§12).
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import { Field, TableFrame } from "@live-commerce/ui";
import { AdminPageHeader } from "./AdminPageHeader";
import type { Store } from "@/lib/model";
import { sessionBoundary } from "@/lib/settings-client";
import { newKey, postConnect, postDraft, postDraftAction, putCapi, putDraft, readDraft, readDrafts, readSettings, AdsReadError, type WriteResult } from "@/lib/ads-client";
import { buildDraftInput, capiBody, copyForm, emptyForm, formFromDraft, formatMinor, type AdsCode, type ConnectError, type Draft, type DraftForm as FormState, type Settings } from "@/lib/ads-model";
import { adsCopy, errorText } from "@/lib/ads-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { Icon } from "./Icon";
import "./ads.css";
import { ConnectionSection } from "./AdsConnection";
import { Badge, DraftFormPanel, DraftDetail, type Mode } from "./AdsDraft";
import { ReportSection, CapiSection } from "./AdsResults";
import { attributionCopy } from "@/lib/attribution-copy";

/** Describes AdsInitialError values shared by this presentation module. */
export type AdsInitialError = "signed-out" | "forbidden" | "unavailable";
type Status = "loading" | "ready" | AdsInitialError;
type Banner = { kind: "ok" | "bad"; text: string } | null;
function when(locale: Locale, iso: string | null, empty: string) {
  if (!iso) return empty;
  const t = Date.parse(iso);
  return Number.isFinite(t)
    ? displayTime(locale, iso)
    : empty;
}
const short = (id: string) => `${id.slice(0, 4)}…${id.slice(-4)}`;

/** Owns merchant ads settings, connection and draft workflows. User actions submit settings, connection and draft commands through ads-client. */
export function Ads({
  locale, stores, store, connect, connectError, draft: initialDraft, initialError,
}: {
  locale: Locale; stores: Store[]; store: Store | null; connect: string; connectError: ConnectError | "";
  draft: string; initialError: AdsInitialError | null;
}) {
  const c = adsCopy[locale];
  const router = useRouter();
  const [status, setStatus] = useState<Status>(initialError ?? "loading");
  const [settings, setSettings] = useState<Settings | null>(null);
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [selected, setSelected] = useState(initialDraft);
  const [detail, setDetail] = useState<Draft | null>(null);
  const [tick, setTick] = useState(0);
  const [banner, setBanner] = useState<Banner>(null);
  const [busy, setBusy] = useState("");
  const [uncertain, setUncertain] = useState("");
  const [mode, setMode] = useState<Mode>({ kind: "none" });
  const [problems, setProblems] = useState<AdsCode[]>([]);
  const [confirmEnd, setConfirmEnd] = useState(false);
  const boundary = useRef("");
  const blocked = useRef(false);
  const keys = useRef(new Map<string, { key: string; body: string }>());
  const last = useRef<(() => Promise<void>) | null>(null);
  const reload = useCallback(() => setTick((n) => n + 1), []);

  // Load settings + drafts (+ selected draft). Session cookie hash is the change fence for later writes.
  useEffect(() => {
    if (!store || initialError) return;
    const active = new AbortController();
    (async () => {
      try {
        boundary.current = await sessionBoundary();
        const [s, list] = await Promise.all([
          readSettings(store.id, active.signal),
          readDrafts(store.id, active.signal),
        ]);
        if (active.signal.aborted) return;
        setSettings(s);
        setDrafts(list);
        setStatus("ready");
      } catch (error) {
        if (active.signal.aborted) return;
        const code = error instanceof AdsReadError ? error.code : "unavailable";
        if (code === "signed-out") blocked.current = true;
        setStatus(
          code === "signed-out" ? "signed-out" : code === "forbidden" ? "forbidden" : "unavailable",
        );
      }
    })();
    return () => active.abort();
  }, [store, initialError, tick]);

  useEffect(() => {
    if (!store || !selected || status !== "ready") {
      setDetail(null);
      return;
    }
    const active = new AbortController();
    readDraft(store.id, selected, active.signal).then(
      (d) => !active.signal.aborted && setDetail(d),
      () => !active.signal.aborted && setDetail(null),
    );
    return () => active.abort();
  }, [store, selected, status, tick]);

  // Sign-out in another tab: drop everything (same events as the orders page).
  useEffect(() => {
    const out = () => {
      blocked.current = true;
      setStatus("signed-out");
      setSettings(null);
      setDrafts([]);
      setDetail(null);
    };
    let channel: BroadcastChannel | null = null;
    const onMessage = (event: MessageEvent) => event.data?.type === "logout" && out();
    const onStorage = (event: StorageEvent) => event.key === "commerce-session-logout" && out();
    try {
      channel = new BroadcastChannel("commerce-session");
      channel.addEventListener("message", onMessage);
    } catch {
      /* storage event still works */
    }
    window.addEventListener("storage", onStorage);
    window.addEventListener("commerce-session-logout", out);
    return () => {
      channel?.removeEventListener("message", onMessage);
      channel?.close();
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("commerce-session-logout", out);
    };
  }, []);

  const url = useCallback(
    (extra: Record<string, string> = {}) => {
    const p = new URLSearchParams();
    if (store) p.set("store", store.id);
    for (const [k, v] of Object.entries(extra)) if (v) p.set(k, v);
    return `/${locale}/ads${p.size ? `?${p}` : ""}`;
  },
    [locale, store],
  );

  // One key per submission; the same key only comes back for the identical slot + bytes (unknown-outcome retry).
  function keyFor(slot: string, body: string) {
    const held = keys.current.get(slot);
    if (held && held.body === body) return held.key;
    const key = newKey("ads");
    keys.current.set(slot, { key, body });
    return key;
  }
  /** Run one write. Definitive failure drops the key (a new attempt is a new command); an unknown outcome keeps it. */
  async function run<T>(
    slot: string,
    body: string,
    exec: (key: string) => Promise<WriteResult<T>>,
    okText: string,
    after?: (value: T) => void,
  ) {
    if (busy || blocked.current) return;
    const go = async () => {
      setBusy(slot);
      setBanner(null);
      const result = await exec(keyFor(slot, body));
      setBusy("");
      if (result.ok) {
        keys.current.delete(slot);
        setUncertain("");
        last.current = null;
        setBanner(okText ? { kind: "ok", text: okText } : null);
        after?.(result.value);
        reload();
        return;
      }
      if (result.uncertain) {
        setUncertain(slot);
        setBanner({ kind: "bad", text: c.uncertain });
        return;
      }
      keys.current.delete(slot);
      setUncertain("");
      last.current = null;
      if (result.code === "unauthorized") {
        blocked.current = true;
        setStatus("signed-out");
      }
      setBanner({ kind: "bad", text: errorText(c, result.code) });
      if (result.code === "revision_changed" || result.code === "attempt_changed" || result.code === "draft_approved") reload();
    };
    last.current = go;
    await go();
  }
  const retrySame = () => void last.current?.();

  async function startConnect() {
    if (!store || busy) return;
    await run(
      "connect",
      "connect",
      (key) => postConnect(store.id, key, boundary.current),
      "",
      (dialog) => {
      // postConnect already proved the origin is exactly https://www.facebook.com.
      if (dialog) window.location.assign(dialog);
    },
    );
  }

  const allowanceOff = !!settings && settings.max_active_budget_minor === 0;
  const selectedDraft = detail && detail.id === selected ? detail : null;

  function openNew() {
    if (!settings || !store) return;
    keys.current.delete("draft-new");
    setProblems([]);
    setBanner(null);
    setUncertain("");
    setMode({
      kind: "new",
      form: emptyForm(settings.allowance_currency || store.currency),
    });
  }
  function openEdit(d: Draft) {
    keys.current.delete(`draft-edit:${d.id}`);
    setProblems([]);
    setBanner(null);
    setUncertain("");
    setMode({ kind: "edit", form: formFromDraft(d), draft: d });
  }
  function openCopy(d: Draft) {
    keys.current.delete("draft-new");
    setProblems([]);
    setBanner(null);
    setUncertain("");
    setMode({ kind: "copy", form: copyForm(d) });
  }
  function closeForm() {
    if (mode.kind === "edit") keys.current.delete(`draft-edit:${mode.draft.id}`);
    else keys.current.delete("draft-new");
    setMode({ kind: "none" });
    setUncertain("");
    last.current = null;
    reload();
  }
  async function submitDraft(form: FormState) {
    if (!store || !settings) return;
    const built = buildDraftInput(
      form,
      Date.now(),
      settings.allowance_currency,
    );
    if (!built.ok) {
      setProblems(built.codes);
      return;
    }
    setProblems([]);
    const body = JSON.stringify(built.input);
    if (mode.kind === "edit") {
      const target = mode.draft;
      await run(
        `draft-edit:${target.id}`,
        body,
        (key) =>
          putDraft(
            store.id,
            target.id,
            key,
            body,
            boundary.current,
            target.revision,
          ),
        c.saved,
        () => setMode({ kind: "none" }),
      );
    } else {
      await run(
        "draft-new",
        body,
        (key) => postDraft(store.id, key, body, boundary.current),
        c.saved,
        (created) => {
        setMode({ kind: "none" });
        if (created) setSelected(created.id);
      },
      );
    }
  }
  function act(d: Draft, action: "approve" | "publish" | "pause" | "end") {
    if (!store) return;
    const body = JSON.stringify(
      action === "approve" ? { revision: d.revision } : action === "publish" ? { publish_attempt: d.publish_attempt } : {},
    );
    const text = action === "approve" ? c.approved : action === "publish" ? c.published : action === "pause" ? c.paused : c.ended;
    setConfirmEnd(false);
    void run(
      `${action}:${d.id}`,
      body,
      (key) => postDraftAction(store.id, d.id, action, key, body, boundary.current),
      text,
    );
  }

  const bar = (
    <div className="ads-controls">
      {stores.length > 1 && (
        <Field id="ads-store" label={c.store}>
          <select
            id="ads-store"
            data-testid="store-selector"
            value={store?.id ?? ""}
            onChange={(e) => router.push(`/${locale}/ads?store=${e.target.value}`)
            }
          >
            {stores.map((s) => (
              <option key={s.id} value={s.id}>{s.name}</option>
            ))}
          </select>
        </Field>
      )}
      <button type="button" data-testid="ads-refresh" disabled={status === "loading"} onClick={() => { blocked.current = false; setStatus("loading"); reload(); }}>
        <Icon name="refresh" size={18} />{c.refresh}
      </button>
    </div>
  );

  return (
    <WorkspaceFrame locale={locale} storeName={store?.name ?? c.noStore} active="ads">
      <div className="ads-page" data-testid="merchant-ads">
        <AdminPageHeader locale={locale} description={c.subtitle} actions={bar}>
          <a data-testid="ads-attribution-link" href={`/${locale}/ads/attribution${store ? `?store=${store.id}` : ""}`}>{attributionCopy[locale].title}</a>
        </AdminPageHeader>
        {status === "loading" && (
          <p className="ads-message" role="status">{c.loading}</p>
        )}
        {(status === "signed-out" || status === "forbidden" || status === "unavailable") && (
          <div className="ads-message" role="status" data-testid="ads-error">
            <p>{!store && status === "unavailable" ? c.noStore : status === "signed-out" ? c.signedOut : status === "forbidden" ? c.forbidden : c.unavailable}</p>
            <button
              type="button"
              onClick={() =>
                initialError || !store
                  ? router.refresh()
                  : ((blocked.current = false), setStatus("loading"), reload())
              }
            >
              {c.retry}
            </button>
          </div>
        )}
        {status === "ready" && settings && store && (
          <>
            <div className="ads-notices">
              {settings.environment === "SANDBOX" && (
                <p className="ads-banner ads-banner-sandbox" role="status" data-testid="ads-sandbox">{c.sandboxBanner}</p>
              )}
              {allowanceOff ? (
                <p className="ads-banner ads-banner-off" role="status" data-testid="ads-allowance-off">{c.allowanceOff}</p>
              ) : (
                <p className="ads-note" data-testid="ads-allowance">
                  {c.allowanceLine(
                    formatMinor(
                      locale,
                      settings.allowance_currency,
                      settings.max_active_budget_minor,
                    ),
                  )}
                </p>
              )}
              <p className="ads-note" data-testid="ads-budget-note">{c.budgetNote}</p>
            </div>
            {banner && (
              <p className={banner.kind === "ok" ? "ads-notice" : "ads-bad"} role={banner.kind === "ok" ? "status" : "alert"} data-testid="ads-banner">
                {banner.text}
                {uncertain && banner.kind === "bad" && (
                  <button type="button" className="ads-inline" data-testid="ads-retry-same" onClick={retrySame}>{c.retrySame}</button>
                )}
              </p>
            )}
            <ConnectionSection when={when} c={c} locale={locale} settings={settings} store={store} connect={connect} connectError={connectError}
              busy={busy} uncertain={uncertain} startConnect={startConnect} run={run} boundary={boundary}
              done={() => { setBanner({ kind: "ok", text: c.connected }); router.replace(url()); reload(); }}
              startAgain={() => { router.replace(url()); void startConnect(); }} />
            <section className="ads-section" aria-labelledby="ads-drafts-h">
              <div className="ads-section-head">
                <h2 id="ads-drafts-h">{c.draftsTitle}</h2>
                <button type="button" className="primary" data-testid="ads-new-draft" onClick={openNew} disabled={mode.kind !== "none"}>
                  <Icon name="plus" size={16} />{c.newDraft}
                </button>
              </div>
              {mode.kind !== "none" && (
                <DraftFormPanel
                  key={mode.kind + (mode.kind === "edit" ? mode.draft.id : "")}
                  c={c}
                  locale={locale}
                  settings={settings}
                  mode={mode}
                  problems={problems}
                  busy={busy.startsWith("draft-")}
                  uncertain={uncertain.startsWith("draft-")}
                  onSubmit={submitDraft}
                  onCancel={closeForm}
                  onRetry={retrySame}
                />
              )}
              {drafts.length === 0 ? (
                <p className="ads-empty" data-testid="ads-drafts-empty">{c.draftsEmpty}</p>
              ) : (
                <TableFrame
                  className="ads-table-scroll"
                  label={c.draftsTitle}
                  scrollHint={attributionCopy[locale].scrollHint}
                >
                  <table className="ads-table" data-testid="ads-drafts">
                    <thead>
                      <tr>
                        <th>{c.colTemplate}</th>
                        <th className="ads-number">{c.colBudget}</th>
                        <th>{c.colSchedule}</th>
                        <th>{c.colStatus}</th>
                        <th>{c.colCreated}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {drafts.map((d) => (
                        <tr
                          key={d.id} className={d.id === selected ? "ads-selected" : ""} data-testid={`ads-draft-${d.id}`}
                        >
                          <td data-label={c.colTemplate}>
                            <button
                              type="button"
                              aria-expanded={d.id === selected}
                              data-testid={`ads-open-${d.id}`}
                              aria-label={`${c.select}: ${c.templates[d.template]} ${short(d.id)}`}
                              onClick={() => {
                                setConfirmEnd(false);
                                setSelected(d.id === selected ? "" : d.id);
                                window.history.replaceState(
                                  null,
                                  "",
                                  url({ draft: d.id === selected ? "" : d.id }),
                                );
                              }}
                            >
                              <Icon
                                name="chevron"
                                size={16}
                                style={{
                                  transform: d.id === selected ? "rotate(90deg)" : undefined,
                                }}
                              />
                              <span>{c.templates[d.template]}</span>
                            </button>
                          </td>
                          <td className="ads-number" data-label={c.colBudget}>
                            {formatMinor(
                              locale,
                              d.currency,
                              d.lifetime_budget_minor,
                            )}
                          </td>
                          <td data-label={c.colSchedule}>
                            {when(locale, d.starts_at, "—")} →{" "}
                            {when(locale, d.ends_at, "—")}
                          </td>
                          <td data-label={c.colStatus}><Badge c={c} status={d.status} /></td>
                          <td data-label={c.colCreated}>{when(locale, d.created_at, "—")}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </TableFrame>
              )}
              {selectedDraft && (
                <DraftDetail when={when} c={c} locale={locale} d={selectedDraft} busy={busy} allowanceOff={allowanceOff} confirmEnd={confirmEnd}
                  setConfirmEnd={setConfirmEnd} onEdit={() => openEdit(selectedDraft)} onCopy={() => openCopy(selectedDraft)} act={act} formOpen={mode.kind !== "none"} />
              )}
            </section>
            <ReportSection when={when} c={c} locale={locale} store={store} />
            <CapiSection
              c={c}
              settings={settings}
              busy={busy === "capi"}
              uncertain={uncertain === "capi"}
              onRetry={retrySame}
              save={(enabled, dataset, test) => {
                const body = JSON.stringify(capiBody(enabled, dataset, test));
                return run(
                  "capi",
                  body,
                  (key) => putCapi(store.id, key, body, boundary.current),
                  c.capiSaved,
                );
              }}
            />
          </>
        )}
      </div>
    </WorkspaceFrame>
  );
}
