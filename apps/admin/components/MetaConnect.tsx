"use client";

// Settings -> Facebook Page / Instagram card (mounted by SettingsWizard.tsx below the storefront card): the merchant connects their
// own Page (and Instagram account) through Facebook Login for Business, picks one Page, and sees Page, Instagram, granted permissions,
// token status and the last comment received; Disconnect destroys the stored token; Reconnect renews an expired one.
// BFF routes (lib/meta-connect-client.ts): POST /api/meta/connect (-> Go meta-connect/start), GET /api/meta/callback (Meta's return,
// 303 back here with ?meta_connect=<state_id> or ?meta_error=<code>), GET /api/stores/{store}/meta-connect/{status,states/{id}},
// POST .../meta-connect/{pick,disconnect} -> Go internal/httpapi/meta_connect.go (contract meta-claims-intake-v1 "Merchant connect (R4)").
// A GET 403 hides the card (no integration:read). Nothing is optimistic: every write re-GETs. No token ever reaches the browser.
import { useEffect, useRef, useState } from "react";
import type { Locale } from "@live-commerce/i18n";
import { sessionBoundary } from "@/lib/settings-client";
import { OrderReadError } from "@/lib/orders-client";
import { newKey, postConnect, postDisconnect, postPick, readPickState, readStatus, type ConnectStatus, type PickState } from "@/lib/meta-connect-client";
import { pickable, type PickPage } from "@/lib/meta-connect-model";
import { metaConnectCopy, type MetaConnectCopy } from "@/lib/meta-connect-copy";
import "./settings.css";

type Load = "loading" | "ready" | "hidden" | "error";
const day = 86_400_000;
const when = (stamp: string, locale: Locale) => new Date(stamp).toLocaleString(locale, { dateStyle: "medium", timeStyle: "short" });
// Permission / Page-task names -> text: tasks are translated, permissions are Meta's own identifiers.
const label = (c: MetaConnectCopy, code: string) => (code === "task_messaging" ? c.task_messaging : code === "task_moderate" ? c.task_moderate : code);

