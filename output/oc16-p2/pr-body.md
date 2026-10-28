Closes humaux/live_commerce_architecture_v1#219

Item 1: stored bank-transfer plans previously fell through the terminal error table and retried after closure. Map the exact PT422 bank_transfer_closed pair to payment_unavailable so the existing executor rolls back and records a fenced ABORT receipt. All existing test assertions remain intact.

RED: tests-only head a3b8c31044eefb156eb7607ef1be6d7735185796, Gates run 40628067835/job 122622547420, synthetic checkout cb89258d27e6bcf4c824cd84ddaa38bc8e1ca056. New TestOrderAttemptExecutorAbortClassification/PT422/bank_transfer_closed fails on its own assertion: got empty string, want payment_unavailable. Raw receipt: output/oc16-p2/red.log.

GREEN PENDING on implementation push; contractdrift PENDING. Local tests/servers NOT_RUN (prohibited); static gofmt clean. Independent integrator acceptance NOT_RUN. E3 for observed RED only; implementation acceptance remains E0.

Remaining units NOT_DONE: buyer error-code contract alignment (skip any other module contract per card boundary), bank-field/table privilege pins, and old-key/different-body replay after closure. OC-15 public error-code entry and exact real-route assertion already exist on base (internal/httperror/error.go:199; tests/foundation/bank_transfer_closed_test.go:146–148). Do not close/merge until remaining units are resolved or explicitly scoped out by integrator.

Outside card write paths: none. No migrations, frozen files, test weakening or dependency changes. Rollback: revert the implementation commit and its test predecessor on this PR branch through normal review. Execution and next-session handoff: output/oc16-p2/handoff.md; Humaux fix stored, own subtask canvas updated.

🤖 Generated with Codex (dispatched by Claude Code integrator).
