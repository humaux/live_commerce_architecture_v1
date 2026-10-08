// Purpose: coarse unresolved-send safety flag across console unmount/reload without private message identifiers.
// Depends on: caller-provided session boundary and sessionStorage; never receives comment refs or reply bodies.
// Used by: CommentReply; boolean flag blocks all replies in the session until explicit external verification.
type StoragePort=Pick<Storage,"getItem"|"setItem"|"removeItem">;
/** Keep only an opaque boolean for a store/live-session/auth scope, not an enumerable buyer/comment hash. */
export class CommentReceipt {
  private key:string;
  constructor(private storage:StoragePort,store:string,session:string,boundary:string) {
    this.key=`live-comment-unresolved:${store}:${session}:${boundary}`;
  }
  /** Missing storage authority fails closed rather than losing an uncertain-send guard. */
  blocked():boolean {try{return this.storage.getItem(this.key)!==null;}catch{return true;}}
  /** Arm synchronously before dispatch so navigation cannot remove uncertainty. */
  arm():boolean {try{if(this.blocked())return false;this.storage.setItem(this.key,"1");return this.storage.getItem(this.key)==="1";}catch{return false;}}
  /** Called only after a definite response or explicit merchant verification, never after a timeout. */
  clear():boolean {try{this.storage.removeItem(this.key);return !this.blocked();}catch{return false;}}
}
