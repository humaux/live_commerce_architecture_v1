// Storefront-design requests of the admin Design page. Browser -> BFF `/api/stores/{store}/design/*` (allowlist in
// lib/design-request.ts) -> Go internal/httpapi/design.go (integration:read / integration:manage). Reads parse the closed Go
// shapes (design-model.ts); every write carries the cookie CSRF + the session fence of useGuardedRead (a changed session
// sends nothing), a fresh Idempotency-Key (the BFF requires one; the Go writes are compare-and-set on versions) and never
// trusts its own answer for state: the caller applies the parsed response or re-reads. A 422 carries the server's field
// `path` so the editor can say which field to fix. The upload is a single multipart `file` part; Go sniffs the type.
import { get } from "./customers-client";
import { csrfCookie, safeError, sessionBoundary } from "./settings-client";
import {
  parseDraft, parseMedia, parseMediaList, parsePreviewToken, parseVersionInfo, parseVersions,
  type Draft, type MediaItem, type PreviewToken, type VersionInfo, type VersionList,
} from "./design-model";

const base = (store: string) => `/api/stores/${store}/design`;
export const mediaURL = (store: string, id: string) => `${base(store)}/media/${id}`;

export type Outcome<T> = { ok: true; value: T } | { ok: false; code: string; path: string; reason: string; uncertain: boolean };

// Go GET design/draft (integration:read): version 0 = never saved (the derived default).
export const readDraft = (store: string, signal: AbortSignal): Promise<Draft> => get(`${base(store)}/draft`, parseDraft, signal);
// Go GET design/versions (integration:read): newest first, live_version = the highest.
export const readVersions = (store: string, signal: AbortSignal): Promise<VersionList> => get(`${base(store)}/versions`, parseVersions, signal);
// Go GET design/media (integration:read).
export const readMedia = (store: string, signal: AbortSignal): Promise<MediaItem[]> => get(`${base(store)}/media`, parseMediaList, signal);

async function send<T>(
  store: string, method: "POST" | "PUT", resource: string, body: BodyInit | undefined, json: boolean, boundary: string,
  parse: (value: unknown) => T,
): Promise<Outcome<T>> {
  const fail = (code: string, uncertain: boolean, path = "", reason = ""): Outcome<T> => ({ ok: false, code, path, reason, uncertain });
  const csrf = csrfCookie();
  try {
    if (!csrf || (await sessionBoundary(csrf)) !== boundary || csrfCookie() !== csrf) return fail("unauthorized", false);
  } catch {
    return fail("unauthorized", false);
  }
  const headers: Record<string, string> = { "X-CSRF-Token": csrf, "Idempotency-Key": crypto.randomUUID() };
  if (json) headers["Content-Type"] = "application/json"; // multipart: the browser sets the boundary itself
  let response: Response;
  try {
    response = await fetch(`${base(store)}/${resource}`, { method, credentials: "same-origin", cache: "no-store", headers, body, signal: AbortSignal.timeout(20000) });
  } catch {
    return fail("retry_later", true);
  }
  const value: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    const details = value && typeof value === "object" ? (value as { details?: Record<string, unknown> }).details : undefined;
    const path = typeof details?.path === "string" ? details.path : "";
    const reason = typeof details?.reason === "string" ? details.reason : "";
    return fail(safeError(value).code, response.status >= 500, path, reason);
  }
  try {
    return { ok: true, value: parse(value) };
  } catch {
    return fail("retry_later", true); // committed but unreadable: the caller re-reads before retrying
  }
}

const jsonBody = (value: unknown) => JSON.stringify(value);

// Go PUT design/draft {expected_version, document}: compare-and-set; 409 conflict = someone else saved; 422 has details.path.
export const saveDraft = (store: string, expectedVersion: number, document: unknown, boundary: string) =>
  send(store, "PUT", "draft", jsonBody({ expected_version: expectedVersion, document }), true, boundary, parseDraft);
// Go POST design/publish {expected_draft_version}: 409 when the draft moved or is already live.
export const publish = (store: string, expectedDraftVersion: number, boundary: string): Promise<Outcome<VersionInfo>> =>
  send(store, "POST", "publish", jsonBody({ expected_draft_version: expectedDraftVersion }), true, boundary, parseVersionInfo);
// Go POST design/rollback {version}: publishes a copy of an older version; 409 when it is already live.
export const rollback = (store: string, version: number, boundary: string): Promise<Outcome<VersionInfo>> =>
  send(store, "POST", "rollback", jsonBody({ version }), true, boundary, parseVersionInfo);
// Go POST design/preview-token {}: 15-minute token bound to the current draft version; shown once, never stored.
export const issuePreview = (store: string, boundary: string): Promise<Outcome<PreviewToken>> =>
  send(store, "POST", "preview-token", "{}", true, boundary, parsePreviewToken);
// Go POST design/media (multipart `file`): identical bytes return the existing item; 409 at 60 images.
export function uploadMedia(store: string, file: File, boundary: string): Promise<Outcome<MediaItem>> {
  const form = new FormData();
  form.append("file", file);
  return send(store, "POST", "media", form, false, boundary, parseMedia);
}
// Go POST design/media/{id}/delete: 409 while the draft or the live version still uses the image. Returns the list.
export const deleteMedia = (store: string, id: string, boundary: string): Promise<Outcome<MediaItem[]>> =>
  send(store, "POST", `media/${id}/delete`, "{}", true, boundary, parseMediaList);

export const MAX_MEDIA_BYTES = 2 * 1024 * 1024;
export const MEDIA_ACCEPT = "image/jpeg,image/png,image/webp";
export const mediaFileProblem = (file: { type: string; size: number }): "type" | "size" | null =>
  !MEDIA_ACCEPT.split(",").includes(file.type) ? "type" : file.size < 1 || file.size > MAX_MEDIA_BYTES ? "size" : null;
