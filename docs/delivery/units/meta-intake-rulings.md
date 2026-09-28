# Meta intake integrator rulings (2026-09-29) — binding for meta-intake-core, meta-intake-reply,
# meta-intake-tests, claim-source-ui

a. `claims.meta_intake.live_media boolean` column + trailing `p_live_media` parameter: ACCEPTED.
b. `origin` (storefront origin) added to the frozen reply request: ACCEPTED.
c. Instagram comment time = the inbox event's `occurred_at`; `p_occurred` NULL or equal: ACCEPTED.
d. KC03 edits: exactly the privilege rows of contract §4.3 (privilege matrix, denied list, schema
   ACL, writer EXECUTE) and the T06 allowlist additions — nothing broader: ACCEPTED.
e. The stricter lease inequality applies only to routes that set `LoadSecret`: ACCEPTED.
f. Graph: token in the JSON body (U6); API version required config, no default (U5); no 4xx
   treated as final until U3 is probed LIVE; Page-token env names `COMMERCE_META_PAGE_TOKEN_
   ACTIVE_KEY_ID` / `COMMERCE_META_PAGE_TOKEN_KEYS_JSON`: ACCEPTED.
g. Merge order for shared files (`tests/foundation/external_operation_authority_test.go`,
   `internal/platform/platform.go`): refund-core merges first, meta-intake-core rebases/merges
   after; the integrator resolves the allowlist union.
h. Gap closed: merchant binding of a live session to a Meta post (`live.put_claim_source`).
   R1 = merchant pastes a Facebook post/live-video URL or id, or an Instagram media URL or id;
   the server parses/validates it, checks it against `meta_inbox.routes` for the store's enabled
   binding, and calls `put_claim_source`. Go route owned by meta-intake-reply phase B
   (admin HTTP under the existing Studio/claims merchant routes); the Studio panel field is unit
   `claim-source-ui` (reuses the approved Studio claims panel; one text field + status line).
   No Graph listing of posts in R1 (needs a Page token read path; R2).
