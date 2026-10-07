// Purpose: enforce in-memory inbox request epochs and platform text limits.
// Depends on: AbortController, Unicode NFC and the authorized Store permission hints.
// Used by: Inbox, InboxThread, BuyerPanel and inbox privacy Node acceptance.
import type { Store } from "../../../lib/model.ts";

type Ticket = { generation: number; signal: AbortSignal };
/** Abort an obsolete inbox scope; only visible current-epoch results may repaint. */
export class InboxFence {
  private generation = 0;
  private visible = true;
  private terminal = false;
  private controller = new AbortController();
  /** Capture the current request scope without storing buyer data. */
  begin(): Ticket {
    return { generation: this.generation, signal: this.controller.signal };
  }
  /** Revoke every pending completion on hide, logout, target or store changes. */
  invalidate(visible: boolean): void {
    this.controller.abort();
    this.generation++;
    this.visible = visible && !this.terminal;
    this.controller = new AbortController();
  }
  /** Check immediately before any read or write result mutates rendered state. */
  current(ticket: Ticket): boolean {
    return (
      !this.terminal &&
      this.visible &&
      ticket.signal === this.controller.signal &&
      !ticket.signal.aborted &&
      ticket.generation === this.generation
    );
  }
  /** A focus/pageshow event alone cannot revive a document that is still natively hidden. */
  reveal(state: string): boolean {
    if (state !== "visible" || this.terminal || this.visible) return false;
    this.invalidate(true);
    return true;
  }
  /** Terminal route departure never regains authority until a new keyed page constructs a fence. */
  revoke(): void {
    this.terminal = true;
    this.invalidate(false);
  }
}

/** Navigation hints fail closed; the Go API independently authorizes every request. */
export function permitted(store: Store | null, permission: string): boolean {
  return (
    !!store &&
    (store.role === "owner" || store.permissions?.includes(permission) === true)
  );
}

/** Provide local NFC-aware guidance; server validation remains authoritative. */
export function textLimit(platform: string, text: string) {
  const normalized = text.normalize("NFC");
  const count =
    platform === "instagram"
      ? new TextEncoder().encode(normalized).length
      : Array.from(normalized).length;
  const max = platform === "instagram" ? 1000 : 2000;
  return { count, max, valid: normalized.trim().length > 0 && count <= max };
}

/** Keep a single immutable reply receipt in memory until a definitive result or privacy boundary. */
export class ReplyReceipt {
  private value: { key: string; body: Record<string, unknown> } | null = null;
  private key: () => string;
  constructor(key: () => string = () => crypto.randomUUID()) {
    this.key = key;
  }
  /** Reuse the first body/key even when newer generations or drafts are available. */
  prepare(body: Record<string, unknown>) {
    this.value ??= { key: this.key(), body: Object.freeze({ ...body }) };
    return this.value;
  }
  /** Only transport uncertainty retains a receipt; a business rejection requires a new reviewed submit. */
  failed(code: string) {
    if (code !== "retry_later" && code !== "unavailable") this.clear();
  }
  /** Forget the private body on definitive completion, hide, logout or unmount. */
  clear() {
    this.value = null;
  }
  /** Return the transient receipt to disable edits while a submission is uncertain. */
  pending() {
    return this.value;
  }
}

/** Admit the intentional privacy defect only in the dedicated loopback browser acceptance harness. */
export function inboxCalibration(
  env: Record<string, string | undefined>,
  origin: string | undefined,
): boolean {
  return (
    env.LC_INBOX_CALIBRATION === "retain-thread" &&
    env.LC_BROWSER_INBOX_ACCEPTANCE === "1" &&
    env.COMMERCE_IDENTITY_ALLOW_LOOPBACK_TESTS === "1" &&
    /^http:\/\/127\.0\.0\.1:[1-9][0-9]{0,4}$/.test(origin ?? "")
  );
}
