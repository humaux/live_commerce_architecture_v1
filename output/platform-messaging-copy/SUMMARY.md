# platform-messaging-copy — P2-N1 soft-launch wording

Status: requested local gates PASS; independent bounded review found no P0/P1/P2.
**法律文本：需 owner/律師審閱。** This is a factual-copy correction, not legal
certification, App Review approval or production verification.

## Scope and source

- Base: `0cc63a1dc8233cbfc776d32f6a2f0f35282a226f` (`r3/integration`).
- Worktree: `/Volumes/data/live_commerce_architecture_v1/.worktrees/platform-messaging-copy`.
- Branch: `unit/platform-messaging-copy`; no push, deployment or Meta mutation.
- `b15750e6`: regression assertions in Node and the platform browser runner.
- `b9154104`: six privacy/terms paragraphs in `platform-legal.ts`.
- `acfd768d`: browser evidence separates text checks from actual clicks.

Only one runtime file changed: `apps/admin/lib/platform-legal.ts`. Three test
files changed: `tests/admin/platform-site.test.ts`, `platform-runner.mjs` and
`platform-messaging-copy.fixture.mjs`. No Go, SQL, config, company facts, route,
permissions, subscription or deployment change. Evidence is committed so it
survives removal of the unit worktree.

## What changed

The old privacy and terms text in zh-TW, zh-CN and en stated inbound Messenger
and Instagram direct-message processing as a present fact. All six paragraphs
now distinguish:

1. Comments/live comments received through the connected Facebook Page and
   Instagram account; privacy names commenter name/username, ID and comment text.
2. One private reply containing an order-claim link sent to the commenter on the
   merchant's behalf, subject to platform permissions and messaging rules. We
   record the send result and **any delivery status Meta provides**; there is no
   promise of confirmed delivery or read receipts.
3. Messenger/Instagram messages **only if Meta actually delivers them to the
   platform webhook**, retaining the comment-ordering and order-follow-up
   purposes of `pages_messaging` / `instagram_manage_messages`. The copy clearly
   states inbound DM events are not currently subscribed and none arrive by this
   route. It does not claim those scopes are newly granted or alter their use.

zh-TW's Meta privacy section now says **主頁**, not 專頁. Other sections,
retention/deletion promises and the single company-facts source `company.ts`
are unchanged. Production subscriptions (Page `feed`, IG `comments` /
`live_comments`) are **owner-provided evidence from 2026-10-05**, not a LIVE
check performed by this unit. Revisit the disclosure if that configuration changes.

## Commands / exit codes / evidence

`pnpm install --offline --frozen-lockfile` exited 0; 49 cached packages reused,
no downloads or lockfile changes. All gates use the task-local runner below;
browser build/run has synthetic `.example.invalid` hosts and contact email.
`LC_TEST_LOCK_WAIT=14400`; the existing machine lock was respected.

| Gate | Command | Result | Evidence |
|---|---|---|---|
| Genuine pre-fix RED | `node --test --test-reporter=spec --experimental-strip-types tests/admin/platform-site.test.ts` | exit 1; 10 PASS, 6 FAIL (three locales × privacy/terms) | `red-copy.log` |
| Legal Node | Same command after the copy fix | exit 0; 16 PASS, 0 FAIL, 0 SKIP | `node-copy.log` |
| Admin TypeScript | `pnpm --filter admin exec tsc --noEmit` | exit 0 | `admin-tsc.log` |
| Registered gates | `bash scripts/dev/check-gates.sh` | exit 0 | `check-gates.log` |
| Copy detector | `node /Users/luolimo/.codex/skills/impeccable/scripts/detect.mjs --json apps/admin/lib/platform-legal.ts` | exit 0; `[]` | `copy-detector.log` |
| Platform browser | `bash scripts/dev/test-local.sh --browser-platform-site` | exit 0; 30 page cases, 390 real clicks, 12 copy cases, 30 reloads; 15 SSR / 16 tag cases | `browser-platform-site.log`, `browser/` |
| Evidence verification | `node output/platform-messaging-copy/verify-evidence.mjs` | exit 0; all ledger rows PASS; 30 PNGs and 12 unique copy cases verified | `verified-evidence.json` |
| Evidence secret scan | `LC_RELEASE_GATE_OUT=output/platform-messaging-copy/g04 bash scripts/dev/release-gate.sh --strict --only G04` | exit 0 after staging evidence; no key-shaped literal | `g04/results.tsv` |

