"use client";
// Extracted ads panel: existing BFF ads-client calls -> Go /v1/admin/stores/{store}/ads; no command or DTO changes.
import { useEffect, useState, type FormEvent } from "react";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "@/lib/model";
import {
  postBindings,
  readConnectState,
  AdsReadError,
  type WriteResult,
} from "@/lib/ads-client";
import {
  accountReady,
  type ConnectError,
  type ConnectState,
  type PickItem,
  type Settings,
} from "@/lib/ads-model";
import type { AdsCopy } from "@/lib/ads-copy";

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
            </li>
          ))}
        </ul>
      )}
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
