// Purpose: Owns the merchant Facebook and Instagram connection controls.
// Depends on: react, @live-commerce/i18n, @live-commerce/ui, @/lib/settings-client, @/lib/orders-client, @/lib/orders-model, @/lib/meta-connect-client, @/lib/meta-connect-model, @/lib/meta-connect-copy, ./settings.css, ./MetaHealthCapabilities, @/lib/meta-health-hook, @/lib/meta-health-client, ./live-sessions/session-tasks.module.css
// Used by: apps/admin/components/SettingsWizard.tsx
// Invariants: official OAuth only; a connected Page does not grant capabilities; uncertain writes require explicit readback.
"use client";

// Purpose: merchant Facebook/Instagram connection controls and per-Page health capabilities.
// Depends on: existing Meta connect BFF, health B1/B2, session fence and locale copy.
// Used by: SettingsWizard; health banner links to this card.
// Settings -> Facebook Page / Instagram card (mounted by SettingsWizard.tsx below the storefront card): the merchant connects their
// own Pages (up to ten, with their Instagram accounts) through Facebook Login for Business, picks one per authorization, and sees
// each Page's permissions and Taipei event/expiry times. Disconnect targets exactly one Page; reauthorization selects a Page anew.
// BFF routes (lib/meta-connect-client.ts): POST /api/meta/connect (-> Go meta-connect/start), GET /api/meta/callback (Meta's return,
// 303 back here with ?meta_connect=<state_id> or ?meta_error=<code>), GET /api/stores/{store}/meta-connect/{status,states/{id}},
// POST .../meta-connect/{pick,disconnect} -> Go internal/httpapi/meta_connect.go (contract meta-claims-intake-v1 "Merchant connect (R4)").
// A GET 403 hides the card (no integration:read). Nothing is optimistic: every write re-GETs. No token ever reaches the browser.
import { useEffect, useRef, useState } from "react";
import { liveSessionsCopy, type Locale } from "@live-commerce/i18n";
import taskStyle from "./live-sessions/session-tasks.module.css";
import { Badge } from "@live-commerce/ui";
import { sessionBoundary } from "@/lib/settings-client";
import { OrderReadError } from "@/lib/orders-client";
import { displayTime } from "@/lib/orders-model";
import {
  newKey,
  postConnect,
  postDisconnect,
  postPick,
  readPickState,
  readStatus,
  type ConnectStatus,
  type PickState,
} from "@/lib/meta-connect-client";
import { pickable, type PickPage } from "@/lib/meta-connect-model";
import { metaConnectCopy, type MetaConnectCopy } from "@/lib/meta-connect-copy";
import { useMetaHealth } from "@/lib/meta-health-hook";
import { refreshMetaHealth } from "@/lib/meta-health-client";
import { MetaHealthCapabilities } from "./MetaHealthCapabilities";
import "./settings.css";

type Load = "loading" | "ready" | "hidden" | "error";
const day = 86_400_000;
// Permission / Page-task names -> text: tasks are translated, permissions are Meta's own identifiers.
const label = (c: MetaConnectCopy, code: string) =>
  code === "task_messaging" ? c.task_messaging : code === "task_moderate" ? c.task_moderate : code;

