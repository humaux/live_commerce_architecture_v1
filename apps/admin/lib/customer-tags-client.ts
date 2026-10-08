// Purpose: fenced W6-01B reads and keyed writes for customer tags and private notes.
// Depends on: settings-client CSRF/session fence; customer-tags-model DTOs; exact tag/note BFF leaf routes.
// Used by: CustomerTags and focused client tests; no note body/key is persisted or logged.
import { csrfCookie, safeError, sessionBoundary } from "./settings-client.ts";
import { get } from "./customers-client";
import { parseTagCatalog, type TagCatalog } from "./customer-tags-model.ts";

/** Read the store catalogue through the existing guarded-page ReadError transport. */
export const readTagCatalog = (store: string, signal: AbortSignal): Promise<TagCatalog> =>
  get(`/api/stores/${store}/customers/tags`, parseTagCatalog, signal);

/** A single in-memory command; UNKNOWN retries must keep this exact key, scope and payload. */
export type TagCommand = { method: "POST" | "PATCH" | "DELETE" | "PUT"; resource: string; body?: string; key: string };
/** Per-attempt result; a settled retry refusal cannot resolve an earlier UNKNOWN command. */
export type TagOutcome<T> = { ok: true; value: T } | { ok: false; code: string; uncertain: boolean };

async function sameSession(boundary: string, csrf: string) {
  try { return !!csrf && csrfCookie() === csrf && await sessionBoundary(csrf) === boundary && csrfCookie() === csrf; }
  catch { return false; }
}

/** Read private tag/note JSON, rejecting a session switch before or after all awaits. */
export async function readTagData<T>(store: string, resource: string, boundary: string, parse: (v: unknown) => T, signal?: AbortSignal): Promise<T> {
  const csrf = csrfCookie();
  if (!await sameSession(boundary, csrf)) throw new Error("unauthorized");
  const response = await fetch(`/api/stores/${store}/${resource}`, {
    credentials: "same-origin", cache: "no-store", signal: signal ?? AbortSignal.timeout(12000),
  });
  if (!await sameSession(boundary, csrf)) throw new Error("unauthorized");
  if (!response.ok) {
    const error = safeError(await response.json().catch(() => null));
    if (!await sameSession(boundary, csrf)) throw new Error("unauthorized");
    throw new Error(error.code);
  }
  if (response.headers.get("content-type")?.split(";", 1)[0] !== "application/json" ||
    !response.headers.get("cache-control")?.split(",").some((s) => s.trim() === "no-store")) throw new Error("retry_later");
  const value = parse(await response.json());
  if (!await sameSession(boundary, csrf)) throw new Error("unauthorized");
  return value;
}

/** Send once; an uncertain result permits only an explicit same-command retry by the user. */
export async function sendTagCommand<T>(store: string, command: TagCommand, boundary: string, parse: (v: unknown) => T): Promise<TagOutcome<T>> {
  const csrf = csrfCookie();
  if (!await sameSession(boundary, csrf)) return { ok: false, code: "unauthorized", uncertain: false };
  try {
    const headers: Record<string, string> = { "X-CSRF-Token": csrf, "Idempotency-Key": command.key };
    if (command.body !== undefined) headers["Content-Type"] = "application/json";
    const response = await fetch(`/api/stores/${store}/${command.resource}`, {
      method: command.method, credentials: "same-origin", cache: "no-store", headers,
      ...(command.body === undefined ? {} : { body: command.body }), signal: AbortSignal.timeout(12000),
    });
    // Dispatch occurred: loss of the response's session fence cannot prove that the write had no effect.
    if (!await sameSession(boundary, csrf)) return { ok: false, code: "unauthorized", uncertain: true };
    if (!response.ok) {
      const error = safeError(await response.json().catch(() => null));
      if (!await sameSession(boundary, csrf)) return { ok: false, code: "unauthorized", uncertain: true };
      return { ok: false, code: error.code, uncertain: response.status >= 500 };
    }
    if (response.headers.get("content-type")?.split(";", 1)[0] !== "application/json" ||
      !response.headers.get("cache-control")?.split(",").some((s) => s.trim() === "no-store"))
      return { ok: false, code: "retry_later", uncertain: true };
    const value = parse(await response.json());
    if (!await sameSession(boundary, csrf)) return { ok: false, code: "unauthorized", uncertain: true };
    return { ok: true, value };
  } catch {
    if (!await sameSession(boundary, csrf)) return { ok: false, code: "unauthorized", uncertain: true };
    return { ok: false, code: "retry_later", uncertain: true };
  }
}
