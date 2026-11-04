# PR394 DB boundary first repair — static author delivery

Evidence level: **SOURCE / E1 only**. Product/runtime acceptance **NOT_RUN**. The 12 accepted real-PG RED cases have **no GREEN-after-fix result yet**. No HTTP exploit is claimed.

- task_id: `26e68645-c2d0-4824-9dc3-bc4503743537` (root claimed; child did not claim or delegate)
- author: `/root/manual_discount_db_fix_0958`, requested Sol/high; actual provider/model ID **UNKNOWN**
- base_commit: `95368d18b8827d7b1581d34192d0e21e461496ed`
- branch: `codex/pr394-offline-diagnostic`
- worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/codex-pr394-offline-diagnostic`
- assigned/changed paths: `migrations/0220_manual_order_discount.sql`, `internal/merchanttools/discount.go`, this report only
- migration number/ownership: unchanged 0220, unmerged PR migration; root integrator owns final migration/contract review and integration
- rollback point: clean assigned worktree at base above; changes are a local commit only, no migration was executed

## Authority and cause

Read repo AGENTS, architecture sections 0/1/2/26/27, the related task DAG and P0 invariants. DAG is acyclic; T11 depends on T04/T06, with T03 identity downstream of T02/T01. Existing dependency status labels are slice labels and do not establish production acceptance. This is a frozen card within the existing manual-order pipeline, not new upstream/downstream parallel implementation.

Authority: root's exact current dispatch plus original `contracts/cart-quote-v1.md:188–204`, `contracts/storefront-v2.md:626–655`, `contracts/invariants.json` and actual base source. Contract hashes: cart quote `f38c8e06fa40a1d85b03ff87f0d8c4e1d99e4390c92c97db7c66d6bae117b074`; storefront `37433f7c7c4a86485d91c429984ba2baeba18f931788125cbdf91fa1d726709f`; invariants `ed4f7fcaad364978c3593a7cbafd45c96d75007cc38de2bd254fa9ca8c652f5f`; task DAG `87e25036acafb5b708e944ad4f4adcbea53d1ef3867bdf0f1c239d9fadc2cf05`.

Independent source report read at root `output/integrator/tools/logs/ci-evidence-1238/review394-D-validation-0904.md`. Root supplied accepted runtime RED: G41421631055 / g05 job125199966385, own2d18c560be5faa4a9d6593e14e1793db5902ea22. Raw log `output/integrator/tools/logs/ci-evidence-1238/394-db-red-g05-own-0956.log`, supplied SHA256 `04d603f6e10e80e8a65782f762e9892514906e1edbda42782d3447a074a3076c`. This child did not rerun or relabel those results.

Root causes: runtime table INSERT bypassed bearer/permission checks and permitted preset quote binding; the callable binder omitted authorization-principal equality and the frozen adjustment match. Existing HTTP permission checks did not secure those separately callable DB boundaries.

## Implementation

1. Runtime keeps SELECT only. A narrow `fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint)` derives tenant/principal from `resolve_access`, checks both existing permissions through `merchant_access_denied`, exact merchant GUCs and absence of buyer context. It accepts no principal, tenant or quote binding input. Definer role gets only the seven explicit INSERT columns; initial quote is constrained NULL by its dedicated INSERT policy. Separate principal-scoped SELECT/UPDATE policies avoid a permissive ALL policy bypass. FORCE RLS, foreign keys, buyer/checkout SELECT-only policy and table constraints remain intact.
2. Fixed Go INSERT becomes the definer call with a SHA256 bearer and the same normalized request digest. Existing ON CONFLICT DO NOTHING, saved request-hash byte comparison and `idempotency_conflict` response remain; no general SQL abstraction, new engine, key, retry or provider operation is added.
3. Binder requires `d.principal_id = resolved principal` and compares the complete frozen adjustment JSON to exactly one expected `manual_discount` object (name, amount, server quote currency), or empty/missing adjustments for the no-discount tombstone. JSON null, wrong types/kind/currency/shape, extra keys/lines, and amount strings/scaled decimal text are refused. Quote column access adds only `currency,snapshot`; snapshots are read, never modified. The existing NULL-or-same-quote guard remains set-once.
4. Both potentially waiting writes perform the existing volatile permission fence after the write. This is a minimal source improvement naturally accompanying the new authorization operation and binder; **issue #505 is not closed**. A lock-wait revocation test is **NOT_RUN**, so no race-resolution claim is made.

The existing `identity.read_order_adjustments` function is byte-preserved. No tests, fixtures, oracle, contract, gate, lockfile, pin or other migration were edited.

## Actual static commands and evidence

`gofmt -w internal/merchanttools/discount.go` exited 0 earlier. Targeted source/contract reads exited 0 except one initial root-shell wildcard read: `migrations/0094*` did not exist in that root checkout and zsh exited 1; corrected exact migration reads in the assigned tree exited 0. An initial filtered coord list returned no rows; the unfiltered list confirmed root's working task. No claim was taken from the queue.

An inline Python static-preservation check exited **0**, comparing every test path and the frozen test bytes against base, exact Go replacement against base, unchanged SQL table/FK/FORCE-RLS prefix, buyer SELECT policy, order-adjustment projection, definer signatures, guard presence and assigned-path subset. This is source inspection, not a PostgreSQL or behavioral test:

```text
PASS: all test paths unchanged; manual_discount_test.go SHA256=2c44a9fd310a50db3338cff002d5f198ce961f6f210200a3074beab1c94911f2
PASS: table DDL/FKs/FORCE RLS, buyer/checkout read policy, order-adjustment projection byte-preserved
PASS: Go diff is exactly fixed INSERT to narrow definer; request digest/idempotency comparison and binder caller unchanged
PASS: static boundary guards, fixed definer signatures/search_path/PUBLIC revoke and post-write fences present (not runtime proof)
PASS: tracked changes confined to assigned paths
```

Recorded final command receipts before this report was written:

```text
$ gofmt -l internal/merchanttools/discount.go
exit_code=0
(no output)
$ git diff --check
exit_code=0
(no output)
$ git diff 95368d18b8827d7b1581d34192d0e21e461496ed -- tests
exit_code=0
(no output)
$ shasum -a 256 tests/foundation/manual_discount_test.go migrations/0220_manual_order_discount.sql internal/merchanttools/discount.go
exit_code=0
2c44a9fd310a50db3338cff002d5f198ce961f6f210200a3074beab1c94911f2  tests/foundation/manual_discount_test.go
a0cb0912b9dcf1830e487c66e4060a96d7005b80118adf57ef45ead5d0b6293a  migrations/0220_manual_order_discount.sql
151f02bca1e6bb7616df57b438b229e562252f530f645173a479fd41745e2ffc  internal/merchanttools/discount.go
$ git diff --stat
exit_code=0
 internal/merchanttools/discount.go        |  4 +-
 migrations/0220_manual_order_discount.sql | 87 ++++++++++++++++++++++++++-----
 2 files changed, 75 insertions(+), 16 deletions(-)