/** Reads Page connections and health; authorized actions use the existing reauthorization flow. */
export function MetaConnect({ store, locale, canManage = false }: { store: string; locale: Locale; canManage?: boolean }) {
  const c = metaConnectCopy[locale], p = liveSessionsCopy[locale];
  const [load, setLoad] = useState<Load>("loading");
  const [status, setStatus] = useState<ConnectStatus | null>(null);
  const [tick, setTick] = useState(0);
  const [boundary, setBoundary] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState("");
  const [uncertain, setUncertain] = useState(false);
  const [problem, setProblem] = useState("");
  const [notice, setNotice] = useState("");
  const [pick, setPick] = useState<PickState | null>(null);
  const [chosen, setChosen] = useState("");
  const [withIG, setWithIG] = useState(true);
  const key = useRef<{ kind: string; value: string } | null>(null);
  const inFlight = useRef(false);
  const health = useMetaHealth(store);
  const pickIntent = useRef<{ id: string; ig: boolean; before: string | null } | null>(null);

  const [step, setStep] = useState<"list" | "intro" | "bridge" | "pick" | "confirm" | "result" | "detail" | "health" | "disconnect">("list");
  const [selected, setSelected] = useState("");
  const [resultPage, setResultPage] = useState<{ name: string; instagram: string } | null>(null);

  useEffect(() => {
    let live = true;
    sessionBoundary().then(
      (v) => live && setBoundary(v),
      () => live && setBoundary(""),
    );
    return () => { live = false; };
  }, [store]);

  useEffect(() => {
    if (load === "ready" && (window.location.hash === "#facebook-instagram" || step !== "list")) { document.getElementById("facebook-instagram")?.scrollIntoView({ block: "start" }); if (step !== "list") document.getElementById("metaconnect-title")?.focus({ preventScroll: true }); }
  }, [store, load, step]);

  // The return from Facebook: ?meta_connect=<state_id> (open the pick list) or ?meta_error=<fixed code>; stripped once read. The
  // ref keeps the parsed values across a dev double-mount, which would otherwise find the URL already stripped.
  const arrival = useRef<{ id: string | null; error: string | null } | undefined>(undefined);
  useEffect(() => {
    if (arrival.current === undefined) {
      const params = new URLSearchParams(window.location.search);
      arrival.current = {
        id: params.get("meta_connect"),
        error: params.get("meta_error"),
      };
      params.delete("meta_connect");
      params.delete("meta_error");
      window.history.replaceState(null, "", `${window.location.pathname}${params.size ? `?${params}` : ""}${window.location.hash}`);
    }
    const { id, error } = arrival.current;
    if (error) setProblem(c.errors[error] ?? c.errors.unavailable);
    if (!id) return;
    const active = new AbortController();
    readPickState(store, id, active.signal).then(
      (state) => { setStep("pick"); setPick(state); const first = state.pages.find((p) => p.missing.length === 0) ?? state.pages[0]; setChosen(first?.page_id ?? ""); setWithIG(!!first?.ig_id && first.ig_missing.length === 0); },
      () => !active.signal.aborted && setProblem(c.errors.state_expired),
    );
    return () => active.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- read once on mount
  }, [store]);

  useEffect(() => {
    const active = new AbortController();
    readStatus(store, active.signal).then(
      (value) => {
        if (active.signal.aborted) return;
        setStatus(value); setLoad("ready");
        if (tick > 0) refreshMetaHealth(store);
        const pendingPage = key.current?.kind.startsWith("disconnect-") ? key.current.kind.slice(11) : null;
        if (pendingPage && !value.pages.some((p) => p.id === pendingPage)) {
          key.current = null; setUncertain(false); setConfirming(""); setProblem(""); setNotice(c.disconnectedNotice); setStep("list"); setSelected("");
        }
        const intent = pickIntent.current;
        const observed = intent && value.pages.find((p) => p.id === intent.id);
        if (intent && observed?.status === "active" && (!intent.ig || observed.instagram) && observed.connected_at !== intent.before) {
          pickIntent.current = null; key.current = null; setUncertain(false); setPick(null); setProblem(""); setNotice(c.connectedNotice); setStep("result");
        }
      },
      (error) => {
        if (active.signal.aborted) return;
        setStatus(null);
        setLoad(
          error instanceof OrderReadError && error.code === "forbidden" ? "hidden" : "error",
        );
      },
    );
    return () => active.abort();
  }, [store, tick]);

  if (load === "hidden") return null;

  // One key per intent. Only start supports keyed retries. Pick/disconnect UNKNOWN must
  // read back, never repeat a write: these backend commands do not yet replay receipts.
  const keyFor = (kind: string) => {
    if (key.current?.kind !== kind) key.current = { kind, value: newKey(`meta-${kind}`) };
    return key.current.value;
  };
  const fail = (code: string, unknown: boolean) => {
    setUncertain(unknown);
    if (!unknown) key.current = null;
    setProblem(
      unknown ? key.current?.kind === "connect" ? c.uncertainStart : c.uncertain : (c.errors[code] ?? c.errors.unavailable),
    );
  };

  async function connect() {
    if (!canManage) return;
    if (inFlight.current || (uncertain && key.current?.kind !== "connect")) return;
    inFlight.current = true;
    setBusy(true); setProblem(""); setNotice("");
    const result = await postConnect(store, keyFor("connect"), boundary);
    if (result.ok) { window.location.assign(result.value); return; } // postConnect proved the origin is exactly https://www.facebook.com
    inFlight.current = false; setBusy(false); fail(result.code, result.uncertain);
  }
  async function submitPick() {
    if (!canManage) return;
    if (inFlight.current || uncertain || !pick || load !== "ready" || !status) return;
    const page = pick.pages.find((p) => p.page_id === chosen);
    if (!page || !pickable(page, false)) return;
    inFlight.current = true;
    setBusy(true);
    setProblem("");
    setNotice("");
    setSelected(chosen); setResultPage({ name: page.name || page.page_id, instagram: withIG && page.ig_username ? `@${page.ig_username}` : "" });
    pickIntent.current = {
      id: chosen,
      ig: withIG && pickable(page, true),
      before: status?.pages.find((p) => p.id === chosen)?.connected_at ?? null,
    };
    const result = await postPick(
      store,
      keyFor(`pick-${pick.state_id}-${chosen}-${withIG}`),
      pick.state_id,
      chosen,
      withIG && pickable(page, true),
      boundary,
    );
    inFlight.current = false;
    setBusy(false);
    if (result.ok) { setResultPage({ name: page.name || page.page_id, instagram: withIG && page.ig_username ? `@${page.ig_username}` : "" }); setStep("result"); pickIntent.current = null; key.current = null; setUncertain(false); setPick(null); setNotice(c.connectedNotice); }
    else { if (!result.uncertain) pickIntent.current = null; fail(result.code, result.uncertain); }
    if (result.ok || !result.uncertain) setTick((v) => v + 1);
  }
  async function disconnect() {
    if (!canManage) return;
    if (inFlight.current || uncertain || !confirming) return;
    inFlight.current = true;
    setBusy(true);
    setProblem("");
    setNotice("");
    const result = await postDisconnect(
      store,
      keyFor(`disconnect-${confirming}`),
      confirming,
      boundary,
    );
    inFlight.current = false;
    setBusy(false);
    if (result.ok) { setStep("list"); setSelected(""); key.current = null; setUncertain(false); setConfirming(""); setNotice(c.disconnectedNotice); }
    else { if (!result.uncertain) setConfirming(""); fail(result.code, result.uncertain); }
    if (result.ok || !result.uncertain) setTick((v) => v + 1);
  }

  const chosenPage: PickPage | undefined = pick?.pages.find((p) => p.page_id === chosen);
  const locked = !canManage || busy || uncertain || !boundary;

  const permissionLabel = (code: string) => code === "pages_messaging" ? c.task_messaging : code === "pages_read_engagement" ? c.task_moderate : code === "pages_manage_metadata" ? p.permissionSubscriptions : code === "pages_show_list" ? p.permissionPages : code === "instagram_manage_comments" ? p.permissionIGComments : code === "instagram_manage_messages" ? p.permissionIGMessages : code === "read_insights" ? p.permissionInsights : code === "task_messaging" || code === "task_moderate" ? label(c, code) : c.permissions;
  const currentPage = status?.pages.find(page => page.id === selected);
  const heading = uncertain && key.current?.kind !== "connect" ? p.uncertain : step === "intro" ? p.intro : step === "bridge" ? p.bridge : step === "pick" ? p.pick : step === "confirm" ? p.confirm : step === "result" ? p.result : step === "detail" ? p.detailTitle : step === "health" ? p.healthTitle : step === "disconnect" ? p.disconnectTitle : p.meta;
  const back = () => { setStep("list"); setSelected(""); };
  const openDetail = (id: string) => { setSelected(id); setStep("detail"); };
  return <section className={`settings-fields metaconnect-card ${taskStyle.page}`} id="facebook-instagram" data-testid="metaconnect-card" aria-labelledby="metaconnect-title">
    <h2 id="metaconnect-title" tabIndex={-1}>{heading}</h2>
    {load === "loading" && <p role="status">{c.loading}</p>}
    {load === "error" && <div role="status"><p>{c.unavailable}</p><button type="button" onClick={() => setTick(value => value + 1)}>{c.retry}</button></div>}
    {problem && <p className="settings-warning" role="alert" data-testid="metaconnect-error">{problem}</p>}
    {notice && <p role="status" data-testid="metaconnect-notice">{notice}</p>}
    {uncertain && key.current?.kind !== "connect" ? <div className={taskStyle.card}>
      <p>{currentPage?.name || resultPage?.name}</p><p>{key.current?.kind.startsWith("disconnect-") ? p.disconnectHelp : p.oauthHelp}</p><button type="button" data-testid="metaconnect-readback" disabled={busy} onClick={() => setTick(value => value + 1)}>{p.readResult}</button>
    </div> : <>
      {(step === "intro" || step === "bridge") && <div className={taskStyle.card}><p>{p.oauthHelp}</p><button className="primary" type="button" disabled={locked && !(uncertain && key.current?.kind === "connect")} onClick={() => step === "intro" ? setStep("bridge") : void connect()}>{p.official}</button><button type="button" disabled={busy} onClick={back}>{p.returnList}</button></div>}
      {pick && (step === "pick" || step === "confirm") && <div data-testid="metaconnect-pick" className={taskStyle.card}>
        {step === "pick" ? <><p>{c.pickIntro}</p>
          {pick.pages.length === 0 ? <p data-testid="metaconnect-nopages">{c.noPages}</p> : <fieldset data-testid="metaconnect-pick-list">{pick.pages.map(page => <label key={page.page_id} className="settings-check"><input type="radio" name="metaconnect-page" checked={chosen === page.page_id} disabled={locked} data-testid={`metaconnect-pick-${page.page_id}`} onChange={() => { setChosen(page.page_id); setWithIG(!!page.ig_id && page.ig_missing.length === 0); }} /><span>{page.name || page.page_id}{page.ig_username && <small> · @{page.ig_username}</small>}</span></label>)}</fieldset>}
          {!!chosenPage?.missing.length && <p className="settings-warning" data-testid="metaconnect-missing">{c.pickMissing} {chosenPage.missing.map(item => permissionLabel(item)).join(", ")}</p>}
          {chosenPage?.ig_id && !chosenPage.missing.length && (!chosenPage.ig_missing.length ? <label className="settings-check"><input type="checkbox" checked={withIG} disabled={locked} data-testid="metaconnect-pick-ig" onChange={event => setWithIG(event.target.checked)} /><span>{c.withInstagram} · @{chosenPage.ig_username}</span></label> : <p data-testid="metaconnect-ig-missing">{c.instagramMissing}</p>)}
          <p className={`${taskStyle.note} ${taskStyle.muted}`}>{p.igHelp}</p>
          <div className={taskStyle.footer}><button type="button" onClick={() => { setPick(null); setStep("list"); setProblem(""); key.current = null; }} disabled={locked}>{c.pickCancel}</button><button className="primary" type="button" data-testid="metaconnect-pick-submit" disabled={locked || !chosenPage || !!chosenPage.missing.length} onClick={() => setStep("confirm")}>{p.pickReview}</button></div>
        </> : <><p>{chosenPage?.name || chosenPage?.page_id}</p><p>{withIG && chosenPage?.ig_username ? `@${chosenPage.ig_username}` : p.noInstagram}</p><p>{p.oauthHelp}</p><div className={taskStyle.footer}><button type="button" disabled={busy} onClick={() => setStep("pick")}>{p.edit}</button><button className="primary" type="button" data-testid="metaconnect-confirm-connect" disabled={locked || !chosenPage || !!chosenPage.missing.length} onClick={() => void submitPick()}>{p.confirmConnect}</button></div></>}
      </div>}
      {step === "result" && <div className={taskStyle.card}><p>{resultPage?.name}</p><p>{resultPage?.instagram || p.noInstagram}</p><button type="button" onClick={back}>{p.returnList}</button></div>}
      {load === "ready" && status && step === "list" && <>
        <p className={taskStyle.muted}>{p.metaHelp}</p><div className={taskStyle.row}><p data-testid="metaconnect-count">{c.count.replace("{count}", String(status.count)).replace("{cap}", String(status.cap))}</p><button className="primary" type="button" data-testid={status.connected ? "metaconnect-add" : "metaconnect-connect"} disabled={locked || status.count >= status.cap} onClick={() => setStep("intro")}>{p.add}</button>{status.connected && <button type="button" data-testid="metaconnect-reconnect" disabled={locked} onClick={() => setStep("intro")}>{c.reconnect}</button>}</div>
        {status.count >= status.cap && <p id="metaconnect-cap">{c.capReached}</p>}
        {!status.connected && <p data-testid="metaconnect-none">{c.notConnected}</p>}
        {health.failed && <p data-testid="meta-health-unavailable">{p.unavailable}</p>}
        {status.pages.map(page => <article key={page.id} data-testid={`metaconnect-row-${page.id}`} className={taskStyle.card}><div data-testid={`metaconnect-page-${page.id}`}><h3 data-testid="metaconnect-page">{page.name || page.id}</h3><p data-testid="metaconnect-ig">{page.instagram ? `@${page.instagram.username}` : c.noInstagram}</p><dl><dd data-testid="metaconnect-token" data-state={page.status}><Badge style={{ whiteSpace: "normal" }} tone={page.status === "active" ? "success" : "warning"}>{page.status === "active" ? c.tokenActive : c.tokenReauth}</Badge></dd></dl><button type="button" onClick={() => openDetail(page.id)}>{p.detail}</button></div></article>)}
      </>}
      {currentPage && (step === "detail" || step === "health" || step === "disconnect") && <>
        <div className={taskStyle.card} data-testid={`metaconnect-row-${currentPage.id}`}><h3 data-testid="metaconnect-page"><span>{currentPage.name || currentPage.id}</span> <small>{currentPage.id}</small></h3><p data-testid="metaconnect-ig">{currentPage.instagram ? `@${currentPage.instagram.username}` : c.noInstagram}</p>
          {step === "detail" && <><p>{currentPage.status === "active" ? p.connected : c.tokenReauth}</p><dl className={taskStyle.facts} data-testid="metaconnect-status"><div><dt>{c.permissions}</dt><dd data-testid="metaconnect-permissions">{currentPage.permissions.map(permission => <span key={permission} data-testid={`metaconnect-permission-${permission}`}>{permissionLabel(permission)} </span>)}</dd></div><div><dt>{c.token}</dt><dd data-testid="metaconnect-token" data-state={currentPage.status}>{currentPage.status === "active" ? c.tokenActive : c.tokenReauth}</dd></div><div><dt>{c.lastEvent}</dt><dd data-testid="metaconnect-last-event">{currentPage.last_event_at ? displayTime(locale, currentPage.last_event_at) : c.never}</dd></div><div><dt>{c.connectedAt}</dt><dd>{displayTime(locale, currentPage.connected_at)}</dd></div><div><dt>{c.routeUntil}</dt><dd>{displayTime(locale, currentPage.route_expires_at)}</dd></div></dl>{Date.parse(currentPage.route_expires_at) - Date.now() < 30 * day && currentPage.status === "active" && <p>{c.renewSoon}</p>}</>}
        </div>
        {step === "detail" && <><p className={taskStyle.note}>{currentPage.permissions.includes("read_insights") ? p.permissionInsights : p.insightsHelp}</p><p className={`${taskStyle.note} ${taskStyle.muted}`}>{p.igHelp}</p><div className={taskStyle.footer}><button type="button" onClick={() => setStep("health")}>{p.health}</button><button type="button" data-testid="metaconnect-disconnect" disabled={locked} onClick={() => { setConfirming(currentPage.id); setStep("disconnect"); }}>{p.disconnect}</button></div></>}
        {step === "health" && <><p className={taskStyle.note}>{p.healthHelp}</p><MetaHealthCapabilities key={`${store}:${currentPage.id}`} store={store} page={health.data?.pages.find(page => page.page_id === currentPage.id)} locale={locale} canManage={canManage} boundary={boundary} failed={health.failed} /></>}
        {step === "disconnect" && <div role="alertdialog" aria-label={p.disconnectTitle} data-testid="metaconnect-confirm" className={taskStyle.card}><p>{p.disconnectHelp}</p><div className={taskStyle.footer}><button type="button" disabled={locked} onClick={() => { setConfirming(""); setStep("detail"); }}>{p.keep}</button><button className="settings-danger" type="button" data-testid="metaconnect-confirm-yes" disabled={locked} onClick={() => void disconnect()}>{p.confirmDisconnect}</button></div></div>}
        <div className={taskStyle.footer}>{step !== "disconnect" && <button type="button" data-testid="metaconnect-reconnect" disabled={locked} onClick={() => setStep("intro")}>{c.reconnect}</button>}<button type="button" disabled={busy} onClick={back}>{p.returnList}</button></div>
      </>}
    </>}
  </section>;
}
