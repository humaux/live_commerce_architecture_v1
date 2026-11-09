// Purpose: private member DTO validation, single-attempt keyed writes, fenced reads and bounded in-memory pagination.
// Depends on: LP1 BFF, settings-client session fence, customers-client guarded lifecycle and React.
// Used by: MemberList/MemberDetail; no PII is persisted, logged or placed in page URLs.
import { useEffect, useRef, useState } from "react";
import { csrfCookie, sessionBoundary } from "./settings-client";
import { ReadError, useGuardedRead, type ReadCode } from "./customers-client";
import { signalLogout } from "./session-events";
import { canonicalUUID } from "./orders-model";

export type MemberRow = {
  id: string;
  display_name: string;
  contact_note: string | null;
  balance: number;
  updated_at: string;
  erased: boolean;
};
export type Member = Omit<MemberRow, "erased"> & {
  notes: string | null;
  version: number;
  created_at: string;
  erased_at: string | null;
};
export type Entry = {
  id: string;
  delta: number;
  reason: "manual_order_award" | "manual_adjust";
  order_id: string | null;
  note: string | null;
  actor: { principal_id: string; display_name: string | null };
  created_at: string;
};
export type MemberListPage = {
  members: MemberRow[];
  next_cursor: string | null;
};
export type MemberDetailPage = {
  member: Member;
  entries: Entry[];
  next_ledger_cursor: string | null;
};
export class MemberReadError extends ReadError {
  constructor(readonly kind: "network" | "protocol") {
    super("unavailable");
  }
}
const invalid = (): never => {
  throw new MemberReadError("protocol");
};
const obj = (v: unknown): Record<string, unknown> =>
  v && typeof v === "object" && !Array.isArray(v)
    ? (v as Record<string, unknown>)
    : invalid();
const str = (v: unknown, max = 10000): string =>
  typeof v === "string" && v.length <= max ? v : invalid();
const nullable = (v: unknown, max = 10000): string | null =>
  v === null ? null : str(v, max);
const id = (v: unknown): string =>
  typeof v === "string" && canonicalUUID.test(v) ? v : invalid();
const num = (v: unknown, min: number, max: number): number =>
  typeof v === "number" && Number.isSafeInteger(v) && v >= min && v <= max
    ? v
    : invalid();
const date = (v: unknown): string =>
  typeof v === "string" && Number.isFinite(Date.parse(v)) ? v : invalid();
const cursor = (v: unknown): string | null =>
  v === null ? null : str(v, 1024) || invalid();
