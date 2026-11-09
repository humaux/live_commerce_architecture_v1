// Legacy signature recognizer retained for direct regression, not a storefront error-gate exemption.
// Depends on browser-engine.mjs; called only by its direct tests and diagnostic calibration.
import { isWebkitCancelledFetch } from "../../../tests/storefront/browser-engine.mjs";

export function isOwnRscPageError(error, origin) {
  if (!isWebkitCancelledFetch(error)) return false;
  try {
    const suffix = " due to access control checks.";
    const target = new URL(`https:/${error.message.slice(0, -suffix.length)}`);
    const keys = [...target.searchParams.keys()];
    return target.origin === origin && !/(?:^|\/)api(?:\/|$)/i.test(target.pathname)
      && keys.length === 1 && keys[0] === "_rsc";
  } catch {
    return false;
  }
}
