# Supplemental review — store-domains-ui

## Audit before edits

Base `015cdfd6` includes backend `727e3553`; the UI strict parser still used the old closed DTO and would reject `kind`/`edge_addresses`. Marker-only UNKNOWN tracking prevented replay after reload. Existing address section and status rows were retained; no new dashboard scaffolding or controls were added.

## Independent source review

Reviewer: `domains_security_review`, read-only security_reviewer; actual runtime model/effort not exposed to that process (UNKNOWN). Base `015cdfd6`, candidate `777d217d`.

- No confirmed introduced P0/P1 in scoped source.
- Journal is persisted before send; exact original action/key/target required for retry; failed retry retains responsibility.
- Component is keyed by store; journal uses store + CSRF hash, and transport rechecks the session before send.
- Platform controls use authoritative `kind`; DNS parser accepts literal addresses, not arbitrary URLs or hostnames.
- Reviewer did not independently run browser/PG tests (NOT_RUN). Running-session switch UI lifecycle not separately exercised by reviewer (UNKNOWN).
- Source commands: targeted git/rg/sed/nl, exit 0. No reviewer edits.

Humaux: `cd60f11f-dbfe-4784-a2bb-dae95be19443` (independent source review); implementation rationale `572e7e2a-8a9f-475d-b3cd-74803e34e13d` linked to indexed parser/journal/command entities.

## Author visual inspection

First pass at `777d217d`: mobile EN platform type/status labels touched; corrected in `f351f44a` by displaying the type on its own line. Apex records are vertically ordered, complete, and each copy button meets 44px. No approved design direction changed. Final-source gates and screenshots are recorded in SUMMARY.md.

Independent visual reviewer `domains_visual_review` (explorer, gpt-6-luna / medium) read all 12 initial screenshots: only that P2, no other P1/P2. Final-source EN desktop/mobile retake was independently checked and the spacing P2 closed. Humaux `25a89557-fb1e-4426-9b3e-1e8fa22bde56`; original review `5111a2ce-317a-40ec-ba07-e36019aeb2c1`. No reviewer file edits or browser runs.

## Test integrity

No Go/SQL/deploy edits. Frozen R3 driver untouched. Original browser assertions remain; added three real-backend replay checks, read-key rejection, platform-button absence, legacy journal refusal and a separate MOCK apex clipboard/viewport matrix. The existing UNKNOWN test still observes one POST after reload/refresh and disabled new writes; the new explicit replay test supplies the separately authorized recovery action.
