VERDICT: PASS

Reviewed SHA: 348ed215 (docs/delivery/DELIVERY-REPORT.md, delta since c10404a6). Reviewer: claude-sonnet-5-5, read-only, no shell. I read the delta and checked it against the repo. I did not re-audit the unchanged parts of the report.

## Round-1 findings
- **P1, B5/B7/B8 missing from the launch table: CLOSED.** Rows now exist at lines 144–146 for B8 (ADR O1), B7 (T21) and B5 (Studio/LiveKit MOCK). B3/B4 are stated as no longer applicable (see N1). B6 is covered by the existing Meta App Review row.
- **P2, §10 checklist items missing: CLOSED.** The deploy approval row now lists preflight `--online`, a backup within 30 days plus a restore drill, the offline key copy, smoke `full` and `deployments.log`. All of these match deploy.md lines 427–432. Each is marked NOT_RUN.
- **P2, B2 too absolute: CLOSED.** The row now allows the password+SMTP path and cites mail.md and merchant-onboarding O2, which says OIDC is optional when `LC_PASSWORD_LOGIN_ENABLED=1`. "MOCK SMTP" is supported by the loopback SMTP fake in test-local.sh:301.
- **P2, LG01 blocker detail: CLOSED.** The row says BLOCKED, names the footer link mount and the owner/draft placeholders, and says to re-verify at the candidate SHA. This matches stripe-live.md:102.
- **P2, loose citations: PARTLY CLOSED.** The #43 scope is fixed (line 91, "仅 zh-TW 金额格式，不是归因来源"). I did not re-check the GATES "供应商例外" wording at line 10 or the line-100 #43 mention. The delta does not touch them, so verify them if they matter.
- **P2, candidate SHA vs HEAD (line 6): not addressed in the delta.** It is docs-only and non-blocking.

## New claims verified
- deploy.md §0 lines 21 and 24–32 match: B2, B5, B7, B8, ADR O1, and the 架构.md §4.2 statement that Compose is only for dev/acceptance (架构.md:171).
- tasks.json T21 has status NOT_STARTED (line 1296).
- The cvs-restock-fix and worker-authority-split unit docs exist and cover T21-01..03.
- merchant-onboarding O2 and mail.md exist and say what the report says.

## New findings
- **P2 (N1), line 146:** "B3/B4（PAYUNi）随 pay-remove-payuni 已不适用" is overstated.
  - pay-remove-payuni.md removes only the W4-01B NotifyURL receiver, so B4 is plausibly closed.
  - The same unit's OPEN-1 keeps the base PAYUNi card path dormant. deploy.md:223 still says PAYUNi LIVE needs B3 approval.
  - Fix: say "B4 closed by pay-remove-payuni; B3 is moot only while no PAYUNi method is enabled in LIVE (OPEN-1)".
- **P2 (N2), line 145:** "T21-01..03 修复已合入" cites worker-authority-split. The merge of that unit is not confirmed by the PR list I could access.
  - Fix: cite the PR numbers, or drop "已合入".
- **P2 (N3), line 144, mail.md:** it only has PA12 (read-only probe) and PA13 (one real send), both LIVE and owner-approved. It does not document the password-login SMTP setup in detail. The citation is acceptable. If you want it tighter, say that the SMTP mailbox is verified through PA12/PA13, both NOT_RUN.

No P0/P1 found in the delta or in the launch table. The report does not claim launch readiness.
