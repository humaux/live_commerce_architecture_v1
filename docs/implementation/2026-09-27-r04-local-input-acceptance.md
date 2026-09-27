# R04 real local input — acceptance in progress

Status: **NOT_ACCEPTED**. This record must not promote R04/T08/T09/G06.
Contract/dependency freeze: `70780cc`. Setup notes:
[r04-local-input.md](r04-local-input.md).

## Ownership

- Integrator: main, inherited root model/effort; owns contract, dependency pins,
  documentation and final merge/rerun.
- Probe author: reused `media_runtime_source`, inherited gpt-6-sol/medium per
  prior role record; worktree `/Volumes/data/worktrees/commerce-meta-inbox-go-20260926`,
  branch `commerce/r04-local-input-20260927`, base `70780cc`; only
  `scripts/dev/r04-local-input.mjs` and optional test fixture HTML.
- Independent tests: `media_runtime_tests`, inherited gpt-6-sol/high per prior
  role record; worktree `/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926`,
  branch `commerce/r04-local-input-tests-20260927`; only
  `tests/media/r04-input-runner.test.mjs`.
- Contract/audio review: reused read-only `livekit_protocol_impl`; model/effort
  not exposed in its runtime; no source edits or provider test execution.

## Retained first failure

Author froze **`7db1416` with FAIL**, not merged into main. Normal probe exit 1.
Evidence:
`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926/output/playwright/r04-input-1790480298154-5e0fc8ec.json`.

RLI01 pins/loopback startup and RLI02 actual remote tracks were observed. During
one 2.5-second sample the receiver decoded 54 frames at 1280×720; audio increased
by 131 packets/32397 bytes, but both remote energy measures were zero. This is
**not usable-audio proof**. The script correctly stopped before declaring PASS.
Cleanup booleans were all true, but are not independent PID/port evidence.

Fault run exit 1:
`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926/output/playwright/r04-input-1790480323533-5bf0d862.json`.
It reports `injected_after_publish`, remote track events and cleanup, but no
decoded-media counters because injection occurred before sampling. That does not
satisfy the requested causal failure-cleanup proof.

The failed source replaced Chromium fake microphone capture with a WebAudio
oscillator. This is synthetic, not a real customer's mic, but it does not prove
the contract's fake getUserMedia microphone path. Do not relabel the substitution.

## Bounded adjudication and open findings

Two author audio repairs did not produce energy; the author stopped and handed
off. Root authorized **one diagnostic run only**, adding publisher PCM RMS,
audio context state/clock, track state and outbound stats, plus receiver context
state/clock. No threshold reduction or speculative third repair was authorized.
Until those measurements identify the boundary, the root cause is unproven.
An unconnected AnalyserNode is not itself an established defect; its documented
operation permits that wiring.

Before source acceptance, independently verify:

1. Nonzero remote audio and actual fake microphone path, not packets alone.
2. Fault occurs after measured media and leaves safe counters; final evidence
   includes owned PID/ports/config path for independent post-exit checks.
3. Expired JWT has an otherwise valid historical nbf/iat; signature tampering
   changes significant bits, not potentially unused base64url trailing bits.
4. JWT denial distinguishes an actual server authentication rejection from a
   generic SDK/network failure; observer denial is labeled at its true layer.
5. Server query confirms empty room, and only task-owned resources are removed.

## Separate root regression evidence

At main `7e82b6f` after the test-only SDK/lock update, root ran
`pnpm run typecheck:admin && pnpm run typecheck:storefront && pnpm run test:i18n`:
exit **0**, both TypeScript checks passed; locale/routing **4 PASS, 0 FAIL/SKIP**.
This is dependency/regression evidence, not RLI acceptance. Go/PG source and
production routes were not changed; previous 652-test evidence is historical,
not represented as a rerun of this slice.
