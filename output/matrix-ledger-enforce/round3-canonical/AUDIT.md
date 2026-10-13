# Scoped canonical-claim audit

Read-only explorer `gpt-6.1-sol`, high, bound to source `05c2af77a6f90cef0fc797520b2f6361d1635527`. Humaux records: source audit `24107b66-b555-4546-93de-d40ec8761ad1`; preliminary artifact audit `3b13e80b-823e-47b5-b14e-2a18ab97d122`; final artifact audit `29f99688-24b5-4cbc-a1d8-c74ab36c63f3`.

The actual `node --input-type=module` probe command exited 0: baseline had zero findings; ten in-memory negative probes each had findings. Six audited source hashes remained unchanged before and after. No source was edited and no browser or full gate was rerun by the auditor.

| Negative probe | Findings |
| --- | ---: |
| Read persisted row zero twice | 3 |
| Change edit-code salt | 12 |
| Replace existing code in fieldsFor | 18 |
| Read archived ID zero twice | 3 |
| Change clock suffix | 45 |
| Read compare price as saved price | 18 |
| Drop confirmation await | 3 |
| Use wrong native replacement source | 3 |
| Certify generated code with self-read | 3 |
| Drop persisted axis price assertion | 3 |

Artifact audit independently checked 18 source Git/current/run hashes, local and browser log hashes/exits, six mutation snapshots/logs, native Playwright pass counts and exact ledger identities. It confirmed 12 PE/review and two CC12 passes, zero skips, raw 286 rows, required 231 unique PASS rows at 390px, 77 per locale. Three native receipts had `accepted: true`, `beforeId: null` and replacement UUID.

This is scoped automated source/artifact evidence. It does not constitute new K3/Claude approval, remote CI or release approval.

The final artifact audit precedes only this record-ID addition and DELIVERY's staged-gate/whitespace receipt references. Its source, run, canonical-copy and red-proof assertions remain unchanged. Root subsequently checked the completed staged gate (exit 0); the executable source stayed frozen.
