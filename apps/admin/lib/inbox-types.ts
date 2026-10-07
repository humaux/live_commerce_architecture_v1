// Purpose: frozen A8-A14 and published-template browser DTOs, including nullable bundle-only rows.
// Depends on: live-console-v1 §§3/4/11 and internal/inbox + msgtemplates projections; no I/O.
// Used by: Inbox, InboxThread and standalone BuyerPanel; absent fields remain unknown.

/** Ledger projection; unknown delivery must not initiate a new send. */
export type SendState = "queued" | "sent" | "failed" | "blocked" | "unknown";
/** A8 conversation or bundle-only item; names exist only in transient authorised memory. */
export type ConversationItem = {
  conversation_id?: string | null;
  bundle_id?: string | null;
  session_id?: string | null;
  platform: string;
  display_name?: string | null;
  last_at: string;
  unread: boolean;
  unreplied: boolean;
  mode: "auto" | "human" | null;
  assignee: string | null;
  window_open_until: string | null;
  linked_customer_id: string | null;
  link_pending_manual?: boolean;
};
/** A8 keyset page; the cursor is an opaque server receipt. */
export type ConversationList = {
  items: ConversationItem[];
  next_cursor: string;
  unread_total: number;
};
/** A9 inbound or sealed-display outbound projection. */
export type Message = {
  direction: "in" | "out";
  seq?: number;
  at: string | null;
  text: string;
  attachments: { type: string }[];
  kind?: string | null;
  send_state?: SendState | null;
  send_code?: string | null;
  principal_id?: string | null;
  unreadable?: boolean;
};
/** A9 current server window and takeover generation. */
export type Thread = {
  items: Message[];
  window_open_until: string;
  mode: "auto" | "human";
  takeover_generation: number;
  human_until: string | null;
};
/** A13 server-only facts; no CAS version is currently published for A14. */
export type BuyerData = {
  display_name?: string | null;
  platform: string;
  purchase_ordinal: number;
  claims: {
    session_id: string;
    offer_id: string;
    keyword: string;
    quantity: number;
  }[];
  claim_total_minor: number;
  orders: {
    order_id: string;
    number: string;
    state: string;
    total_minor: number;
    created_at: string;
  }[];
  auto_reply?: { send_state: SendState };
  linked_customer_id?: string | null;
  window_open_until?: string | null;
  link_pending_manual: boolean;
};
/** Latest published version listed by message-templates; template content is never synthesised. */
export type Template = {
  template_id: string;
  version: number;
  name: string;
  kinds: string[];
  public_safe: boolean;
  created_at: string;
};
