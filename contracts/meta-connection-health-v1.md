# Meta connection health v1 — proactive token/permission/subscription probe, capability table, merchant alert (W1-01B)

Status: **DRAFT (2026-10-05, author: Claude Opus subagent for the integrator; awaiting adversarial review and freeze)**.
Unit W1-01B of `output/arch-conformance/IMPLEMENTATION-PLAN.md` (§5 card and dispatch item 3). Base `r3/integration`
`d90d7fe3`. Evidence label of this file: DESIGN; every gate NOT_RUN.

Goal (plan card): when a Page token is revoked or expires, or a permission, Page task or webhook subscription is withdrawn,
the merchant sees it **before** the next live, not as silently missing claims during the live.

Consumes, without redefining: `live-console-v1.md` §6 (FROZEN) — the capability names `read_comment`, `private_reply`,
`dm_session`, `reply_public`; the states `ok | missing_permission | missing_task | not_subscribed | reauth_required |
review_required | unsupported | unknown`; the evidence values `DESIGN | MOCK | LIVE_READ | LIVE_SEND`; the per-capability
requirement rows; the UI and server rules. This contract owns the table, the probe that fills it, the alert, and the read
route that §6 says is "exposed through the W1-01B route".

Amends, without editing them (amendment text in §13, recorded by the **integrator at freeze**, never by an implementer):
`meta-claims-intake-v1.md` (Amendment "Merchant connect": clause 4 gains a second `reauth_required` writer; clause 6
privilege delta), `storefront-v2.md` §E / migration 0090 (a second, non-order merchant mail table in schema `notify`).
`invariants.json` unchanged. Migration number is a **placeholder `0127`**; the integrator assigns the real number at
dispatch.

## 0. Facts

### 0.1 Meta (developers.facebook.com; retrieved 2026-10-01 unless noted; not re-retrieved for this draft)

| # | Fact | Source |
| --- | --- | --- |
| H1 | `GET /{page-id}/subscribed_apps` with a Page token lists the apps subscribed to the Page with `subscribed_fields`. | page/subscribed_apps reference (also cited at `internal/metaconnect/graph.go:194`) |
| H2 | `GET /debug_token?input_token=…` returns `is_valid`, `expires_at`, `data_access_expires_at`, `scopes`, `granular_scopes`; the token being inspected is a **query parameter**. | debug-token reference; owner run 2026-10-03 (Humaux `d5b008b9`, value not output) |
| H3 | Graph error code 190 = invalid access token (subcodes 458 app removed, 460 password changed, 463 expired, 467 invalid). Codes 4, 17, 32, 613 = rate limits; 10 / 200-299 = permission errors; 100 = invalid parameter / nonexisting field or object. | Graph error-handling reference |
| H4 | Login for Business permissions without App Review have Standard Access: only users with a role on the app can grant/use them. | app-review / access-levels reference (meta-claims-intake "Merchant connect" clause 2) |

UNKNOWN (each fails safe as stated; closed by the §11.3 LIVE probe):
- **MCH-U1** whether `GET /me/permissions` answers with a **Page** token (with `/me` = the Page it may return code 100
  "nonexisting field"). Fallback: permissions come from the connect-time snapshot `meta_connections.scopes`
  (`perm_source='snapshot'`), and runtime permission errors reported by senders demote the capability (§4.4).
- **MCH-U2** whether any Page-token read returns the connecting user's Page **tasks** (`tasks` is a field of
  `/me/accounts`, a user-token edge; the user token is not stored, clause 1). Until closed: tasks come from the
  connect-time pick check (connect refuses a Page without `MESSAGING` and `MODERATE`, `graph.go` `entryOf`) and runtime
  task errors demote (§4.4).
