# Current buyer destination — recovery projection v1

Status: IMPLEMENTING; gates are not accepted until real PostgreSQL/HTTP tests pass.

`GET /v1/buyer/destination` (private) and same-origin
`GET /api/buyer/destination` (browser BFF) return exactly
`{"destination": null}` or `{"destination": <existing safe destination DTO>}`.
Reuse buyer capability, publication/origin admission, owner/store RLS and the
existing projection. No query/body/idempotency header, no new tables or grants;
no-store responses. The historical `GET /destinations/{id}` is unchanged.

Why: after PUT commits but the response is lost, reload may lose the returned
UUID and destination head version. Persisting recipient/address/phone locally is
not the recovery strategy. This bounded read discovers the existing cart's head
and immutable snapshot, including expired selections and old cart versions, so
the user can inspect and explicitly confirm or replace it with head CAS.

It is **not** a receipt lookup or an eligibility guarantee. A current head does
not establish which lost command committed; null does not prove that an inflight
command cannot still commit. Do not automatically replay a PII write with a new
key. A concurrent/late writer still uses its original expected version, and
checkout revalidates current head, cart, source and expiry. Concurrent reads may
show the head observed at read time; they never renew its validity.

Gate: absent owner/cart → null; exact DTO/no private attestation fields; owned
current and historical reads; expiry readable without renewal; CAS replacement
and late old-version conflict; other owner/host denied or null as appropriate;
strict GET admission; read leaves destination events, receipts, orders and holds
unchanged. Browser UI must separately prove reload and explicit confirmation.
