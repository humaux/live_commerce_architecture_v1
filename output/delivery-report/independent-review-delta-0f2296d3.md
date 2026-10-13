VERDICT: PASS

Reviewed SHA: 0f2296d3
Scope: delta only (output/ext-agents/claude-review-pr48-delta3/delta.diff), on top of the 5d549660 full review (PASS). Read-only; no commands run.

Verified against sources
- Row "生产部署批准": the split into "本次升级前强制备份（pre-upgrade-<tag>）成功" and "最近 30 天内完成过恢复演练" matches deploy.md §10 (line 430) and the forced-backup steps (deploy.md lines 114, 144, 182: `pg-ops.sh backup --tag pre-upgrade-<tag>`, abort on failure). The other §10 items in the row are unchanged and correct. The row stays NOT_RUN / RCG 待追加.
- New row (a): deploy.md §6.4 item 5 (line 319) names `ads.capi_contexts`/`ads.capi_events`/`ads.insights_daily` as unregistered in customers-billing CD7 and says CAPI must not be enabled in production until they are. Matches. meta-ads-v1 §12 (line 571) agrees on the blocker, with the narrower wording noted in P2 below.
- New row (b): §6.4 item 6 (line 320): `create_initial_store` does not grant `ads:*`, grants need owner approval. Matches. The report omits the spend-cap default of NT$0 (not an error).
- New row (c): §6.5 item 3 (lines 348–354): stores created before 0079 need `customers:read`, `customers:privacy`, `billing:manage`, with owner approval in chat. Matches. All three permission names are correct.
- Label is **NOT_RUN**. Nothing is claimed as done or verified. The rest of the report (lines 91, 132) only says MOCK / no real CAPI, so there is no contradiction. The row's citations (§6.4 items 5–6, §6.5 item 3, meta-ads §12) resolve.

Findings
- P0: none.
- P1: none.
- P2 (report row 17, line ~149, optional): the row says all three tables must be registered in CD7. meta-ads §12 names a retention class for `insights_daily`/`capi_events` and *erasure* handling for `capi_contexts`. The runbook, which the row cites, does say all three need registration, so the row is consistent with it. To remove the wording gap, say "保留类／擦除（capi_contexts）登记". It does not affect the verdict.
- P2 (optional): row 17 could add "每店广告上限默认 NT$0（须运营另设）" from §6.4 item 6, so the grant is not read as ready to spend.
