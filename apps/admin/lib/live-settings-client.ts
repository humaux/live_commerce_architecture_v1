// Purpose: private W3-U2 reads and immutable, explicitly retried command receipts.
// Depends on: shared CSRF/session fences and live-settings-model; browser fetch only.
// Used by: LiveSettings and BuyerPanel restriction controls; never logs or persists private values.
import { csrfCookie, sessionBoundary } from "./settings-client";
import {
  settingsBody,
  settingsCodes,
  settingsData,
  settingsQuery,
  settingsResource,
  settingsUUID,
  type SettingsReceipt,
} from "./live-settings-model";
/** A fixed code carries failure/UNKNOWN state, without raw HTTP diagnostics. */
export class LiveSettingsError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
  ) {
    super(code);
  }
}
async function boundary(cookie: string) {
  try {
    return await sessionBoundary(cookie);
  } catch {
    throw new LiveSettingsError("unauthorized", 401);
  }
}
async function call<T>(
  store: string,
  resource: string,
  signal: AbortSignal,
  receipt?: SettingsReceipt,
): Promise<T> {
  const [path] = resource.split("?", 1),
    route = settingsResource(path),
    method = receipt?.method ?? "GET";
  const url = new URL(`https://local.invalid/${resource}`);
  if (
    !settingsUUID.test(store) ||
    !route ||
    !route.methods.includes(method) ||
    !settingsQuery(route.kind, method, url)
  )
    throw new LiveSettingsError("invalid_request", 422);
  if (
    receipt &&
    (!/^[A-Za-z0-9_.:-]{8,128}$/.test(receipt.key) ||
      (receipt.body !== undefined && !settingsBody(route.kind, receipt.body)))
  )
    throw new LiveSettingsError("invalid_request", 422);
  const cookie = csrfCookie(),
    before = await boundary(cookie);
  signal.throwIfAborted();
  let response: Response;
  try {
    response = await fetch(`/api/stores/${store}/${resource}`, {
      method,
      credentials: "same-origin",
      cache: "no-store",
      redirect: "error",
      referrerPolicy: "no-referrer",
      signal: AbortSignal.any([
        signal,
        AbortSignal.timeout(
          receipt && route.kind === "reminders" ? 70000 : 10000,
        ),
      ]),
      headers: {
        "X-CSRF-Token": cookie,
        ...(receipt ? { "Idempotency-Key": receipt.key } : {}),
        ...(receipt?.body !== undefined
          ? { "Content-Type": "application/json" }
          : {}),
      },
      ...(receipt?.body === undefined ? {} : { body: receipt.body }),
    });
  } catch {
    signal.throwIfAborted();
    throw new LiveSettingsError("retry_later", 503);
  }
  if (response.status === 401 || response.status === 403)
    throw new LiveSettingsError(
      response.status === 401 ? "unauthorized" : "forbidden",
      response.status,
    );
  let value: unknown;
  try {
    if (
      !response.headers.get("cache-control")?.includes("no-store") ||
      response.headers.get("content-type")?.split(";", 1)[0] !==
        "application/json"
    )
      throw Error();
    value = await response.json();
  } catch {
    throw new LiveSettingsError("retry_later", 503);
  }
  if ((await boundary(cookie)) !== before)
    throw new LiveSettingsError("unauthorized", 401);
  signal.throwIfAborted();
  if (!response.ok) {
    const c =
      value && typeof value === "object" && "code" in value ? value.code : null;
    throw new LiveSettingsError(
      typeof c === "string" && settingsCodes.has(c) ? c : "retry_later",
      response.status,
    );
  }
  try {
    return settingsData(route.kind, method, value) as T;
  } catch {
    throw new LiveSettingsError("retry_later", 503);
  }
}
/** Read once within the caller's visibility/store scope; no polling or response caching. */
export function readLiveSettings<T>(
  store: string,
  resource: string,
  signal: AbortSignal,
) {
  return call<T>(store, resource, signal);
}
/** Replay the caller's exact frozen key/body; never generate a replacement key or retry automatically. */
export function writeLiveSettings<T>(
  store: string,
  receipt: SettingsReceipt,
  signal: AbortSignal,
) {
  return call<T>(store, receipt.resource, signal, receipt);
}
