# Studio browser-input client transport

Base: `f2166a7`. Scope: existing Next admin client only; no runtime activation,
UI controls, SDK connection, database migration or provider request.

## Implementation and dependencies

- `studio-client.ts` adds `startStudioInput` and `requestStudioInputToken` using
  its existing `write` function. That function owns CSRF, pre/post login-boundary
  checks, same-origin/no-store transport, the supplied idempotency key, and the
  existing 12-second request timeout. It does not retry ambiguous writes.
- `studio-request.ts` exposes a typed exact six-field token guard shared with
  the existing BFF. The browser additionally rejects a different attempt or
  already expired grant. The returned credential belongs only in the active
  publisher's memory, never the UI recovery journal, storage, logs or URLs.
- Input Stop still uses `stopStudioRehearsal`. There is no second cleanup API.
- Native Node 24 TypeScript transformation runs the real client in tests; a
  test-only resolver handles the Next app's extensionless relative imports.
  No new runtime or development dependency was introduced.

## Executed acceptance

- `node --test --experimental-transform-types tests/admin/studio-request.test.ts
  tests/admin/studio-input.test.ts tests/admin/studio-input-client.test.ts`:
  exit 0, 12 tests passed, no failures/skips. Six new client tests cover exact
  POST/body/headers, missing/duplicate/changed login, post-response login change,
  wrong attempt/expired/malformed credential, sanitized errors, ambiguous
  responses without retries, and existing Stop reuse.
- `pnpm run typecheck:admin`: exit 0.
- `pnpm run build:admin`: exit 0.
- Independent read-only review by `studio_input_transport_review` found no
  P0/P1 in this diff and independently reran all six new client tests: exit 0.
  Role: security/test reviewer; requested model `gpt-6-sol`, high reasoning;
  read-only root worktree, no author edits or shared builds.
- Existing `--browser-input-delivery` runner now includes these client tests
  before its separate real signed HTTPS/PG fixture. That full fixture was **not
  rerun for this client-only increment**; previous BRW05 evidence is historical.

Logs (outside Git, retained):

- `/Volumes/data/output/studio-input-client-20260928.log`
- `/Volumes/data/output/studio-input-client-build-20260928.log`

## Remaining product gates

Studio does not call these new methods yet. Approved camera/microphone controls,
device lifecycle and permission handling, actual product-token-to-SFU decoded
audio/video proof, new-input crash recovery, Cloud qualification and production
deployment remain separate pending work. Unit transport tests do not substitute
for any of those gates or for full current-tree regression.
