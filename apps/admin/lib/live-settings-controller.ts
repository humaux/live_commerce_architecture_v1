// Purpose: W3-U2 private page lifetime, reads, pagination and immutable command receipts.
// Depends on: live-settings client/model, Studio reads, existing Inbox privacy fences and Next navigation.
// Used by: LiveSettings view; no network outside the client and no persisted private drafts.
"use client";
import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@live-commerce/i18n";
import type { Store } from "./model";
import {
  readStudioPage,
  StudioError,
  type StudioErrorCode,
} from "./studio-client";
import { csrfCookie, sessionBoundary } from "./settings-client";
import {
  readLiveSettings,
  writeLiveSettings,
  LiveSettingsError,
} from "./live-settings-client";
import { liveSettingsCopy } from "./live-settings-copy";
import {
  merchantTemplate,
  validSoldOutText,
  type SoldOutSettings,
  type SettingsReceipt,
  type TemplateReceipt,
  type ReminderReport,
  type ReminderResult,
  type RestrictedPage,
} from "./live-settings-model";
import { permitted } from "../src/features/messages/privacy";
import { useInboxPrivacy } from "../src/features/messages/use-privacy";
type SessionPage = Awaited<ReturnType<typeof readStudioPage>>;
/** Map only closed transport error codes. */
export const liveSettingsFailure = (cause: unknown) =>
  cause instanceof LiveSettingsError
    ? cause.code
    : cause instanceof StudioError
      ? cause.code === "signed-out"
        ? "unauthorized"
        : cause.code
      : "unavailable";
async function sessionStamp(cookie: string) {
  try {
    return await sessionBoundary(cookie);
  } catch {
    throw new LiveSettingsError("unauthorized", 401);
  }
}
/** Detect a revoked session/permission boundary. */
export const liveSettingsDenied = (cause: unknown) =>
  (cause instanceof LiveSettingsError && [401, 403].includes(cause.status)) ||
  (cause instanceof StudioError &&
    ["signed-out", "forbidden"].includes(cause.code));

