# Merchant logistics and payments wizard

User approval: 2026-09-24, **A horizontal steps + right status column**.
Approved comp: `mocks/merchant-settings-a.png`; exact generation prompt and
approval travel in its JSON sidecar and embedded PNG metadata. B/C are retained
decision evidence, not shipping assets. Existing `DESIGN.md` remains authority.

## Direction contract

The merchant is connecting their own provider account for the first time. Keep
the existing navy navigation, quiet store/language header and teal primary action.
Use four horizontal numbered steps: choose platform, connect account, configure
methods, check status. Within the working surface, the active form occupies about
two thirds and a quiet read-only status column one third. One primary action
advances the current step; previous/back is secondary. On narrow viewports the
stepper becomes compact and the status follows the form, not a competing panel.
The UI must distinguish saved credentials, provider qualification, method enabled,
buyer visibility and runtime availability. Saving cannot promise provider approval
or start a payment. This composition approval is not live integration acceptance.

## Component grammar and asset inventory

Use existing Arial/PingFang stack, 29px page heading and 14px body baseline;
nav #193c61, ink #142942, muted #64758a, line #dde5ee, canvas #f5f7fa,
white and teal #247965. Reuse 5px control/7px panel radii and flat 1px borders,
no new elevation/gradient. Contrast must be measured in rendered code.

|Ingredient|Medium|Implementation commitment|
|---|---|---|
|Navigation/header|Existing semantic components and SVG icons|Preserve incumbent shell, real store and language; no invented store data|
|Four-step sequence|HTML ordered list, CSS connectors|Active/completed/future text, keyboard focus, no color-only status|
|Account form|Labels, fieldset, inputs, radio controls|Empty secret inputs; browser autocomplete off as appropriate; no prefilled dot placeholders|
|Right status column|Semantic definition list|Only server-derived metadata; saved is not verified or buyer-available|
|Primary action|Native button + CSS|One teal submit/continue, pending/disabled/error, preserve non-secret idempotency context|
|Safety explanation|Semantic note and SVG icon|Always distinguish configuration from provider/live capability; localized exact meaning|
|Responsive arrangement|CSS grid/media query|Form first, status below on mobile; no horizontal overflow at 390px|

Independent asset-producer inspected actual PNG: produce=[], direct=[]; all
ingredients semantic. Humaux research: `Merchant settings A approved comp asset
producer manifest`. Do not ship screenshot crops or generate decorative assets.

## Do not literalize generated text

- The account form must start with blank HashKey/HashIV fields, not fake masked data.
- Do not imply the platform itself approves accounts immediately after the wizard.
- Do not show live enabled when adapters/qualifications remain gated.
- PAYUNi is the currently implemented account credential schema, not a claim that
  every provider in the intended product catalog is connected.
- Never store secret values in URL, localStorage, sessionStorage, telemetry, SSR
  data or retry journal. Clear them after submission/navigation; uncertain results
  recover through safe metadata and explicit re-entry.

## Acceptance

Backend transport gate is `contracts/merchant-account-http-v1.md` AC01-AC05.
UI separately needs three locales (zh-CN, zh-TW, en), desktop/mobile screenshots,
keyboard/error/pending/empty states, secret cleanup and real saved-state reload.
Full method configuration requires real market discovery and existing settings
API coverage; it may not be replaced by a fabricated success screen. External
sandbox/live provider acceptance remains a distinct gate. Current comp is approved;
this document alone does not mark runtime UI delivered.
