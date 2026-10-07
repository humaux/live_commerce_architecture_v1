// Purpose: Renders ads account connection controls and connection outcomes.
// Depends on: react, @live-commerce/i18n, @/lib/model, @/lib/ads-client, @/lib/ads-model, @/lib/ads-copy
// Used by: apps/admin/components/Ads.tsx
"use client";
// Extracted ads panel: existing BFF ads-client calls -> Go /v1/admin/stores/{store}/ads; no command or DTO changes.
import { useEffect, useRef, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import {
  postUnbind,
  readCatalogFeed,
  postBindings,
  readConnectState,
  AdsReadError,
  type WriteResult,
} from "@/lib/ads-client";
import {
  type AdsCode,
  type AdsInFlight,
  type CatalogFeed,
  accountReady,
  type ConnectError,
  type ConnectState,
  type PickItem,
  type Settings,
} from "@/lib/ads-model";
import type { AdsCopy } from "@/lib/ads-copy";

/** Renders ads account connection controls and connection outcomes. User actions submit connection commands through ads-client. */
export function ConnectionSection({
  c,
  locale,
  settings,
  store,
  connect,
  connectError,
  busy,
  uncertain,
  startConnect,
  run,
  boundary,
  done,
  startAgain,
  when,
}: {
  when: (locale: Locale, iso: string | null, empty: string) => string;
  c: AdsCopy;
  locale: Locale;
  settings: Settings;
  store: Store;
  connect: string;
  connectError: ConnectError | "";
  busy: string;
  uncertain: string;
  startConnect: () => Promise<void>;
  boundary: { current: string };
  done: () => void;
  startAgain: () => void;
  run: <T>(
    slot: string,
    body: string,
    exec: (key: string) => Promise<WriteResult<T>>,
    okText: string,
    after?: (value: T) => void,
  ) => Promise<void>;
}) {
  const [unbindAccount, setUnbindAccount] = useState<string | null>(null);
  const [inFlight, setInFlight] = useState<{account:string;value:AdsInFlight} | null>(null);
  const [unbindError, setUnbindError] = useState<AdsCode | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (unbindAccount) dialog.current?.showModal();
    else dialog.current?.close();
  }, [unbindAccount]);
  // Store/session changes dismiss the old account and safe operation details.
  useEffect(() => {setUnbindAccount(null);setInFlight(null);setUnbindError(null);}, [store.id, boundary.current]);
  const canManage = store.role === "owner" || store.permissions?.includes("ads:manage") === true;
  function unbind() {
    if (!canManage || !unbindAccount || busy || uncertain) return;
    const account = unbindAccount;
    const body = JSON.stringify({ad_account_id:account});
    setInFlight(null);setUnbindError(null);
    void run(`unbind:${account}`,body,async (key) => {
      const result = await postUnbind(store.id,key,body,boundary.current);
      if (!result.ok) {
        if (result.details) setInFlight({account,value:result.details});
        if (!result.uncertain) setUnbindError(result.code);
      }
      return result;
    },c.unbound,() => setUnbindAccount(null));
  }
  const accounts = settings.connections.filter(
    (x) => x.provider === "meta_ads",
  );
  const datasets = settings.connections.filter(
    (x) => x.provider === "meta_dataset",
  );
  return (
    <section
      className="ads-section"
      aria-labelledby="ads-conn-h"
      data-testid="ads-connection"
    >
      <div className="ads-section-head">
        <h2 id="ads-conn-h">{c.connTitle}</h2>
        <button
          type="button"
          className="primary"
          data-testid="ads-connect"
          disabled={busy !== "" || !!connect}
          onClick={() => void startConnect()}
        >
          {busy === "connect" ? c.connecting : c.connectButton}
        </button>
      </div>
      <p className="ads-note">{c.connectHint}</p>
      {settings.recent_refusals.length > 0 && (
        <div data-testid="ads-meta-refusals">
          <h3>{c.metaResponse}</h3>
          <ul className="ads-list">
            {settings.recent_refusals.map((r) => (
              <li key={r.operation_id}>
                <small>
                  {r.action} · {r.code} · {when(locale, r.updated_at, "—")}
                </small>
                <p className="ads-meta-message">{r.error_user_msg}</p>
              </li>
            ))}
          </ul>
        </div>
      )}
      {connectError && (
        <div className="ads-bad" role="alert" data-testid="ads-connect-error">
          {c.connectErrors[connectError]}{" "}
          <button type="button" className="ads-inline" onClick={startAgain}>
            {c.startAgain}
          </button>
        </div>
      )}
      {connect && (
        <PickStep
          c={c}
          store={store}
          stateID={connect}
          busy={busy}
          uncertain={uncertain}
          run={run}
          boundary={boundary}
          done={done}
          startAgain={startAgain}
        />
      )}
      {accounts.length === 0 && datasets.length === 0 ? (
        <p className="ads-empty" data-testid="ads-conn-empty">
          {c.connEmpty}
        </p>
      ) : (
        <ul className="ads-list" data-testid="ads-connections">
          {[...accounts, ...datasets].map((x) => (
            <li key={x.binding_id}>
              <strong>{c.providers[x.provider] ?? x.provider}</strong>{" "}
              <span className="ads-mono">{x.asset_id}</span>
              <small>
                {c.connBusiness}:{" "}
                <span className="ads-mono">{x.client_business_id}</span> ·{" "}
                {c.connAt}: {when(locale, x.connected_at, "—")}
              </small>
              <span className={x.enabled ? "ads-tone-ok" : "ads-tone-warn"}>
                {x.enabled ? c.connEnabled : c.connDisabled}
              </span>
              {canManage && x.provider === "meta_ads" && x.enabled && (
                <button type="button" className="ads-inline ads-danger" data-testid="ads-unbind"
                  aria-label={`${c.unbindButton}: ${x.asset_id}`} disabled={busy !== "" || !!uncertain || !!connect}
                  onClick={() => {setInFlight(null);setUnbindError(null);setUnbindAccount(x.asset_id);}}>{c.unbindButton}</button>
              )}
            </li>
          ))}
        </ul>
      )}
      <dialog ref={dialog} className="ads-unbind-dialog" aria-labelledby="ads-unbind-h" aria-describedby="ads-unbind-copy"
        data-testid="ads-unbind-confirm" onCancel={(event) => {if (busy) event.preventDefault();else setUnbindAccount(null);}}
        onClose={() => setUnbindAccount(null)}>
        <h3 id="ads-unbind-h">{c.unbindTitle}</h3>
        <p className="ads-mono">{unbindAccount}</p>
        <div id="ads-unbind-copy"><p>{c.unbindHistory}</p><p>{c.unbindPause}</p><p>{c.unbindLocal}</p><p>{c.unbindCapi}</p></div>
        <div className="ads-actions">
          <button type="button" className="ads-danger" data-testid="ads-unbind-yes" disabled={!canManage || busy !== "" || !!uncertain} onClick={unbind}>
            {busy.startsWith("unbind:") ? c.sending : c.unbindConfirm}</button>
          <button type="button" data-testid="ads-unbind-cancel" autoFocus disabled={busy !== ""} onClick={() => setUnbindAccount(null)}>{c.cancel}</button>
        </div>
        {unbindError && unbindError !== "operations_in_flight" && <p role="alert" data-testid="ads-unbind-error">{c.errors[unbindError]}</p>}
        {uncertain.startsWith("unbind:") && <p role="alert">{c.uncertain}</p>}
        {inFlight && inFlight.account === unbindAccount && <InFlightDetails c={c} value={inFlight.value} />}
      </dialog>
      <CatalogFeedCard c={c} store={store} />
      <h3>{c.identitiesTitle}</h3>
      {settings.identities.length === 0 ? (
        <p className="ads-empty">{c.identitiesEmpty}</p>
      ) : (
        <ul className="ads-list" data-testid="ads-identities">
          {settings.identities.map((x) => (
            <li key={x.binding_id}>
              <strong>{c.providers[x.provider] ?? x.provider}</strong>{" "}
              <span className="ads-mono">{x.asset_id}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function PickStep({
  c,
  store,
  stateID,
  busy,
  uncertain,
  run,
  boundary,
  done,
  startAgain,
}: {
  c: AdsCopy;
  store: Store;
  stateID: string;
  busy: string;
  uncertain: string;
  boundary: { current: string };
  done: () => void;
  startAgain: () => void;
  run: <T>(
    slot: string,
    body: string,
    exec: (key: string) => Promise<WriteResult<T>>,
    okText: string,
    after?: (value: T) => void,
  ) => Promise<void>;
}) {
  const [state, setState] = useState<
    | { kind: "loading" }
    | { kind: "expired" }
    | { kind: "error" }
    | { kind: "ready"; value: ConnectState }
  >({ kind: "loading" });
  const [account, setAccount] = useState("");
  const [dataset, setDataset] = useState("");
  useEffect(() => {
    const active = new AbortController();
    readConnectState(store.id, stateID, active.signal).then(
      (value) => !active.signal.aborted && setState({ kind: "ready", value }),
      (error) => {
        if (active.signal.aborted) return;
        const code = error instanceof AdsReadError ? error.code : "unavailable";
        setState({
          kind:
            code === "state_expired" ||
            code === "not-found" ||
            code === "state_mismatch"
              ? "expired"
              : "error",
        });
      },
    );
    return () => active.abort();
  }, [store.id, stateID]);
  if (state.kind === "loading")
    return (
      <p className="ads-message" role="status">
        {c.loading}
      </p>
    );
  if (state.kind !== "ready")
    return (
      <div className="ads-bad" role="alert" data-testid="ads-pick-expired">
        {state.kind === "expired" ? c.pickExpired : c.errors.retry_later}{" "}
        <button type="button" className="ads-inline" onClick={startAgain}>
          {c.startAgain}
        </button>
      </div>
    );
  const accounts = state.value.picks.filter(
    (p: PickItem) => p.kind === "ad_account",
  );
  const datasets = state.value.picks.filter(
    (p: PickItem) => p.kind === "dataset",
  );
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!account) return;
    const body = JSON.stringify(
      dataset
        ? { state_id: stateID, ad_account_id: account, dataset_id: dataset }
        : { state_id: stateID, ad_account_id: account },
    );
    void run(
      "bind",
      body,
      (key) => postBindings(store.id, key, body, boundary.current),
      "",
      done,
    );
  };
  return (
    <form className="ads-form" onSubmit={submit} data-testid="ads-pick">
      <fieldset disabled={busy === "bind" || uncertain === "bind"}>
        <legend>{c.pickTitle}</legend>
        {accounts.length === 0 && (
          <p className="ads-empty">{c.pickNoAccounts}</p>
        )}
        <div
          role="radiogroup"
          aria-label={c.pickAccounts}
          className="ads-radios"
        >
          {accounts.map((p) => (
            <label key={p.id} className="ads-radio">
              <input
                type="radio"
                name="ad_account"
                value={p.id}
                checked={account === p.id}
                onChange={() => setAccount(p.id)}
                data-testid={`ads-pick-${p.id}`}
              />
              <span>
                <strong>{p.name || p.id}</strong>{" "}
                <span className="ads-mono">{p.id}</span>
                <small>
                  {p.currency}
                  {p.timezone ? ` · ${p.timezone}` : ""} ·{" "}
                  <span
                    className={
                      accountReady(p.account_status)
                        ? "ads-tone-ok"
                        : "ads-tone-warn"
                    }
                  >
                    {c.accountStatus(p.account_status)}
                  </span>
                </small>
              </span>
            </label>
          ))}
        </div>
        {datasets.length > 0 && (
          <label>
            {c.pickDatasets}
            <select
              value={dataset}
              onChange={(e) => setDataset(e.target.value)}
              data-testid="ads-pick-dataset"
            >
              <option value="">{c.pickNoDataset}</option>
              {datasets.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name || p.id} ({p.id})
                </option>
              ))}
            </select>
          </label>
        )}
        <div className="ads-actions">
          <button
            type="submit"
            className="primary"
            disabled={!account}
            data-testid="ads-pick-submit"
          >
            {busy === "bind" ? c.sending : c.pickSubmit}
          </button>
          <button type="button" onClick={startAgain}>
            {c.startAgain}
          </button>
        </div>
      </fieldset>
    </form>
  );
}


function InFlightDetails({c,value}:{c:AdsCopy;value:AdsInFlight}) {
  return <div className="ads-bad" role="alert" data-testid="ads-unbind-in-flight">
    <p>{c.errors.operations_in_flight}</p><p data-testid="ads-unbind-total">{c.inFlightTotal(value.operations_total,value.operations.length)}</p>
    <ul className="ads-list">{value.operations.map((op)=> <li key={op.operation_id}>
      <span className="ads-mono">{op.operation_id}</span><span>{op.action}</span><span>{c.opStates[op.state]}</span>
    </li>)}</ul>
  </div>;
}

function CatalogFeedCard({c,store}:{c:AdsCopy;store:Store}) {
  const [feed,setFeed] = useState<CatalogFeed | null>(null);
  const [status,setStatus] = useState<"loading"|"ready"|"signed-out"|"forbidden"|"error">("loading");
  const [tick,setTick] = useState(0);
  const [copy,setCopy] = useState<""|"ok"|"error">("");
  const current = useRef("");
  current.current = store.id;
  useEffect(()=> {
    const active = new AbortController();
    setFeed(null);setStatus("loading");setCopy("");
    readCatalogFeed(store.id,active.signal).then((value)=> {
      if (!active.signal.aborted) {setFeed(value);setStatus("ready");}
    },(error:unknown)=> {
      if (!active.signal.aborted) setStatus(error instanceof AdsReadError && (error.code === "signed-out" || error.code === "forbidden") ? error.code : "error");
    });
    return ()=>active.abort();
  },[store.id,tick]);
  async function copyURL() {
    if (!feed?.feed_url) return;
    const id = store.id;
    try {await navigator.clipboard.writeText(feed.feed_url);if (current.current === id) setCopy("ok");}
    catch {if (current.current === id) setCopy("error");}
  }
  return <div className="ads-feed" data-testid="ads-catalog-feed" aria-labelledby="ads-feed-h">
    <h3 id="ads-feed-h">{c.feedTitle}</h3>
    {status === "loading" && <p role="status">{c.loading}</p>}
    {status !== "loading" && status !== "ready" && <div role="alert">
      <p>{status === "signed-out" ? c.signedOut : status === "forbidden" ? c.forbidden : c.unavailable}</p>
      {status === "error" && <button type="button" data-testid="ads-feed-retry" onClick={()=>setTick(n=>n+1)}>{c.retry}</button>}
    </div>}
    {status === "ready" && feed && (feed.feed_url ? <>
      <p className="ads-note">{c.feedHowTo}</p>
      <label className="ads-feed-url" htmlFor="ads-feed-url">{c.feedTitle}
        <textarea id="ads-feed-url" data-testid="ads-feed-url" readOnly value={feed.feed_url} rows={2} /></label>
      <button type="button" data-testid="ads-feed-copy" onClick={()=>void copyURL()}>{c.feedCopy}</button>
      {copy && <p role={copy === "ok" ? "status" : "alert"} data-testid="ads-feed-copy-result">{copy === "ok" ? c.feedCopied : c.feedCopyFailed}</p>}
    </> : <p className="ads-empty" data-testid="ads-feed-empty">{c.feedEmpty}</p>)}
  </div>;
}
