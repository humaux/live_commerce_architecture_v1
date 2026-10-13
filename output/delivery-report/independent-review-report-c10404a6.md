VERDICT: FAIL

Reviewed SHA: c10404a6 (docs/delivery/DELIVERY-REPORT.md, full report). Reviewer: claude-sonnet-5-5, read-only, no shell.

## Verified
- Evidence labels: MOCK / SANDBOX / LIVE / NOT_RUN are used consistently with AGENTS.md. No SANDBOX or LIVE PASS is claimed. RCG is NOT_RUN. The report explicitly says it does not authorize launch.
- PR numbers, titles and merge state match prs.txt and git-log.txt. Merged: #2–#5, #8, #12–#16, #18, #20, #22–#31, #33–#37, #39–#44 and #28 (= 0bfe36e5, with #43 = 5a54d41b as its parent). OPEN: #32, #46, #47 and the new #48. PR #1 (OPEN) is not cited.
- Mode names exist in scripts/dev/test-local.sh. I checked 41 of the cited `browser-*` modes by grep. `--stripe-browser` is correctly stated as excluded from the PR set (GATES.md line 279).
- All 14 cited units/*.md files exist. deploy.md §§6.3, 6.6, 6.7, 7 and 10 exist. deploy/README.md "Known limits recorded by the R1 deploy unit" exists. The stripe-live runbook policy_pages step (LG01) and steps 0–6 exist.
- PROCESS.md anchors #1, #2 and #4 resolve.
- release-gate.sh covers G01–G07, G06n, G07z, G90, G91 and G99 only. tasks.json T19 is NOT_STARTED, and T22 (release readiness) is present. The R1-vs-G15 scope statement holds.
- reporting-v2 is DRAFT, as the report says. The owner-only red lines match AGENTS.md. No secrets or buyer PII found.
- CI run IDs (38357392862, 38338136795, 38352001282, 38352001264, 38293593095, 38293840385) and the job IDs cannot be fetched, so they are cited-but-unverified. The 5a54d41b gates.yml#L9-L18 permalink was also not checked.

## Findings

### P1 — Launch blockers in deploy.md §0 are only partly carried over (lines 136–146)
- deploy.md lines 24–32 list B5 (Studio/LiveKit MOCK only), B7 (T21 independent review not done) and B8 (Compose as production vs architecture §4.2; the owner must accept non-HA risk via ADR O1, deploy.md line 21). The report lists B2 and LG01 and only touches B6 indirectly (Meta App Review).
- B3 and B4 are PAYUNi items, which look stale (payuni-removal) and need no mention.
- The report is presented as the owner's list of "steps before launch", yet it omits the ADR O1 acceptance and B7/B8.
- Fix: add rows for (a) the owner ADR O1 accepting single-host non-HA Compose (B8), (b) B7 / T21 independent review NOT_RUN, and (c) the B5 Studio media limit, which the report only notes at line 107. Alternatively state that B3–B8 were assessed and which ones are stale.

### P2 — deploy.md §10 checklist items are missing (line 142)
- The deploy approval row omits: preflight `--online`, a backup successful and a restore drill done within the last 30 days, the offline key copy updated, smoke `full` evidence, and `deployments.log`.
- It names only "备份/恢复安排".
- Fix: list these as NOT_RUN prerequisites, or link them as "all of §10" with a note that none were run.

### P2 — B2 is stated more absolutely than the runbook (line 144)
- The report says "未完成前商家无法在生产登录" and "NOT_RUN 上线阻塞 B2".
- merchant-onboarding.md O2 (line 22) says OIDC is optional when password login is enabled, and password login needs a dedicated SMTP mailbox. The row does mention "approved alternative", but it also cites only "密码认证测试" as MOCK.
- Fix: say that B2 is blocked unless pure password login (+ SMTP, mail.md) is the owner-chosen path, and add the SMTP prerequisite.

### P2 — LG01 row omits the known blocker detail (line 145)
- stripe-live.md line 102 says LG01 is BLOCKED because the footer is not mounted (B8, before customers-billing-ui) and the policy text has owner/draft placeholders. Merge state of customers-billing-ui is not stated in the report.
- Fix: add one clause that the footer mount and placeholder text are the concrete blockers, and re-verify whether the footer is mounted at the candidate SHA.

### P2 — Weak or loose citations
- Line 10: "GATES：Pull-request gates 的供应商例外". GATES.md has no such heading; the exclusion is the "Excluded from the PR browser set" paragraph under "Pull-request gates". Fix: cite that paragraph.
- Line 91 (and line 100): #43 (VISUAL-POLISH-R1) is cited as ads-capi / ads-attribution delivery. It is only the zh-TW ads money display fix. Fix: say "金额显示 #43" and do not list it as the attribution source.

### P2 — Candidate SHA scope vs HEAD (line 6)
- The report is pinned to 0bfe36e5, while the checked-out HEAD is c10404a6 (docs only) — acceptable, but the title/date should say that this docs commit carries no CI evidence of its own.
