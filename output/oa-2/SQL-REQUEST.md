# OA-2 SQL request (integrator assigns the number)

Status: REQUEST / NOT_RUN. No file under `migrations/` is added by this unit.
Authority: `contracts/order-attempt-v1.md` §4 (paired outbox/operation), §6g/l,
and `docs/delivery/units/oa-2-order-attempt-notify.md` (write paths and
"Root/post-River migration numbers, exact ACL/COMMENT/upgrade pins and R2 pin:
integrator assigns").

One root migration is required. It adds two reminder-identity columns, one
`void_code` value, seven coordinator definers, two checkout definers, one
`integration.retry_operation` replacement, and the grants/ACL pins below.
No new job kind and no `external_operation_v1` arg change (`{operation_id,
version: 1}` stays exact). Lease expiry reuses `integration.operations.lease_until`
and `inbox.order_payment_notifications.worker_generation`; no lease column is added.

## Schema delta (existing outbox, `migrations/0188_order_attempt_notification.sql`)

```sql
ALTER TABLE inbox.order_payment_notifications
  ADD COLUMN client_reminder_key text,
  ADD COLUMN reminder_intent_key bytea;
-- initial rows keep both NULL; reminder rows require exactly one current key pair.
ALTER TABLE inbox.order_payment_notifications
  ADD CONSTRAINT order_payment_reminder_identity CHECK (
    (kind='initial' AND client_reminder_key IS NULL AND reminder_intent_key IS NULL)
    OR (kind='reminder' AND client_reminder_key ~ '^[A-Za-z0-9_.:-]{1,128}$'
        AND octet_length(reminder_intent_key)=32));
CREATE UNIQUE INDEX order_payment_reminder_version_key
  ON inbox.order_payment_notifications(tenant_id,store_id,order_id,link_version,kind,client_reminder_key)
  WHERE kind='reminder';
-- Cross-version replay: rotation and lost responses reuse this row.
CREATE UNIQUE INDEX order_payment_reminder_intent_key
  ON inbox.order_payment_notifications(tenant_id,store_id,order_id,kind,reminder_intent_key)
  WHERE kind='reminder';
-- The existing void_code CHECK must also admit the cancel/rotation reasons:
-- 'cancelled' and 'rotated'.
```

The existing `guard_order_payment_notification` identity list must include
`client_reminder_key`/`reminder_intent_key` as immutable, and must not allow a
`READY`/`DISPATCHING`/`ACCEPTED`/`DELIVERY_UNKNOWN` row to be promoted from
`VOID`. Extend the same guard so `kind='reminder'` cannot change `link_version`.

## Definers (all `SECURITY DEFINER SET search_path=pg_catalog`, PUBLIC revoked)

Owner: `commerce_order_attempt_owner` (new OA-2 coordinator owner; OA-1 already
created it). Required owner grants: `UPDATE` on `inbox.order_payment_notifications`
and `integration.operations`; `INSERT` on `integration.operation_events`; `SELECT`
on `checkout.orders`, `checkout.order_pay_entries`, `social.conversations`,
`inbox.conversation_state`, `integration.bindings`, `buyer.owners`,
`control.tenants`, `control.stores`; `EXECUTE` on
`integration.claim_operation`, `integration.complete_operation`, `inbox.check_send`,
`checkout.order_payment_payable`; `EXECUTE` on the two `payment_dm_void_ready`
helpers below. It must not be a member of a domain owner and must not receive
direct domain-table DML.

Required `EXECUTE` to `commerce_claims_worker` only:
`inbox.payment_dm_c3`, `inbox.payment_dm_c4`, `inbox.payment_dm_read`,
`inbox.payment_dm_expired`, `inbox.payment_dm_expire`. The API runtime does not
receive these entries.

```sql
-- 1. C3: order -> conversation -> binding -> outbox -> operation.
-- Returns {"disposition":"claimed|busy|terminal|void|blocked_binding",
--          "generation":n,"mode":"dispatch|reconcile","notification":{...}}
-- Pre-C3 refusal pairs VOID with operation CANCELLED/BLOCKED_POLICY, sets
-- void_stage='before_c3' and NEVER sets dispatch_authorized_at.
CREATE FUNCTION inbox.payment_dm_c3(p_operation uuid,p_lease_seconds integer,p_token bytea)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- a. unlocked discovery: SELECT n.*,o.* FROM outbox n JOIN operations o ON
--    o.id=n.operation_id WHERE n.operation_id=p_operation AND o.action='meta.dm_send'
--    AND o.request->>'policy'='order-attempt-notification/v1'. Missing -> PT404.
-- b. take hashtextextended('checkout.order|tenant|store|order',0) ExclusiveLock
--    (same key the C2 Begin/pay-entry path already uses).
-- c. re-read outbox FOR UPDATE; verify order/conversation/binding unchanged.
-- d. lock inbox.conversation_state FOR UPDATE (if conversation_id is not null).
-- e. lock integration.bindings FOR SHARE; verify enabled and semantic_version =
--    o.binding_version and asset/provider identical.
-- f. revalidate under those locks: checkout.order_payment_payable(order,link_version),
--    active tenant/store/owner, pay entry version unrevoked/unexpired, current
--    takeover/window/consent through inbox.check_send(p_operation)='OK', and no
--    same-order meta.dm_send operation created in the last 30 seconds.
-- g. on refusal: operation generation+1 state=CANCELLED (or BLOCKED_POLICY when
--    check_send names consent/policy), outbox VOID/before_c3/void_code, insert
--    operation_events, return disposition='void'. Zero HTTP.
-- h. otherwise PERFORM claim_operation(p_operation,p_lease_seconds,p_token).
-- i. when disposition='claimed': outbox state=DISPATCHING (mode=dispatch) or keeps
--    DELIVERY_UNKNOWN (mode=reconcile); worker_generation=claimed generation;
--    dispatch_authorized_at=coalesce(dispatch_authorized_at,clock_timestamp()).
--    Never clear dispatch_authorized_at and never touch a terminal row.
-- j. return the JSON shape above plus notification fields.
END $$;

-- 2. C4: generation+token fence, paired outcome in one commit.
-- p_state in ('ACCEPTED','DELIVERY_UNKNOWN','VOID').
CREATE FUNCTION inbox.payment_dm_c4(p_operation uuid,p_generation bigint,p_token bytea,
 p_state text,p_code text,p_reference text) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- a. same order->conversation->binding->outbox locks, then claim_operation's
--    operation lock. Verify outbox.worker_generation=p_generation and state
--    DISPATCHING or DELIVERY_UNKNOWN; else PT409/40001 (stale generation).
-- b. PERFORM complete_operation(p_operation,p_generation,p_token,
--    CASE p_state WHEN 'ACCEPTED' THEN 'SUCCEEDED'
--                 WHEN 'DELIVERY_UNKNOWN' THEN 'UNKNOWN' ELSE 'FAILED_FINAL' END,
--    p_code,p_reference). complete_operation already clears the lease.
-- c. outbox state = ACCEPTED | DELIVERY_UNKNOWN | VOID; VOID uses
--    void_stage='after_c3' and the fixed code. worker_generation stays p_generation.
-- d. return the paired notification JSON.
END $$;

-- 3. Lock-free discovery: expired DISPATCHING leases, bounded, one row per tx later.
CREATE FUNCTION inbox.payment_dm_expired(p_limit integer) RETURNS SETOF uuid
LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT o.id FROM integration.operations o
 JOIN inbox.order_payment_notifications n ON n.operation_id=o.id
 WHERE n.state='DISPATCHING' AND o.state='DISPATCHING'
   AND o.lease_until IS NOT NULL AND o.lease_until<=clock_timestamp()
 ORDER BY o.lease_until,o.id LIMIT p_limit
$$;

-- 4. Per-row sweep: lock the same suffix, bump generation, pair UNKNOWN.
CREATE FUNCTION inbox.payment_dm_expire(p_operation uuid) RETURNS jsonb
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- Lock order/conversation/binding/outbox/operation. Recheck DISPATCHING and
-- lease_until<=clock_timestamp(). UPDATE operations SET state='UNKNOWN',
-- generation=generation+1, lease_mode='',lease_until=NULL,lease_token_hash=NULL,
-- result_code='lease_expired'. UPDATE outbox SET state='DELIVERY_UNKNOWN',
-- void_stage='none',void_code=NULL,worker_generation=operations.generation.
-- Insert operation_events. Return the paired JSON. Never renew dispatch authority.
END $$;

-- 5. Narrow paired read; worker roles have no outbox SELECT.
CREATE FUNCTION inbox.payment_dm_read(p_operation uuid) RETURNS jsonb
LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
 SELECT to_jsonb(x) FROM (
  SELECT n.id AS notification_id,n.state,n.operation_id::text,n.order_id::text,
         n.link_version,n.worker_generation,n.void_stage,coalesce(n.void_code,'') AS void_code,
         n.conversation_id::text,n.binding_id::text,n.dispatch_authorized_at
  FROM inbox.order_payment_notifications n WHERE n.operation_id=p_operation) x
$$;

-- 6. Cancel/rotation before C3: caller already holds the order advisory lock.
CREATE FUNCTION inbox.payment_dm_void_ready(p_order uuid,p_code text) RETURNS integer
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- Assert the order advisory lock is held (pg_locks, mode ExclusiveLock).
-- Lock READY outbox rows FOR UPDATE (sorted by id); for each: operation
-- generation+1 state=CANCELLED, outbox VOID/before_c3/p_code, worker_generation=
-- operation.generation, insert operation_events. DISPATCHING/ACCEPTED/
-- DELIVERY_UNKNOWN rows are untouched and their evidence preserved. Return count.
END $$;

-- 7. Read-only copy, under the order lock, never rotation/extension.
CREATE FUNCTION checkout.order_pay_entry_copy_read(p_order uuid,p_expected_link_version bigint)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- Assert the order advisory lock. Require current unrevoked pay entry with
-- link_version=p_expected_link_version, expires_at>clock_timestamp(), order
-- payable and owner active. Return key_version, credential_ciphertext,
-- link_version, issued_at, expires_at. Else typed link_unavailable JSON. No UPDATE.
END $$;

-- 8. Explicit CAS rotation. Refuses only a DISPATCHING paired row.
CREATE FUNCTION checkout.order_pay_entry_rotate(p_order uuid,p_expected_link_version bigint,
 p_key_version text,p_token_hash bytea,p_ciphertext bytea,p_issued_at timestamptz,p_expires_at timestamptz)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- Assert the order lock; SELECT current entry FOR UPDATE; require
-- link_version=p_expected_link_version. If any paired notification is
-- DISPATCHING -> PT409 dispatch_in_flight and zero changes. Revoke the old entry
-- (revoked_at=clock_timestamp()), insert link_version+1 with the Go-sealed
-- credential (expires_at<=min(order deadline,p_issued_at+72h)), call
-- inbox.payment_dm_void_ready(p_order,'rotated') in the same tx, audit
-- actor/store/order/reason/old+new version; never store the URL or contact.
END $$;

-- 9. Point-in-time payable predicate used by C3 and reminders (under order lock).
CREATE FUNCTION checkout.order_payment_payable(p_order uuid,p_link_version bigint)
RETURNS boolean LANGUAGE sql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- Same predicate as 0188 checkout.order_attempt_notification_payable but keyed
-- by order+version, without requiring an attempt plan. Read-only.
$$;

-- 10. Reminder admission: one READY pair + its external_operation_v1 job.
CREATE FUNCTION inbox.order_payment_reminder_admit(p_order uuid,p_link_version bigint,
 p_client_key text,p_intent_key bytea,p_request_hash bytea,p_material jsonb)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
-- Replay by (tenant,store,order,kind='reminder',reminder_intent_key) BEFORE
-- reading the current version; return the original row (any state). Else
-- revalidate payable+check_send; insert READY outbox + operation + River
-- external_operation_v1 job (args exactly {operation_id,version:1}) + send
-- secret exactly like 0188 finish_notification; semantic_key is the Go
-- ReminderSemanticKey and is unique per version. Buyer A's order can never
-- collide with buyer B's. The generic operation API cannot address VOID rows.
END $$;

-- 11. M4: generic retry is forbidden for every paired payment-DM operation.
-- Replace integration.retry_operation before its first mutation:
-- IF EXISTS(SELECT 1 FROM inbox.order_payment_notifications WHERE operation_id=p_id)
--   THEN RAISE EXCEPTION 'not_retryable' USING ERRCODE='PT409'; END IF;
```

## Composition paths the integrator must assign before edits

- `cmd/claims-worker/main.go`: register the payment-DM handler and
  `PaymentDMLeaseSweep` once on startup + every 30s on the main worker; route
  `meta.dm_send` with `request->>'policy'='order-attempt-notification/v1'` to
  `metareply.OrderPaymentDispatcher` before `core.Dispatcher` claims the
  operation.
- `internal/integrations/core/dispatcher.go`: the pre-route must happen before
  `claimOperation` (the generic path locks binding first and is not the OA-2
  suffix). Do not add a second claim.
- Cancel paths: `0129_lc_b6_order_for_buyer.sql` (521–611),
  `0155_returns.sql` (664–795), native cancel `0193_order_attempt_native_cancel.sql`
  and their Go callers: call `inbox.payment_dm_void_ready(order,'cancelled')`
  in the same cancellation Tx after the order lock.
- Copy/rotation: `internal/merchanttools` pay-entry handlers + BFF; copy calls
  only `checkout.order_pay_entry_copy_read`; rotation requires
  `expected_link_version` and calls `checkout.order_pay_entry_rotate`.
  Permission `orders:link:revoke` + `orders:read` for security revocation.
- Reminder: `0144_checkout_reminders.sql` / merchanttools reminder commands and
  the batch admission freeze: use `inbox.order_payment_reminder_admit`; child
  keys derive from `(batch_intent_id,order_id)`.
- No-op hooks, dispatcher-side sleeps, widened timeouts, new retries or a second
  job kind are not acceptable substitutes.

## Pins to increment with the assigned number

- Exact `COMMENT`/ACL pins and `tests/foundation` schema count, plus the R2
  count/comment increment (one per root migration) in the migration itself.
- Actual-login negative tests: runtime/worker direct DML and direct helper
  calls must be refused; only `commerce_claims_worker` may EXECUTE 1–5.

## Why this is blocked

`inbox.order_payment_notifications` has no `UPDATE` grant to any runtime and the
worker has no outbox `SELECT`; the pair trigger requires both rows in one Tx.
The C3 suffix also needs the order advisory lock before
`integration.claim_operation` (which locks binding first), and the reminder
identity columns do not exist yet. Implementing this in Go or as table DML would
bypass the frozen authority boundary.
