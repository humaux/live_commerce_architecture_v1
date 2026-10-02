# Independent review and disposition

Review date: 2026-10-02. Source reviewed at `0b23136`; the subsequent `9f55dc3` changes one zh-TW receipt phrase only.

## Source / safety

Reviewer: `/root/domains_security_review`, read-only `security_reviewer`, gpt-6-sol / high. No files changed. Scoped `git`, `rg` and `nl` commands returned 0. The reviewer did not independently run PG/browser acceptance; that part is **NOT_RUN** for the reviewer.

- Static/image GET/HEAD no longer bypass the canonical-origin resolver. Target comes from the private Go endpoint; UI-controlled redirect targets are rejected. Source closure confirmed.
- Domain commands persist a store/session-scoped pending marker before sending. UNKNOWN retains it and disables writes through reload; refresh issues GET only. Source closure confirmed; author's browser gate separately covers lost-response containment.
- **P0 / BLOCKED:** Go ignores the forwarded Idempotency-Key. Repeating a request rotates the TXT token/version in SQL. The UI containment is not I02 compliance, cross-tab coordination, or durable server deduplication. Real-PG same-key replay proof remains NOT_RUN.
- Missing authoritative platform/custom row discriminator blocks removal of platform-row actions. Do not guess from hostname, token, order or serving status.
- Apex DTO has no exact edge IP. UI no longer presents a hostname as an A record; complete apex setup remains backend-dependent.
- Initial-store receipt validation checks syntax/handle prefix, not a separate base-zone allowlist unavailable to admin. The receipt producer is the trusted Go service; this is a producer-contract dependency, not a demonstrated externally controlled open redirect.

Humaux titles: `Store-domains UI final review correction: domain Idempotency-Key ignored`; `Store-domains UI 0b23136 source closure: redirects and UNKNOWN read-only`.

## Visual

Reviewer: `/root/domains_visual_review`, read-only explorer, gpt-6-luna / medium. Six locale/viewport captures were reviewed against the owner-supplied overview language and SURFACE.md. Visual-only disposition: **ship** for this narrow settings extension; not release approval.

- 1586×992 and 390×844 captures for zh-TW, zh-CN and en are readable; long addresses/TXT wrap; scoped controls are at least 44px.
- White surface, flat borders and restrained orange verification action preserve the incumbent shell. No shell redesign or unsupported controls introduced.
- Follow-up reviewed `domains-zh-CN-desktop-dns.png`: record names/values and copy action are readable, no clipping/overlap. Minor non-blocking observation: nested bordered DNS group could be visually flatter. Retained to avoid expanding this scoped handoff.
- Platform-row controls visible in screenshots are the explicitly blocked functional item, not visual acceptance of that behavior.

Humaux title: `Store-domains UI visual finish review (2026-10-02)`; follow-up DNS review also stored by reviewer.

## Author verification versus approval

The author ran the required gates; see SUMMARY.md for exact exits and evidence boundaries. An independent reviewer checked source and visuals, but did not independently rerun the full gate set. The integrator owns final acceptance. Outstanding backend P0 and item 2 prevent an unqualified merge/release recommendation.
