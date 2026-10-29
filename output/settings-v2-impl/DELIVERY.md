# SETTINGS-V2-IMPL — tests-only RED handoff

2026-10-29; role: dispatched UI implementer; Codex (runtime exact model/effort not exposed).
Base: e324e01a93219c099d6d2a9d35ed1f112595fe90. Branch unit/settings-v2-ui.
Author PASS is E0; static checks E1 only. UI implementation and E3/E4 NOT_RUN.
See PLAN.md for exact paths. Historical blocked conclusions are superseded by #287.

## DESIGN PARITY

|Required fact|Desktop/mobile and review assertion|Status|
|---|---|---|
|Profile name, email, optional phone|All fields filled, review checked, save receipt and refresh checked|CI pending|
|Five delivery states|All five rows; unavailable without existing services; enabled with MOCK existing mapping|CI pending|
|Stripe/default/timing/no COD|Platform summary and review safety copy|CI pending|
|Manual shipping/carrier/tracking/no automatic labels|Review facts asserted|CI pending|
|First product draft/currency/price/stock|Three-step review plus actual save/refresh assertions|CI pending|
|Directory date/public URL|Must use existing responses only; never sample Figma facts|Implementation pending|

## UI PROCESS — BOURDIEU

Q1 three-row overview reduces recognition burden; five-second comprehension UNKNOWN.
Q2 direct edit and skip for recent task experts. Q3 retain payment/shipping constraints.
Q4 no new store-code task. Q5 mobile/desktop share required facts. Q6 aesthetic judgment
UNKNOWN; follow supplied sign-off. Novice/expert means recent experience with this task.

## UI PROCESS — PROJECT UNDERSTANDING

Read AGENTS.md; architecture §0/1/2/26/27; merchant-settings-wizard-v1 and
merchant-service-settings-v1. They impose evidence separation, backend transaction
truth, platform Stripe, isolated ownership, quick-start marker and configured-versus-
qualified separation. Current profile amendment freezes CAS/replay and five existing
MANUAL service mappings, forbids market/policy seeding and guessed codes.
Read current source rather than relying on historical output reports.

## UI PROCESS — MOBBIN → EAGLE

Signed design uses module folder MVQL9YZXCBO8X and candidates MVOVLMG9N4TCQ
(task grouping), MVOVLMGBSNKXG (one choice/state per row). This implementation
session does not claim fresh Eagle image review/import. No new visual pattern proposed;
no new Mobbin query needed. Existing signed design remains the sole UI authority.

## UI PROCESS — FIGMA OFFICIAL GUIDANCE

Signed handoff cites G1 auto layout, G2 semantic variables, G3 components/variants,
G4/G5 prototyping and G6/G7 handoff. No Figma writes. Read-only Fleet status shows one
connected document; fresh get_design_context reads mobile overview 1429:32392.
Implementation will translate signed layout into incumbent CSS, no redesign.
No local Figma write/lint or production acceptance is claimed.

## Click ledger (tests written; execution pending)

|Control|Operation / expected outcome|Actual|
|---|---|---|
|Three rows + Edit, three locales|Click opens editor|NOT_RUN|
|Profile fields / review / back|Fill, review preserves values|NOT_RUN|
|Save / result / refresh|Actual profile receipt and persisted values|NOT_RUN|
|Definite failure / back|Visible failure, draft retained|NOT_RUN|
|Uncertain / reload|Read original state, zero blind retry|NOT_RUN|
|Three steps / back / product / save|Draft product and persisted summary|NOT_RUN|
|Skip / refresh|Existing server marker persists dismissal|NOT_RUN|
|Five delivery switches|Click, preserve existing fields, save, refresh|NOT_RUN (MOCK receipts)|

## Languages / evidence / limits

Three-locale signed strings are literal test expectations; production entries are pending.
Do not confuse authored tests with successful clicks. Existing safety suite untouched apart
from importing/calling the new acceptance helper. No local test/server was run. CI output
will be recorded in red.log; implementation requires causal RED first. No live credentials,
provider calls, external messages, production changes, merge or auto-merge.
Humaux search/grep/store/canvas/coord NOT_RUN (tools absent). No background local process.
