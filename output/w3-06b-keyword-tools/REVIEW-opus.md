# W3-06B keyword tools and match simulator: independent review (Opus)

- Reviewer: Claude Opus 5.5. Read-only: no edits to the code and no commits. Only this file was written.
- Reviewed tree: `unit/w3-06b-keyword-tools` at HEAD `a9cd7088`. The code is identical to `45574380`, because `a9cd7088` adds logs only. Base `5862caf8`, merge of r3/integration `5194559f`.
- Unit-only diff: `git diff 5194559f^2 HEAD`. It touches 22 files. No existing test file is modified. The only existing source file it modifies is `internal/httpapi/claims.go`.
- Included the follow-up commit `45574380`: when `match_mode` is omitted, the simulator uses the window's mode, and falls back to EXACT when the session has no window.
- Commands I ran myself:
  - DB-free, at HEAD: `go test -count=1 ./internal/claims/ ./internal/claims/grammar/ ./internal/httpapi/ ./tests/claims/...` exited 0.
  - Grammar probes on a scratch copy of `internal/claims/grammar`.
  - Four DB-free mutations on a `git archive HEAD` scratch export (results in section 5).
  - PG gates: NOT_RUN by me. The author's `green-mode.log` binds a run to 45574380, and CI runs PG.

## Verdict: **MERGE**. No P0 and no P1 findings. The P2 items below should be handled in a small follow-up or by the integrator.

---

## 1. Simulator purity and parity

**Purity: confirmed.**
- `SimulateClaim` (`internal/claims/simulate.go:63-92`) issues only these reads:
  - `authorize`, which calls resolve_access;
  - `requireSession`, a plain SELECT;
  - `readWindow(...,"")` at :73, with no lock;
  - `readOffers` at :77;
  - `SELECT clock_timestamp()` at :82.
- It makes no `command.Run`, no Audit, no River enqueue, no Graph or metabridge call, and no INSERT, UPDATE or DELETE.
- The transaction is the standard `platform.WithScope` READ COMMITTED **read-write** transaction (`internal/platform/platform.go:629`). It is not a database-enforced read-only transaction (see P2-6).
- The PG gate checks purity with a digest. It digests the `live`, `claims`, `storefront` and `inventory` schemas and counts `ops.command_results` rows over 24 simulations (`tests/foundation/keyword_tools_test.go:219-235`).

**Parity: confirmed.** The order is ParseForIngest → effective → lookup → offerReason, the same as ingest:
- Simulator, `simulate.go:122-147`:
  - `effective(grammar.ParseForIngest(text), mode)`;
  - a NO_MATCH early return;
  - a keyword lookup;
  - `offerReason(p, mode, offer{...}, now)`.
- Ingest:
  - `manual.go:74` calls `grammar.ParseForIngest`;
  - `ingest.go:166` applies `effective(p, w.mode)`;
  - :167 is the NO_MATCH branch;
  - :180-186 is the lookup, `keyword=$4 FOR SHARE`;
  - :190 calls `offerReason(p, w.mode, o, in.OccurredAt)`.
- Small differences that do not affect the result:
  - The lookup is a Go loop (`simulate.go:130-135`), while ingest uses SQL equality. Stored keywords are CHECK-constrained ASCII `^[A-Z0-9]{1,16}$` (0060:53), so the two lookups are equivalent.
  - `now` is `clock_timestamp()`, exactly as `manual.go:110` stamps `occurred_at`.
- The Meta path uses the same `ParseForIngest`, `effective` and `offerReason` (`internal/integrations/meta/claim_intake.go:177`). It resolves the offer by the staged id, and the parity gate does not exercise it. DELIVERY lists this risk, which is acceptable.

**Parity gate: it is real.**
- The gate is `TestSimulateParityWithIngest`, at `keyword_tools_test.go:114-183`.
- For each mode it runs `h.claim`, which is `RecordManualClaim` (`live_claims_test.go:281-288`), with a fresh actor label per comment, so BUNDLE_LIMIT cannot be reached.
- It covers 3 window modes × the kw-v1, kwc-v1 and kwc-adversarial corpora (more than 150 comments, enforced at :120), plus 5 unresolved heads.
- It compares outcome, reason, offer id, keyword and quantity, and checks the target against the quantity actually set (:158-163).
- It asserts that the corpus reached ACCEPTED and all six reachable reasons (:174-182), so an empty corpus cannot pass.
- It also asserts that the omitted-mode result equals the window-mode result for every comment (:146).
- Mutation evidence:
  - `fault-parity.log` (mode gate removed) goes red on `#A01` in EXACT.
  - `fault-omitted-mode.log` (fallback ignored) goes red.
  - My DB-free mutation M1 (dropping `effective`) turns `TestSimulateDecisionTable` and `TestSimulateParsedFields` red.

