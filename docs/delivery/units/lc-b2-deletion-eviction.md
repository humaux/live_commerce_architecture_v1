# Unit LC-B2-DEL: comment poller honours user deletions (contract live-console-v1 §2.2)

Owner: Codex-4 · branch `unit/lc-b2-deletion-eviction` · worktree `.worktrees/lc-b2-deletion-eviction` (from r3/integration)
Found by: Codex-1 while fixing PR #18 thread 4227227603 (output/pr18-deletion/DELIVERY.md:40-46 on unit/lc-u2a-comment-stream).

## Gap (privacy)
- Contract §2.2 (`contracts/live-console-v1.md:126-129`): "every 60 s the poller re-reads the ids of buffer entries younger than 30 min with one batched `GET /?ids=<≤50 ids>&fields=id` (same token transport, §2.4, inside the shared rate budget) and evicts ids Graph no longer returns".
- Trunk: `internal/integrations/metareply/comment_poll.go:643-649` evicts only by age (2 h) and cap ("a deleted comment leaves the buffer within one age-out"). A buyer's deleted comment is served to every console tab for up to 2 h.
- `tests/foundation/live_console_comments_test.go:3` claims "deletion eviction" coverage, but no test exercises it. LCN01 in the contract test matrix lists "deleted comment evicted".

## Scope (Go only; no contract, SQL, OpenAPI or UI change)
1. In the poller loop, at most once per 60 s per source: collect up to 50 buffer refs younger than 30 min (newest first) and issue one batched ids GET through the existing token transport. It must count against the SAME per-asset rate budget and backoff (§2.4): a 429 or a backoff code skips the check and never evicts.
2. Evict only on a SUCCESSFUL response that omits an id. On a transport error, a partial or unknown response, or a token problem, evict nothing (fail safe: never drop comments on uncertainty). Eviction keeps `seq` monotonic. Do not send `reset` for a deletion.
3. The token stays out of URLs and logs (LCN01 already asserts the fake Graph never sees `access_token` in a URL). Reuse that assertion.
4. Fix the misleading comment at comment_poll.go:643 and the test-file header claim.
5. If more than 50 eligible refs exist, rotate them across checks (the oldest-checked first) so every young entry is checked within a bounded number of cycles; document the bound.

## Tests (red first, MOCK fake Graph, foundation package with the existing LCN01 harness)
- deleted id evicted within one check: A2 no longer returns it, and the bridge serves the next page without it;
- a 429, a 5xx and a malformed body all evict nothing;
- the check stays inside the shared budget: two sources on one asset make one combined call count;
- entries older than 30 min are not checked; the 50-id batching plus rotation is bounded;
- no token in URLs or logs.
- Also add, if feasible, a test-fixture seam so `--browser-live-console` can delete one synthetic comment (MOCK Graph). That would let the PR #18 browser deletion case run; otherwise record NOT_RUN.

Gates: focused Go regex for live console comments (REAL_PG where the harness needs it), `go vet`, check-gates, test-node. Commit, then stop. Do not push; the integrator arranges the independent review (privacy → K3 deep).
