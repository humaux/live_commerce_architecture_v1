<!-- Purpose: current K3 source review.
Depends on: original raw SHA256=3877bde4723110cbb7be2c8e4d45345ad34b8b7c0f3ee405cdfa02c2277ed83e.
Used by: integrator normal/calibration. -->
<!-- Purpose: independent, narrowly scoped W5 K3 source/test review of PRIVATEALL, terminal replay and safe CSV corrections.
Depends on: parent W5 worktree b20e1bcf plus the ten SHA256-bound candidate paths below and the current owner PRIVATEALL ruling.
Used by: parent task a83bad3d-2b4d-4b4a-b598-85f9e4a99cea and integrator; this report does not certify runtime or CI acceptance. -->

Source review: no concrete P0/P1 found in this ten-path candidate. Reviewed on 2026-10-07 by `/root/w1_ci_diagnosis`, configured same-family gpt-6.1-sol/high; exact runtime model is not exposed. Parent worktree HEAD was `b20e1bcf998b4e66450b9e18a957fe5c6fb8183c`, with the ten modified paths listed below. No implementation, test execution, merge, push or external action was performed by this reviewer.

- **PRIVATEALL:** `import-proxy.ts:14-26` normalizes every returned import response, including local/auth/upstream/malformed-response refusals, to exact `private, no-store`. The wrapper mutates the existing constructed response rather than replacing it, preserving body/status, Set-Cookie, request-ID and Allow. The shared auth helper remains authoritative and unchanged. The exact leaf explicitly refuses HEAD/PUT/PATCH/DELETE/OPTIONS using that wrapper, so its implicit OPTIONS path cannot escape the policy. This current owner ruling supersedes the earlier review's acceptance of bounded `no-store` coded errors; the earlier error-classification repair remains historical fact.
- **Known saved mismatch terminates:** `import-proxy.ts:69-73` first parses the closed receipt DTO, rejecting fresh count mismatch while permitting a valid replayed mismatch. `import-client.ts:52-64` still enforces PRIVATEALL, closed refusal envelopes and the post-response session fence. Only a valid `replayed: true` mismatched receipt becomes `terminal`; malformed and first-response mismatches remain uncertain. No automatic resend was added.
- **UI recovery:** `ImportWizard.tsx:136-145` clears pending and UNKNOWN, stores the authoritative receipt and shows localized mismatch guidance without granting new customer readiness. The result renders no Confirm or Retry control; Back/type/restart reset the file/mapping before another operation. The retained earlier positive customer receipt, if any, remains the only readiness authority. Normal UNKNOWN still retains the frozen original File/mapping/count until an explicit retry or scope teardown.
- **Safe failure CSV:** `import-view-model.ts:12-16` now produces only failed rows under `row,outcome,code`; source cells/IDs and success rows remain absent. Warning metadata is retained in the existing preview UI. The browser-spec delta aligns both local and server projected headers and adds actual Go422 5001-row / Go409 conflict expectations, no Retry and persistence-count assertions. The existing case and calibration logic was not removed or weakened in the reviewed diff.

The client tests invoke the actual route/auth/settings/helpers with native Requests and replace network edges plus minimal unavailable browser DOM facilities. Malformed-response substitutions are explicit negative cases, not helper stand-ins. The coordinator transpiles the actual Form AST but uses synthetic hooks; its new terminal case asserts absence of Retry, frozen retry identity, no customer bootstrap, and no unconfirmed payload on teardown. Navigation safety above is source inspection, not a browser claim.

Author/parent-reported green counts are not independent reviewer test evidence. No Node, TypeScript, Go, PostgreSQL, Next, browser, calibration, full test-local or GitHub gate was executed in this review. All such acceptance remains NOT_RUN by this reviewer; parent fresh checks and integrator CI must bind their own receipts to these bytes. The prior W6 cancelled run does not confer acceptance on this candidate. This is an independent source assessment within the same model family, not independent runtime verification. The reviewer started no persistent process.

| Candidate path | SHA256 |
|---|---|
| apps/admin/app/api/stores/[store]/imports/[param]/[action]/route.ts | 7a39640ca8fd5b6f33b6798a8108178e0ec7d83d993bf7ac6faf8e8e7158f9dd |
| apps/admin/components/ImportWizard.tsx | 5841479c9ca9697c2dca6ac242ec1601698e248173982d283eebba4ac8fe9480 |
| apps/admin/lib/import-client.ts | a719c5c7653d1a7ae1d00ed322d7b0aef27709aa1bb1ca5ebab4515417c33d0b |
| apps/admin/lib/import-proxy.ts | 56b278c358a7aa745f6175e89b97bcd2a4425dc8c51a874205978cf22f55c966 |
| apps/admin/lib/import-view-copy.ts | fc271cdbe510400e13a7c38eb977cf0632cd7b523405c36cbd404730ecea4b9a |
| apps/admin/lib/import-view-model.ts | c82133417e1e743f4dcfc4fa604775df795bbcd7c0a31980a869445820fdd160 |
| tests/admin/import-client.test.mjs | 4428bbfeca2ab7276a007ed5341df4ad56260e7ac75eea394229a17ce47060f2 |
| tests/admin/import-coordinator.test.mjs | 6d1843ee4182427d95ec0613a57711cb834a7af2f798b9f61fed232be66c1818 |
| tests/admin/import-view.test.ts | 6526428dd56286d34f6a46df8830c30b9a9c58a681288592fd26240758bbff58 |
| tests/admin/import-wizard.spec.ts | 7ad40ef290a5e15710929de77f716f094b2f60ae633bd44d0cc5f1b4fe978f80 |