/** Operate only explicit scoped commands; any uncertain write retains its exact receipt until retry or privacy teardown. */
export function useLiveSettingsController({
  locale,
  store,
  scene,
  initialError,
}: {
  locale: Locale;
  store: Store | null;
  scene: string;
  initialError: StudioErrorCode | null;
}) {
  const c = liveSettingsCopy(locale),
    router = useRouter(),
    storeID = store?.id ?? "";
  const [sessions, setSessions] = useState<SessionPage | null>(null),
    [settings, setSettings] = useState<SoldOutSettings | null>(null),
    [enabled, setEnabled] = useState(false),
    [draft, setDraft] = useState("");
  const [report, setReport] = useState<ReminderReport | null>(null),
    [result, setResult] = useState<ReminderResult | null>(null),
    [blocked, setBlocked] = useState<RestrictedPage | null>(null);
  const [cursor, setCursor] = useState(""),
    [history, setHistory] = useState<string[]>([]),
    [remove, setRemove] = useState<string | null>(null);
  const [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(true),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [revision, setRevision] = useState(0);
  const [pending, setPending] = useState<SettingsReceipt | null>(null),
    [unknown, setUnknown] = useState(false);
  const receipt = useRef<SettingsReceipt | null>(null),
    inFlight = useRef(false),
    pageLoad = useRef(0);
  const clear = useCallback(() => {
    setSessions(null);
    setSettings(null);
    setEnabled(false);
    setDraft("");
    setReport(null);
    setResult(null);
    setBlocked(null);
    setRemove(null);
    setError("");
    setNotice("");
    setBusy(false);
    setLoading(false);
    setUnknown(false);
    setPending(null);
    receipt.current = null;
    inFlight.current = false;
    pageLoad.current++;
  }, []);
  const privacy = useInboxPrivacy(clear),
    canRead = permitted(store, "live:read"),
    canManage = permitted(store, "live:manage"),
    canReport = permitted(store, "inbox:read"),
    canRemind = canManage && permitted(store, "inbox:reply");
  const fail = useCallback(
    (cause: unknown) => {
      if (liveSettingsDenied(cause)) {
        privacy.expire();
        return;
      }
      if (cause instanceof LiveSettingsError && cause.status === 404) {
        privacy.fence.invalidate(true);
        clear();
      }
      setError(liveSettingsFailure(cause));
    },
    [clear, privacy.expire],
  );
  useEffect(() => {
    clear();
    if (
      !canRead ||
      !privacy.visible ||
      privacy.blocked.current ||
      initialError ||
      !storeID
    )
      return;
    privacy.fence.invalidate(true);
    const ticket = privacy.fence.begin();
    setLoading(true);
    const current = () => privacy.fence.current(ticket);
    const accept = <T>(work: Promise<T>, set: (value: T) => void) =>
      work
        .then((v) => {
          if (current()) set(v);
        })
        .catch((e) => {
          if (current()) fail(e);
        });
    const sessionRead = async () => {
      const cookie = csrfCookie(),
        stamp = await sessionStamp(cookie);
      const value = await readStudioPage(storeID, "", ticket.signal);
      if ((await sessionStamp(cookie)) !== stamp)
        throw new LiveSettingsError("unauthorized", 401);
      return value;
    };
    const work: Promise<unknown>[] = [
      accept(sessionRead(), setSessions),
      accept(
        readLiveSettings<SoldOutSettings>(
          storeID,
          "live-settings/sold-out-reply",
          ticket.signal,
        ),
        (v) => {
          setSettings(v);
          setEnabled(v.enabled);
        },
      ),
    ];
    if (scene) {
      if (canReport)
        work.push(
          accept(
            readLiveSettings<ReminderReport>(
              storeID,
              `live-sessions/${scene}/reminders`,
              ticket.signal,
            ),
            setReport,
          ),
        );
      work.push(
        accept(
          readLiveSettings<RestrictedPage>(
            storeID,
            `live-sessions/${scene}/claims/blocklist?limit=20`,
            ticket.signal,
          ),
          (v) => {
            setBlocked(v);
            setCursor("");
            setHistory([]);
          },
        ),
      );
    }
    void Promise.all(work).finally(() => {
      if (current()) setLoading(false);
    });
    return () => privacy.fence.invalidate(false);
  }, [
    storeID,
    scene,
    canRead,
    canReport,
    initialError,
    privacy.visible,
    privacy.revision,
    privacy.blocked,
    privacy.fence,
    clear,
    fail,
    revision,
  ]);
  async function page(next: string, previous = false) {
    if (!scene || inFlight.current || receipt.current || !privacy.visible)
      return;
    const sequence = ++pageLoad.current,
      ticket = privacy.fence.begin();
    setLoading(true);
    setError("");
    try {
      const data = await readLiveSettings<RestrictedPage>(
        storeID,
        `live-sessions/${scene}/claims/blocklist?limit=20${next ? `&cursor=${encodeURIComponent(next)}` : ""}`,
        ticket.signal,
      );
      if (sequence !== pageLoad.current || !privacy.fence.current(ticket))
        return;
      setHistory((old) => (previous ? old.slice(0, -1) : [...old, cursor]));
      setCursor(next);
      setBlocked(data);
      setRemove(null);
    } catch (e) {
      if (sequence === pageLoad.current && privacy.fence.current(ticket))
        fail(e);
    } finally {
      if (sequence === pageLoad.current && privacy.fence.current(ticket))
        setLoading(false);
    }
  }
  async function run(first: SettingsReceipt) {
    // Disabled controls cannot revoke already captured callbacks: UNKNOWN admits only its retained receipt.
    if (inFlight.current || (receipt.current && receipt.current !== first) || !privacy.visible || privacy.blocked.current) return;
    inFlight.current = true;
    receipt.current = first;
    setPending(first);
    setBusy(true);
    setUnknown(false);
    setError("");
    setNotice("");
    const ticket = privacy.fence.begin();
    let action = first;
    try {
      for (;;) {
        const value = await writeLiveSettings<unknown>(
          storeID,
          action,
          ticket.signal,
        );
        if (!privacy.fence.current(ticket)) return;
        if (action.afterPublish) {
          const published = value as TemplateReceipt;
          if (!merchantTemplate(published.template_id))
            throw new LiveSettingsError("invalid_request", 422);
          action = {
            method: "PUT",
            resource: "live-settings/sold-out-reply",
            key: crypto.randomUUID(),
            body: JSON.stringify({
              ...action.afterPublish,
              template_id: published.template_id,
              template_version: published.version,
            }),
          };
          receipt.current = action;
          setPending(action);
          continue;
        }
        receipt.current = null;
        setPending(null);
        setUnknown(false);
        if (action.resource === "live-settings/sold-out-reply") {
          const saved = value as SoldOutSettings;
          setSettings(saved);
          setEnabled(saved.enabled);
          setDraft("");
          setNotice(c.saved);
        } else if (action.resource.endsWith("/reminders")) {
          setResult(value as ReminderResult);
          try {
            const fresh = await readLiveSettings<ReminderReport>(
              storeID,
              `live-sessions/${scene}/reminders`,
              ticket.signal,
            );
            if (privacy.fence.current(ticket)) setReport(fresh);
          } catch (e) {
            if (privacy.fence.current(ticket)) fail(e);
          }
        } else {
          setBlocked((old) =>
            old
              ? {
                  ...old,
                  items: old.items.filter(
                    (row) => !action.resource.endsWith(`/entries/${row.id}`),
                  ),
                }
              : old,
          );
          setRemove(null);
        }
        break;
      }
    } catch (e) {
      if (!privacy.fence.current(ticket)) return;
      if (e instanceof LiveSettingsError && e.status >= 500) {
        setUnknown(true);
        setError("");
      } else {
        receipt.current = null;
        setPending(null);
        if (
          e instanceof LiveSettingsError &&
          e.status === 409 &&
          action.resource === "live-settings/sold-out-reply"
        )
          setSettings(null);
        fail(e);
      }
    } finally {
      if (privacy.fence.current(ticket)) {
        inFlight.current = false;
        setBusy(false);
      }
    }
  }
  function save() {
    if (!settings || !canManage || receipt.current || inFlight.current) return;
    if (draft && !validSoldOutText(draft)) {
      setError("invalid_request");
      return;
    }
    const input = { enabled, expected_version: settings.version };
    if (draft) {
      void run({
        method: "POST",
        resource: "message-templates",
        key: crypto.randomUUID(),
        body: JSON.stringify({
          template_id: `merchant-sold-out-${crypto.randomUUID()}`,
          name: c.soldout,
          kinds: ["private_reply"],
          public_safe: false,
          body: draft,
        }),
        afterPublish: input,
      });
    } else if (merchantTemplate(settings.template_id)) {
      void run({
        method: "PUT",
        resource: "live-settings/sold-out-reply",
        key: crypto.randomUUID(),
        body: JSON.stringify({
          ...input,
          template_id: settings.template_id,
          template_version: settings.template_version,
        }),
      });
    }
  }
  async function copy(link: string) {
    const ticket = privacy.fence.begin();
    try {
      const cookie = csrfCookie();
      await sessionStamp(cookie);
      if (!privacy.fence.current(ticket)) return;
      await navigator.clipboard.writeText(link);
      await sessionStamp(cookie);
      if (privacy.fence.current(ticket)) setNotice(c.copied);
    } catch (e) {
      if (privacy.fence.current(ticket)) {
        if (liveSettingsDenied(e)) fail(e);
        else setNotice(c.copyFailed);
      }
    }
  }
  const unavailable = !!initialError || !canRead,
    locked = busy || !!pending || loading;
  return {
    c,
    router,
    storeID,
    sessions,
    settings,
    enabled,
    setEnabled,
    draft,
    setDraft,
    report,
    result,
    blocked,
    history,
    remove,
    setRemove,
    busy,
    loading,
    error,
    notice,
    unknown,
    receipt,
    canManage,
    canReport,
    canRemind,
    privacy,
    setRevision,
    run,
    page,
    save,
    copy,
    unavailable,
    locked,
  };
}
