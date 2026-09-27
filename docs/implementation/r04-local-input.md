# R04 local real-media probe: operation and limits

Contract: [R04 probe](../../contracts/r04-local-input-probe-v1.md).
Bounded probe: **ACCEPTED_LOCAL_REAL_WEBRTC** after independent and root five-case
runs at tested main `8884d59`. Product R04/G06 remains unaccepted. Exact revisions,
the original failures and final artifacts are in
the [acceptance log](2026-09-27-r04-local-input-acceptance.md).

## What this measures

Synthetic camera/microphone → actual Chromium publisher → actual local LiveKit
SFU → independent Chromium subscriber. Acceptance requires advancing decoded
video frames and audio energy/traffic, not just `connected` or `trackSubscribed`.
The test-only fixture is not a merchant screen or an approved Studio design.

This is a prerequisite capability check, not the authenticated product workflow.
It does not cover Egress/encoding/RTMP, customer hardware or Internet quality,
Cloud's strict participant revocation, Meta public visibility or global G06.
The current prepared-media and attempt records remain MOCK-only and untouched.

## Fixed dependency and trust chain

| Dependency | Pin and use |
| --- | --- |
| LiveKit server | 1.13.7, task-local native ARM64 executable, no global installation |
| macOS distribution | Homebrew/core `arm64_golden_gate` bottle, no runtime dependencies reported by formula metadata |
| Bottle SHA256 | `23ce9068dae09785cbb8da9f4378f27f7e178cb7a8e427f4fb1708968f0b6d8d` |
| Extracted executable SHA256 | `2b06c267be38bae34e2314ea648826c39220f93bd9ed25286d6fc17585e28f91` |
| Browser SDK | `livekit-client@2.22.3`, root **devDependency**, integrity in `pnpm-lock.yaml` |
| Browser harness | Existing `@playwright/test@1.63.0`, cached Chromium 1243 |

Sources: [LiveKit local setup](https://docs.livekit.io/transport/self-hosting/local/),
[server release](https://github.com/livekit/livekit/releases/tag/v1.13.7),
[Homebrew formula metadata](https://formulae.brew.sh/api/formula/livekit.json),
[client release](https://github.com/livekit/client-sdk-js/releases/tag/v2.22.3).
The upstream server release has no Darwin artifact; do not invent one. Homebrew
is a separate distribution trust source, recommended for macOS by LiveKit docs.
The GHCR public bottle endpoint requires its normal anonymous registry token
flow; the initial unauthenticated request returned 401, not a missing asset.
No registry token was logged or persisted.

On macOS 27.0 ARM64, the verified executable's `--version` returned 1.13.7.
Retained setup artifacts, not customer data:

- `/Volumes/data/output/livekit-1.13.7-arm64_golden_gate.bottle.tar.gz`
- `/Volumes/data/output/commerce-r04-tools-20260927.OdpS5m/livekit/1.13.7/bin/livekit-server`
- Adjacent upstream LICENSE and NOTICE, extracted from the same checked bottle.

These retained tools are referenced by the repeatable gate; do not age-clean
them while this reference is live. An upgrade must deliberately update the
contract checksum/lock and rerun the probe. No `latest` fallback is permitted.

## Invocation and operational boundary

Set `COMMERCE_R04_LIVEKIT_BINARY` to the checked executable. From the repo root:

```sh
node scripts/dev/r04-local-input.mjs
node --test tests/media/r04-input-runner.test.mjs
```

The first command is one normal probe; the second runs all five causal cases.
`COMMERCE_R04_FAULT=after-publish` and `timeout-after-publish` are deliberate
**nonzero** cleanup tests, not production options. Unknown configuration fails
before child startup. The normal test exits zero; both intentional runtime
failures are expected by the suite, not hidden or converted into probe PASS.

The runner creates a one-second synthetic PCM WAV, loops it through Chromium's
fake getUserMedia capture, and removes it with its private config directory.
Receiver audio must be attached/played via the SDK before measuring remote PCM;
the initial packet-only/zero-energy failure and single-variable fix are retained.
Browser/network waits use a remaining 75-second operation budget; finalization
has separate bounded closes. `run()` is awaited directly. Per-call timeouts for
context/page creation are contained by closing the owned browser; the independent
suite has a 120-second per-process guard and tests the injected timeout path.

The runner owns its temporary credentials/config, free loopback ports, server,
HTTP fixture and browser processes. It must release only these resources in
`finally`; it must never kill a service by common port/name or touch a user's
profile. Evidence is sanitized counters/classifications and cleanup results;
safe PID/port/config-directory fields let the independent suite verify teardown
after process exit (and again after 500 ms for the timeout case).
JWTs, secret-bearing URLs/config and raw traces/console dumps are forbidden.
No actual microphone/camera permission, customer identity, external destination,
database, registry network or Cloud account is needed during the media run.

Publication denial is deliberately layered: invalid JWTs receive actual server
401; the no-publish observer is confirmed by server participant state plus SDK
403 on canvas-track publishing. It is not a malicious client bypass test.

The local Docker engine was observed at about 1.92 GiB memory; this is not a
benchmark or capacity promise. Native local SFU plus browser observer avoids
adding an Egress process solely to test that input tracks can traverse WebRTC.

## Follow-on product obligations

Current merchant/session/project/generation scope and lifetime must be reviewed
before a product token endpoint is introduced. Reuse existing identity and
durable media mechanisms; do not create a second transaction/queue framework.
Treat participant input and Egress output as separate resource responsibilities.
An ended Egress cannot certify an empty input room.

[LiveKit token lifecycle](https://docs.livekit.io/frontends/reference/tokens-grants/)
explicitly distinguishes token admission expiry from existing connections and
refresh. Self-hosted removal does not invalidate cached tokens; Cloud strict
removal needs deliberate cutoff handling. A successful local run cannot close
those authorization or production acceptance gates.
