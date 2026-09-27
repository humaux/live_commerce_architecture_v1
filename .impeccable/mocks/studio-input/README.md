# Studio B input controls — proposals, not approved implementation

2026-09-28. Base application `04c8246`. Operate mode; local extension of the
approved Studio B scene list + editor + status rail, not a new visual identity.
All three files are synthetic design comps, **not browser acceptance evidence**.
Their built-in image generation prompts are retained verbatim in sibling JSON
files and embedded into each PNG. No option is approved yet.

## Audit before design

Reviewed `Studio.tsx` editor/action region, its `.studio-*` CSS, root PRODUCT.md
and DESIGN.md, the existing Studio surface brief and actual `studio-b/hero-repro.png`.
The reference capture is historical; its dated README is not current gate status.

- The existing joined surface already separates list, edit and persisted state.
  Preserve it; no dashboard cards, second scene navigator or repeated Start CTA.
- No local device/preview region currently exists. Repeating its editor form in
  a second setup wizard would add duplication without solving that gap.
- Existing media facts represent Egress only. New controls must visibly separate
  local capture, server input custody and platform output. None implies another.
- The current component's Stop eligibility uses Egress nonterminal status; input
  integration must instead consume authoritative input `can_stop` when present.
  Terminal output alone must not hide remaining input cleanup responsibility.

## Three local composition options

|Option|Image|Tradeoff|
|---|---|---|
|A — preview first|`a-preview-first.png`|Large central preview; compact metadata; easiest visual check before starting.|
|B — device check first|`b-device-first.png`|Device rows precede preview; saved settings collapse into a summary.|
|C — inline expansion|`c-inline.png`|Existing form stays primary; device panel expands below; smaller preview.|

All keep the left scene list and right fact rail. Recommendation A is a proposal,
not approval. One async selection question has been sent with all three images
visible. The skill's approval point pauses **UI implementation only**; unrelated
backend work may continue. No app code, global design system or approved surface
brief was overwritten by these proposals.

## Build rules after selection

- Local preview requires an explicit click and browser permission. No server
  room, token request or Egress Start before the explicit server action.
- Device choices before permission may be anonymous/default; never invent real
  device names. Preview stays local and muted to prevent feedback.
- Keep scene/session identity, real prepared authorization, pending command key,
  input and Egress cleanup separate. Token result stays in publisher memory.
- Render actual permission-denied/no-microphone, connection, disconnection,
  uncertain request and retained cleanup states; no false public-live claims.
- Generated text/icons and neutral placeholder are design material, not a new
  backend contract. Fix spelling and retain labelled accessible native controls.
  C's generated thumbnail is not a media aspect contract: real media must respect
  the chosen 16:9 or 9:16 aspect without distortion.
- BRI07 requires approved composition, zh-CN/zh-TW/en, desktop and 390px, keyboard
  and permission/error/reconnect/UNKNOWN checks. BRW06 additionally requires the
  actual authenticated product token to a separate SFU receiver with decoded
  video and nonzero audio; render/build or WebSocket handshake cannot substitute.

Full SaaS, real media output, Cloud qualification, current-tree regressions and
production deployment remain pending; these comps do not change their gates.
