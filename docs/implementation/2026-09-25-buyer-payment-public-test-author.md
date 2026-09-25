# Independent BPT01–05 public payment test author evidence — 2026-09-25

Frozen contract: `contracts/buyer-payment-public-v1.md`; baseline `a78a108`.
Isolated branch/worktree: `codex/commerce-buyer-payment-public-tests-20260925`
at `/Volumes/data/worktrees/commerce-buyer-payment-public-tests-20260925`.
Role `test_worker`; delegation specified `gpt-6-sol/high` (runtime model not
independently exposed). Authored only three new test files: commits `dc93caa`
and `acc6325`. Dependency source commit `145f05c` was cherry-picked as
`fe38aed` for execution; no source file was authored or modified by this task.
Test head `acc6325` and runtime Node `v24.15.0`.

| Gate | Exit/observed result | Evidence |
| --- | --- | --- |
| `node --check` on each of the three new `.mjs` files | 0, syntax only | terminal readback |
| `node --test --experimental-strip-types apps/storefront/tests/payment-contract.test.mjs apps/storefront/tests/buyer-payment-server.test.mjs apps/storefront/tests/buyer-payment-client.test.mjs` | 0, 11 pass / 0 fail / 0 skip | `/Volumes/data/output/buyer-payment-public-author-3.log` SHA-256 `a59ad943eb97ebc9c86f3e69ade1e713103fa3d924286a2162666a5e69f314aa` |
| `node --test --experimental-strip-types apps/storefront/tests/*.test.mjs` | 0, 45 pass / 0 fail / 0 skip; pre-existing 34 remain | `/Volumes/data/output/buyer-payment-public-author-all-node-2.log` SHA-256 `0278c45d714ef653b95e1214e7ca9a82dfc7b6a114e159003e16f168f3984d9d` |
| `pnpm --filter @live-commerce/storefront typecheck` | 1, environment blocked: isolated worktree has no `node_modules`, `tsc: command not found`; **not a typecheck pass** | `/Volumes/data/output/buyer-payment-public-author-typecheck-1.log` SHA-256 `c932593378d26a2e3ec8177fe8e292a5008e22b17ca5bdacec6c75b89abd9243` |

Coverage: BPT01 exact private path/headers/body/key, three locale prepare,
one fetch per operation, no public bearer; BPT02 local path/query/fragment,
wrong method, header/CSRF/context/cookie (absent, forged, expired, duplicated),
key and JSON shape/duplicate/null/UTF-8/size/body-read denials before fetch;
BPT03 exact nine-field view, method-name Unicode limits, Go nanosecond UTC and
invalid calendar dates, prepared amount constraints, exact form and replay,
hostile upstream extra/private/mismatched/malformed/nested-duplicate JSON,
status 201/204, encoding/content-type/size/body-read sanitization; BPT04
nonretryable and no-store handoff errors even before route admission, on
disabled/bad config, 429/503, network loss, abort and synthetic deadline;
BPT05 client no-body/no-key exception only for exact handoff, pending journal
blocks, lost response makes one fetch and stores no form.

Evidence level is mocked-upstream Node transport. BPT06 independent root
typecheck/build/full regression/security review, real-browser/UI/native POST,
Go/PG, provider sandbox and production are **NOT_RUN by this task**. Tests
contain only synthetic values; no real provider request or production write.
