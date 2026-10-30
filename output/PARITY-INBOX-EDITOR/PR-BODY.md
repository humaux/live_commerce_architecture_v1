Tests-only RED candidate for PARITY-INBOX-EDITOR. No application changes yet; do not merge.

RELAY 2: Current-head Foundation `41053520136` / job `124055190539` passed. Gates `41053520134` / job `124057814067` (`bundle:06`, synthetic merge `bda615b1264c376d512599abb545b1f99060c6e9`) reported 18 inbox failures / 70 passes but could not upload the full assertion evidence: GitHub rejects `:` in screenshot paths. Counts alone are NOT causal UI RED. Targeted artifact fix 1 normalizes only physical screenshot paths in both parity helpers; original signed frame IDs remain in attachment labels and this table. No assertions changed. Fresh assertion-level RED remains PENDING; implementation NOT_STARTED.

RELAY 1: Foundation `41052476505` / job `124051679004` at `6f212733ab43` failed in the inbox discovery harness (69 collected versus the original 39). This is not UI assertion RED. Targeted fix 1 preserves all original 39-case checks and separately asserts the exact 30 added parity cases; browser assertions stay unchanged. New-head CI and implementation remain PENDING.

Pins signed copy/control checks to real `/[locale]/messages` and `/[locale]/design` routes, adds the open-window DM follow-up regression and frame-named screenshots. Existing gate-adapter and adversarial inbox assertions stay intact. Both existing browser specs retain their registered CI owners.

Validation: node --check on both specs exit 0; deterministic ownership generation/check exit 0. Local tsc unavailable (command exit 254); no dependencies installed. No local tests, builds, servers, PG or browsers. RED on own assertions, GREEN and independent visual review PENDING. See output/PARITY-INBOX-EDITOR/red.log and DELIVERY.md.

DEFERRED → INBOX-PREVIEW: list-row last-message preview on all list-containing frames, per integrator ruling (privacy review of decrypted A8 bodies). Implement list rows without that line or an empty slot. No API/contract change; all other parity remains in this card. Frame fixtures use server-shaped MOCK versions 42/27→43/28 and synthetic buyer/channel/content facts, not hard-coded Figma example facts. Screenshots must be reviewed for layout and fixed copy; sample values differ by design.

Preview-line DEFERRED frames: desktop `1429:37361`, `1429:37452`, `1429:37543`, `1429:37634`, `1429:37725`, `1429:37816`, `1429:37907`, `1429:37998`, `1429:38090`, `1429:38182`, `1429:38274`, `1429:38366`, `1429:38458`, `1429:38550`, `1429:38642`, `1429:40688`, `1429:40780`, `1429:40872`, `1429:61092`, `1429:61183`; mobile lists `1429:38826`, `1429:38876`, `1429:38926`, `1429:38976`, `1429:39026`. Only the list-row preview line is deferred; all other content/states remain in scope.

Outside card write paths: `tests/admin/inbox-discovery.test.ts` (same inbox module harness required for the added cases; integrator review requested); generated output/ci-select-backend-browser/browser-ownership.json (explicitly pre-approved); task-local output/PARITY-INBOX-EDITOR/ receipts/documentation. No other app, backend, contract, migration, dispatcher baseline or lockfile changed.

Parity sheet (screenshots PENDING, never claimed produced until browser CI). Desktop 1440×900; mobile 390×844. Artifact filenames map `:` in frame IDs to `-` because GitHub rejects colons in upload paths; attachment labels retain the exact frame IDs. Paths are under each suite's browser artifact test-results. 43 inbox current frames and 32 merchant-editor current frames; buyer-only editor frames excluded from this admin card. Current inbox polish PNGs supersede simplify exports for seven roots. Every mapped screenshot is requested by tests; a prior assertion failure can prevent its creation.