**Reasons that are not simulated.** These are WINDOW_CLOSED (including comments older than `opened_at`), BUNDLE_LIMIT, RATE_LIMITED, sold-out, and the previous quantity or versions.
- The contract documents all of them (`contracts/live-keyword-claims-v1.md:1323-1325`).
- The response carries `window_state` and `window_match_mode`.
- There is **no machine-readable flag** for the others (see P2-1).
- Sold-out does not change the claim outcome. Inventory is not locked at claim time, and W3-04B only changes the reply, so the outcome parity holds.

## 2. Conflict check

- **Normalisation is the frozen grammar, not NFKC.**
  - `grammar.NormalizeKeyword` is the width map + trim + ASCII upper (`grammar.go:110-116,167-181`). The package explicitly excludes NFKC (`grammar.go:7`).
  - The check uses the same function as ingest, so the check and ingest cannot diverge.
- **Collisions are enforced in SQL, not only checked.**
  - `live.offers.keyword CHECK (keyword ~ '^[A-Z0-9]{1,16}$' AND octet_length=char_length)` together with `UNIQUE(tenant_id,store_id,session_id,keyword)` (0060:53,63) allow only canonical ASCII, unique per session whether active or not.
  - Two offers can therefore never collide after folding. `normalization_collision` is UX only.
- **`quantity_lookalike` is correct.**
  - Probe results: `a1x2` → kw-v1 MATCH head `A1X2` qty 1; `我要A1x2` → kwc-v2 MATCH `A1X2`; `A1 x2`, `A1*2` and `A1×2` → NO_MATCH. This matches R09 at contract :172.
  - Pairs found by `quantityLookalike` (`keyword_tools.go:200-206`) and the skip in `nextKeywords` are correct.
  - Not reporting A1/A10 and H1/2H1 is correct: kwc carves one maximal fragment, and `2H1+1` resolves to head `2H1`.
- **The `numeric_in_contains` warning is justified, but its stated reason is wrong** (P2-2).
- `keyword_taken`, `duplicate_in_request` and `invalid_keyword` are checked against all offers in the session, active or not. That is correct because the unique constraint ignores `active`.

## 3. Batch offers (the money-adjacent part)

**Rename = retire + create.**
- The UPDATE `active=false,version+1` is guarded by `version=$5` (`keyword_tools.go:483-491`).
- The INSERT copies `o.SKUID`, `o.MaxQuantityPerClaim` and `o.LivePriceMinor` (:500-502), read under the row lock (:419-431).
- `live.offers` columns: tenant_id, store_id, id, session_id, keyword, sku_id, max_quantity_per_claim, active, activated_at, version, principal_id, created_at, updated_at, live_price_minor (0060:50-61, 0092:51). Nothing commercial is lost.
- The PG test asserts the same SKU, the cap of 4, live price 777, version 1 and a new id (`keyword_tools_test.go:401`).

**Live price grant and use rows.**
- Every one of these hangs off claim lines or bundles:
  - `claims.live_price_uses` is keyed by (bundle_id, offer_id) and written only from claim lines (0105:21,145);
  - `claims.merchant_origin_grants` is keyed by bundle (0129:49);
  - `cart_lines.claim_offer_id` comes from claim lines.
- A rename is refused with `has_claims` when **any** `claims.lines` row exists for the offer (:441-457, :583).
- So a renamable offer cannot have grant, use or cart-origin rows. There is nothing to carry over, and no price can drift.
- My mutation M2 (dropping `has_claims`) turns `TestKeywordToolsPlanBatch` red.

**offer_timeline:** it is not carried over. 'featured' rows stay on the retired offer, which is correct for append-only history. However, the console's `readRecommended` (`internal/live/console.go:314-318`) then points at an inactive offer (P2-4).

**All-or-nothing.**
- `planBatch` decides every item before any write (:469-474).
- Any conflict returns `applied=false` with no writes. The digest check in `keyword_tools_test.go:463` asserts this, and the `fault-batch.log` mutation goes red.
- A write error later in the batch (for example a 23505 from a concurrent M4 reactivation on the same SKU) returns an error, and `WithScope` rolls back the whole transaction.
- Optimistic versions are checked twice: in the plan (:572) and in the UPDATE guard.

