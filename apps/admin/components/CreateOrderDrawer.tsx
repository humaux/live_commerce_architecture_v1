// Purpose: buyer-scoped order drawer with immutable retries, server pricing and clipboard-only links.
// Depends on: A15/A16 tools BFF, shared ManualOrder fields, native dialog and inbox privacy fences.
// Used by: BuyerPanel and CommentStream; private data is discarded on hide/logout/unmount.
"use client";
import {
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
  type FormEvent,
} from "react";
import type { Locale } from "@live-commerce/i18n";
import { money, displayTime } from "@live-commerce/format";
import type { Store } from "@/lib/model";
import {
  emptyManualForm,
  type ManualFormValues,
} from "@/lib/manual-order-form";
import { draftProblem, type ManualOption } from "@/lib/merchant-tools-model";
import {
  readManualOptions,
  regenerateManualLink,
} from "@/lib/merchant-tools-client";
import {
  readOrderPrefill,
  createForBuyerOrder,
  CreateOrderError,
} from "@/lib/create-order-client";
import {
  prefillForm,
  makeForBuyerBody,
  type DrawerTarget,
  type OrderPrefill,
  type ForBuyerResult,
} from "@/lib/create-order-model";
import {
  createOrderCopy,
  createOrderError,
  createOrderReason,
} from "@/lib/create-order-copy";
import {
  OrderAttempt,
  OrderGuard,
  retainOrderAttempt,
} from "@/lib/create-order-attempt";
import { csrfCookie, sessionBoundary } from "@/lib/settings-client";
import { readMetaHealth } from "@/lib/meta-health-client";
import { inboxRead } from "@/lib/inbox-client";
import { toolsCopy } from "@/lib/merchant-tools-copy";
import { permitted } from "@/src/features/messages/privacy";
import { useInboxPrivacy } from "@/src/features/messages/use-privacy";
import { ManualOrderFormFields } from "./ManualOrderFormFields";
import "./merchant-tools.css";
import styles from "./CreateOrderDrawer.module.css";

/** Normalize a changed or missing cookie into a revocation, including failures during hashing. */
async function checkedBoundary(expected?: string, cookie = csrfCookie()) {
  try {
    const value = await sessionBoundary(cookie);
    if (expected && value !== expected) throw new Error();
    return value;
  } catch {
    throw new CreateOrderError("unauthorized", 401);
  }
}
function denied(cause: unknown): boolean {
  return (
    cause instanceof Error &&
    (("status" in cause && (cause.status === 401 || cause.status === 403)) ||
      ("code" in cause &&
        ["signed-out", "unauthorized", "forbidden"].includes(
          String(cause.code),
        )))
  );
}

