Align merchant order labels with their task queues in zh-TW, zh-CN and en. Explain old-system customer numbers beside import mapping controls and clarify that the existing failed-row CSV contains only safe metadata. State codes, filters, CSV serialization, erasure protection and original-request retry behavior retain their existing implementations.

CP-05 is skipped per accepted OC-Q6: no 管理連結 label or disconnect action.

RED at 26d5f721a9e519fe6e4e5736a0a4c1288854903b:
- Foundation run 40645100067 / job 122671969067: ORD-01 assertion fails, Unpaid vs Awaiting payment.
- Gates run 40645100039 / job 122683775768: CU-01 assertion fails, sourceIdHint undefined.

The two inherited RED files tests/admin/orders-v2.test.ts and tests/admin/import-view.test.ts are outside original card JSON paths; their existing-file registration was explicitly authorized by the integrator. They are unchanged in the implementation commit. No other out-of-card files remain changed.

Validation: allowed packet structure check and node --check for both copy modules exit 0 (E1 only). No local tests/servers. Implementation GREEN, typecheck, real browser clicks, three-locale rendering/refresh and independent review remain pending. Evidence and limits: output/ui-copy/DELIVERY.md and red.log. Integrator pushes the local implementation head and supplies its own CI run/job IDs; no GREEN claim here.

🤖 Generated with Codex (dispatched by Claude Code integrator).
