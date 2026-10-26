<!-- Purpose: PR #194 SA-0 contract review corrections and factual handoff; no implementation acceptance claim.
Depends on: contracts/stripe-platform-account-v1.md §6.2/§6.8/§6.9/§9; contracts/tasks.json SA-0/SA-1/SA-3;
  review-194-5cbedd076.md; migrations/0150_platform_settlement.sql (read-only baseline).
Used by: integrator and the cross-family money reviewer; SA-1 implementer and SA-3 independent test author. -->
# SA-0 PR #194 review correction delivery

- Date: 2026-10-26. Branch: `docs/settlement-contract-freeze`.
- Base / reviewed SHA: `5cbedd0768a9221b88833bfc371cc7ee59a6ea51`.
- Unit: contract/docs/task-scope correction only. No Go, SQL migration, runtime, test, gate-script or provider change.
- Author: Codex-4. Exact current model subversion / reasoning effort: UNKNOWN (not exposed in this runtime; no inference from old sessions).
  Read-only caller inventory: explorer `sa0_scope_inventory`, requested `gpt-6.1-sol` / `medium`, same base and worktree, no write paths.
- Authority: integrator task card dated 2026-10-26; complete review `output/issue-107/review-194-5cbedd076.md`;
  baseline SQL `0150:480–490`, registrar call and exact-identity inventories.

## Review disposition

| Finding | Correction |
| --- | --- |
| P1 recorded-side net / deduplication | §6.8 preserves both branches' stored `settle_net` and the exact balance_txn_id anti-join from 0150. PF17 freezes unmapped→attributed net 1440/count 1 and unsupported NULL-fee net −500/count 1, with separate mutations. |
| P1 stale clean observations mask new discrepancies | §6.8 gives committed observations a database sequence under the existing environment close lock. Every required segment first selects its latest observation, then checks all currencies; the rule includes grandfathered and partially overlapping runs. PF17 proves clean coverage → discrepant resync refuses → clean repair succeeds, without rewriting history. |
| P1 old record_settlement_lines overload remains executable | §6.8 requires 02xx to DROP the exact nine-argument identity and retain only the ten-argument owner/definer/search_path/ACL. SA-1 owns settlement.go:203, platform_settlement_test.go:273,310 and SQL probes, plus the registrar signature inventory and nine-argument unit assertion. PF17 f checks old-identity absence, exact new ACL, missing/invalid manifest rejection and zero writes. |
| P2 PF17 lacks the required fortnight | §6.9 names R `[2026-08-31T00:00:00+08:00,2026-09-14T00:00:00+08:00)`, U and W each 7 days, with W indispensable. The clean close probe is rolled back so it cannot make later attempts idempotent. Error witnesses must reach the intended branch; missing coverage/functions/privileges do not qualify. |

`docs/delivery/units/settlement-api-plan.md` now points to the frozen contract as the unique authority and reflects these rules in
§2, §3.1/§3.2/§3.4 and §4. `contracts/tasks.json` adds the necessary SA-1 caller/ACL files and acceptance details, preserving SA-1/2/3
write-path separation. This card does not allocate a migration number or alter an existing migration.

## Actual validation and limits

- Local command authorized by this card: `python3 scripts/check_packet.py` → **exit 0**.
- Native result at `2026-10-26T10:10:03.316416+00:00`: `PASS_PACKET_STRUCTURE_ONLY`, tasks=27, requirements=22, invariants=24,
  gates=15; DAG includes SA-0 before SA-1/2/3. `delivery_baseline_checked=false`.
- Evidence: **E1 / DESIGN**, packet structure only. The arithmetic above is a frozen test expectation, not executed money-path proof.
- **NOT_RUN:** Go/PG/Node/browser suites, PF16–PF18 behavioral red→green, all named mutations, SANDBOX/LIVE provider calls and new CI
  acceptance. SA-1/SA-3 run their implementation/independent gates on GitHub CI; no other local gate was run.
- **Independent review:** the original review is REQUEST_CHANGES; this correction awaits the integrator's new cross-family review.
  No self-approval, auto-merge or merge.
- Delivery transport: the host denies `.git/worktrees/sa0/index.lock` writes (`git add` exit 128) and the gh CLI cannot reach GitHub.
  The connected GitHub API is available; publish the exact four worktree file blobs as one commit on the same branch, with a checked
  expected parent and a fast-forward ref update. Local Git HEAD remains at the base; the edited files are preserved in sa0. The packet
  check's generated `experiments/results/packet-check.json` timestamp was restored to its original bytes to keep the patch in scope.
- Rollback point: the base SHA above. Revert this contract correction if rejected; there is no data or migration to roll back.