/** One keyed target owns one draft. Unknown writes can only replay the frozen request. */
export function CreateOrderDrawer({
  locale,
  store,
  conversationId,
  bundleId,
  onClose,
  onUnauthorized,
}: DrawerTarget & {
  locale: Locale;
  store: Store;
  onClose: () => void;
  onUnauthorized?: () => void;
}) {
  const c = createOrderCopy[locale],
    heading = useId(),
    dialog = useRef<HTMLDialogElement>(null);
  const [values, setValues] = useState<ManualFormValues>(() =>
    emptyManualForm(locale),
  );
  const [prefill, setPrefill] = useState<OrderPrefill | null>(null),
    [available, setAvailable] = useState<ManualOption[]>([]);
  const [boundary, setBoundary] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [uncertain, setUncertain] = useState(false),
    [guarded, setGuarded] = useState(false);
  const [result, setResult] = useState<Omit<
    ForBuyerResult,
    "buyer_link"
  > | null>(null);
  const [duplicate, setDuplicate] = useState<{ id: string | null } | null>(
    null,
  );
  const [send, setSend] = useState(false),
    [cap, setCap] = useState({ allowed: false, review: false, closed: false });
  const [copyState, setCopyState] = useState(""),
    [copyBusy, setCopyBusy] = useState(false);
  const attempt = useRef(new OrderAttempt()),
    guard = useRef<OrderGuard | null>(null),
    privateLink = useRef<string | null>(null);
  const regeneration = useRef<{ key: string; locale: Locale } | null>(null),
    sending = useRef(false),
    copying = useRef(false);
  const clear = useCallback(() => {
    attempt.current.clear();
    privateLink.current = null;
    regeneration.current = null;
    sending.current = false;
    copying.current = false;
    guard.current = null;
    setValues(emptyManualForm());
    setPrefill(null);
    setAvailable([]);
    setBoundary("");
    setBusy(false);
    setError("");
    setUncertain(false);
    setGuarded(false);
    setResult(null);
    setDuplicate(null);
    setSend(false);
    setCap({ allowed: false, review: false, closed: false });
    setCopyState("");
    setCopyBusy(false);
  }, []);
  const privacy = useInboxPrivacy(clear);
  const deniedCallback = useRef(onUnauthorized);
  deniedCallback.current = onUnauthorized;
  const expire = useCallback(() => {
    privacy.expire();
    deniedCallback.current?.();
  }, [privacy.expire]);
  const canCreate =
    permitted(store, "inventory:reserve") && permitted(store, "orders:read");
  useEffect(() => {
    if (!privacy.visible) return;
    const element = dialog.current,
      previous = document.activeElement;
    if (element && !element.open) element.showModal();
    return () => {
      element?.close();
      if (previous instanceof HTMLElement && previous.isConnected)
        previous.focus();
    };
  }, [privacy.visible]);
  useEffect(() => {
    clear();
    if (!privacy.visible || privacy.blocked.current || !canCreate) return;
    privacy.fence.invalidate(true);
    const ticket = privacy.fence.begin();
    void Promise.all([
      readOrderPrefill(
        store.id,
        conversationId ? { conversationId } : { bundleId },
        ticket.signal,
      ),
      readManualOptions(store.id, ticket.signal),
      checkedBoundary(),
    ])
      .then(async ([data, options, scope]) => {
        if (
          !privacy.fence.current(ticket) ||
          (await checkedBoundary(scope)) !== scope ||
          !privacy.fence.current(ticket)
        )
          return;
        let receipt: OrderGuard;
        try {
          receipt = new OrderGuard(window.sessionStorage, store.id, scope);
        } catch {
          setGuarded(true);
          return;
        }
        guard.current = receipt;
        setGuarded(receipt.blocked());
        setBoundary(scope);
        setPrefill(data);
        setAvailable(options);
        setValues(prefillForm(data, locale, store.currency));
      })
      .catch((cause: unknown) => {
        if (!privacy.fence.current(ticket)) return;
        if (denied(cause)) {
          expire();
          return;
        }
        setError(
          cause instanceof CreateOrderError ? cause.code : "unavailable",
        );
      });
    if (
      conversationId &&
      permitted(store, "inbox:reply") &&
      permitted(store, "store:read")
    ) {
      void Promise.all([
        inboxRead<{ binding_id?: unknown; window_open_until?: unknown }>(
          store.id,
          `inbox/conversations/${conversationId}/messages?limit=1`,
          ticket.signal,
        ),
        readMetaHealth(store.id, ticket.signal),
        checkedBoundary(),
      ])
        .then(async ([thread, health, scope]) => {
          if (
            !privacy.fence.current(ticket) ||
            (await checkedBoundary(scope)) !== scope ||
            !privacy.fence.current(ticket)
          )
            return;
          const matches =
            typeof thread.binding_id === "string"
              ? health.pages
                  .flatMap((p) => p.capabilities)
                  .filter(
                    (x) =>
                      x.binding_id === thread.binding_id &&
                      x.capability === "dm_session",
                  )
              : [];
          setCap({
            allowed:
              matches.length > 0 &&
              matches.every(
                (x) => x.state === "ok" || x.state === "review_required",
              ),
            review: matches.some((x) => x.state === "review_required"),
            closed:
              typeof thread.window_open_until === "string" &&
              Date.parse(thread.window_open_until) <= Date.now(),
          });
        })
        .catch((cause: unknown) => {
          if (privacy.fence.current(ticket) && denied(cause)) {
            expire();
            return;
          }
          if (privacy.fence.current(ticket))
            setCap({ allowed: false, review: false, closed: false });
        });
    }
    return () => {
      privacy.fence.invalidate(false);
      attempt.current.clear();
      privateLink.current = null;
      regeneration.current = null;
    };
  }, [
    store,
    conversationId,
    bundleId,
    locale,
    canCreate,
    privacy.visible,
    privacy.revision,
    privacy.blocked,
    privacy.fence,
    expire,
    clear,
  ]);
  const option =
    available.find((x) => x.option_key === values.optionKey) ?? null;
  const problem = draftProblem({
    ...values,
    option,
    locale: values.buyerLocale,
  });
  const mapOnly =
    !!option &&
    option.delivery_kind !== "home" &&
    option.pickup_selection !== "buyer_entered";
  const pending = !!attempt.current.pending();
  const close = () => {
    if (sending.current || (pending && !attempt.current.resolved()) || copying.current) return;
    clear();
    onClose();
  };
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (
      sending.current ||
      !canCreate ||
      !prefill ||
      !boundary ||
      guarded ||
      result ||
      duplicate ||
      attempt.current.resolved() ||
      (!pending && (problem || mapOnly || prefill.bundles.length > 5))
    )
      return;
    const ticket = privacy.fence.begin();
    if (!privacy.fence.current(ticket)) return;
    sending.current = true;
    setBusy(true);
    setError("");
    try {
      if (
        (await checkedBoundary(boundary)) !== boundary ||
        !privacy.fence.current(ticket)
      ) {
        expire();
        return;
      }
      if (!attempt.current.pending()) {
        const body = makeForBuyerBody(
          values,
          available,
          prefill,
          conversationId ? { conversationId } : { bundleId },
          send && cap.allowed,
        );
        if (!guard.current?.arm()) {
          setGuarded(true);
          return;
        }
        attempt.current.prepare(body);
      }
      const outcome = await createForBuyerOrder(
        store.id,
        attempt.current.pending()!,
        boundary,
        ticket.signal,
      );
      if (!privacy.fence.current(ticket)) return;
      if (outcome.ok) {
        const { buyer_link, ...safe } = outcome.value;
        privateLink.current = buyer_link;
        setResult(safe);
        setError("");
        setUncertain(false);
        attempt.current.finish();
        guard.current?.clear();
      } else {
        setError(outcome.code);
        if (outcome.code === "bundle_already_ordered") {
          setDuplicate({ id: outcome.order_id ?? null });
          attempt.current.finish();
          guard.current?.clear();
          setUncertain(false);
        } else if (retainOrderAttempt(outcome, uncertain)) {
          setUncertain(true);
        } else {
          attempt.current.finish();
          guard.current?.clear();
        }
        if ([401, 403].includes(outcome.status)) expire();
      }
    } catch (cause) {
      if (privacy.fence.current(ticket) && denied(cause)) {
        expire();
        return;
      }
      if (privacy.fence.current(ticket)) {
        setError(
          cause instanceof CreateOrderError ? cause.code : "unavailable",
        );
        if (attempt.current.pending()) setUncertain(true);
      }
    } finally {
      if (privacy.fence.current(ticket)) {
        sending.current = false;
        setBusy(false);
      }
    }
  }
  function newAttempt() {
    if (sending.current || result || duplicate || !attempt.current.startNew()) return;
    setError(""); setUncertain(false);
  }
  async function copy() {
    if (copying.current || !result || !privacy.visible) return;
    const ticket = privacy.fence.begin();
    copying.current = true;
    setCopyBusy(true);
    setCopyState("");
    try {
      const cookie = csrfCookie();
      if (
        (await checkedBoundary(boundary, cookie)) !== boundary ||
        !privacy.fence.current(ticket)
      )
        return;
      if (!privateLink.current) {
        regeneration.current ??= {
          key: crypto.randomUUID(),
          locale: values.buyerLocale,
        };
        const response = await regenerateManualLink(
          store.id,
          regeneration.current.key,
          { order_id: result.order_id, locale: regeneration.current.locale },
          boundary,
        );
        if (
          !privacy.fence.current(ticket) ||
          (await checkedBoundary(boundary, cookie)) !== boundary ||
          !privacy.fence.current(ticket)
        )
          return;
        if (!response.ok) {
          if ([401, 403].includes(response.status))
            throw new CreateOrderError("unauthorized", response.status);
          throw new Error("link unavailable");
        }
        privateLink.current = response.value.buyer_link;
      }
      if (!privacy.fence.current(ticket)) return;
      await checkedBoundary(boundary, cookie);
      if (!privacy.fence.current(ticket)) return;
      await navigator.clipboard.writeText(privateLink.current!);
      if (privacy.fence.current(ticket)) setCopyState(c.copied);
    } catch (cause) {
      if (privacy.fence.current(ticket) && denied(cause)) {
        expire();
        return;
      }
      if (privacy.fence.current(ticket)) setCopyState(c.copyFailed);
    } finally {
      if (privacy.fence.current(ticket)) {
        copying.current = false;
        setCopyBusy(false);
      }
    }
  }
  if (!privacy.visible) return null;
  const orderId = result?.order_id ?? duplicate?.id;
  const orderHref = `/${locale}/orders?store=${encodeURIComponent(store.id)}${orderId ? `&order=${encodeURIComponent(orderId)}` : ""}`;
  return (
    <dialog
      ref={dialog}
      className={styles.drawer}
      data-testid="create-order-drawer"
      aria-labelledby={heading}
      onCancel={(e) => {
        e.preventDefault();
        close();
      }}
    >
      <header className={styles.header}>
        <h2 id={heading}>{c.title}</h2>
        <button
          type="button"
          data-testid="drawer-close"
          disabled={busy || (pending && !attempt.current.resolved()) || copyBusy}
          onClick={close}
        >
          {c.close}
        </button>
      </header>
      {!canCreate ? (
        <p role="status">{c.permission}</p>
      ) : guarded ? (
        <p role="alert">
          {c.guard} <a href={orderHref}>{c.orders}</a>
        </p>
      ) : !prefill && !error ? (
        <p role="status">{c.loading}</p>
      ) : null}
      {error && (
        <p role="alert" data-testid="drawer-error">
          {uncertain ? c.uncertain : createOrderError(locale, error)}
        </p>
      )}
      {duplicate && <p>{c.duplicate}</p>}
      {orderId && (
        <a data-testid="drawer-view-order" href={orderHref}>
          {c.view}
        </a>
      )}
      {result ? (
        <section data-testid="drawer-result" aria-live="polite">
          <h3>{c.success}</h3>
          <p>{money(locale, result.currency, result.total_minor)}</p>
          <p>{c.expires} · {displayTime(locale, result.expires_at)}</p>
          <p data-testid="drawer-live-result">
            {result.live_price === "applied" ? c.applied : c.notApplied}{" "}
            {createOrderReason(locale, result.live_price_reason)}
          </p>
          <p data-testid="drawer-send-result">
            {result.send.state === "queued" ? c.queued : c.notSent}{" "}
            {result.send.state === "not_sent"
              ? createOrderReason(locale, result.send.reason)
              : ""}
          </p>
          {result.link_state === "configured" && (
            <button
              type="button"
              data-testid="drawer-copy-link"
              disabled={copyBusy}
              onClick={() => void copy()}
            >
              {c.copy}
            </button>
          )}
          {copyState && <p role="status">{copyState}</p>}
        </section>
      ) : (
        prefill &&
        !guarded &&
        !duplicate && (
          <form data-testid="drawer-form" onSubmit={submit}>
            {prefill.customer && (
              <p data-testid="drawer-linked-customer">{c.linked}</p>
            )}
            {(!prefill.live_price_eligible ||
              !permitted(store, "live:manage")) && (
              <p role="status" data-testid="drawer-live-warning">
                {c.liveWarning}{" "}
                {!permitted(store, "live:manage") ? c.livePermission : ""}
              </p>
            )}
            <p>{c.quote}</p>
            {prefill.items.length > 0 && (
              <ul aria-label={c.claims}>
                {prefill.items.map((line, index) => (
                  <li key={index} data-testid={`drawer-claim-${index}`}>
                    {line.name} · {line.variant} · {line.keyword}:{" "}
                    {c.claimRemaining(line.live_quantity_remaining)}
                    {!line.sellable ? ` · ${c.unavailableItem}` : ""}
                  </li>
                ))}
              </ul>
            )}
            {prefill.bundles.length > 5 && <p role="alert">{c.tooMany}</p>}
            <fieldset className={styles.fields} disabled={busy || pending}>
              <ManualOrderFormFields
                locale={locale}
                store={store}
                value={values}
                available={available}
                optionsReady={true}
                onChange={(patch) => {
                  if (!sending.current && !attempt.current.pending())
                    setValues((old) => ({ ...old, ...patch }));
                }}
                idPrefix="drawer"
                quantityControls="stepper"
              />
              <label className={styles.send}>
                <input
                  type="checkbox"
                  data-testid="drawer-send-payment-link"
                  checked={send}
                  disabled={!cap.allowed}
                  onChange={(e) => setSend(e.target.checked)}
                />
                {c.send}
              </label>
              {!cap.allowed && <p>{c.sendDisabled}</p>}
              {cap.review && <p>{c.review}</p>}
              {cap.closed && <p>{c.window}</p>}
            </fieldset>
            {problem && !pending && (
              <p role="status">{toolsCopy[locale].manual.problems[problem]}</p>
            )}
            <button
              type="submit"
              data-testid="drawer-submit"
              disabled={
                busy ||
                attempt.current.resolved() ||
                !boundary ||
                (!pending &&
                  (!!problem || mapOnly || prefill.bundles.length > 5))
              }
            >
              {uncertain ? c.retry : c.create}
            </button>
            {attempt.current.resolved() && <button type="button" data-testid="drawer-new-attempt" disabled={busy} onClick={newAttempt}>{c.newAttempt}</button>}
          </form>
        )
      )}
    </dialog>
  );
}
