# Final source provenance

- Branch: `unit/ads-attribution`.
- Frozen implementation/test commit: `068874fc62a6f580ccd97ce55d7eb3b17534c641`.
- `git merge-base --is-ancestor cd61add5 HEAD`: exit0.
- `git diff --exit-code cd61add5 HEAD -- migrations/0112_meta_ads_refusal_text.sql`: exit0; the actual file exists and is preserved.
- Attribution migration: `migrations/0113_ads_attribution.sql`.
- Root final-gate runner checks HEAD and clean tracked runtime/test directories before G07 and after it returns. Only output evidence changes during the run.
- Static, focused PG, browser and click-sweep execution receipts: `exit-codes.tsv`.
- The G07 phase will be launched separately after prior harness cleanup and three quiet-machine samples, using an exact process-name check. This avoids an orchestration command line matching its own literal `foundation.test` text.
- This is local REAL_PG/MOCK verification. No Meta writes or production deployment.
