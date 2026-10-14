# PE-FIX-1 round 2 delivery (PR #64)

Status: NOT_RUN. Nothing was executed locally (per instruction); CI is the first run.

## Fixed
- P1-1: `ProductDocumentForm` passes `null` photos to `write.save` in edit mode when the image list is unknown, so `needsMainImage` skips the client check. The server still enforces it. `write.save` never sends photos in edit mode.
- P2 batch: if an upload succeeded but clearing the journal failed, the batch now stops and the summary shows the "unknown, do not re-upload" copy instead of a plain failure.
- Test added: `tests/admin/product-media-v2.spec.ts`, "an active product saves a name edit while the image list cannot be read". It stubs the image-list GET to 500 (`// FIXTURE`), then edits the name with real clicks, saves, reloads, and checks the name and the detail-row count.

## Open P2s (not data integrity)
- Oversize fixture is a non-decodable 2 MiB+ buffer. It exercises the preprocess refusal, not the `MAX_PHOTO_BYTES` guard in `product-media-client.ts`. A corrupt JPEG over 2 MiB now says "compress and upload again".
- Leave-guard copy ("wait for it to finish") is misleading when `write.recoveryBlocked` is the lock cause, since that state does not clear itself.
- Integrator should run `git diff origin/r3/integration HEAD -- tests/ | grep '^-'` to confirm no assertion was removed.

## Click ledger (new test)
| Page | Control | Action | Expected | Actual |
|---|---|---|---|---|
| editor, list read failing | product-name | fill | value changes | NOT_RUN |
| editor | product-save | click | PUT 200; name persists after reload; images unchanged | NOT_RUN |
