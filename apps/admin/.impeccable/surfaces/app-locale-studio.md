---
version: 1
slug: "app-locale-studio"
primary_target: "app/[locale]/studio"
related_targets: ["route:/{locale}/studio"]
---

# Studio B — approved composition, UI NOT_RUN

Mode: Operate. Audience: a merchant preparing a programme, selecting a session,
and checking its persisted local rehearsal state without losing list context.
Scope: `/{locale}/studio`, zh-CN / zh-TW / en, in the existing workspace shell.

## Direction and approval

User chose **B 场次列表＋双区工作台** on 2026-09-27. Option `split`, form seed
`6373bb3f`, ranked structure 1. Comp-led reference from repository root:
`.impeccable/mocks/decision/studio-split.png`, native 1586×992.
Preserve PRODUCT.md / DESIGN.md identity; no new visual-world selection.
Orders C remains a separate decision. The image is synthetic design material,
not a screenshot proving working features or connected destinations.

## First viewport and interaction

Retain the incumbent navigation and top bar. Below the title/local-only note,
one joined work surface has a narrow left scene list (about 300px in the actual
approved comp), a broad central programme editor and a right factual status rail.
Thin vertical rules separate responsibilities; no repeated KPI or CTA cards.
New scene appears once above the list, Save draft once under the editor, and the
eligible local rehearsal action once in the status rail. Preserve the comp's
actual proportions over its approximate generation-prompt measurements.

Selecting a scene preserves the list and replaces editor/status context together.
Separate unsaved edits from persisted programme version, prepared authorization
and observed media state. Cancel scene navigation or confirm before discarding
dirty edits. Keep labelled keyboard focus and feedback on failures or conflicts.
On narrow screens use list → editor → status, not a shrunken three-column view;
all three locales must retain every action and actual value without clipping.

## Truth boundaries and states

Use only the frozen `contracts/studio-v1.md` routes and existing domain constraints.
Scheduled time is optional; the generated required marker and 50-character
counter are not requirements. Saving a draft does not issue authorization.
Prepared, expired/stale, missing-authority, read-only, loading, empty, error,
starting, unknown, stopping and terminal states must be distinct and truthful.
Show platform output as unverified unless independently observed; MOCK success
never means Facebook or Instagram is live. No invented camera, chat, recording,
viewer counts, channel connections or automatic grants.

## Acceptance and unresolved work

Composition is approved; UI implementation, screenshots and review are NOT_RUN.
Before merging, reproduce the first viewport at the comp's dimensions; prove
STU04 with real OIDC → Next BFF → Go/PG on desktop/mobile across three locales,
including save/reopen, read-only/no-prepared, start/stop, CSRF and cross-store
denial. Browser must not retain bearer tokens or persistent media payloads.
Run typecheck/build/regressions and independent visual finish review, then record
the built result. Real browser input, Cloud output and deployment have separate
runtime gates; approving this layout does not satisfy or remove them.