function rows<T>(
  v: unknown,
  max: number,
  parse: (v: unknown) => T & { id: string },
): T[] {
  if (!Array.isArray(v) || v.length > max) return invalid();
  const result = v.map(parse);
  if (new Set(result.map((r) => r.id)).size !== result.length) return invalid();
  return result;
}
function row(v: unknown): MemberRow {
  const r = obj(v);
  return {
    id: id(r.id),
    display_name: str(r.display_name),
    contact_note: nullable(r.contact_note),
    balance: num(r.balance, 0, 1e12),
    updated_at: date(r.updated_at),
    erased: typeof r.erased === "boolean" ? r.erased : invalid(),
  };
}
export function parseMembers(v: unknown): MemberListPage {
  const p = obj(v);
  return {
    members: rows(p.members, 20, row),
    next_cursor: cursor(p.next_cursor),
  };
}
export function parseMember(v: unknown, expected: string): MemberDetailPage {
  const p = obj(v),
    m = obj(p.member);
  if (id(m.id) !== expected) return invalid();
  const member: Member = {
    id: id(m.id),
    display_name: str(m.display_name),
    contact_note: nullable(m.contact_note),
    balance: num(m.balance, 0, 1e12),
    updated_at: date(m.updated_at),
    notes: nullable(m.notes),
    version: num(m.version, 1, Number.MAX_SAFE_INTEGER),
    created_at: date(m.created_at),
    erased_at: m.erased_at === null ? null : date(m.erased_at),
  };
  const entries = rows(p.entries, 50, (v) => {
    const e = obj(v),
      a = obj(e.actor);
    return {
      id: id(e.id),
      delta: num(e.delta, -1e6, 1e6),
      reason:
        e.reason === "manual_adjust" || e.reason === "manual_order_award"
          ? e.reason
          : invalid(),
      order_id: e.order_id === null ? null : id(e.order_id),
      note: nullable(e.note),
      actor: {
        principal_id: id(a.principal_id),
        display_name: nullable(a.display_name),
      },
      created_at: date(e.created_at),
    } satisfies Entry;
  });
  return { member, entries, next_ledger_cursor: cursor(p.next_ledger_cursor) };
}
export function memberQuery(value: string): string | null {
  const q = value.trim();
  if (Array.from(q).length > 60 || /[\p{C}\p{Zl}\p{Zp}]/u.test(q))
    return invalid();
  return q || null;
}
async function read<T>(
  path: string,
  signal: AbortSignal,
  parse: (v: unknown) => T,
  body?: string,
): Promise<T> {
  const csrf = csrfCookie();
  let before: string;
  try {
    before = await sessionBoundary(csrf);
  } catch {
    throw new ReadError("signed-out");
  }
  if (signal.aborted) throw new MemberReadError("network");
  let response: Response;
  try {
    response = await fetch(path, {
      method: body === undefined ? "GET" : "POST",
      credentials: "same-origin",
      cache: "no-store",
      signal: AbortSignal.any([signal, AbortSignal.timeout(15000)]),
      ...(body === undefined
        ? {}
        : {
            body,
            headers: {
              "Content-Type": "application/json",
              "X-CSRF-Token": csrf,
            },
          }),
    });
  } catch {
    throw new MemberReadError("network");
  }
  if (response.status === 401) {
    signalLogout();
    throw new ReadError("signed-out");
  }
  if (response.status === 403) throw new ReadError("forbidden");
  if (response.status === 404) throw new ReadError("not-found");
  if (!response.ok)
    throw new MemberReadError(response.status >= 500 ? "network" : "protocol");
  const cache = response.headers
    .get("cache-control")
    ?.split(",")
    .map((v) => v.trim());
  if (
    response.headers.get("content-type")?.split(";")[0] !==
      "application/json" ||
    !cache?.includes("private") ||
    !cache.includes("no-store")
  )
    return invalid();
  let value: T;
  try {
    const reader = response.body?.getReader();
    if (!reader) return invalid();
    const chunks: Uint8Array[] = [];
    let size = 0;
    try {
      for (;;) {
        const part = await reader.read();
        if (part.done) break;
        size += part.value.byteLength;
        if (size > 1048576) {
          await reader.cancel();
          return invalid();
        }
        chunks.push(part.value);
      }
    } finally {
      reader.releaseLock();
    }
    const bytes = new Uint8Array(size);
    let offset = 0;
    for (const part of chunks) {
      bytes.set(part, offset);
      offset += part.length;
    }
    value = parse(
      JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes)),
    );
  } catch {
    return invalid();
  }
  try {
    if (signal.aborted || (await sessionBoundary()) !== before)
      throw new Error();
  } catch {
    throw new ReadError("signed-out");
  }
  return value;
}
export function searchMembers(
  store: string,
  q: string,
  after: string | null,
  signal: AbortSignal,
) {
  id(store);
  if (after !== null) cursor(after);
  return read(
    `/api/stores/${store}/members/search`,
    signal,
    parseMembers,
    JSON.stringify({
      q: memberQuery(q),
      include_erased: false,
      cursor: after,
      limit: 20,
    }),
  );
}
export function readMember(
  store: string,
  member: string,
  after: string | null,
  signal: AbortSignal,
) {
  id(store);
  id(member);
  if (after !== null) cursor(after);
  return read(
    `/api/stores/${store}/members/${member}${after === null ? "" : `?ledger_cursor=${encodeURIComponent(after)}`}`,
    signal,
    (v) => parseMember(v, member),
  );
}

