# W3-U2 readiness audit — 2026-10-08

Status: HISTORICAL — all questions below were resolved by RULINGS.md on 2026-10-09. Current implementation and acceptance are in DELIVERY.md. The audit below records the original pre-implementation state, not an active blocker.

## Dependency
- Clean local HEAD: 9ce09803b8bd76209efa70a44515c3d1aedb6f42.
- `git fetch origin && git merge origin/unit/lc-u2a-comment-stream` -> exit 1: remote ref absent (fetch succeeded).
- `git ls-remote --heads origin '*lc-u2a*' 'r3/integration'` -> exit 0: only integration e705f70a694f7d0f4ed1c2967883e3f5d8d62803, no LC-U2a ref.
- `gh pr list --state all --head unit/lc-u2a-comment-stream --json number,state,mergeCommit,headRefOid` -> exit 0: [].
- Local dependency branch exists at 34f3854c06ea0a66f6ebb2b346e28641301ee58d (K3 P2 fixes committed).
- Pending integrator clarification: merge that local committed branch or wait for its remote publication. No dependency merge performed.

## Frozen brief versus implemented contract
1. Automatic reminder switch and delay: brief lines 15–16 request them. Contract `live-console-v1.md:1106-1109` expressly defers automatic triggering and removes GET/PUT live-settings/reminder. `internal/httpapi/reminders.go:5` confirms no route. Proposed scope: manual trigger/report/copy only, with automatic settings honestly unavailable until a backend unit provides them. Do not fabricate a working save or worker.
2. Sold-out switch/CAS: `contracts/live-keyword-claims-v1.md:1242-1248,1262` provides Go/SQL getters/setters and says W3-U2 adds HTTP, but UI brief write paths and current assignment permit UI/BFF only. No HTTP adapter/registration calls GetSoldOutReply/SetSoldOutReply. Needs either an authorized thin Go adapter with frozen route/body/permissions, or backend follow-up and unavailable UI. Template publication alone cannot save the selected switch/version. Fixed template IDs are immutable (templates.go:71–86); publish a merchant template then select its actual returned ID/version.
3. Blocklist list: actual frozen result `BlockedActor` (internal/claims/blocklist.go:56–63) contains id/platform/note/source_bundle_id/created_at, not display_name. Do not infer buyer identity from names or IDs. Use an honest unavailable display-name state unless the backend adds an explicit field.
4. Order drawer warning: drawer is absent in this base; LC-U3 currently owns its implementation. Current write paths omit it. Provide/reuse blocklist check through an agreed seam; drawer wiring belongs to the coordinated follow-up or requires explicit scope.
5. Browser gate registration: brief assigns --browser-live-settings registration to integrator; mode does not exist in current runner/GATES. New route page and Node real-seam tests/registration are needed to make the requested route and red tests executable; current literal write list omits those files. Align write paths before coding.

## Ready existing seams
- Reminders: GET/POST live-sessions/{sid}/reminders, optional per-bundle POST, no request body; keyed trigger requires inbox:reply AND live:manage; report inbox:read.
- Blocklist: live-sessions/{sid}/claims/blocklist GET/POST; DELETE entries/{id}; GET check?bundle_id. live:read for reads and live:manage for writes; key for POST/DELETE. Store-scoped list despite session path; opaque entry IDs, never actor_key input.
- Templates: GET/POST message-templates; POST live:manage and idempotent; fixed IDs cannot be overwritten.
- Comment marks: existing A2 private_reply.kind out_of_stock and claim.reason restricted; restricted badge already present in CommentStream. Preserve current intent and only add missing localized sold-out copy.

## Validation
E1/read-only source audit only. Red/green, Node, typecheck/build, check-gates and browser modes: NOT_RUN (no implementation yet). No Go/SQL/OpenAPI/product edits. No push/deploy/live messages.
