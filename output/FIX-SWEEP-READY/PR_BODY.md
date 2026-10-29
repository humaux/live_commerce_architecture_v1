FIX-SWEEP-READY: a page can have quiet DOM/network while its loading placeholder is still rendered. The sweep then checks headings or enumerates controls too early. Add bounded readiness to first navigation and recovery, preserving the heading, error and control assertions and all existing timeout values.

RED first: tests-only c5c473a32000ecca3301ef7454ee9e1080759bad failed all six NEW readiness cases in Foundation run 40845524523 / job 123351940744 (Node unit suites, exit 1). Quiet busy/status/skeleton returned early; persistent busy did not reject; reload returned early; first/fresh navigation lacked readiness wiring. See output/FIX-SWEEP-READY/red.log.

Implementation: opt in to readiness on first navigation, fresh contexts, reload and layer-close recovery. Read-only DOM probes check main/descendant aria-busy and visible loading status/skeleton markers inside the existing settle deadline. A permanently busy page throws `page stayed busy`; heading/error/control assertions and timeout values remain unchanged. No pages or controls exempted. GREEN and independent review remain pending this PR's implementation CI.

Validation: CI only; local `node --check` syntax check only. No local tests or servers. Browser ownership regenerated with the required static generator after syncing origin/r3/integration.

Scope: tests/ui/click-sweep-lib.test.mjs, tests/ui/click-sweep-lib.mjs and tests/ui/click-sweep.mjs; generated output/ci-select-backend-browser/browser-ownership.json; output/FIX-SWEEP-READY evidence. Outside-card approval list: none; generated ownership and evidence are card-required. Gate-adapter assertions remain untouched. No PR merge or auto-merge authorized.

Evidence: E3 causal CI RED, E1 local syntax only; GREEN and independent acceptance pending. Local tests/servers NOT_RUN (prohibited). Base d8093bc43f16f8eb4e209eeb6c7b698efcd27b0e, worktree .worktrees/sweep-ready, branch unit/sweep-ready. Integrator-dispatched implementer Codex; runtime model/effort configuration UNKNOWN. Humaux causal RED record queued and own sub-canvas update submitted under acquired locks. Rollback: integrator can revert the implementation commit through its normal PR process.

🤖 Generated with Codex (dispatched by Claude Code integrator).
