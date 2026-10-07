// Purpose: merchant failed/UNKNOWN ledger list and accessible detail drawer with server-authorized actions.
// Depends on: React, Next navigation, shared format, WorkspaceFrame/AdminPageHeader, operations-client/model/copy, session fence.
// Used by: /[locale]/settings/operations; Go operations.go is the authoritative state/capability boundary.
// Invariants: I01/I02/I06/I11; no optimistic success, no provider payloads, uncertain requests keep identical keys.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import { displayTime } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import { OrderReadError } from "@/lib/orders-client";
import { sessionBoundary } from "@/lib/settings-client";
import {
  readOperation,
  readOperations,
  writeOperation,
} from "@/lib/operations-client";
import {
  operationFilters,
  operationObjectHref,
  canOperate,
  type OperationAction,
  type OperationDetail,
  type OperationFilter,
  type OperationItem,
} from "@/lib/operations-model";
import { operationQueryWaitSeconds } from "@/lib/operations-request";
import { operationsCopy, operationReasonText } from "@/lib/operations-copy";
import { WorkspaceFrame } from "./WorkspaceFrame";
import { AdminPageHeader } from "./AdminPageHeader";
import { OperationFacts, OperationEvents } from "./OperationFacts";
import "./operations.css";
type Status = "loading" | "ready" | "signed-out" | "forbidden" | "unavailable";
type Command = {
  id: string;
  action: OperationAction;
  body: string;
  key: string;
  boundary: string;
};
/** Render the ledger; commands are fenced to the authenticated session and confirmed against fresh server state. */
export function OperationsLedger({
  locale,
  stores,
  store,
  initialError,
  operation,
}: {
  locale: Locale;
  stores: Store[];
  store: Store | null;
  initialError: Exclude<Status, "ready" | "loading"> | null;
  operation: string;
}) {
  const c = operationsCopy[locale];
  const router = useRouter();
  const canWrite = canOperate(store);
  const [status, setStatus] = useState<Status>(initialError ?? "loading");
  const [filter, setFilter] = useState<OperationFilter>("attention");
  const [cursor, setCursor] = useState("");
  const [rows, setRows] = useState<OperationItem[]>([]);
  const [next, setNext] = useState("");
  const [selected, setSelected] = useState(operation);
  const [detail, setDetail] = useState<OperationDetail | null>(null);
  const [detailStatus, setDetailStatus] = useState<Status>("loading");
  const [tick, setTick] = useState(0);
  const [confirm, setConfirm] = useState<OperationAction | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [uncertain, setUncertain] = useState<Command | null>(null);
  const [queryWait, setQueryWait] = useState<{
    id: string;
    until: number;
    seconds: number;
  } | null>(null);
  const drawer = useRef<HTMLDialogElement>(null);
  const confirmDialog = useRef<HTMLDialogElement>(null);
  // WorkspaceFrame mounts children after authentication. Open a deep-linked drawer when its node actually mounts.
  const mountDrawer = useCallback((node: HTMLDialogElement | null) => {
    drawer.current = node;
    if (node && selected && !node.open) node.showModal();
  }, [selected]);
  const boundary = useRef("");
  const blocked = useRef(false);
  const sending = useRef(false);
  const generation = useRef(0);
  const refresh = useCallback(() => setTick((n) => n + 1), []);
  const expire = useCallback(() => {
    blocked.current = true;
    generation.current++;
    setStatus("signed-out");
    setRows([]);
    setDetail(null);
    setSelected("");
    setConfirm(null);
    setUncertain(null);
    setMessage("");
    setBusy(false);
  }, []);
  const problem = (error: unknown): Status =>
    (error instanceof Error && error.message === "session_changed") ||
    (error instanceof OrderReadError && error.code === "signed-out")
      ? "signed-out"
      : error instanceof OrderReadError && error.code === "forbidden"
        ? "forbidden"
        : "unavailable";
  const reason = (code: string) => operationReasonText(locale, code);
  useEffect(() => {
    if (!store || initialError || blocked.current) return;
    const controller = new AbortController();
    setStatus("loading");
    setRows([]);
    (async () => {
      try {
        const fence = await sessionBoundary();
        if (boundary.current && boundary.current !== fence) {
          expire();
          return;
        }
        boundary.current = fence;
        const v = await readOperations(
          store.id,
          filter,
          cursor,
          controller.signal,
        );
        if (controller.signal.aborted || blocked.current) return;
        if ((await sessionBoundary()) !== fence) {
          expire();
          return;
        }
        if (controller.signal.aborted || blocked.current) return;
        setRows(v.items);
        setNext(v.next_cursor);
        setStatus("ready");
      } catch (e) {
        if (!controller.signal.aborted && !blocked.current) {
          if (problem(e) === "signed-out") {
            expire();
            return;
          }
          setStatus(problem(e));
          setRows([]);
          setNext("");
        }
      }
    })();
    return () => controller.abort();
  }, [store, filter, cursor, tick, initialError, expire]);
  useEffect(() => {
    setDetail(null);
    if (!store || !selected || blocked.current) return;
    const controller = new AbortController();
    setDetailStatus("loading");
    (async () => {
      try {
        const fence = await sessionBoundary();
        if (boundary.current && boundary.current !== fence) {
          expire();
          return;
        }
        const v = await readOperation(store.id, selected, controller.signal);
        if (controller.signal.aborted || blocked.current) return;
        if ((await sessionBoundary()) !== fence) {
          expire();
          return;
        }
        if (controller.signal.aborted || blocked.current) return;
        setDetail(v);
        setDetailStatus("ready");
      } catch (e) {
        if (!controller.signal.aborted && !blocked.current) {
          if (problem(e) === "signed-out") expire();
          else setDetailStatus(problem(e));
        }
      }
    })();
    return () => controller.abort();
  }, [store, selected, tick, expire]);
  useEffect(() => {
    if (!queryWait) return;
    const timer = window.setTimeout(
      () => setQueryWait(null),
      Math.max(0, queryWait.until - Date.now()),
    );
    return () => window.clearTimeout(timer);
  }, [queryWait]);
  useEffect(() => {
    const el = drawer.current;
    if (selected && !el?.open) el?.showModal();
    else if (!selected && el?.open) el.close();
  }, [selected]);
  useEffect(() => {
    const el = confirmDialog.current;
    if (confirm && !el?.open) el?.showModal();
    else if (!confirm && el?.open) el.close();
  }, [confirm]);
  useEffect(() => {
    const out = expire;
    const onStorage = (e: StorageEvent) => {
      if (e.key === "commerce-session-logout") out();
    };
    const onFocus = () => {
      if (boundary.current)
        sessionBoundary().then((v) => {
          if (v !== boundary.current) out();
        }, out);
    };
    let channel: BroadcastChannel | null = null;
    try {
      channel = new BroadcastChannel("commerce-session");
      channel.onmessage = (e) => {
        if (e.data?.type === "logout") out();
      };
    } catch {
      /* same-tab and cookie fences remain */
    }
    window.addEventListener("commerce-session-logout", out);
    window.addEventListener("storage", onStorage);
    window.addEventListener("focus", onFocus);
    return () => {
      generation.current++;
      channel?.close();
      window.removeEventListener("commerce-session-logout", out);
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("focus", onFocus);
    };
  }, [expire]);
  async function submit(command: Command) {
    if (!store || sending.current || blocked.current) return;
    sending.current = true;
    setBusy(true);
    setMessage("");
    const version = generation.current;
    try {
      const result = await writeOperation(
        store.id,
        command.id,
        command.action,
        command.body,
        command.key,
        command.boundary,
      );
      if (version !== generation.current || blocked.current) return;
      try {
        if ((await sessionBoundary()) !== command.boundary) {
          expire();
          return;
        }
      } catch {
        expire();
        return;
      }
      if (version !== generation.current || blocked.current) return;
      if (result.ok) {
        setUncertain(null);
        setConfirm(null);
        setMessage(c.saved);
        refresh();
      } else {
        setMessage(result.uncertain ? c.uncertain : reason(result.code));
        setUncertain(result.uncertain ? command : null);
        if (result.retryAfter)
          setQueryWait({
            id: command.id,
            until: Date.now() + result.retryAfter * 1000,
            seconds: result.retryAfter,
          });
        if (result.code === "unauthorized") {
          blocked.current = true;
          setRows([]);
          setDetail(null);
          setSelected("");
          setStatus("signed-out");
        }
        setConfirm(null);
        refresh();
      }
    } finally {
      sending.current = false;
      if (version === generation.current) setBusy(false);
    }
  }
  function begin() {
    if (
      !canWrite ||
      !store ||
      !detail ||
      !confirm ||
      !detail.actions[confirm].available ||
      uncertain ||
      blocked.current
    )
      return;
    const command = {
      id: detail.operation_id,
      action: confirm,
      body: JSON.stringify({ expected_attempts: detail.attempts }),
      key: crypto.randomUUID(),
      boundary: boundary.current,
    };
    void submit(command);
  }
  function object(item: OperationItem) {
    const v = item.object;
    if (!v) return "—";
    const href = operationObjectHref(locale, store?.id ?? "", v);
    return href ? (
      <a href={href}>
        {v.kind} · {v.id}
      </a>
    ) : (
      <span>
        {v.kind} · {v.id}
      </span>
    );
  }
  const stateText = (state: string) =>
    c.states[state as keyof typeof c.states] ?? state;
  const statusText = (value: Status) =>
    value === "loading"
      ? c.loading
      : value === "signed-out"
        ? c.signedOut
        : value === "forbidden"
          ? c.forbidden
          : c.unavailable;
  return (
    <WorkspaceFrame
      locale={locale}
      active="settings"
      storeName={store?.name ?? c.noStore}
      locked={busy}
    >
      <div className="operations-page" data-testid="operations-ledger">
        <AdminPageHeader locale={locale} description={c.subtitle} />
        <nav className="operations-links" aria-label={c.title}>
          <a href={`/${locale}/settings${store ? `?store=${store.id}` : ""}`}>
            {c.settings}
          </a>
          <a href={`/${locale}/ads${store ? `?store=${store.id}` : ""}`}>
            {c.ads}
          </a>
        </nav>
        {stores.length > 1 && (
          <label>
            {c.switchStore}
            <select
              data-testid="operations-store"
              aria-label={c.switchStore}
              disabled={busy || !!uncertain}
              value={store?.id ?? ""}
              onChange={(e) =>
                router.push(
                  `/${locale}/settings/operations?store=${e.target.value}`,
                )
              }
            >
              {stores.map((v) => (
                <option value={v.id} key={v.id}>
                  {v.name}
                </option>
              ))}
            </select>
          </label>
        )}
        {!store ? (
          <p>{c.noStore}</p>
        ) : (
          <>
            <div className="operations-toolbar">
              <label>
                {c.filter}
                <select
                  data-testid="operations-filter"
                  value={filter}
                  disabled={busy}
                  onChange={(e) => {
                    setFilter(e.target.value as OperationFilter);
                    setCursor("");
                  }}
                >
                  {operationFilters.map((v) => (
                    <option value={v} key={v}>
                      {stateText(v)}
                    </option>
                  ))}
                </select>
              </label>
              <button
                data-testid="operations-refresh"
                disabled={busy || status === "signed-out"}
                onClick={refresh}
              >
                {c.refresh}
              </button>
            </div>
            {message && (
              <p
                className="operations-feedback"
                data-testid="operations-feedback"
                role="status"
              >
                {message}
              </p>
            )}
            {queryWait && (
              <p role="status">
                {c.waiting}: {queryWait.seconds}
              </p>
            )}
            {uncertain && (
              <button
                data-testid="operation-repeat"
                disabled={busy}
                onClick={() => void submit(uncertain)}
              >
                {c.retryRequest}
              </button>
            )}
            {status !== "ready" ? (
              <p role={status === "loading" ? "status" : "alert"}>
                {statusText(status)}
              </p>
            ) : rows.length === 0 ? (
              <p>{c.empty}</p>
            ) : (
              <ul className="operations-list">
                {rows.map((v) => (
                  <li
                    key={v.operation_id}
                    data-testid={`operation-row-${v.operation_id}`}
                  >
                    <div>
                      <strong>{v.action}</strong>
                      <span>{stateText(v.state)}</span>
                    </div>
                    <p>
                      {c.provider}: {v.provider} · {c.attempts}: {v.attempts}
                    </p>
                    <time dateTime={v.updated_at}>
                      {displayTime(locale, v.updated_at)}
                    </time>
                    <p className="operations-object">{object(v)}</p>
                    <button
                      data-testid={`operation-open-${v.operation_id}`}
                      onClick={() => {
                        setSelected(v.operation_id);
                        setMessage("");
                      }}
                    >
                      {c.open}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            {status === "ready" && next && (
              <button
                data-testid="operations-next"
                onClick={() => setCursor(next)}
                disabled={busy}
              >
                {c.next}
              </button>
            )}
          </>
        )}
      </div>
      <dialog
        ref={mountDrawer}
        className="operations-drawer"
        aria-labelledby="operation-title"
        data-testid="operation-drawer"
        onCancel={(e) => {
          if (busy) {
            e.preventDefault();
            return;
          }
          setSelected("");
        }}
        onClose={() => {
          if (!busy) setSelected("");
        }}
      >
        <header>
          <h2 id="operation-title">{c.detail}</h2>
          <button
            data-testid="operation-close"
            disabled={busy}
            onClick={() => setSelected("")}
          >
            {c.close}
          </button>
        </header>
        {detailStatus !== "ready" || !detail ? (
          <>
            <p role="status">{statusText(detailStatus)}</p>
            <button onClick={refresh} disabled={busy}>
              {c.refresh}
            </button>
          </>
        ) : (
          <>
            <OperationFacts
              detail={detail}
              locale={locale}
              object={object(detail)}
            />
            {detail.state === "UNKNOWN" && <p>{c.unknown}</p>}
            <div className="operations-actions">
              {(["query", "cancel", "retry"] as const).map((a) =>
                canWrite && detail.actions[a].available ? (
                  <button
                    key={a}
                    data-testid={`operation-${a}`}
                    disabled={
                      busy ||
                      !!uncertain ||
                      (a === "query" &&
                        operationQueryWaitSeconds(
                          detail.operation_id,
                          queryWait,
                        ) > 0)
                    }
                    onClick={() => setConfirm(a)}
                  >
                    {c[a]}
                  </button>
                ) : (
                  <p key={a} data-testid={`operation-${a}-reason`}>
                    {c[a]}:{" "}
                    {reason(canWrite ? detail.actions[a].reason : "forbidden")}
                  </p>
                ),
              )}
            </div>
            {message && (
              <p role="status" data-testid="operation-feedback">
                {message}
              </p>
            )}
            {queryWait?.id === detail.operation_id && (
              <p>
                {c.waiting}: {queryWait.seconds}
              </p>
            )}
            {uncertain?.id === detail.operation_id && (
              <button
                data-testid="operation-repeat-drawer"
                disabled={busy}
                onClick={() => void submit(uncertain)}
              >
                {c.retryRequest}
              </button>
            )}
            <button
              data-testid="operation-detail-refresh"
              onClick={refresh}
              disabled={busy}
            >
              {c.refresh}
            </button>
            <OperationEvents detail={detail} locale={locale} />
          </>
        )}
      </dialog>
      <dialog
        ref={confirmDialog}
        className="operations-confirm"
        aria-labelledby="operation-confirm-title"
        data-testid="operation-confirm"
        onCancel={(e) => {
          if (busy) e.preventDefault();
          else setConfirm(null);
        }}
        onClose={() => {
          if (!busy) setConfirm(null);
        }}
      >
        <h2 id="operation-confirm-title">{confirm ? c[confirm] : c.confirm}</h2>
        <p>{c.confirmHint}</p>
        <p className="operations-id">{selected}</p>
        <button
          data-testid="operation-confirm-dismiss"
          disabled={busy}
          onClick={() => setConfirm(null)}
        >
          {c.dismiss}
        </button>
        <button
          data-testid="operation-confirm-submit"
          disabled={
            busy ||
            !canWrite ||
            !confirm ||
            !detail?.actions[confirm]?.available ||
            !!uncertain
          }
          onClick={begin}
        >
          {c.confirm}
        </button>
      </dialog>
    </WorkspaceFrame>
  );
}