**Lock order against ingest: safe.**
- Batch order: command advisory → `claims-offers` advisory (same as CreateOffer, `merchant.go:397`) → touched offers `FOR NO KEY UPDATE ORDER BY id` → unlocked reads of `claims.lines` and the catalog → writes.
- Ingest order: claim-source advisory → window FOR SHARE → **one** offer FOR SHARE → bundle → line (`ingest.go:9-11`).
- FOR SHARE conflicts with FOR NO KEY UPDATE. Ingest never holds two offer rows, and the batch never takes window, bundle or line locks, so no cycle is possible.
- If the batch locks first, ingest's FOR SHARE re-reads the updated row after the batch commits (EvalPlanQual), and the comment gets OFFER_INACTIVE.
- If ingest locks first, the batch waits. Its next statement sees the new line under READ COMMITTED, so the rename gets `has_claims`.
- `INSERT INTO claims.lines` exists only in `ingest.go:224`, so there is no other writer to race with.
- No concurrent ingest-vs-batch test exists (P2-5).

**OPEN window.** The amendment explicitly allows a rename during OPEN (contract :1348). That is consistent with M3/M4, and it satisfies arch §5 ("an enabled keyword is not silently re-pointed to another SKU": the new keyword gets the same SKU, and the old keyword is retired and reserved).

**One receipt and one audit row.**
- `command.Run(..."live.claim.offer.batch"...)` (:382) writes one receipt.
- `AuditDetails("live.claim.offer.batch.applied", counts)` (:518) writes one audit row, and only when the batch is applied.
- The PG test checks the audit row and that it carries no keyword (:415-421), and checks replay and same-key-different-body handling (:423-433).

## 4. Auth

- Permissions:
  - simulate, check and next use `claimsScoped(pool,"live:read")` (`internal/httpapi/keyword_tools.go:36,45,55`). The functions re-check `readPermission` before and after the work (`simulate.go`, `keyword_tools.go:102,113,246,260`).
  - batch uses `claimsBodyRoute`, which requires live:manage (`claims.go:239`). `BatchOffers` re-checks `managePermission` before, inside and after `command.Run` (:365,386,402).
- Tenant and store come from `identity.resolve_access` for the bearer token and the path store (`platform.go:651-675`). Unknown body keys such as `tenant_id`/`store_id` are rejected with 400.
- `requireSession` filters on the scope's tenant and store (`claims/claims.go:145-150`). A session from another store returns 404 for simulate, check, next and batch (`keyword_tools_test.go:245,359-360,505`).
- Batch offer ids are also filtered by `session_id` (:419-421), so a foreign offer gets `offer_not_found`.
- **The DB-free router test** (`internal/httpapi/keyword_tools_test.go`) builds the full `NewHandler`, so a ServeMux conflict would panic.
  - It covers: key rules (optional on read POSTs, required on batch, forbidden on GET), strict bodies, the query whitelist for `next`, 405s, and the PATCH `offers/{id}` route still working next to the literal `offers/batch`.
  - It cannot check permissions, because a nil pool answers 401. The PG tests cover permissions: HTTP 403 for simulate, and function-level `ErrForbidden` for check, next and batch.
- The `claimsRoute` refactor (`claims.go:186-227`) behaves the same as before for existing routes (GET forbids a key; any other method requires one).

## 5. Tests

- **Red evidence:**
  - Compile red: `red.log`. Route-mount red: `red-http.log`.
  - Three fault injections with red logs: parity mode gate, batch apply-despite-conflict, omitted-mode fallback.
- **Green evidence:** `green-mode.log`, which carries SHA 45574380 in its header.
- **No weakened pins:** `git diff --name-status 5194559f^2 HEAD` shows only new test files. No existing test, fixture or threshold was changed.
- **My DB-free mutations:**

| Mutation | Result |
|---|---|
| M1: drop the mode gate | red |
| M2: drop `has_claims` | red |
| M4: `numeric_in_contains` in every mode | red |
| M3: rename without live price | not caught DB-free; the PG assert at `keyword_tools_test.go:401` covers it |

---

## P0
None.

## P1
None.

## P2

