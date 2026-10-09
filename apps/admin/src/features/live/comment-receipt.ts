// Purpose: unresolved-send flags across console unmount/reload; public non-terminal sends also fence their comment.
// Depends on: caller-provided scope, optional public comment ref and sessionStorage; never receives reply bodies.
// Used by: CommentReply; boolean flags survive until definite outcome or explicit external verification.
type StoragePort = Pick<Storage, "getItem" | "setItem" | "removeItem">;
/** Persist only a boolean; optional public ref narrows queued-send fencing without blocking other comments. */
export class CommentReceipt {
  private key: string;
  constructor(
    private storage: StoragePort,
    store: string,
    session: string,
    boundary: string,
    publicRef?: string,
  ) {
    this.key = publicRef === undefined
      ? `live-comment-unresolved:${store}:${session}:${boundary}`
      : `live-comment-public-pending:${store}:${session}:${boundary}:${encodeURIComponent(publicRef)}`;
  }
  /** Missing storage authority fails closed rather than losing an uncertain-send guard. */
  blocked(): boolean {
    try {
      return this.storage.getItem(this.key) !== null;
    } catch {
      return true;
    }
  }
  /** Arm synchronously before dispatch so navigation cannot remove uncertainty. */
  arm(): boolean {
    try {
      if (this.blocked()) return false;
      this.storage.setItem(this.key, "1");
      return this.storage.getItem(this.key) === "1";
    } catch {
      return false;
    }
  }
  /** Called only after a definite response or explicit merchant verification, never after a timeout. */
  clear(): boolean {
    try {
      this.storage.removeItem(this.key);
      return !this.blocked();
    } catch {
      return false;
    }
  }
}