```

Neither pglast nor sqlparse is installed (read-only import availability check exited 0). No SQL parser, compiler or runtime result is inferred from text inspection.

## Shared coordination and handoff

`pr394:author` was acquired by this author before edits, lease to 10:29:20Z, and is released after local commit. Humaux `humaux:live-commerce:dispatch` first returned acquired=true; a `[fix] PR394 DB boundary首修1静态候选0958` memory was stored (grep-confirmed `743896e2-790f-42b7-a417-cffdfdcf2db9`). A canvas update without required Mermaid failed and wrote nothing. Lock was released; the next acquire returned BUSY (c4-edit-design-0917), so all shared writes stopped. Root explicitly accepted responsibility for final canvas and code_index/code_memory_link after integration. No alternative lock or parent-canvas overwrite was attempted.

## Remaining gates and risk

**NOT_RUN:** Go compile/test/race, SQL parse/migration/PG integration, the 12 RED-after-fix GREEN assertions and all positive controls at this candidate SHA, HTTP/browser/end-to-end, #505 lock-wait race, CI dispatch, provider calls, production actions. No push. No local background process, fixture, container or port was created.

**UNKNOWN:** effective SQL execution, current candidate full PR acceptance, all new cross-family/independent review outcomes. Earlier D and other family reviews do not approve this new code. Root must run the unchanged real-PG negative and matching positive controls plus affected/full mandatory CI on this exact candidate, inspect ACL/RLS/function signatures and grants, then obtain independent high-risk and cross-family review with zero unresolved P0/P1 before integration. PostgreSQL privilege/RLS behavior and new function invocation are the principal runtime risks. No acceptance standard was weakened.

Post-report check: initial `git diff --cached --check` exited **2** on whitespace in the embedded diff only (space before Go tabs and a blank diff context line). The first local commit was created despite that check. The report display was normalized to spaces with trailing whitespace removed; the product bytes were unchanged, and the local commit is amended only after a clean staged check. Embedded diff below is complete with display whitespace normalized.

## Complete product diff against base

```diff
diff --git a/internal/merchanttools/discount.go b/internal/merchanttools/discount.go
index dd046454a..5b130181f 100644
--- a/internal/merchanttools/discount.go
+++ b/internal/merchanttools/discount.go
@@ -29,8 +29,8 @@ func (m *ManualOrders) authorizeDiscount(ctx context.Context, token, ownerID, st
        if in.Discount != nil {
            name, amount = in.Discount.Name, in.Discount.AmountMinor
        }
-       if _, err := tx.Exec(ctx, `INSERT INTO storefront.manual_order_discounts(tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor)
-   VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(tenant_id,store_id,owner_id) DO NOTHING`, s.TenantID, s.StoreID, ownerID, s.PrincipalID, hash[:], name, amount); err != nil {
+       bearerHash := sha256.Sum256([]byte(token))
+       if _, err := tx.Exec(ctx, `SELECT fulfillment.authorize_manual_order_discount($1,$2::uuid,$3::uuid,$4,$5::text,$6::bigint)`, bearerHash[:], s.StoreID, ownerID, hash[:], name, amount); err != nil {
            return err
        }
        var saved []byte
diff --git a/migrations/0220_manual_order_discount.sql b/migrations/0220_manual_order_discount.sql
index 94a962a53..1c9405ed3 100644
--- a/migrations/0220_manual_order_discount.sql
+++ b/migrations/0220_manual_order_discount.sql
@@ -20,39 +20,87 @@ CREATE TABLE storefront.manual_order_discounts (
 ALTER TABLE storefront.manual_order_discounts ENABLE ROW LEVEL SECURITY;
 ALTER TABLE storefront.manual_order_discounts FORCE ROW LEVEL SECURITY;
 REVOKE ALL ON storefront.manual_order_discounts FROM PUBLIC;
-GRANT SELECT,INSERT ON storefront.manual_order_discounts TO commerce_runtime;
+GRANT SELECT ON storefront.manual_order_discounts TO commerce_runtime;
 GRANT SELECT ON storefront.manual_order_discounts TO commerce_buyer_runtime,commerce_checkout_runtime;
 GRANT SELECT,UPDATE(quote_id) ON storefront.manual_order_discounts TO commerce_auth;
+GRANT INSERT(tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor)
+ ON storefront.manual_order_discounts TO commerce_auth;
 GRANT USAGE ON SCHEMA storefront TO commerce_auth;
-GRANT SELECT(tenant_id,store_id,owner_id,id) ON storefront.quotes TO commerce_auth;
+GRANT SELECT(tenant_id,store_id,owner_id,id,currency,snapshot) ON storefront.quotes TO commerce_auth;
 CREATE POLICY manual_binding_quote_read ON storefront.quotes FOR SELECT TO commerce_auth
  USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid);
-CREATE POLICY merchant_authorization ON storefront.manual_order_discounts TO commerce_runtime
+CREATE POLICY merchant_authorization ON storefront.manual_order_discounts FOR SELECT TO commerce_runtime
  USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
-  AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL)
- WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
-  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
-  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid);
+  AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL);
 CREATE POLICY buyer_authorization_read ON storefront.manual_order_discounts FOR SELECT TO commerce_buyer_runtime,commerce_checkout_runtime
  USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
   AND owner_id=nullif(current_setting('app.buyer_id',true),'')::uuid
   AND current_setting('app.principal_id',true)='');