Exact source SHAs and completion timestamps: [results.tsv](results.tsv).
Reproducible commands and fixture environment: [run-gates.sh](run-gates.sh).
Static logs are from `b9154104`; the later `acfd768d` changes browser reporting
only, not TypeScript/runtime code. The final browser runner also executes all 16
Node checks before its production Next build.

The initial browser run on `b9154104` passed all behavior checks, but its existing
counter classified the 12 new text assertions as clicks (402, incorrectly).
`acfd768d` only corrects this accounting and reports text checks separately.
Historical stdout/result/ledger remain `browser-initial.log`,
`browser-initial-result.json`, `browser-initial-ledger.json`; **402 is not a
real-click acceptance count**. No assertion was removed or loosened.

## Browser / review evidence

The runner covers all five public pages at **390×844 / 1586×992**, each in
**zh-TW / zh-CN / en**, with production Next and Chromium behind a synthetic local
Host-preserving transport. It checks no overflow, company facts, indexability,
language/navigation, reload and link behavior. New assertions check the visible
privacy/terms text and again after **real footer-link clicks**; not just imports,
screenshots or DOM mutation. Optional verification-tag and SSR controls remain.
Screenshots and click ledger are under [browser/](browser/).

Root inspected zh-TW mobile privacy and English desktop terms: the narrowed copy
is visible and wraps without clipping; existing layout and company sections stay
unchanged. `impeccable` clarify preserved factual boundaries and incumbent layout;
the requested Playwright runner, rather than a new UI, verifies real navigation.

Independent non-author review (`security_reviewer`, configured `gpt-6.1-sol`,
medium reasoning) checked the four-file `0cc63a1d..b9154104` diff and found no
concrete P0/P1/P2 defect or weakened assertion. Task
`b0832231-2a67-4fb8-b370-949feaef3b9b`; Humaux title
`platform-messaging-copy b9154104 bounded final independent review`.
It was read-only, no code edits, provider calls or delegated work; its test runs
are NOT_RUN. Root Codex authored the unit and ran the gates; underlying root
model/effort is not exposed and is not inferred. No second writer was used.
The same reviewer checked `b9154104..acfd768d` separately: the accounting-only
correction changes neither behavior assertions nor runtime code; no finding.
Review task `0490497e-1cd6-4fed-9c7c-6925e504ba27`, stored in Humaux as
`platform-messaging-copy acfd768d independent count correction review`.

The task's generated build output is retained as `build.log`; the older tracked
`output/platform-site/build.log` was restored after copying, so this unit does
not overwrite another delivery's evidence. Browser teardown closed its own
Chromium, Next servers and authentication fixture; no other task was stopped.
The post-run process inventory found no remaining unit runner or standalone
server. Changed files were submitted to Humaux's incremental code index. Its
MJS helper entity was not resolved for linking; the rationale is linked instead
to the indexed `PlatformDocument` consumer, with exact test/commit paths above.

## NOT_RUN / limits

- Owner/qualified legal review: **NOT_RUN — 需 owner/律師審閱**.
- Production, live webhook verification, Meta App Review: **NOT_RUN**, outside scope.
- Full G07, Caddy/compose smoke, global click sweep: **NOT_RUN**, not requested;
  no migration/ACL/checkout/storefront-runtime change in this copy-only unit.
- No email or private message was sent to a real user. Company/contact fixtures
  are not a replacement for deployment configuration.
