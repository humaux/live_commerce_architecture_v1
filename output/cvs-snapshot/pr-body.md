CVS-SNAP resolves buyer store identity from snapshot 20261028 by (kind, code). The definer returns the exact directory name/address and ignores legacy buyer text. The storefront keeps only code entry and displays canonical identity read-only; the region/search picker remains a separate card.

Migration 0206 owns a platform-wide, definer-only directory of 13615 rows across four chains. The original 20439-row snapshot (including unused UNIMARTFREEZE) is committed at SHA-256 25390346cf849ebd565ecea990efbc663a1a3043c6d73f283c06d8c3debc167c. A standard-library Python generator emits deterministic INSERTs into the existing embedded migration path, so fresh installs and R2 upgrades consume identical SQL. Refresh uses a new migration/snapshot.

Per the integrator ruling of 2026-10-28, 7-11/FamilyMart/OK use six digits; Hi-Life uses four digits or one uppercase A-Z letter plus three digits. This covers every snapshot code, including D-prefix stores. Go, SQL, TypeScript, all three existing locales and both CVS golden tables agree. Added full-snapshot format coverage and D010 exact identity through HTTP/SQL. The source label stays buyer_entered; this does not establish current deliverability or COD.

**Authorized write-path exception:** apps/storefront/tests/backend-parity.test.mjs, only the CVS subtest plus its necessary import. Superseded name/address bounds are replaced with per-kind Go/TypeScript regex source parity and all specified positives/negatives. The RETURN_PATH versus migration 0093 assertion and every unrelated subtest are unchanged. No other outside-card files are changed. Dependency map was regenerated and is byte-identical. No buyerhttp, baseline, dispatcher, gate-adapter or gate inventory changes.

RED (E3, GitHub CI) on tests-only head 03aa811f1f5c37d6e321504507be86d7267eb5a8 / synthetic merge 9f332d7015074823f3ffff2b378b61273c2ce87b:
- Foundation 40628271158/job122618341936: new empty-legacy-fields assertion failed false !== true.
- Gates 40628271224/job122624122933: new OK/Hi-Life code-format assertions failed.
- Gates 40628271224/job122624123599: exact-directory identity, code-only and missing-store assertions failed.
- Gates 40628271224/job122624123324: browser no-name-input assertion failed; job122624122728 WebKit CVS child also failed. Evidence: output/cvs-snapshot/red.log and red-browser-*.log.

Implementation GREEN is pending this PR's own CI. Final run/job IDs will be a PR comment after GREEN, with no evidence-only push. The integrator will resume this lane after the registered workflow runs finish. Independent acceptance is pending; the author's own PASS is E0.

Static checks only: gofmt, go build and go vet for checkout/fulfillment/migrations; node --check on touched .mjs files; depmap regeneration; python3 scripts/check_packet.py (structure only); git diff --check, exit 0. tsc unavailable locally. No local tests, PG, browsers or servers. Existing replay, stale-cart, limits, namespace isolation, version reuse/expiry, ECPay-map tests and browser click/reload assertions remain. Disposable-PG owner-only directory-refresh fixture restores its row via cleanup.

Migration reservation remains 0206 / R2 pin136. OC-05 owns prerequisite 0205/pin135; 0205 is absent in this worktree and was still absent from r3/integration when checked before this push. It is disclosed rather than renumbered or hidden. Any unrelated CI blocker must return to the integrator.

Rollback: revert this card's implementation and tests-only commits on the integration line; no production migration or deployment has been performed. Later snapshot refreshes require a new forward migration. NOT_RUN: implementation CI/GREEN, contractdrift, fresh/R2 database acceptance, independent review, local tests/PG/browser/server, LIVE/provider/production acceptance.

🤖 Generated with Codex (dispatched by Claude Code integrator).
