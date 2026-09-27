# R04 real local media input probe

Status: **FROZEN_FOR_LOCAL_PROBE / EXECUTED_NOT_ACCEPTED**. Main base `05472bd`, 2026-09-27.
Independent bounded preflight of `d46da36` found no material P0/P1. This freezes
the probe only; execution and failures are recorded in the
[acceptance log](../docs/implementation/2026-09-27-r04-local-input-acceptance.md),
not implied to be product acceptance.
This is a bounded dependency probe, not a merchant feature or G06 acceptance.

## Decision and scope

Before exposing a browser room credential, prove that a real browser can send
camera/microphone tracks through the selected SFU and a separate observer can
decode them. Existing Egress/Studio tests use local provider doubles, and cannot
supply that evidence. Reuse installed Chromium/Playwright and Node; add only the
official, exact-version `livekit-client` as a test dependency. Do not add React
components, a server SDK, Redis, Egress, a product route or a database migration.

Use LiveKit server **1.13.7** in task-local tools, not a global install, `latest`,
an installer shell script or an existing service. Its GitHub release has no
Darwin asset; use the pinned Homebrew/core ARM64 bottle (the official docs route
macOS users through Homebrew), not an invented upstream macOS release. Bottle
SHA256: `23ce9068dae09785cbb8da9f4378f27f7e178cb7a8e427f4fb1708968f0b6d8d`.
Record its source/OS compatibility and extracted binary checksum before running.
Verified on this host (macOS 27.0 ARM64): `livekit-server --version` returned
1.13.7; extracted binary SHA256
`2b06c267be38bae34e2314ea648826c39220f93bd9ed25286d6fc17585e28f91`.
Pin test-only `livekit-client@2.22.3` with registry integrity in the lockfile.
Dependency download
is setup only; the media run must need no external network. Runtime must bind to
loopback, use free task-owned ports, randomly generated fixture credentials and
opaque fixture identities. Refuse an unsupported platform or a missing binary
with an actionable error, never a skip/pass or silent alternate dependency.

Existing MOCK authorization/attempt rows and their `lc_…` rooms are untouched.
Use a distinct `r04_probe_…` test room. No merchant data, production credentials,
camera/microphone hardware, Cloud account, Meta destination, RTMP output, charge,
real broadcast or recording is involved. This does not change the managed Cloud
deployment baseline or the Studio visual approval boundary.

## Runnable contract

One explicit command, `node scripts/dev/r04-local-input.mjs`, takes the pinned
binary path from `COMMERCE_R04_LIVEKIT_BINARY`. All test media is synthetic.
The runner checks the binary checksum/version, creates private ephemeral config,
starts only its child server, serves a loopback fixture page and SDK, and launches
two isolated Chromium contexts using the existing installation. Runtime ceilings,
bounded output, and `finally` cleanup are mandatory on both success and failure.
Test-only `COMMERCE_R04_FAULT=after-publish` deliberately fails after tracks
arrive so independent tests can prove the failure cleanup path. Empty/unset is
normal; any other value is rejected before child startup. This is not a product
configuration flag. Injected failure must exit nonzero and preserve safe evidence.

Fixture-only JWT signing uses Node's standard crypto, with one exact room and
separate identities. Publisher grants: join, publish camera/microphone only;
no subscribe, data publish, metadata update, room admin/create/list/record or
ingress grants. Observer: join and subscribe only, no publish/data/admin grants.
No PII in identity/room. This signer is not shared with product authentication.

No JWT, secret, config, token-bearing URL, session, console/network dump or
trace is written to the evidence. Evidence contains only bounded classifications,
versions, aggregate frame/audio counters, timing, success/failure and cleanup.
No persistent browser context or user profile is used. SDK requests are local;
unexpected external browser requests fail the probe. The test pages are harness
fixtures, not Studio UI and not screenshots of delivered merchant functionality.

## Acceptance gates

| Gate | Required observation |
| --- | --- |
| RLI01 | Verified pinned official server and SDK; isolated loopback startup with ready check; no other service stopped/reconfigured |
| RLI02 | Publisher uses Chromium fake camera/microphone and observer receives two real remote tracks from that publisher, not locally reattached tracks |
| RLI03 | Observer video frames are decoded and advance across samples, with nonzero dimensions; inbound audio packets/bytes and nonzero energy advance. A connected socket or track event alone is insufficient |
| RLI04 | Actual server rejects expired and tampered JWTs; observer cannot publish; fixture claims are exact least privilege. Do not label client-side rejection alone a server enforcement test |
| RLI05 | Publisher stops tracks/disconnects; observer sees removal, then disconnects. Authoritative room query confirms no participants; owned browser/server/HTTP listener/config are cleaned even on injected failure |
| RLI06 | Independent fixed-source review and rerun; root rerun; retained sanitized evidence, dependency and troubleshooting notes. Nonzero or missing gate is failure, not PASS |

Freeze the implementation boundary after independent review. Integrator owns
this contract and dependency lock. Author owns only the probe runner and fixture;
independent tester owns counterexamples and validation, not product source.

## Known limits and next product gate

Result level is **LOCAL_REAL_WEBRTC**, not MOCK, SANDBOX payment, Cloud, customer
device/network quality, merchant-authenticated workflow, Egress or audience LIVE.
Do not set R04, T08/T09 or G06 to passed based on this probe. Production join
issuance must subsequently bind current merchant/session/project/generation,
enforce server-side lifecycle/revocation and survive crash/logout/permission loss.

Short JWT TTL is not a connection lifetime: LiveKit refreshes tokens and a token
expiry does not end an already-connected session. Self-host RemoveParticipant
does not revoke the old JWT; Cloud's strict cutoff requires a deliberate
`revoke_token_ts`. Egress terminal evidence does not prove the input room closed.
These are explicitly outside this transport probe, not postponed implicit safety.

Sources checked 2026-09-27: [tokens and grants](https://docs.livekit.io/frontends/reference/tokens-grants/),
[local server](https://docs.livekit.io/transport/self-hosting/local/), and
[official Egress API](https://docs.livekit.io/reference/other/egress/api/).
The last source confirms existing `StartEgress`; no rollback to deprecated RPCs.
