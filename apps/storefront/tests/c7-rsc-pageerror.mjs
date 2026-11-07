// Classifies only the existing WebKit cancellation signature for the storefront browser gate.
// Depends on browser-engine.mjs; called by shop-real-gate and its negative replay tests.
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
