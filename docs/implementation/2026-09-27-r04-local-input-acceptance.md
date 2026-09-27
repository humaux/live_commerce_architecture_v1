# R04 real local input — bounded probe acceptance

Status: **ACCEPTED_LOCAL_REAL_WEBRTC_PROBE_ONLY** at tested main `8884d59`.
This record must not promote product R04/T08/T09/G06 or production readiness.
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

## Bounded adjudication and prior open findings

Two author audio repairs did not produce energy; the author stopped and handed
off. Root authorized **one diagnostic run only**, adding publisher PCM RMS,
audio context state/clock, track state and outbound stats, plus receiver context
state/clock. No threshold reduction or speculative third repair was authorized.
The diagnostic commit `01a1f99` ran once and exited 1. Receipt:
`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926/output/playwright/r04-input-1790480564004-fcbb4e4a.json`.
Publisher local PCM energy was 0.5041885, its running audio clock advanced
1.035 seconds, and outbound audio increased 52 packets / 12880 bytes.
The observer's running clock advanced 2.613 seconds and received 131 packets /
32393 bytes, but PCM energy remained zero. Both tracks were live, enabled and
unmuted. Missing outbound `totalAudioEnergy` must not be interpreted as encoded
silence. The failure is at/after publication; its precise cause is unproven.
An unconnected AnalyserNode is not itself an established defect; its documented
operation permits that wiring.

Independent tests `70e2007` against source `7db1416` (test-worktree cherry-pick
`00cf8fb`) exited 1: **2 PASS / 2 FAIL**, log
`/Volumes/data/output/r04-independent-tests-source7db1416.log`.
Invalid executable and invalid-fault preflights passed without child startup.
Normal media failed the audio energy assertion; deliberate failure lacked media
counters. This corroborates the defects without lowering acceptance thresholds.