export function MetaConnect({ store, locale }: { store: string; locale: Locale }) {
  const c = metaConnectCopy[locale];
  const [load, setLoad] = useState<Load>("loading");
  const [status, setStatus] = useState<ConnectStatus | null>(null);
  const [tick, setTick] = useState(0);
  const [boundary, setBoundary] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [pick, setPick] = useState<PickState | null>(null);
  const [chosen, setChosen] = useState("");
  const [withIG, setWithIG] = useState(true);
  const key = useRef<{ kind: string; value: string } | null>(null);

  useEffect(() => {
    let live = true;
    sessionBoundary().then((v) => live && setBoundary(v), () => live && setBoundary(""));
    return () => { live = false; };
  }, [store]);

  // The return from Facebook: ?meta_connect=<state_id> (open the pick list) or ?meta_error=<fixed code>; stripped once read. The
  // ref keeps the parsed values across a dev double-mount, which would otherwise find the URL already stripped.
  const arrival = useRef<{ id: string | null; error: string | null } | undefined>(undefined);
  useEffect(() => {
    if (arrival.current === undefined) {
      const params = new URLSearchParams(window.location.search);
      arrival.current = { id: params.get("meta_connect"), error: params.get("meta_error") };
      params.delete("meta_connect");
      params.delete("meta_error");
      window.history.replaceState(null, "", `${window.location.pathname}${params.size ? `?${params}` : ""}`);
    }
    const { id, error } = arrival.current;
    if (error) setProblem(c.errors[error] ?? c.errors.unavailable);
    if (!id) return;
    const active = new AbortController();
    readPickState(store, id, active.signal).then(
      (state) => { setPick(state); const first = state.pages.find((p) => p.missing.length === 0) ?? state.pages[0]; setChosen(first?.page_id ?? ""); setWithIG(!!first?.ig_id && first.ig_missing.length === 0); },
      () => !active.signal.aborted && setProblem(c.errors.state_expired),
    );
    return () => active.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- read once on mount
  }, [store]);

  useEffect(() => {
    const active = new AbortController();
    readStatus(store, active.signal).then(
      (value) => { setStatus(value); setLoad("ready"); },
      (error) => {
        if (active.signal.aborted) return;
        setStatus(null);
        setLoad(error instanceof OrderReadError && error.code === "forbidden" ? "hidden" : "error");
      },
    );
    return () => active.abort();
  }, [store, tick]);

  if (load === "hidden") return null;

  // One Idempotency-Key per intent; a retry of the same intent after an unknown outcome reuses it.
  const keyFor = (kind: string) => { if (key.current?.kind !== kind) key.current = { kind, value: newKey(`meta-${kind}`) }; return key.current.value; };
  const fail = (code: string, uncertain: boolean) => setProblem(uncertain ? c.uncertain : (c.errors[code] ?? c.errors.unavailable));

  async function connect() {
    if (busy) return;
    setBusy(true); setProblem(""); setNotice("");
    const result = await postConnect(store, keyFor("connect"), boundary);
    if (result.ok) { window.location.assign(result.value); return; } // postConnect proved the origin is exactly https://www.facebook.com
    setBusy(false); fail(result.code, result.uncertain);
  }
  async function submitPick() {
    if (busy || !pick) return;
    const page = pick.pages.find((p) => p.page_id === chosen);
    if (!page || !pickable(page, false)) return;
    setBusy(true); setProblem(""); setNotice("");
    const result = await postPick(store, keyFor(`pick-${pick.state_id}-${chosen}-${withIG}`), pick.state_id, chosen, withIG && pickable(page, true), boundary);
    setBusy(false);
    if (result.ok) { key.current = null; setPick(null); setNotice(c.connectedNotice); } else fail(result.code, result.uncertain);
    setTick((v) => v + 1);
  }
  async function disconnect() {
    if (busy) return;
    setBusy(true); setProblem(""); setNotice("");
    const result = await postDisconnect(store, keyFor("disconnect"), boundary);
    setBusy(false); setConfirming(false);
    if (result.ok) { key.current = null; setNotice(c.disconnectedNotice); } else fail(result.code, result.uncertain);
    setTick((v) => v + 1);
  }

  const chosenPage: PickPage | undefined = pick?.pages.find((p) => p.page_id === chosen);
  const connected = status?.connected === true ? status : null;
  const soon = connected && Date.parse(connected.route_expires_at) - Date.now() < 30 * day;

  return (
    <section className="settings-fields metaconnect-card" data-testid="metaconnect-card" aria-labelledby="metaconnect-title">
      <h2 id="metaconnect-title" className="settings-section-title settings-subtitle">{c.title}</h2>
      <p className="settings-note">{c.intro}</p>
      {load === "loading" && <p role="status">{c.loading}</p>}
      {load === "error" && (
        <div role="status"><p>{c.unavailable}</p><button type="button" onClick={() => setTick((v) => v + 1)}>{c.retry}</button></div>
      )}
      {problem && <p className="settings-warning" role="alert" data-testid="metaconnect-error">{problem}</p>}
      {notice && <p className="settings-note" role="status" data-testid="metaconnect-notice">{notice}</p>}

      {pick && (
        <div data-testid="metaconnect-pick">
          <h3>{c.pickTitle}</h3>
          <p className="settings-note">{c.pickIntro}</p>
          {pick.pages.length === 0 ? <p className="settings-warning" data-testid="metaconnect-nopages">{c.noPages}</p> : (
            <fieldset className="metaconnect-pages" data-testid="metaconnect-pick-list">
              {pick.pages.map((p) => (
                <label key={p.page_id} className="settings-check">
                  <input type="radio" name="metaconnect-page" value={p.page_id} checked={chosen === p.page_id} data-testid={`metaconnect-pick-${p.page_id}`}
                    onChange={() => { setChosen(p.page_id); setWithIG(!!p.ig_id && p.ig_missing.length === 0); }} />
                  <span>{p.name || p.page_id} <small>{p.page_id}{p.ig_username ? ` · @${p.ig_username}` : ""}</small></span>
                </label>
              ))}
            </fieldset>
          )}
          {chosenPage && chosenPage.missing.length > 0 && (
            <p className="settings-warning" data-testid="metaconnect-missing">{c.pickMissing} {chosenPage.missing.map((m) => label(c, m)).join(", ")}</p>
          )}
          {chosenPage?.ig_id && chosenPage.missing.length === 0 && (chosenPage.ig_missing.length === 0 ? (
            <label className="settings-check">
              <input type="checkbox" checked={withIG} data-testid="metaconnect-pick-ig" onChange={(e) => setWithIG(e.target.checked)} />
              <span>{c.withInstagram} <small>@{chosenPage.ig_username}</small></span>
            </label>
          ) : (
            <p className="settings-note" data-testid="metaconnect-ig-missing">{c.instagramMissing} {chosenPage.ig_missing.join(", ")}</p>
          ))}
          <div className="settings-actions">
            <button type="button" disabled={busy} onClick={() => { setPick(null); setProblem(""); }}>{c.pickCancel}</button>
            <button className="primary" type="button" data-testid="metaconnect-pick-submit" disabled={busy || !chosenPage || chosenPage.missing.length > 0} onClick={() => void submitPick()}>
              {busy ? c.saving : c.pickSubmit}
            </button>
          </div>
        </div>
      )}

      {load === "ready" && !pick && !connected && (
        <>
          <p data-testid="metaconnect-none">{c.notConnected}</p>
          <div className="settings-actions">
            <button className="primary" type="button" data-testid="metaconnect-connect" disabled={busy} onClick={() => void connect()}>{busy ? c.connecting : c.connect}</button>
          </div>
        </>
      )}

      {load === "ready" && !pick && connected && (
        <>
          <dl className="settings-status-list" data-testid="metaconnect-status">
            <div><dt>{c.page}</dt><dd data-testid="metaconnect-page">{connected.page.name || connected.page.id} <small>{connected.page.id}</small></dd></div>
            <div><dt>{c.instagram}</dt><dd data-testid="metaconnect-ig">{connected.instagram ? `@${connected.instagram.username}` : c.noInstagram}</dd></div>
            <div><dt>{c.permissions}</dt><dd data-testid="metaconnect-permissions">{connected.permissions.join(", ")}</dd></div>
            <div><dt>{c.token}</dt><dd data-testid="metaconnect-token" data-state={connected.status}>{connected.status === "active" ? c.tokenActive : c.tokenReauth}</dd></div>
            <div><dt>{c.lastEvent}</dt><dd data-testid="metaconnect-last-event">{connected.last_event_at ? when(connected.last_event_at, locale) : c.never}</dd></div>
            <div><dt>{c.connectedAt}</dt><dd>{when(connected.connected_at, locale)}</dd></div>
            <div><dt>{c.routeUntil}</dt><dd>{when(connected.route_expires_at, locale)}</dd></div>
          </dl>
          {soon && connected.status === "active" && <p className="settings-warning">{c.renewSoon}</p>}
          {confirming ? (
            <div className="settings-pending" role="alertdialog" aria-label={c.disconnect} data-testid="metaconnect-confirm">
              <p>{c.confirmDisconnect}</p>
              <div className="settings-actions">
                <button type="button" disabled={busy} onClick={() => setConfirming(false)}>{c.cancel}</button>
                <button className="primary" type="button" data-testid="metaconnect-confirm-yes" disabled={busy} onClick={() => void disconnect()}>{busy ? c.saving : c.confirm}</button>
              </div>
            </div>
          ) : (
            <div className="settings-actions">
              <button type="button" data-testid="metaconnect-disconnect" disabled={busy} onClick={() => setConfirming(true)}>{c.disconnect}</button>
              {(connected.status === "reauth_required" || soon) && (
                <button className="primary" type="button" data-testid="metaconnect-reconnect" disabled={busy} onClick={() => void connect()}>{c.reconnect}</button>
              )}
            </div>
          )}
        </>
      )}
    </section>
  );
}