// Guarded head read owns logout/visibility; extra pages share its boundary and are never exposed across head/scope changes.
export function useMemberPages<T>(
  scope: string,
  loader: ((cursor: string | null, signal: AbortSignal) => Promise<T>) | null,
  initialError: ReadCode | null,
  next: (page: T) => string | null,
  merge: (old: T, page: T) => T,
) {
  const [kind, setKind] = useState<"network" | "protocol">("network");
  const guarded = useGuardedRead(
    scope,
    loader
      ? async (signal) => {
          try {
            return await loader(null, signal);
          } catch (e) {
            if (!signal.aborted && e instanceof MemberReadError)
              setKind(e.kind);
            throw e;
          }
        }
      : null,
    initialError,
  );
  const [extra, setExtra] = useState<{
    scope: string;
    head: T;
    data: T;
  } | null>(null);
  const [paging, setPaging] = useState(false),
    [pageError, setPageError] = useState<"network" | "protocol" | null>(null);
  const active = useRef<AbortController | null>(null),
    epoch = useRef(0),
    consumed = useRef(new Set<string>());
  const liveScope = useRef(scope);
  liveScope.current = scope;
  const head = guarded.data;
  const data =
    guarded.status === "ready" && head
      ? extra?.scope === scope && extra.head === head
        ? extra.data
        : head
      : null;
  useEffect(() => {
    epoch.current++;
    active.current?.abort();
    active.current = null;
    consumed.current.clear();
    setExtra(null);
    setPaging(false);
    setPageError(null);
    return () => {
      epoch.current++;
      active.current?.abort();
    };
  }, [scope, head, guarded.status]);
  async function more() {
    if (!data || !loader || active.current || guarded.status !== "ready")
      return;
    const after = next(data);
    if (after === null || consumed.current.has(after)) return;
    const controller = new AbortController();
    active.current = controller;
    const generation = epoch.current,
      boundary = guarded.boundary;
    const live = () =>
      !controller.signal.aborted &&
      epoch.current === generation &&
      liveScope.current === scope;
    setPaging(true);
    setPageError(null);
    try {
      try {
        if ((await sessionBoundary()) !== boundary) throw new Error();
      } catch {
        throw new ReadError("signed-out");
      }
      const page = await loader(after, controller.signal);
      if (!live()) return;
      try {
        if ((await sessionBoundary()) !== boundary) throw new Error();
      } catch {
        throw new ReadError("signed-out");
      }
      if (!live()) return;
      if (
        next(page) === after ||
        (next(page) !== null && consumed.current.has(next(page)!))
      )
        return invalid();
      consumed.current.add(after);
      setExtra({ scope, head: head!, data: merge(data, page) });
    } catch (e) {
      if (!live()) return;
      if (e instanceof ReadError && e.code !== "unavailable") {
        if (e.code === "signed-out") signalLogout();
        else guarded.reload();
        setExtra(null);
      } else setPageError(e instanceof MemberReadError ? e.kind : "network");
    } finally {
      if (live()) {
        active.current = null;
        setPaging(false);
      }
    }
  }
  return { ...guarded, data, kind, more, paging, pageError };
}
export function appendMembers(
  old: MemberListPage,
  page: MemberListPage,
): MemberListPage {
  return {
    ...page,
    members: [
      ...old.members,
      ...page.members.filter((r) => !old.members.some((o) => o.id === r.id)),
    ],
  };
}
export function appendEntries(
  old: MemberDetailPage,
  page: MemberDetailPage,
): MemberDetailPage {
  return {
    ...page,
    entries: [
      ...old.entries,
      ...page.entries.filter((r) => !old.entries.some((o) => o.id === r.id)),
    ],
  };
}

export type CreateMemberInput = {
  display_name: string;
  contact_note: string | null;
  notes: string | null;
};
export type AdjustPointsInput = {
  delta: number;
  note: string;
  order_id?: string | null;
};
/** Go loyalty.Entry, distinct from the enriched detail-read Entry. */
export type AdjustmentEntry = {
  id: string;
  member_id: string;
  delta: number;
  reason: "manual_adjust";
  order_id: string | null;
  note: string | null;
  actor_principal_id: string;
  created_at: string;
};
export type AdjustPointsResult = { entry: AdjustmentEntry; balance: number };
export type MemberWriteCode =
  | "invalid_request"
  | "signed-out"
  | "forbidden"
  | "not-found"
  | "points_insufficient"
  | "member_erased"
  | "idempotency_conflict"
  | "conflict"
  | "limit_reached"
  | "unknown";
