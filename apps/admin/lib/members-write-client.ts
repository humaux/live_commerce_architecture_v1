// Purpose: single-attempt member command transport with bounded private response handling.
// Depends on: settings-client CSRF/session fences, session-events and native Fetch/AbortController.
// Used by: members-client stateless create/update/adjust/export/erase methods; no React or member DTO dependency.
// Invariants: caller key retained, no automatic replay/storage/logging, one 15s deadline and 1MiB response bound.
import { csrfCookie, sessionBoundary } from "./settings-client";
import { signalLogout } from "./session-events";

export type MemberWriteCode =
  | "invalid_request"
  | "signed-out"
  | "forbidden"
  | "not-found"
  | "reauth_required"
  | "export_too_large"
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
export const invalidWrite = (): never => {
  throw new MemberWriteError("invalid_request", "not-sent");
};
/** Decode only refusal envelopes; successful private DTO validation belongs to the caller. */
function errorObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw new Error("invalid refusal");
  return value as Record<string, unknown>;
}
/** One keyed command, no receipt inference/replay/storage; all awaits share one 15s deadline and caller scope signal. */
export async function writeMemberCommand<T>(
  path: string,
  body: string,
  key: string,
  signal: AbortSignal,
  parse: (v: unknown) => T,
  method: "POST" | "PATCH" = "POST",
  privacy = false,
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
        method,
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
    if (response.status === 403 && !privacy)
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
    if (response.status === 403)
      throw new MemberWriteError(
        errorObject(raw).code === "reauth_required"
          ? "reauth_required"
          : "forbidden",
        "rejected",
      );
    if (privacy && response.ok && response.status !== 200) throw new Error();
    if (response.status === 409 || response.status === 422) {
      const code = errorObject(raw).code;
      if (
        (response.status === 409 &&
          [
            "export_too_large",
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