**P2-1. The simulator outcome is not a "live" prediction, and only the window state is machine-readable.**
- Evidence: `simulate.go:85-87`; `keyword_tools_test.go:195` asserts `Outcome=ACCEPTED` while `WindowState=CLOSED`; contract :1323-1325.
- Why it matters: the integrator ruling says "the merchant sees exactly what would happen live", but `outcome` is the rule outcome only. BUNDLE_LIMIT and RATE_LIMITED have no flag. The separate UI unit has no gate that forces it to show WINDOW_CLOSED.
- Fix, either one:
  - add `live_reason` (for example `WINDOW_CLOSED` when the window is CLOSED) or `caveats: ["BUNDLE_LIMIT","RATE_LIMITED"]` to the response; or
  - at minimum, make "render WINDOW_CLOSED when `window_state`=CLOSED" an acceptance item of the UI unit (W3-U2).

**P2-2. The stated reason for `numeric_in_contains` is wrong.**
- Evidence: contract :1331 says that `20+5 minutes` "carries a valid KW+N fragment".
- Probe results: `20+5 minutes` → NO_MATCH, because `MINUTES` is a second fragment. `20+5!` → kwc-v2 MATCH `20`×5. `我要101+2` → NO_MATCH (kwc-v2 rule 3: an all-digit head with `+N` needs an all-ASCII comment).
- Why it matters: the warning is still right, because numeric keywords behave badly under CONTAINS. But the explanation the merchant UI will copy is false, and it hides the real effect: Chinese `+N` sentences never match a numeric keyword.
- Fix: reword the text, for example "a numeric keyword with +N only matches an all-ASCII comment (我要101+2 is NO_MATCH); an ASCII line like `20+5!` matches".

**P2-3. Defaults and stale text are inconsistent across the tools after 45574380.**
- Evidence:
  - The keyword check defaults to EXACT (`keyword_tools.go:98`).
  - Batch warnings are always computed in EXACT (`keyword_tools.go:564`), so `numeric_in_contains` never appears for a CONTAINS session.
  - `internal/httpapi/keyword_tools.go:5` still says "an omitted match_mode is EXACT".
  - The brief still says `Status: IMPLEMENTING` (`docs/delivery/units/w3-06b-keyword-tools.md:3`).
- Fix:
  - Pass `window.MatchMode` as the check's fallback and as the mode for batch warnings, or have the UI always send the mode.
  - Update the header comment and the brief status.

**P2-4. A rename does not carry the 'featured' state.**
- Evidence: `live.offer_timeline` is append-only (0122:58-79). `readRecommended` returns the latest featured `offer_id` (`internal/live/console.go:314-318`).
- Why it matters: after renaming a featured offer, the console's "recommended" shows the retired, inactive offer. Per-offer results and sales also split across the old and new offer ids. This is a merchant-console issue only; no buyer or money effect.
- Fix: in the rename branch, append a `featured` row for the new offer when the old offer is the current recommended one, or document that the host re-features after a rename.

**P2-5. There is no concurrency test of batch versus an in-flight ingest.**
- Evidence: the only race test is CreateOffer vs rename (`keyword_tools_test.go:476-496`).
- The lock reasoning holds (section 3), but nothing pins it.
- Fix: hold an ingest transaction on the offer FOR SHARE, run the batch rename in parallel, and assert either `has_claims` or (rename applied and ingest → OFFER_INACTIVE).

**P2-6. Purity is enforced by code review and a digest, not by the database.**
- Evidence: `WithScope` opens a read-write transaction. The purity digest leaves out `ops.audit_events` and River.
- Fix:
  - Call `SELECT set_config('transaction_read_only','on',true)` at the start of `SimulateClaim`, `CheckKeywords` and `NextKeywords`. Support mode already proves `resolve_access` works read-only (`platform.go:691`).
  - Add `ops` and `river` to the digest.

**P2-7. Deactivating a claimed offer drops the live price for buyers who have not checked out.**
- This is existing M4 behaviour, but batch makes it 50× easier, and the contract's `has_claims` hint ("deactivate and create instead", :1343) steers merchants toward it.
- Evidence: `claims.live_prices` requires `o.active` (0129:445). Lines of an inactive offer revert to the catalog price at checkout.
- Fix: warn on the response (`deactivate_has_claims` with a live price), and qualify the contract hint.

**P2-8. Evidence binding.**
- `fault-parity.log` and `fault-batch.log` were produced on 5b70e520 and carry no SHA header. `simulate.go` changed after that (45574380).
- The mutation still applies, but per the evidence rule, re-run them on the final SHA, or add the SHA to the log header.

**Nits.**
- On an archived session, a batch that is a no-op or conflicts still returns 200 with a receipt. Only a batch that writes gets the 422 promised in contract :1348.
- The parity gate could also compare `grammar_version` and `kind` at no extra cost.