After reviewing the new measurements, root authorized exactly one controlled
receiver-playback variant and normal run: attach/play the remote audio using the
SDK, holding publisher generation, analyser and thresholds fixed. The current
source attaches only video; the official JavaScript
[subscription example](https://docs.livekit.io/transport/media/subscribe/)
attaches audio/video to their media elements. This is a testable hypothesis,
not a root-cause declaration. No simultaneous WAV substitution or further blind
audio retries are authorized; freeze the result for adjudication.

The playback-only commit `e47f2e0` (16 added lines) ran exactly once and exited
0; receipt:
`/Volumes/data/worktrees/commerce-meta-inbox-go-20260926/output/playwright/r04-input-1790480955243-80c37684.json`.
It retained the same publisher source and energy assertions. Audio attachment,
`startAudio()` and `play()` succeeded; the receiver decoded 53 video frames and
130 audio packets / 32146 bytes, with energy delta 2.53 and PCM RMS energy 0.50095.
This controlled contrast supports the missing receiver playback attachment as
the cause in this test harness. Root inspected the synthetic camera screenshot;
it is not a merchant UI screenshot. The old script's `PASS` still does **not**
close the outstanding gates below or independent/root acceptance.

Root freeze `280ac66` authorizes the next bounded batch to restore actual fake
getUserMedia capture and close the listed proof gaps, including both post-media
fault paths. Independent tests must use the exact resulting source revision.

The pre-acceptance findings were:

1. Nonzero remote audio and actual fake microphone path, not packets alone.
2. Fault occurs after measured media and leaves safe counters; final evidence
   includes owned PID/ports/config path for independent post-exit checks.
3. Expired JWT has an otherwise valid historical nbf/iat; signature tampering
   changes significant bits, not potentially unused base64url trailing bits.
4. JWT denial distinguishes an actual server authentication rejection from a
   generic SDK/network failure; observer denial is labeled at its true layer.
5. Server query confirms empty room, and only task-owned resources are removed.
6. Overall timeout must stop/settle the running operation before cleanup and
   receipt. A `Promise.race` alone does not cancel `run()` and can race cleanup.
7. Failure classifications are fixed safe codes, not arbitrary SDK error text
   with punctuation removed; secrets may be alphanumeric.

## Final fixed-source acceptance

Author lineage: `7db1416` → `01a1f99` → `e47f2e0` → `b770e6a` → `85972f8`.
The last two restore actual fake getUserMedia WAV, exact auth failures, owned
resource receipts, measured-media fault injection, safe errors and bounded
browser/network waits. Whole-run detached timeout was removed. Independent
tests: `70e2007` → `bab201d` → `e1bf43d`; final integration **`8884d59`**.

The first five-case independent run was 4 PASS / 1 FAIL solely because its
timeout expected-label disagreed with the fixed deadline class. That log remains
`/Volumes/data/output/r04-independent-final-five-20260927.log`.
`e1bf43d` corrects exactly one expected string to `injected_operation_timeout`;
no media, auth or cleanup assertion was weakened.

Final fixed-source static review of **`85972f8`**, including its full lineage,
found no confirmed blocking P0/P1. The reviewer did not execute the probe.
The broad lockfile-integrity pattern was noted as a future robustness limit;
the current exact SDK entry/hash and setup chain were verified, not inferred
from that regex alone. Updating dependencies requires pin review and reruns.

| Gate | Final observation |
| --- | --- |
| RLI01 | Exact binary/SDK pins, local server readiness; invalid executable/fault rejected before child execution |
| RLI02–03 | Real fake getUserMedia capture → local SFU → separate receiver; advancing decoded video and nonzero audio energy, not only events/packets |
| RLI04 | Expired and tampered JWT: specific NotAllowed/401; observer server no-publish grant and zero tracks, with **client-permission** PublishTrackError/403 on canvas publish |
| RLI05 | Normal room query empty; normal and both measured-media faults have independently absent PID, rebound TCP/UDP ports and removed config/WAV directory; timeout rechecked after 500 ms |
| RLI06 | Separate fixed-source review, independent five-case execution, root five-case rerun and retained documentation |

Independent final run: **exit 0, 5 PASS / 0 FAIL / 0 SKIP**.
Log `/Volumes/data/output/r04-independent-final-five-e1bf43d-20260927.log`, SHA256
`eee9203455f330a9d8b231f262d1d5d043013a12cf4c7c7ce86c6fcb9be26e84`.
Its normal receipt:
`/Volumes/data/worktrees/commerce-meta-inbox-tests-20260926/output/playwright/r04-independent-2MsSGI/r04-input-1790481630258-9df5118c.json`.
Fault receipts are under the same `output/playwright` directory:
`r04-independent-OvBeuW/r04-input-1790481634438-42170c3f.json` and
`r04-independent-kcVds3/r04-input-1790481641180-1e84b915.json`.

Root actual command at **`8884d59`**:

```sh
COMMERCE_R04_LIVEKIT_BINARY=/Volumes/data/output/commerce-r04-tools-20260927.OdpS5m/livekit/1.13.7/bin/livekit-server node --test tests/media/r04-input-runner.test.mjs
```

Result: **exit 0, 5 PASS / 0 FAIL / 0 SKIP**, 18072.165708 ms.
Log `/Volumes/data/output/r04-root-fivecase-20260927.log`, SHA256
`b85546ee5904b5fcd1eaea29595744517b4eec349e8d06c8a2fe17b8ea770879`.
Root receipts under the main repo's `output/playwright`:

- Normal `r04-independent-Q9x067/r04-input-1790481649777-e6f972d9.json`:
  54 decoded frames, 1280×720, 130 audio packets / 32110 bytes, energy +0.633835,
  PCM RMS energy 0.123987; actual server auth and normal room-empty checks passed.
- Fault `r04-independent-IqUvDC/r04-input-1790481653516-8c5a3446.json`:
  intentional exit 1 after measured media, `injected_after_publish`.
- Timeout `r04-independent-yAbspl/r04-input-1790481660141-69fc6bbc.json`:
  intentional exit 1 after measured media, `injected_operation_timeout`.

Root inspected the final normal-run synthetic-camera screenshot. It proves only
fixture rendering, not merchant UI or a real hardware capture. Independent tests
verified owned resources after each runtime, without killing unrelated processes.
The pinned server archive/binary and evidence remain because these docs reference
them; temporary configs/WAVs/listeners/browsers from the runs were removed.

No product, Go/PG, Cloud, Meta, Egress or customer environment was changed. No
production resource or live broadcast was stopped. Merchant authorization/lifetime,
approved Studio UI, managed Cloud/media-output and global deployment remain gates.

## Separate root regression evidence

At main `7e82b6f` after the test-only SDK/lock update, root ran
`pnpm run typecheck:admin && pnpm run typecheck:storefront && pnpm run test:i18n`:
exit **0**, both TypeScript checks passed; locale/routing **4 PASS, 0 FAIL/SKIP**.
This is dependency/regression evidence, not RLI acceptance. Go/PG source and
production routes were not changed; previous 652-test evidence is historical,
not represented as a rerun of this slice.

After integration at `8884d59`, root repeated the same TypeScript/i18n command:
exit 0, both TS checks and **4/4 i18n** passed. No full Go/PG rerun was claimed.

## Traceability tool limitation

Root submitted both changed `.mjs` files to Humaux `code_index`. The indexer
finished with zero files/entities; a qualified `startPlayback` memory-link
attempt returned entity-not-found. The independent tester observed the same
parser limitation. No unrelated same-name entity was linked. Fix rationale,
test receipts and progress are retained in Humaux memories/canvas and this log;
the JavaScript code-graph edge remains unavailable rather than falsely complete.