/** UNKNOWN after dispatch is unresolved: refresh may read current state but MUST NOT resend. */
export class MemberWriteError extends Error {
  constructor(
    readonly code: MemberWriteCode,
    readonly outcome: "not-sent" | "rejected" | "unknown",
  ) {
    super(code);
  }
}
const invalidWrite = (): never => {
  throw new MemberWriteError("invalid_request", "not-sent");
};
function writeText(value: unknown, max: number, required = false): string {
  if (
    typeof value !== "string" ||
    /\p{Cs}/u.test(value) ||
    Array.from(value).length > max ||
    (required && (!value.trim() || /[\p{Cc}\p{Cf}]/u.test(value)))
  )
    return invalidWrite();
  return value;
}
function writeID(value: string): string {
  return typeof value === "string" && canonicalUUID.test(value)
    ? value
    : invalidWrite();
}
function writeKey(value: string): string {
  return typeof value === "string" && /^[A-Za-z0-9_.:-]{8,128}$/.test(value)
    ? value
    : invalidWrite();
}
/** Snapshot all caller-owned values synchronously, before session hashing yields. */
export function createMember(
  store: string,
  input: CreateMemberInput,
  key: string,
  signal: AbortSignal,
): Promise<{ member: Member }> {
  const path = `/api/stores/${writeID(store)}/members`,
    stableKey = writeKey(key);
  const body = JSON.stringify({
    display_name: writeText(
      writeText(input.display_name, 10000)
        .replace(
          /^[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+|[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+$/gu,
          "",
        )
        .normalize("NFC"),
      60,
      true,
    ),
    contact_note:
      input.contact_note === null ? null : writeText(input.contact_note, 120),
    notes: input.notes === null ? null : writeText(input.notes, 1000),
  });
  return writeMemberCommand(path, body, stableKey, signal, (v) => {
    const m = obj(obj(v).member);
    return {
      member: parseMember(
        { member: m, entries: [], next_ledger_cursor: null },
        id(m.id),
      ).member,
    };
  });
}
export function adjustPoints(
  store: string,
  member: string,
  input: AdjustPointsInput,
  key: string,
  signal: AbortSignal,
): Promise<AdjustPointsResult> {
  const target = writeID(member),
    path = `/api/stores/${writeID(store)}/members/${target}/adjustments`,
    stableKey = writeKey(key);
  const delta = input.delta,
    note = writeText(input.note, 200, true),
    order = input.order_id;
  if (!Number.isSafeInteger(delta) || delta === 0 || Math.abs(delta) > 1e6)
    return invalidWrite();
  const body = JSON.stringify({
    delta,
    note,
    order_id: order == null ? null : writeID(order),
  });
  return writeMemberCommand(path, body, stableKey, signal, (v) => {
    const p = obj(v),
      e = obj(p.entry);
    if (id(e.member_id) !== target || e.reason !== "manual_adjust")
      return invalid();
    const entry: AdjustmentEntry = {
      id: id(e.id),
      member_id: id(e.member_id),
      delta: num(e.delta, -1e6, 1e6),
      reason: "manual_adjust",
      order_id: e.order_id === null ? null : id(e.order_id),
      note: nullable(e.note),
      actor_principal_id: id(e.actor_principal_id),
      created_at: date(e.created_at),
    };
    if (entry.delta !== delta || entry.order_id !== (order ?? null))
      return invalid();
    return { entry, balance: num(p.balance, 0, 1e12) };
  });
}
/** One POST, no receipt inference/replay/storage; all awaits share one 15s deadline and caller scope signal. */
async function writeMemberCommand<T>(
  path: string,
  body: string,
  key: string,
  signal: AbortSignal,
  parse: (v: unknown) => T,
): Promise<T> {
  if (new TextEncoder().encode(body).length > 16384) return invalidWrite();
  const csrf = csrfCookie(),
    controller = new AbortController();
  const deadline = setTimeout(() => controller.abort(), 15000);
  const fence = AbortSignal.any([signal, controller.signal]);
  let dispatched = false,
    before = "";
  const bounded = <V>(promise: Promise<V>): Promise<V> =>
    new Promise((resolve, reject) => {
      const cleanup = () => fence.removeEventListener("abort", abort);
      const abort = () => {
        cleanup();
        reject(new Error("aborted"));
      };
      fence.addEventListener("abort", abort, { once: true });
      if (fence.aborted) abort();
      // Crypto cannot be canceled; detach now and ignore any late settlement.
      promise.then(
        (value) => {
          cleanup();
          resolve(value);
        },
        (error) => {
          cleanup();
          reject(error);
        },
      );
    });
  const sameSession = async () => {
    try {
      return (
        !!before &&
        csrfCookie() === csrf &&
        (await bounded(sessionBoundary(csrf))) === before &&
        csrfCookie() === csrf
      );
    } catch (error) {
      // An unchanged session canceled mid-hash has an unknown command outcome.
      if (fence.aborted && csrfCookie() === csrf) throw error;
      return false;
    }
  };
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;
  try {
    try {
      before = await bounded(sessionBoundary(csrf));
    } catch {
      if (fence.aborted && csrf && csrfCookie() === csrf) return invalidWrite();
      signalLogout();
      throw new MemberWriteError("signed-out", "not-sent");
    }
    if (fence.aborted)
      throw new MemberWriteError("invalid_request", "not-sent");
    dispatched = true;
    const response = await bounded(
      fetch(path, {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        signal: fence,
        body,
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrf,
          "Idempotency-Key": key,
        },
      }),
    );
    if (response.status === 401) {
      signalLogout();
      throw new MemberWriteError("signed-out", "rejected");
    }
    if (!(await sameSession())) {
      signalLogout();
      throw new MemberWriteError("signed-out", "unknown");
    }
    if (fence.aborted) throw new Error();
    if (response.status === 403)
      throw new MemberWriteError("forbidden", "rejected");
    if (response.status === 404)
      throw new MemberWriteError("not-found", "rejected");
    const cache = response.headers
      .get("cache-control")
      ?.split(",")
      .map((v) => v.trim());
    if (
      response.headers.get("content-type")?.split(";")[0] !==
        "application/json" ||
      !cache?.includes("no-store") ||
      (response.ok && !cache.includes("private"))
    )
      throw new Error();
    reader = response.body?.getReader();
    if (!reader) throw new Error();
    const chunks: Uint8Array[] = [];
    let size = 0;
    for (;;) {
      const part = await bounded(reader.read());
      if (part.done) break;
      size += part.value.byteLength;
      if (size > 1048576) throw new Error();
      chunks.push(part.value);
    }
    const bytes = new Uint8Array(size);
    let offset = 0;
    for (const part of chunks) {
      bytes.set(part, offset);
      offset += part.length;
    }
    const raw: unknown = JSON.parse(
      new TextDecoder("utf-8", { fatal: true }).decode(bytes),
    );
    if (!(await sameSession())) {
      signalLogout();
      throw new MemberWriteError("signed-out", "unknown");
    }
    if (fence.aborted) throw new Error();
    if (response.status === 409 || response.status === 422) {
      const code = obj(raw).code;
      if (
        (response.status === 409 &&
          [
            "points_insufficient",
            "member_erased",
            "idempotency_conflict",
            "conflict",
          ].includes(String(code))) ||
        (response.status === 422 &&
          ["invalid_request", "limit_reached"].includes(String(code)))
      )
        throw new MemberWriteError(code as MemberWriteCode, "rejected");
    }
    if (response.status !== 200 && response.status !== 201) throw new Error();
    return parse(raw);
  } catch (error) {
    if (error instanceof MemberWriteError) throw error;
    const changed = csrfCookie() !== csrf;
    if (changed) signalLogout();
    throw new MemberWriteError(
      changed ? "signed-out" : "unknown",
      dispatched ? "unknown" : "not-sent",
    );
  } finally {
    clearTimeout(deadline);
    // Do not await cancellation: a hostile/stalled body must not hold the deadline open.
    if (reader) {
      void reader.cancel().catch(() => {});
      reader.releaseLock();
    }
  }
}