|Frame|Real-route browser artifact|Current DELIVERY PNG|
|---|---|---|
|1429:37361|`parity/1429-37361.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37361-1793201291248-0.png`|
|1429:37452|`parity/1429-37452.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37452-1793204688746-0.png`|
|1429:37543|`parity/1429-37543.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37543-1793204688888-0.png`|
|1429:37634|`parity/1429-37634.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37634-1793204689022-0.png`|
|1429:37725|`parity/1429-37725.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-37725-1793204689206-0.png`|
|1429:37816|`parity/1429-37816.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37816-1793201292448-0.png`|
|1429:37907|`parity/1429-37907.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37907-1793201292673-0.png`|
|1429:37998|`parity/1429-37998.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-37998-1793201292910-0.png`|
|1429:38090|`parity/1429-38090.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38090-1793201293146-0.png`|
|1429:38182|`parity/1429-38182.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38182-1793201293392-0.png`|
|1429:38274|`parity/1429-38274.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38274-1793201293638-0.png`|
|1429:38366|`parity/1429-38366.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38366-1793201293854-0.png`|
|1429:38458|`parity/1429-38458.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38458-1793201294258-0.png`|
|1429:38550|`parity/1429-38550.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38550-1793201294486-0.png`|
|1429:38642|`parity/1429-38642.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38642-1793201294691-0.png`|
|1429:38826|`parity/1429-38826.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-38826-1793201294846-0.png`|
|1429:38876|`parity/1429-38876.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-38876-1793204689348-0.png`|
|1429:38926|`parity/1429-38926.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-38926-1793204689433-0.png`|
|1429:38976|`parity/1429-38976.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/evidence/polish-20261029/exports/get_screenshot-1429-38976-1793204689530-0.png`|
|1429:39026|`parity/1429-39026.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39026-1793201295525-0.png`|
|1429:39076|`parity/1429-39076.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39076-1793201295683-0.png`|
|1429:39114|`parity/1429-39114.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39114-1793201295839-0.png`|
|1429:39152|`parity/1429-39152.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39152-1793201296040-0.png`|
|1429:39190|`parity/1429-39190.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39190-1793201296229-0.png`|
|1429:39229|`parity/1429-39229.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39229-1793201296395-0.png`|
|1429:39268|`parity/1429-39268.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39268-1793201296548-0.png`|
|1429:39307|`parity/1429-39307.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39307-1793201296722-0.png`|
|1429:39346|`parity/1429-39346.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39346-1793201296894-0.png`|
|1429:39385|`parity/1429-39385.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39385-1793201297061-0.png`|
|1429:39424|`parity/1429-39424.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39424-1793201297244-0.png`|
|1429:39463|`parity/1429-39463.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-39463-1793201297398-0.png`|
|1429:40688|`parity/1429-40688.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-40688-1793201297634-0.png`|
|1429:40780|`parity/1429-40780.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-40780-1793201297872-0.png`|
|1429:40872|`parity/1429-40872.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-40872-1793201298299-0.png`|
|1429:41042|`parity/1429-41042.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-41042-1793201298555-0.png`|
|1429:41081|`parity/1429-41081.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-41081-1793201298753-0.png`|
|1429:41120|`parity/1429-41120.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-41120-1793201298915-0.png`|
|1429:58696|`parity/1429-58696.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-58696-1793201299400-0.png`|
|1429:58822|`parity/1429-58822.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-58822-1793201299608-0.png`|
|1429:61092|`parity/1429-61092.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61092-1793201299842-0.png`|
|1429:61183|`parity/1429-61183.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61183-1793201300071-0.png`|
|1429:61312|`parity/1429-61312.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61312-1793201300219-0.png`|
|1429:61350|`parity/1429-61350.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-inbox/output/ux-inbox/shots-simple/get_screenshot-1429-61350-1793201300370-0.png`|
|1429:63114|`parity/1429-63114.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-63114-1793196701844-0.png`|
|1429:63170|`parity/1429-63170.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-63170-1793196703459-0.png`|
|1429:64602|`parity/1429-64602.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64602-1793196709928-0.png`|
|1429:64684|`parity/1429-64684.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64684-1793196710010-0.png`|
|1429:64758|`parity/1429-64758.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64758-1793196710144-0.png`|
|1429:64840|`parity/1429-64840.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64840-1793196710221-0.png`|
|1429:64914|`parity/1429-64914.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-64914-1793197063522-0.png`|
|1429:65008|`parity/1429-65008.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65008-1793197062030-0.png`|
|1429:65094|`parity/1429-65094.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65094-1793196710551-0.png`|
|1429:65151|`parity/1429-65151.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65151-1793196710628-0.png`|
|1429:65280|`parity/1429-65280.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65280-1793196710747-0.png`|
|1429:65337|`parity/1429-65337.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65337-1793196710822-0.png`|
|1429:65386|`parity/1429-65386.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65386-1793197063807-0.png`|
|1429:65477|`parity/1429-65477.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65477-1793197062635-0.png`|
|1429:65560|`parity/1429-65560.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65560-1793196711158-0.png`|
|1429:65613|`parity/1429-65613.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65613-1793196711240-0.png`|
|1429:65658|`parity/1429-65658.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65658-1793196711369-0.png`|
|1429:65714|`parity/1429-65714.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65714-1793196711450-0.png`|
|1429:65762|`parity/1429-65762.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65762-1793196711577-0.png`|
|1429:65800|`parity/1429-65800.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65800-1793197063175-0.png`|
|1429:65830|`parity/1429-65830.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65830-1793196711777-0.png`|
|1429:65868|`parity/1429-65868.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65868-1793196711855-0.png`|
|1429:65898|`parity/1429-65898.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65898-1793196711979-0.png`|
|1429:65934|`parity/1429-65934.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65934-1793196712060-0.png`|
|1429:65962|`parity/1429-65962.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-65962-1793196712183-0.png`|
|1429:66000|`parity/1429-66000.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66000-1793196712266-0.png`|
|1429:66030|`parity/1429-66030.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66030-1793196712392-0.png`|
|1429:66073|`parity/1429-66073.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66073-1793196712469-0.png`|
|1429:66108|`parity/1429-66108.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66108-1793196712592-0.png`|
|1429:66154|`parity/1429-66154.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1429-66154-1793196712669-0.png`|
|1430:68109|`parity/1430-68109.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1430-68109-1793196712796-0.png`|
|1430:68181|`parity/1430-68181.png`|`/Volumes/data/live_commerce_architecture_v1/.worktrees/ux-editor/output/ux-editor/shots-simple/get_screenshot-1430-68181-1793196712876-0.png`|

🤖 Generated with Codex (dispatched by Claude Code integrator).