-CREATE POLICY authorization_binding ON storefront.manual_order_discounts TO commerce_auth
+CREATE POLICY authorization_read ON storefront.manual_order_discounts FOR SELECT TO commerce_auth
  USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
-  AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL)
+  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
+  AND nullif(current_setting('app.buyer_id',true),'') IS NULL);
+CREATE POLICY authorization_insert ON storefront.manual_order_discounts FOR INSERT TO commerce_auth
  WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
   AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
-  AND nullif(current_setting('app.principal_id',true),'') IS NOT NULL);
+  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
+  AND nullif(current_setting('app.buyer_id',true),'') IS NULL AND quote_id IS NULL);
+CREATE POLICY authorization_binding ON storefront.manual_order_discounts FOR UPDATE TO commerce_auth
+ USING (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
+  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
+  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
+  AND nullif(current_setting('app.buyer_id',true),'') IS NULL)
+ WITH CHECK (tenant_id=nullif(current_setting('app.tenant_id',true),'')::uuid
+  AND store_id=nullif(current_setting('app.store_id',true),'')::uuid
+  AND principal_id=nullif(current_setting('app.principal_id',true),'')::uuid
+  AND nullif(current_setting('app.buyer_id',true),'') IS NULL);
 COMMENT ON TABLE storefront.manual_order_discounts IS
  'storefront-v2 G3 amendment; immutable merchant-only request authorization, including null-discount tombstone; quote_id bound once. No buyer PII or plaintext capability. Retain with financial order/quote evidence; no automatic deletion or historical money rewrite.';

