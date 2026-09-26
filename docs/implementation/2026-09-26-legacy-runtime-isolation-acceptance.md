# Legacy runtime maintenance isolation evidence

Status: **ROOT_CAUSE_REPRODUCED / CONTRACT_FROZEN / PRODUCT_FIX_PENDING**.
This is a LOCAL unit under T11/T06, not a production or whole-SaaS acceptance.
Contract: [legacy runtime isolation v1](../../contracts/legacy-runtime-isolation-v1.md).

## Causal RED

Product base `c45e8a7`; independent test-only commit
`d6448295da9ea9bb06988d1961f81d172fa304b6`. Test author `meta_consumer_pg`,
role `test_worker`, actual model `gpt-6-sol/high`, worktree
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`, branch
`commerce/legacy-maintenance-tests-20260926`. Write paths were only the new
foundation test and its author evidence note; no product edits.

| Run | Result | Log and SHA256 |
| --- | --- | --- |
| Author | Actual PG18/race, exit 1, foundation 8.682s | `/Volumes/data/output/legacy-runtime-isolation-first-20260926.log`; `86d047fd6f303c9128e47b1dcca0e28f0ec86f03d9c7d3e6572c6eaafb03f430` |
| Root independent | Actual PG18/race, exit 1, foundation 9.027s | `/Volumes/data/output/legacy-runtime-isolation-root-real-red-20260926.log`; `8fc59ec77cb532732e33729d69bc4886c25b71e1ae6a5fab297501db164b8176` |

Both used `GOTOOLCHAIN=go1.27.1 go test -race -count=1 -timeout=120s -run
'^TestLegacyRuntimeIsolationExpiryDoesNotMaintainForeignFamilies$' -v
./tests/foundation` with explicitly admitted, task-owned loopback fixtures.
The actual expiry CLI alone discarded linked payment-query and external-operation
stale-running jobs and cleaned a linked terminal payment-reconcile job. Its own
scheduled/retryable/stale/terminal positive controls passed. All compared business
receipts, orders, reservations, observations and facts remained byte-equal.
CLI logs were preserved by the test, including root
`/Volumes/data/output/meta-runtime-process-legacy-expiry-maintenance-1cd697e434f2.log`.
These are real failures, not successful product tests.

Two earlier root launcher logs (`legacy-runtime-isolation-root-red-20260926.log`
and `legacy-runtime-isolation-root-red2-20260926.log`) are **NOT_RUN/SKIP**:
macOS Bash `source <(...)` did not import the fixture environment. A separate
safe probe reproduced the missing shell variable. The corrected launcher used
the inspected fixture-setup segment with explicit required-environment checks;
it did not weaken any test or product fence. Keep all three results distinct.
All task-labelled containers were absent after root completion.

## Independent decision review

Read-only explorer `meta_consumer_go` (`gpt-6-sol/high`) confirmed same payment
kinds/default maintenance policy and the per-profile independent sequence/UNIQUE
cost. Security reviewer `meta_consumer_preflight` (`gpt-6-astra/high`) independently
verified source and author RED hash, then reviewed contract draft hash
`86652a6dd379cbcd4570cc2d9257b92051fb2e94617f33c0f4cbd09b371d8cf7`.
Round one: no concrete P0/P1/P2; bounded freeze approval only. Keep ordinary
`commerce_worker` and state the semantic, not DB-principal, boundary explicitly.

## Pending

- Forward migration and Go wiring; causal GREEN and all LRI01–06 gates.
- Root independent full regression and affected order/payment browser inspection.
- Independent exact-source final signoff before merging the product increment.
- Production backup/impact/approval, real-provider qualification and full SaaS
  acceptance are outside this unit and remain unproven.
