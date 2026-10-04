# Final evidence packaging postflight

Source: `068874fc62a6f580ccd97ce55d7eb3b17534c641`.

- Final full G07 exited0; source-postflight.log records unchanged HEAD and runtime/test diff exit0.
- G04 strict subset exited0 with final source and staged evidence. See g04/results.tsv. This subset is not a full release verdict.
- Staged files were checked with `git diff --cached --name-only | awk '!/^output\// {bad=1; print} END {exit bad}'`: exit0, no non-output path.
- `git diff --cached --check`: initially exit2 solely from generated Next build progress trailing spaces/CR. Only the eight log files named in staged-diff-check.log and that diagnostic log had trailing whitespace normalized. Red test outcomes remain unchanged. Recheck exit0.
- `ps -axo pid=,comm=` filtered for foundation.test/Next/Playwright/test-local/release-gate: no foundation.test, Playwright or active test harness; nine surviving Next processes.
- `lsof -a -p 2773,33586,43957,51715,51926,52202,52294,52425,63908 -d cwd -Fpcn`: none under this ads-attribution worktree. Their other-worktree servers were not modified or terminated.
- `docker ps --format '{{.Names}}'`: only pre-existing humaux-thread-qdrant and humaux-thread-pg. No LC test container remains.
- No secret files were read, no credentials copied into evidence, no Meta network mutation, no push/deploy.

Tests were not rerun during packaging; only output artifacts changed after the final source commit. Historical red evidence is retained.
