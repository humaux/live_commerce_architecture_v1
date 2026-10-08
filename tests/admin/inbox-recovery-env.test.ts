// Purpose: await actual recovery state/actions and optionally vary Node scheduling/date without changing crypto results.
// Depends on: actual generic hook-host, component action promises, Node WebCrypto.digest and LC_RECOVERY_TEST_* profiles.
// Used by: recovery assertions and optional node --import timing/date profiles; no production changes or authored test cases.
// Status: MOCK scheduling pressure, not crypto, provider, browser or performance acceptance.
import { nodes, textOf, type Host } from "./inbox-review-host.test.ts";
import { inboxCopy } from "../../apps/admin/src/features/messages/copy.ts";
const installed = Symbol.for("lc.inbox.recovery.env.installed");
if (!(globalThis as any)[installed]) {
  (globalThis as any)[installed] = true;
  const date = process.env.LC_RECOVERY_TEST_NOW;
  if (date) {
    const epoch = Date.parse(date);
    if (!Number.isFinite(epoch)) throw new Error("Invalid recovery test date profile");
    Date.now = () => epoch;
  }
  const delay = Number(process.env.LC_RECOVERY_TEST_DIGEST_DELAY_MS ?? "0");
  if (!Number.isSafeInteger(delay) || delay < 0 || delay > 1000) throw new Error("Invalid recovery test digest delay");
  if (delay) {
    const original = crypto.subtle.digest.bind(crypto.subtle);
    Object.defineProperty(crypto.subtle, "digest", {
      configurable: true,
      value: (...args: Parameters<typeof original>) =>
        original(...args).then((value) => new Promise<ArrayBuffer>((done) => setTimeout(() => done(value), delay))),
    });
  }
}

/** Locate the real mounted generic child; no production state or readiness rule is replicated. */
export function childHost(host: Host, id: string): Host {
  for (const child of host.children.values()) {
    if (child.host.raw?.props?.["data-testid"] === id) return child.host;
    try {
      return childHost(child.host, id);
    } catch {
      /* Inspect the next generic child host. */
    }
  }
  throw new Error(`Actual child component missing: ${id}`);
}
/** Bound an actual request arrival/completion event without polling or guessed event-loop turns. */
export function bounded<T>(work: Promise<T>, description: string): Promise<T> {
  let timer!: ReturnType<typeof setTimeout>;
  const timeout = new Promise<never>((_resolve, reject) => {
    timer = setTimeout(() => reject(new Error(`Timed out waiting for ${description}`)), 5000);
  });
  return Promise.race([work, timeout]).finally(() => clearTimeout(timer));
}
/** Await the mounted recovery locale's actual idle control commit and refresh its parent rendering. */
export async function recoveryDone(host: Host) {
  const recovery = childHost(host, "bundle-recovery");
  const mountedLocale = (parent: Host): string | undefined => {
    for (const child of parent.children.values()) {
      if (child.host === recovery) return child.props.locale;
      const locale = mountedLocale(child.host);
      if (locale !== undefined) return locale;
    }
  };
  const locale = mountedLocale(host);
  if (typeof locale !== "string") throw new Error("Actual recovery locale missing from mounted props");
  const loading = inboxCopy(locale).loading;
  await recovery.waitFor(
    () => nodes(recovery.output).some((n) => n.type === "button" && textOf(n) !== loading),
    "actual recovery action status and idle control committed",
  );
  host.flush();
}
/** Await actual A9/A10 completion, including the DM render and cleared in-flight control. */
export async function threadReady(host: Host) {
  await host.waitFor(
    () => host.output?.props["aria-busy"] === false && textOf(host.output).includes("MOCK_DM"),
    "actual A9/A10 thread authority committed",
  );
}
