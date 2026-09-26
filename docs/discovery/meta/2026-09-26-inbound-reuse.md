# Meta inbound implementation evidence — 2026-09-26

Scope: read-only comparison, no provider calls with credentials, no live
subscriptions or customer messages. SaaS baseline `eb95a91`.

## Reuse decision

| Existing component | Reuse | Do not carry forward |
| --- | --- | --- |
| Daerdo IG receiver | Raw-byte HMAC, explicit challenge, bounded request, fail-closed storage errors | Fixed account allowlist; metadata-only retention of 200 events for 24 hours |
| Daerdo Page console | Page envelope and individual message receipt concepts | Slice limits that omit signed events before a 200 response; one-hour diagnostic history |
| SaaS T06 core | Authenticated merchant bindings, scoped authority, River/PG transaction conventions | Outbound operations are not inbound receipts; current bindings do not establish globally unique Meta ownership |
| SaaS PSP keyring | AES-GCM and key rotation approach as reference | Payment-specific credentials/AAD are not a Meta payload encryption interface |

Smallest suitable implementation: standard-library verifier + complete
normalizer, followed by a dedicated durable admission transaction using the
existing PG/River infrastructure. No new middleware service or queue product.

Source comparison root `/Volumes/data/my-platform`, HEAD
`415faac4fada8dcf93026ba807e59b5e45d7a308`. Working-file SHA-256 (not a claim
that the entire other worktree is clean):

| File under `tools/` | SHA-256 |
| --- | --- |
| `daerdo-instagram-receiver/core.mjs` | `c447d74f944cd78da557756a484fd160ac2cee458a76f8f7b758773ccc43ffe1` |
| `daerdo-instagram-receiver/worker.mjs` | `159f3fb68c1de05a9b7080a04a1f46d232cd8ba7120c199c99279815be7b59f6` |
| `daerdo-social-console/core.mjs` | `6d012f01c1135310d4a9859fc6b30259c9c77e160c9bfb810ee41a83d2323fd3` |
| `daerdo-social-console/worker.mjs` | `6316e5475c9cc3c9be54d6ef656bb33ab34a79623e4368bb22e854512740e714` |

Independent explorer ran the existing IG core/worker Node suite: 9 pass,
0 fail. This checks the old diagnostic receiver only. Research memory
`4eb05130-be03-4038-a83d-a2952b625aba` records eight symbol pointers.

## External evidence limits

- `web.run` at 2026-09-26 approximately 09:19 UTC returned HTTP 429 for
  [Graph webhook docs](https://developers.facebook.com/docs/graph-api/webhooks/getting-started/)
  and [Instagram webhook docs](https://developers.facebook.com/docs/instagram-platform/webhooks/).
  The newer `/documentation/` URL forms were also unavailable. No login or
  alternative tool was used to bypass that result.
- The public [Meta Messenger sample](https://github.com/fbsamples/messenger-platform-samples/blob/main/quick-start/app.js)
  is readable and demonstrates a GET challenge and Page envelope. Its
  [signature helper](https://github.com/fbsamples/messenger-platform-samples/blob/main/utils/webhook-signature.js)
  still uses SHA-1 and ordinary string comparison. Do not copy it as a current
  security contract. The local receiver's SHA-256 implementation and synthetic
  fixtures are implementation evidence, not a newly captured Meta signature.
- Real app/subapp signing-secret association, API version, asset authorization,
  App Review, subscriptions and payload samples remain separate external gates.

## Remaining T07 boundary

[Protocol contract](../../../contracts/meta-webhook-protocol-v1.md) precedes
PG admission. Before public mounting: encrypted raw payload with bounded
retention, receipts and jobs in the same transaction, permanent event uniqueness,
deterministic server-owned app/object/asset-to-tenant/store routing, quarantine
of missing/conflicting bindings, independent role/RLS/rollback/concurrency gates,
rate limits and operational monitoring. Never merge these events into site chat,
infer consent from comments, or send from the receiving HTTP request.

### Trust correction for the next database increment

Independent review traced `core.Service.RegisterBinding` at
`internal/integrations/core/service.go:121`: an authenticated merchant with
`integration:manage` may submit an arbitrary provider/asset identifier. The
scoped FKs in `migrations/0008_external_operations.sql` do not prove Meta asset
ownership or App authorization. Therefore **matching a signed entry.id against
that table is not sufficient to route customer content**. A malicious merchant
could pre-register another merchant's public Page ID.

The next contract must put only identifiers in a trusted app/object/asset route
registry, activated through a server-verified authorization/administrative
boundary with auditable evidence, never ordinary merchant self-assertion.
Store content in a separate scoped encrypted inbox or restricted quarantine.
Mixed-asset batches need atomic receipts/jobs; unknown or ambiguous routes stay
quarantined. Workers recheck binding semantics before processing. This is an
identified design requirement, not an implemented route registry or PG gate.