+CREATE FUNCTION fulfillment.authorize_manual_order_discount(p_hash bytea,p_store uuid,p_owner uuid,
+ p_request_hash bytea,p_name text,p_amount bigint) RETURNS void
+LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
+DECLARE s record; v_auth_error text;
+BEGIN
+ IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_owner IS NULL
+  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32
+  OR current_setting('transaction_isolation')<>'read committed' THEN
+  RAISE EXCEPTION 'invalid discount authorization' USING ERRCODE='PT400'; END IF;
+ SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
+ IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
+ IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
+ IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
+ IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
+  OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
+  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text
+  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL THEN
+  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
+ v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
+ IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
+ -- The caller cannot choose a principal, tenant, or initial quote binding.
+ INSERT INTO storefront.manual_order_discounts(tenant_id,store_id,owner_id,principal_id,request_hash,name,amount_minor)
+  VALUES(s.tenant_id,p_store,p_owner,s.principal_id,p_request_hash,p_name,p_amount)
+  ON CONFLICT(tenant_id,store_id,owner_id) DO NOTHING;
+ IF NOT EXISTS(SELECT 1 FROM storefront.manual_order_discounts d
+  WHERE d.tenant_id=s.tenant_id AND d.store_id=p_store AND d.owner_id=p_owner AND d.principal_id=s.principal_id) THEN
+  RAISE EXCEPTION 'discount authorization conflict' USING ERRCODE='PT409'; END IF;
+ -- ON CONFLICT may have waited; a revoked scope must roll this transaction back.
+ v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
+ IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
+END $$;
+ALTER FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) OWNER TO commerce_auth;
+REVOKE ALL ON FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) FROM PUBLIC;
+GRANT EXECUTE ON FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) TO commerce_runtime;
+COMMENT ON FUNCTION fulfillment.authorize_manual_order_discount(bytea,uuid,uuid,bytea,text,bigint) IS
+ 'internal/merchanttools only; rechecks merchant inventory:reserve+catalog:read; inserts an immutable unbound authorization or retains its existing request hash for the caller replay check; no buyer, quote, money or provider writes.';
+
 CREATE FUNCTION fulfillment.bind_manual_order_discount(p_hash bytea,p_store uuid,p_quote uuid) RETURNS void
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
-DECLARE s record; v_owner uuid; v_auth_error text; n integer;
+DECLARE s record; v_owner uuid; v_currency text; v_adjustments jsonb; v_auth_error text; n integer;
 BEGIN
  IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_quote IS NULL
   OR current_setting('transaction_isolation')<>'read committed' THEN
@@ -63,18 +111,29 @@ BEGIN
  IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
  IF current_setting('app.tenant_id',true) IS DISTINCT FROM s.tenant_id::text
   OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
-  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text THEN
+  OR current_setting('app.principal_id',true) IS DISTINCT FROM s.principal_id::text
+  OR nullif(current_setting('app.buyer_id',true),'') IS NOT NULL THEN
   RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
  v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
  IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
- SELECT q.owner_id INTO v_owner FROM storefront.quotes q
+ SELECT q.owner_id,q.currency,coalesce(q.snapshot->'adjustments','[]'::jsonb)
+  INTO v_owner,v_currency,v_adjustments FROM storefront.quotes q
   WHERE q.tenant_id=s.tenant_id AND q.store_id=p_store AND q.id=p_quote;
  IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
  UPDATE storefront.manual_order_discounts d SET quote_id=p_quote
   WHERE d.tenant_id=s.tenant_id AND d.store_id=p_store AND d.owner_id=v_owner
-   AND (d.quote_id IS NULL OR d.quote_id=p_quote);
+   AND d.principal_id=s.principal_id AND (d.quote_id IS NULL OR d.quote_id=p_quote)
+   -- Full JSON equality rejects extra lines/keys, wrong kinds/types/currencies,
+   -- JSON null and malformed arrays; a tombstone allows only absent/empty adjustments.
+   AND v_adjustments=CASE WHEN d.name IS NULL THEN '[]'::jsonb ELSE
+    jsonb_build_array(jsonb_build_object('kind','manual_discount','name',d.name,
+     'amount_minor',d.amount_minor,'currency',v_currency)) END
+   AND (d.name IS NULL OR v_adjustments->0->>'amount_minor'=d.amount_minor::text);
  GET DIAGNOSTICS n=ROW_COUNT;
  IF n<>1 THEN RAISE EXCEPTION 'discount quote conflict' USING ERRCODE='PT409'; END IF;
+ -- UPDATE can wait for another binder; fence before the transaction may commit.
+ v_auth_error:=identity.merchant_access_denied(p_hash,p_store,ARRAY['inventory:reserve','catalog:read'],s.tenant_id,s.principal_id,s.authz_revision);
+ IF v_auth_error IS NOT NULL THEN RAISE EXCEPTION 'discount access denied' USING ERRCODE=v_auth_error; END IF;
 END $$;
 ALTER FUNCTION fulfillment.bind_manual_order_discount(bytea,uuid,uuid) OWNER TO commerce_auth;
 REVOKE ALL ON FUNCTION fulfillment.bind_manual_order_discount(bytea,uuid,uuid) FROM PUBLIC;
```
