# Legacy family isolation independent PG test author

Status: candidate LRI01 only; LRI02–04 and full regression are NOT_RUN at this checkpoint.

- Baseline `1fec2b3` retains the original causal RED source from `d644829` and root RED log `/Volumes/data/output/legacy-runtime-isolation-root-real-red-20260926.log` (SHA256 `8fc59ec77cb532732e33729d69bc4886c25b71e1ae6a5fab297501db164b8176`, exit 1).
- Product candidate cherries: SQL `5b3e286` (from `3a03f53`), Go `85cc3af` (from `69f64d4`), selector `bcc2e2f` (from `732b10e`). These are candidates, not acceptance.
- Test author first focused `bash scripts/dev/test-local.sh --legacy-isolation` exit 0: `TestLegacyRuntimeIsolationExpiryDoesNotMaintainForeignFamilies` PASS 5.74s, foundation 7.249s under `-race` in an isolated local PG18. Log `/Volumes/data/output/legacy-isolation-author-causal-candidate-20260926.log`, SHA256 `c397bb20670dc0f89bec94fdd4553c8863d443c41d6fb3742617175f6f8f57ee`.
- The unchanged causal structure still uses the actual expiry CLI and four own scheduler/rescuer/cleaner controls. Test-only table binding now snapshots payment rows in `river_payment`, expiry controls in `river_expiry`, and external rows in `river`; it compares full foreign job rows and business receipts. Independently sequenced IDs are never resolved without a family table.
- Historical queue-migration tests need the exact pre-0032 fixture and old `river` clients; current fixture test references are being separated. This checkpoint does not claim those tests pass.