- **MCH-U3** whether a Page token derived from a long-lived Login-for-Business user token is subject to
  `data_access_expires_at` (the owner's user token showed one, 2026-12-30). Without `debug_token` (§12 R1) expiry is
  detected reactively, by the next probe after Meta starts answering 190 (≤ 6 h, or at the pre-live recheck §5.3).

### 0.2 Code facts (read 2026-10-05 at `d90d7fe3`)

- `integration.meta_connections` (0095, re-keyed in 0108 to `(tenant_id, store_id, page_id)`, ≤ 10 per store): `page_id`,
  `fb_binding`, `ig_binding`, `ig_id`, `scopes text[]` (granted permissions at connect), `status active|reauth_required`,
  `route_expires_at`. No tasks column. Writers today: `meta_connect_finish`, `meta_connect_disconnect`,
  `meta_connect_mark_reauth` (table comment, 0108:25).
- Connect requires `pages_show_list, pages_manage_metadata, pages_read_engagement, pages_messaging` + tasks
  `MESSAGING, MODERATE`; IG adds `instagram_basic, instagram_manage_comments, instagram_manage_messages`
  (`internal/metaconnect/graph.go:35-36`). `pages_manage_engagement` (needed by `reply_public`) is **not** required at
  connect: existing connections lack it until reconnect (added to the FLfB configuration 2026-10-05, live-console §13.3).
- Connect subscribes `subscribed_fields=feed` only (`graph.go:198`); `messages` is added by LC-B3 (live-console §3.1,
  migration 0122 resubscribe job). This unit only **reads back**.
- Page tokens open only in `cmd/claims-worker`: v2 (HPKE) via `pageopen.Keyring.Open`, v1 (AES, operator CLI) via
  `metareply.PageTokenKeyring` (`internal/integrations/metareply/unsubscribe.go:27-33`). Sealed copies reach the worker
  through a claiming definer (`integration.claim_meta_unsubscribe`, 0100) — the pattern reused here.
- All Graph calls go through `metaoauth.Graph.Do` (host allowlist, no redirect, bounded body, token in the
  `Authorization` header, never a URL; `graph.go:13`).
- A Graph 190 on a private reply already flips the connection via `integration.meta_connect_mark_reauth(operation)`
  (`metareply/routes.go:108-110`). That passive path is unchanged.
- claims-worker runs River with `PeriodicJobs` (`cmd/claims-worker/main.go:246`, retention + storefrontdomains).
- Merchant mail: `notify.outbox` is keyed `(order_id, kind)` (0090:36) — it cannot carry a non-order mail; it is drained
  by `internal/notify.Worker` in `cmd/expiry-worker` (pool `commerce_expiry_worker`), owner addresses come from
  `identity.staff_password_email` (0090:20). Store banner precedent: `apps/admin/components/BillingBanner.tsx`, mounted
  in `WorkspaceFrame.tsx:408`, `store:read`, "any failure shows nothing".

## 1. Scope

In: periodic read-only probe per connected Page; `integration.binding_capabilities` (the §6 vocabulary) and its read
paths; the derivation rules; `reauth_required` on a probed 190; merchant alert (admin banner + one owner e-mail per
episode); re-check and re-auth path; the `CapabilityReader` swap for LC-B3; evidence-label upgrade hook for LC-B2/B4.

Out: ads and live-broadcast capabilities (§10.2 rest); any Meta write (subscribe/resubscribe is LC-B3); `debug_token`;
token refresh (a Page token cannot be refreshed without the user; re-auth = reconnect); IG app-level webhook field
verification (MCH-OPEN-4); any change to how sends authorize (they keep re-reading the capability at Check, live-console
§6 server rule).

## 2. Table `integration.binding_capabilities` (migration 0127)

```sql
CREATE TABLE integration.binding_capabilities (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, page_id text NOT NULL,
 binding_id uuid NOT NULL,
 provider text NOT NULL CHECK(provider IN ('facebook','instagram')),
 capability text NOT NULL CHECK(capability IN ('read_comment','private_reply','dm_session','reply_public')),
 state text NOT NULL CHECK(state IN ('ok','missing_permission','missing_task','not_subscribed','reauth_required',
   'review_required','unsupported','unknown')),
 reason text NOT NULL CHECK(reason ~ '^[a-z0-9_]{1,40}$'),          -- fixed codes §3.3
 evidence text NOT NULL CHECK(evidence IN ('DESIGN','MOCK','LIVE_READ','LIVE_SEND')),
 checked_at timestamptz NOT NULL, evidence_at timestamptz,
 PRIMARY KEY(tenant_id, store_id, binding_id, capability),
 FOREIGN KEY(tenant_id, store_id, page_id) REFERENCES integration.meta_connections(tenant_id, store_id, page_id) ON DELETE CASCADE,
 FOREIGN KEY(tenant_id, store_id, binding_id) REFERENCES integration.bindings(tenant_id, store_id, id));
```

- FORCE RLS. Rows are written **only** by the definers of §8 (owner `commerce_integration_writer`).
- Disconnect deletes the connection row → rows cascade away; a reconnect of the same Page with a new binding id creates
  new rows (evidence starts again, §3.4).
- One row per (binding, capability): 4 per Facebook binding, 4 per Instagram binding. Bounded by the 10-Page cap (I23).

## 3. Derivation (one pure Go function, `metaconnect.Derive`, also used by the default reader of §7)

### 3.1 Inputs

`Reading = {token: valid|invalid(subcode)|page_gone|unknown, perms: set|unknown, perm_source: graph|snapshot,
tasks: set (snapshot) minus tasks demoted (§4.4), fb_fields: set|unknown, app_review: set}` where
`app_review` = the permissions with Advanced Access, from config `COMMERCE_META_ADVANCED_ACCESS` (comma list, default
empty: the app is not reviewed, so every granted permission is Standard Access).

### 3.2 Rule (first match wins, per capability and binding)

1. `token=invalid` → `reauth_required` (reason `token_expired` 463, `token_revoked` 458/460, `token_invalid` other).
2. `token=page_gone` (code 100 on `/{page-id}`) → `reauth_required`, reason `page_unavailable`.
3. `token=unknown` (no probe yet and no snapshot) → `unknown`, reason `not_probed`.
4. A required permission (live-console §6 table, IG column for IG bindings) not in `perms` → `missing_permission`,
   reason `perm_<name>` (e.g. `perm_pages_manage_engagement`).
5. A required task not in `tasks` → `missing_task`, reason `task_moderate` | `task_messaging`.
6. Required Page field (`feed` for `read_comment`, `messages` for `dm_session`) not in `fb_fields` → `not_subscribed`,
   reason `sub_feed` | `sub_messages`; `fb_fields=unknown` → `unknown`, reason `sub_unread`. IG bindings: the app-level
   subscription is not readable with a Page token → treated as present, reason recorded only when everything else is ok:
   `ok_app_level_assumed` (MCH-OPEN-4).
7. `dm_session` only: deployment flag `COMMERCE_META_DM_RECEIVER_CONFIRMED` (default false; set by the integrator after
   live-console probe R4 closes LC-U11) false → `unknown`, reason `lc_u11_open`.
8. Any required permission not in `app_review` → `review_required`, reason `standard_access`.
9. Otherwise `ok`, reason `ok` (or `ok_snapshot` when `perm_source=snapshot`, `ok_app_level_assumed` per 6).

`unsupported` is reserved by §6 and never written in v1.

### 3.3 Reason codes (closed list; i18n keys in the UI)

`ok, ok_snapshot, ok_app_level_assumed, token_expired, token_revoked, token_invalid, page_unavailable, not_probed,
perm_<permission>, task_moderate, task_messaging, sub_feed, sub_messages, sub_unread, lc_u11_open, standard_access,
runtime_permission, runtime_task`. `perm_<permission>` takes only names from the §6 table.

### 3.4 Evidence label

The probe **never** changes `evidence`. New rows start at `MOCK` when the Graph base URL is loopback (fake Graph,
`metareply.Config.Validate` already distinguishes it) else `DESIGN`. Upgrades only by `integration.mark_capability_evidence`
(§8) called by LC-B2/LC-B4 on the §6 events (first successful console poll → `LIVE_READ`; first SUCCEEDED private reply /
DM / public reply → `LIVE_SEND`), monotonic (`DESIGN < MOCK < LIVE_READ < LIVE_SEND`, never lowered).

## 4. Probe

### 4.1 Scheduling (River, claims-worker)

- Periodic job `meta_health_sweep_v1` (args `{}`; `river.PeriodicJob`, every 5 min, `RunOnStart`), unique by kind, so at
  most one sweep in flight per worker fleet (I23).
- State row per connection: `integration.meta_health_probes` (§8), created/reset by an AFTER INSERT/UPDATE trigger on
  `meta_connections` (columns `status, scopes, fb_binding, ig_binding`) with `next_due_at = now()`; cascades on delete.
- A sweep calls `integration.claim_meta_health_probes(20)`: due rows (`next_due_at <= now()`), `FOR UPDATE SKIP
  LOCKED`, lease 60 s (`lease_token` random 32 bytes, `generation+1`), returning per row: tenant, store, page_id,
  bindings, IG id, and the **FB binding's current head credential** (`key_id, version, nonce, ciphertext`). The
  ciphertext leaves SQL only to `commerce_claims_worker` through this definer (0100 pattern; no runtime SELECT on
  credentials).
- Cadence after a result: severity `none` → +6 h; `warning`/`blocking` → +30 min; rate-limited → +15 min × 2^n
  (n ≤ 4); ±10 % jitter. Manual re-check (§6) and any connection change → due now.

### 4.2 Calls (per Page, sequential, each ≤ 10 s, whole probe ≤ 30 s < lease)

Open the token (v2 `pageopen`, v1 `PageTokenKeyring`) into a buffer zeroed after the probe. Through `metaoauth.Graph.Do`
with the token in the header:

| # | Call | Purpose | Classification |
| --- | --- | --- | --- |
| P1 | `GET /{page-id}?fields=id` | token validity (cheapest equivalent of `debug_token`) | 2xx with matching id → valid; 190 → invalid(subcode); 100 → page_gone; 4/17/32/613/429 → rate_limited (stop, keep previous states); transport/5xx/garbled → unknown (stop, keep previous states) |
| P2 | `GET /me/permissions` | granted permissions (`status=granted` only) | 2xx → perms, `perm_source=graph`; 100/10/200-299 → perms = snapshot, `perm_source=snapshot` (MCH-U1); other failures as P1 |
| P3 | `GET /{page-id}/subscribed_apps` | Page webhook fields of **our** app (`data[].id == COMMERCE_META_PAGE_APP_ID`) | 2xx → `fb_fields` (absent app → empty set); failures → `fb_fields=unknown` |

P2/P3 run only after P1 = valid. No call writes Meta state, so a timed-out GET is simply re-read on the next cycle
(no UNKNOWN ledger needed; I06 concerns side effects). `paging.next` URLs are never followed (they may embed
`access_token`; same rule as LCN01). Nothing from a response body is logged; logs carry page id, call #, HTTP status,
Graph code/subcode and duration only (I11).

### 4.3 Recording

`integration.record_meta_health(p_page text, p_generation bigint, p_lease bytea, p_result jsonb)` (lease-fenced like
`finish_meta_unsubscribe`; a stale lease or a vanished connection is a no-op returning `stale`). `p_result` =
`{outcome: probed|rate_limited|unknown, token, subcode?, perms?, perm_source, fb_fields?, graph_codes[]}` — no token,
no body. In one transaction it: upserts the 4/8 capability rows with the Go-derived states (SQL re-derives nothing but
CHECKs the vocabulary and that `binding_id` belongs to the row's Page); on `token=invalid|page_gone` flips
`meta_connections.status` `active → reauth_required` (same one-way rule as `meta_connect_mark_reauth`, audit
`meta.connect.reauth_required` with `{page_id, reason}`); computes severity (§5.1); opens/closes the alert episode
(§5.2); sets `next_due_at`, `last_checked_at`, `consecutive_failures`, `last_outcome`.

`rate_limited` and `unknown` outcomes change **no** capability state (a Meta outage or our egress failure must not block
live selling); after 6 consecutive `unknown` outcomes (≈ 3 h at the 30-min cadence) severity becomes at least `warning`
with reason `probe_failing` (banner only, no mail).

### 4.4 Runtime demotion (closes the MCH-U1/U2 gap without new Graph calls)

`integration.report_capability_failure(p_operation uuid, p_capability text, p_kind text)` (`p_kind ∈ permission|task`),
EXECUTE `commerce_worker` and `commerce_claims_worker`, called by the send/read adapters (LC-B2/B4, and the existing
private-reply route) when Graph answers a permission error (10, 200-299) on an operation of that binding. Sets that
capability to `missing_permission`/`missing_task` with reason `runtime_permission`/`runtime_task` and makes the probe
due now. A later probe can clear `runtime_permission` only with `perm_source=graph`; `runtime_task` clears only on
reconnect (the trigger of §4.1 resets the rows to `not_probed` and re-probes).

## 5. Degradation and notification

### 5.1 Severity per Page

- `blocking`: `meta_connections.status = reauth_required`, or `read_comment` or `private_reply` of any binding of the
  Page in a state other than `ok | review_required` (live selling stops).
- `warning`: `dm_session` or `reply_public` not `ok | review_required | unknown(lc_u11_open)`, or `probe_failing`.
- `none` otherwise. Store severity = worst Page.

### 5.2 Episode, banner, e-mail

- An episode opens when a Page's severity goes from `none|warning` to `blocking` (`episode+1`, `episode_opened_at`), and
  closes when it returns to `none|warning` (`episode_closed_at`). Episodes are per Page.
- **Banner** (UI unit, §10): shown on every workspace page while store severity ≠ `none`; text by worst reason; CTA
  「重新連結」→ Settings › Facebook (existing `MetaConnect` card). `warning` = neutral style, `blocking` = warning style.
  Like `BillingBanner`: advice, never a gate; any fetch failure shows nothing.
- **E-mail**: on opening a `blocking` episode, `record_meta_health` calls `notify.enqueue_merchant_alert(tenant, store,
  'meta_health', page_id, episode)` (§8). One mail per (Page, episode); **cool-down**: no mail if the same Page had a
  mail in the previous 24 h (flapping). Recipients: the store's owners (same resolution as `merchant_new`). Content:
  store name, Page name, which capabilities stopped and why (reason → copy), the admin link; never a token, permission
  dump or Graph body. `warning` never mails (the never-had `reply_public` of every pre-0127 connection must not mail all
  merchants on deploy).
- Mail transport: the existing `internal/notify.Worker` (expiry-worker) drains the new `notify.merchant_alerts` table
  through `notify.claim_merchant_alerts(p_limit, p_daily_cap)` / `notify.record_merchant_alert(p_id, p_state,
  p_recipient_hash)`, sharing the 60 % notify budget of §E3; same states PENDING → SENDING → SENT | UNKNOWN (never
  re-sent) | FAILED | SKIPPED, 24 h staleness, 3 attempts.

### 5.3 Pre-live freshness (recommended, not required)

LC-B1 (`POST …/lifecycle` start) and the A5-3 live picker may call `metaconnect.RequestRecheck(page)` (same SQL as §6
recheck, no permission of its own) so the console shows a capability state at most minutes old when the merchant goes
live.

## 6. Re-auth path

1. Banner/e-mail → Settings › Facebook card lists each Page with its capability rows (state, reason copy, checked time)
   and two actions: 「重新檢查」 (`POST …/meta/health/recheck`) and 「重新連結」 (the existing `meta-connect/start` → dialog →
   `pick` of the same Page; clause 3 appends credential version +1 and `meta_connect_finish` sets `status=active`).
2. The `meta_connections` UPDATE fires the §4.1 trigger: capability rows reset to `unknown/not_probed`, probe due now; the
   next sweep (≤ 5 min) re-derives; when severity returns to `none` the episode closes and the banner disappears.
3. A missing **permission** is fixed by reconnect (the dialog re-asks; FLfB configuration must contain it). A missing
   **task** needs the Page admin to grant the task in Meta Business Suite, then reconnect (copy says so). A missing
   **subscription** field is fixed by reconnect (connect subscribes) or by LC-B3's resubscribe job.

## 7. Compatibility with LC-B3's `CapabilityReader`

LC-B3 (in flight) introduces a `CapabilityReader` whose default implementation derives states from what meta-connect
stored. This contract fixes the seam so W1-01B swaps the implementation without touching callers:

1. **The interface is LC-B3's.** If LC-B3 has merged a signature, W1-01B adopts it byte-for-byte. If not yet merged, LC-B3
   uses this shape (package `internal/metaconnect`, file `capability.go`, owned by LC-B3 until W1-01B merges):
   ```go
   // CapabilityState is one live-console §6 row. Reason is a §3.3 code; CheckedAt is zero when never probed.
   type CapabilityState struct {
       BindingID, Provider, Capability, State, Reason, Evidence string
       CheckedAt time.Time
   }
   // CapabilityReader returns the §6 rows of the scope's store (all Pages, or one binding when bindingID != "").
   // Read-only; runs inside the caller's scoped transaction (RLS on tenant/store GUCs).
   type CapabilityReader interface {
       Capabilities(ctx context.Context, tx pgx.Tx, scope platform.Scope, bindingID string) ([]CapabilityState, error)
   }
   ```
2. **LC-B3's default** (`SnapshotReader`) calls `metaconnect.Derive` (§3) with `token=valid` when `status=active` else
   `invalid`, `perms = meta_connections.scopes`, `perm_source=snapshot`, tasks = connect snapshot, `fb_fields` = what
   connect subscribed (`feed`, plus `messages` once LC-B3's 0122 resubscribe job SUCCEEDED for that Page),
   evidence `DESIGN`. Because both readers call the same `Derive`, vocabulary and rules cannot drift. If LC-B3 merges
   first it ships `Derive` (this §3) in `internal/metaconnect/derive.go`; W1-01B then owns the file.
3. **W1-01B's implementation** (`TableReader`) reads `integration.binding_capabilities` under RLS; for a binding with no
   row yet (connection created before the first probe) it falls back to `SnapshotReader` for that binding, so no caller
   ever sees fewer rows or a new state during rollout.
4. **The swap** is one line at each construction site (`cmd/api` wiring and, if LC-B4 needs it at Check, the worker
   wiring): `SnapshotReader{}` → `TableReader{Fallback: SnapshotReader{}}`. No caller signature, JSON field or error code
   changes. Gate MCH08 runs LC-B3's own reader tests against both implementations (table-driven: the same fixtures must
   produce identical rows when the table is empty, and table rows when present).
5. Server-side Check (live-console §6 "Check re-reads it") uses `integration.binding_capability_state(tenant, store,
   binding, capability) RETURNS text` (§8) in workers — table row if present, else the snapshot derivation done in
   SQL by the same §3.2 steps 1, 4, 8 (status, scopes, Advanced Access list passed by Go). LC-B4 calls this definer, not
   the Go reader.

## 8. Migration 0127 (placeholder) — objects and exact privilege delta

Owner of every new object: `commerce_integration_writer` (except `notify.*`: `commerce_checkout_writer`, 0090 pattern).
All functions `SECURITY DEFINER SET search_path=pg_catalog`, `REVOKE ALL … FROM PUBLIC`, READ COMMITTED asserted on
writers.

| Object | Purpose | EXECUTE / grant |
| --- | --- | --- |
| table `integration.binding_capabilities` | §2 | FORCE RLS; `commerce_runtime` SELECT via policy `app.tenant_id`/`app.store_id` GUCs |
| table `integration.meta_health_probes(tenant_id, store_id, page_id PK→meta_connections ON DELETE CASCADE, next_due_at, lease_token bytea, lease_until, generation bigint, consecutive_failures smallint, last_outcome, last_checked_at, perm_source, severity CHECK(none,warning,blocking), episode bigint, episode_opened_at, episode_closed_at, last_mail_at)` | §4–5 | FORCE RLS; `commerce_runtime` SELECT(page_id, severity, last_checked_at, next_due_at, episode_opened_at) via the same policy |
| trigger fn `integration.meta_health_on_connection()` on `meta_connections` AFTER INSERT OR UPDATE OF status, scopes, fb_binding, ig_binding | §4.1, §6.2 | none (trigger) |
| `integration.claim_meta_health_probes(p_limit int)` | §4.1 (1..50) | `commerce_claims_worker` |
| `integration.record_meta_health(text, bigint, bytea, jsonb)` | §4.3 | `commerce_claims_worker` |
| `integration.report_capability_failure(uuid, text, text)` | §4.4 | `commerce_worker`, `commerce_claims_worker` |
| `integration.mark_capability_evidence(uuid, text, text)` (operation, capability, evidence) | §3.4; binding taken from the operation, never from the caller | `commerce_worker`, `commerce_claims_worker` |
| `integration.binding_capability_state(uuid, uuid, uuid, text, text[])` | §7.5 | `commerce_worker`, `commerce_claims_worker` |
| `integration.request_meta_health_recheck(p_hash bytea, p_store uuid, p_page text)` | §9 B2; `integration:manage` via `identity.resolve_access`; no-op if checked < 60 s ago | `commerce_runtime` |
| table `notify.merchant_alerts(id uuid PK, tenant_id, store_id, kind CHECK('meta_health'), subject text (page id), episode bigint, state, attempts, next_attempt_at, batch_id, claimed_at, sent_at, recipient_hash, skip_reason, created_at, UNIQUE(tenant_id, store_id, kind, subject, episode))` | §5.2 | FORCE RLS; no direct grant |
| `notify.enqueue_merchant_alert(uuid, uuid, text, text, bigint)` | §5.2 (cool-down inside) | `commerce_integration_writer` |
| `notify.claim_merchant_alerts(int, int)`, `notify.record_merchant_alert(uuid, text, bytea)` | §5.2 | `commerce_expiry_worker` |

Also: `commerce_integration_writer` gains column UPDATE(status, updated_at) use on `meta_connections` inside
`record_meta_health` (it already owns the definers that write it; no new role grant if the owner is unchanged — the
implementer verifies against 0095/0108 and reports the exact delta in DELIVERY.md); one INSERT policy on
`ops.audit_events` for `commerce_integration_writer` limited to action `meta.connect.reauth_required`. Housekeeping:
`claim_merchant_alerts` deletes non-PENDING alert rows older than 90 days (≤ 200 per call). No new role, no River queue
guard change beyond registering the job kind `meta_health_sweep_v1` in the post_river kind allowlist if one applies to
claims-worker (implementer checks post_river 0014 and reports).

## 9. HTTP (base `/v1/admin/stores/{store_id}`; merchant bearer → server-resolved tenant/store, I01; strict decoder;
`Cache-Control: no-store`; errors `{code}`)

| # | Route | Permission | Response | Errors |
| --- | --- | --- | --- | --- |
| B1 | GET `/meta/health` | `store:read` (banner for every member, like billing standing) | `{severity, pages: [{page_id, page_name, status, severity, checked_at, next_check_at, episode_opened_at?, capabilities: [CapabilityState…]}]}` | 404 other store |
| B2 | POST `/meta/health/recheck` | `integration:manage` | body exactly `{page_id}` → 202 `{next_check_at}`; no `Idempotency-Key` (not a command: it only moves `next_due_at` earlier, naturally idempotent) | 404 page not connected to this store, 429 `recheck_too_soon` (< 60 s) |

The live-console §7.1 read model embeds the same `capabilities` array through the `CapabilityReader` (no second
format). BFF allowlist additions (UI unit): B1, B2. File: `internal/httpapi/meta_health.go` (new).

## 10. Privacy and security

- The Page token exists in plaintext only inside the claims-worker probe, in a buffer zeroed after the third call; never
  in River args (sweep args `{}`), PG text, logs, URLs or the mail (I11). Fake Graph in MCH tests **fails the request**
  if `access_token` appears in a URL or query, and a sentinel token is scanned for in logs, River rows, audit, alert and
  capability rows.
- Probe rows and capability rows hold ids, codes and timestamps only; Graph bodies are parsed into sets and dropped.
- B1 cross-store → 404; B2 cannot target another store's Page (the definer joins `meta_connections` on the resolved store).
- The mail goes only to the store's owners; its link is the admin origin, never a storefront or Meta URL.

## 11. Evidence classes, gates, LIVE probe

### 11.1 Classes

| Class | Proves | Cannot prove |
| --- | --- | --- |
| MOCK (REAL_PG + fake Graph `httptest`) | derivation, scheduling, lease, recording, severity/episode/cool-down, reauth flip, mail enqueue and drain, reader swap, privilege delta, leak scan | real Graph shapes (MCH-U1..U3), Page-token answers for `/me/permissions`, real expiry |
| BROWSER (MOCK backend) | banner, card rows, recheck/reconnect by real clicks (AGENTS.md UI rule), click ledger | anything about Meta |
| LIVE_READ (pilot, app-role test user, real OAuth) | §11.3 | non-role merchants (Advanced Access) |

Everything Meta-facing stays **MOCK until §11.3 passes** on the pilot through the real OAuth flow with an app-role test
user. I17: no capability is marked production-supported from MOCK.

### 11.2 Gates (all NOT_RUN)

New mode `scripts/dev/test-local.sh --meta-health` (REAL_PG + fake Graph; tests `tests/foundation/meta_health_*_test.go`
and `internal/metaconnect/derive_test.go`), `--browser-meta-health` (UI unit), plus `release-gate.sh --strict --only G07`
at acceptance and after merge (migration + GRANT).

| Gate | Class | Required |
| --- | --- | --- |
| MCH01 | UNIT | `Derive` table: every §3.2 branch, FB and IG, first-match order, reason codes closed list, `unsupported` never produced |
| MCH02 | MOCK | Probe outcomes: 190/458, 190/460, 190/463, 190/467 → `reauth_required` + connection flipped once + audit; 100 on P1 → `page_unavailable`; permission revoked (P2 omits one) → `missing_permission`; P2 code 100 → snapshot + `ok_snapshot`; `messages` missing → `dm_session` `not_subscribed`, `feed` missing → `read_comment` `not_subscribed`; app absent from `subscribed_apps` → both |
| MCH03 | MOCK | 429 and codes 4/17/32/613 → no state change, backoff ladder; timeout/5xx/garbled → no state change; 6 consecutive → `probe_failing` warning, no mail |
| MCH04 | MOCK | Leases: two worker processes, one probe per Page; expired lease → record `stale`, no write; disconnect during a probe → no row resurrected; ≤ 20 per sweep, one sweep in flight |
| MCH05 | MOCK | Episodes: blocking opens one alert row and one mail; repeated probes in the same episode → no second row; close + reopen within 24 h → no mail; after 24 h → mail; warning never mails; pre-existing connections without `pages_manage_engagement` → warning, zero mails at deploy |
| MCH06 | MOCK | Leak scan: sentinel token absent from URLs (fake Graph refuses), logs, River rows, audit, alert, probe and capability rows; `paging.next` with `access_token` never fetched |
| MCH07 | MOCK | B1/B2: cross-tenant/store 404; `viewer` and `live_operator` can B1, cannot B2; B2 < 60 s → 429; recheck then sweep → fresh `checked_at` |
| MCH08 | MOCK | Reader swap: LC-B3 reader fixtures identical under `SnapshotReader` and `TableReader` on an empty table; table rows win when present; `binding_capability_state` equals the Go reader for every fixture |
| MCH09 | MOCK | Re-auth: reauth_required → reconnect same Page → rows reset → next sweep `ok` → episode closed → B1 severity `none` |
| MCH10 | MOCK | Privilege delta exactly §8 (MCI02/KC03 expected rows updated by the integrator) |
| MCH11 | BROWSER | banner appears on every workspace page for blocking/warning, hidden for none or fetch failure; card rows; 重新檢查 and 重新連結 by real clicks; zh-TW, zh-CN, en; 390 px |
| MCH12 | LIVE_READ | §11.3 |

### 11.3 LIVE probe (folds into W1-02X / live-console LC-X1 R1; pilot, app-role test user, real OAuth)

Preconditions as live-console §13.3 (owner-controlled test Page, app 大夢 in development mode, Meta UI only through Kimi
WebBridge with owner consent). Steps: (1) connect the test Page through the real dialog; (2) run one sweep; record P1–P3
status codes and the redacted response **shapes** (MCH-U1: does `/me/permissions` answer with the Page token? MCH-U2:
any Page-token source of tasks?); (3) in Meta settings, the test user removes one permission from the app (owner consent)
→ next recheck shows `missing_permission` and the banner; reconnect restores `ok`; (4) record whether the owner's
existing pilot connection shows `reply_public` `missing_permission` (expected) and that no mail was sent. Evidence:
`output/meta-health-probe/<date>/`; no token, no names of real people.

## 12. Rejected alternatives

- **R1 `debug_token`** (task wording "or the cheapest equivalent"): it carries the inspected token in the query string
  (H2), violating I11 and the "no secret in a URL" rule (plan card). The Graph batch form (token inside a
  `relative_url` string in the body) was also rejected: still a URL-shaped secret, and unsupported by our `metaoauth.Graph`
  allowlist. P1 is the cheapest equivalent for validity; expiry prediction is lost (MCH-U3).
- **R2 Persisting the user token** to read `/me/accounts` tasks: violates clause 1 (user token never persisted).
- **R3 Blocking sends on a stale probe**: a Meta outage or our egress failure would stop live selling; capability
  states change only on definite answers (§4.3).
- **R4 Reusing `notify.outbox`**: keyed by order; widening its PK touches every buyer-mail path frozen in 0090/0098.
- **R5 Mail on every degradation including `warning`**: would mail every merchant at deploy (`reply_public` never
  granted on pre-2026-10-05 connections).

## 13. OPEN questions (each with the recommended answer; integrator rules at freeze)

- **MCH-OPEN-1 Probe cadence.** Recommended: 6 h healthy, 30 min degraded, sweep every 5 min, recheck on demand and on
  every connection change; ≤ 3 GETs per Page per cycle (≈ 12/day/Page healthy) — negligible against Page rate limits.
- **MCH-OPEN-2 Who sees the banner.** Recommended: every store member (`store:read`), because a `live_operator` running the
  live must know claims will stop; only `integration:manage` can act (recheck/reconnect), the banner says 「請店主重新連結」
  otherwise.
- **MCH-OPEN-3 Mail recipients.** Recommended: store owners only (same as `merchant_new`); admins added later on request.
- **MCH-OPEN-4 IG app-level subscription.** Recommended: assumed present (`ok_app_level_assumed`), verified once per
  deployment by the LIVE probe; a later unit may read `GET /{app-id}/subscriptions` with the app token if claims-worker
  gets `COMMERCE_META_APPS_JSON` (not mounted there today).
- **MCH-OPEN-5 `review_required` source.** Recommended: config `COMMERCE_META_ADVANCED_ACCESS` (owner sets it after App
  Review); default empty, so production honestly shows 「僅測試帳號」 until review passes.
- **MCH-OPEN-6 Pre-live recheck (§5.3).** Recommended: yes, as an optional call by LC-B1/A5-3, not a dependency of this unit.
- **MCH-OPEN-7 Where `Derive` and the interface live if LC-B3 merges first.** Recommended: `internal/metaconnect`
  (`capability.go`, `derive.go`); LC-B3 owns them until W1-01B merges, then W1-01B; no copy in `internal/inbox`.

## 14. Units

| Unit | Owner | Covers | Write paths | Migration | Gate | Depends |
| --- | --- | --- | --- | --- | --- | --- |
| W1-01B backend | DeepSeek V4-Pro | §2–§9 | `internal/metaconnect/{health*.go,derive.go,capability.go (reader impl only)}`, `internal/integrations/metareply/probe.go`, `cmd/claims-worker/main.go` (job + worker registration only), `internal/httpapi/meta_health.go`, `internal/notify/{merchant_alerts.go}` + `internal/notify/worker.go` (one extra claim call in the loop), `cmd/api` reader wiring (one line), `migrations/0127_meta_connection_health.sql`, `tests/foundation/meta_health_*_test.go` (author smoke) | 0127 | `--meta-health` MCH01–MCH10, G07, `bash scripts/dev/check-gates.sh` | contract frozen; LC-B3 interface (§7) agreed |
| W1-01U banner + card | Codex | §5.2 banner, §6 card rows and actions, BFF B1/B2 | `apps/admin/components/{MetaHealthBanner.tsx,MetaConnect.tsx (capability rows + 重新檢查 only)}`, `apps/admin/lib/meta-health-*.ts`, BFF allowlist, i18n zh-TW/zh-CN/en; integrator mounts the banner in `WorkspaceFrame.tsx` | — | `--browser-meta-health` MCH11 + click ledger; `--browser-meta-connect` stays green | W1-01B API frozen; serial with other `MetaConnect.tsx` edits |
| W1-01T independent | Kimi K3 | MCH02/03/05/06/08 adversarial, written before reading the diff | `tests/foundation/meta_health_gate_test.go` | — | red run then green | W1-01B delivered |
| MCH12 LIVE | Claude Sonnet + Kimi WebBridge | §11.3 | `output/meta-health-probe/` | — | MCH12 | W1-01B merged, owner preconditions |

## 15. Amendment text for the integrator (recorded at freeze)

- `meta-claims-intake-v1.md`, "Amendment: Merchant connect": clause 4 — "A probed Graph 190 or code 100 on the Page node
  (meta-connection-health-v1 §4.3) also flips the connection `active → reauth_required` through
  `integration.record_meta_health`; the same one-way rule applies." Clause 6 — append the §8 privilege delta.
- `0108` table comment of `integration.meta_connections`: writers += `integration.record_meta_health (status only)`.
- `storefront-v2.md` §E: "Merchant operational alerts (non-order) use `notify.merchant_alerts` (meta-connection-health-v1
  §5.2), sharing the §E3 budget."
