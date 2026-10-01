# Unit deploy-r4 — make the R3/R4 tree deployable on the pilot (compose, logins, secrets, preflight, smoke, runbook)

Role: integration_worker. Base `r3/integration` (latest). Worktree `.worktrees/deploy-r4`, branch `unit/deploy-r4`. No migration.
Read: docs/runbooks/deploy.md (upgrade path), deploy/README.md, deploy/compose.yml, deploy/postgres/logins.tsv, deploy/secrets.manifest.tsv,
deploy/scripts/{preflight,secrets-init,smoke,deploy,ops-admin}.sh, and output/r4-backlog/P2.md line "DEPLOY". By symbol only.
The pilot (GCE) runs 4dc08b3 with REAL owner data (1 store, products) — the upgrade must be forward-only and data-preserving.
## Scope
1. Every R3/R4 process/login is wired: worker-authority split (migrations/0096 + post_river/0019): each worker container uses its own login
   (logins.tsv already maps; verify compose env DSN names + provision-logins + secrets manifest; retire commerce_worker membership);
   buyer-comms mail loop in expiry-worker: SMTP config + secret mounted into expiry-worker, egress network for SMTP only
   (COMMERCE_BUYER_MAIL_ENABLED default 0 until the owner flips it; reuse the existing commerce_smtp_password secret);
   meta-connect: COMMERCE_META_LOGIN_CONFIG_ID (pilot value 2952863798433821, not secret), COMMERCE_META_LOGIN_REDIRECT_URI,
   COMMERCE_META_LOGIN_GRAPH_VERSION, HPKE meta-page-token-v2 PUBLIC ring into api, PRIVATE ring only into claims-worker;
   store-admin (storefront-publish) registrar login + compose service (profile ops); design/catalog media roles if provision-logins needs them.
2. deploy/env/*.example + preflight: each new key validated (shape, required-when-profile-on), negatives added to smoke S10-style cases.
3. smoke.sh: S13 membership matrix follows the split; any new service/login has a case.
4. docs/runbooks/deploy.md: "Upgrade 4dc08b3 → R4" section: pre-upgrade backup (deploy.sh upgrade does it), new secrets to create
   (secrets-init idempotent), env keys to add, the exact commands, post-checks, and app-rollback note (refused after these migrations → forward-fix only).
## Done when
`bash deploy/scripts/smoke.sh static` exit 0 (shellcheck if installed), `docker compose -f deploy/compose.yml config -q` for every profile set,
preflight negative cases red/green, check-gates, go build. A local `smoke.sh full` needs root + free 80/443: NOT_RUN locally (CI/deploy host runs it).
Evidence → /Volumes/data/live_commerce_architecture_v1/output/deploy-r4/. Commit, do not merge.
