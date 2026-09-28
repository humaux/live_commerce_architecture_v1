<!--
File: docs/runbooks/deploy.md
Purpose: 部署运行手册 — 主机准备、配置、首次部署、升级、回滚决策、功能上线顺序、密钥轮换、证书、迁移到两台主机/K8s、发布检查清单。
Runs as/in: 文档（运维人员在部署主机上以 root 执行其中命令）。
Reads env / secrets: 无（命令读取 /etc/live-commerce/compose.env 与 secrets 目录，本文不含任何密钥值）。
Used by: 运维/owner/集成者；deploy/README.md 链接到此。
Depends on: deploy/scripts/*.sh, deploy/compose.yml, deploy/env/*.env.example。
Status: DESIGN。未在生产执行过；smoke full 目前 BLOCKED（缺 cmd/migrate，I1）。
Change rules: 命令必须与脚本保持一致；改脚本行为时同步本文。
-->
# 部署运行手册（live-commerce）

## 0. 范围与状态

- 形态：单机 Docker Compose + Caddy 边缘 + PostgreSQL 18（唯一交易真源）。**非高可用**。
  架构.md §4.2 把 Compose 定位为开发和可重复验收环境，所以生产使用前必须由 owner 通过 ADR（O1）接受这个风险。
- **任何生产部署、LIVE 支付、真实退款、营销重播、破坏性迁移都必须有 owner 明确批准**（AGENTS.md）。
  本手册只描述获批后的操作步骤。
- 以下上线阻塞项关闭前，不得宣称"可生产"：
  - B1：`cmd/migrate` 尚不存在（REQUIRES_INTEGRATOR I1），所以 smoke full 和部署都会阻塞。
  - B2：未选定 IdP（OIDC），商家无法登录。
  - B3：PAYUNi 商户资质和 LIVE 批准。
  - B4：PAYUNi NotifyURL 没有接收端（`/payuni/notify` 由 Caddy 保留并返回 404）。支付结果只来自 worker 查询。
  - B5：直播/Studio（LiveKit 仅 MOCK）。
  - B6：Meta 未开放给客户。
  - B7：T21 独立评审未完成。
  - B8：Compose 作为生产环境与 §4.2 的定位冲突。
- 状态词汇：DESIGN / MODEL_ONLY / MOCK / SANDBOX / LIVE / NOT_RUN / BLOCKED。本地 smoke 通过不等于产品通过。

## 1. 主机准备

1. 主机规格：至少 4 vCPU / 8 GiB / 80 GiB SSD。Docker `data-root` 放在数据盘，`LC_BACKUP_DIR` 放在**另一块盘**（容量 ≥ 2×DB + WAL）。
2. 防火墙：放行 22/tcp（只允许管理来源）、80/tcp、443/tcp、443/udp。
   **注意：Docker 发布的端口会绕过 ufw/firewalld 的 INPUT 规则**，需要用 `DOCKER-USER` 链或 `LC_BIND_ADDR` 限制。
3. 时间同步：启用 chrony 或 systemd-timesyncd，`timedatectl show -p NTPSynchronized` 应输出 `yes`。
4. 以 root 在部署 checkout（例如 `/opt/live-commerce`）执行：
   ```sh
   deploy/scripts/host-setup.sh            # 检查版本，创建 GID 10500 组、目录和权限，安装模板（不覆盖）
   ```
   按输出提示合并推荐的 `/etc/docker/daemon.json`（json-file 日志轮转 + live-restore）。脚本**不会自动修改**这个文件。
5. 安装定时任务：`cp deploy/host/crontab.example /etc/cron.d/live-commerce`，并修改路径。

## 2. 配置

1. 编辑 `/etc/live-commerce/compose.env`：
   - 四个域名：`LC_ADMIN_HOST`、`LC_STORE_HOST`、`LC_API_HOST`、`LC_HOOKS_HOST`。
   - `IMAGE_TAG`、`COMPOSE_PROFILES`。
   - 共享开关：`LC_IDENTITY_ENABLED`、`LC_OIDC_ISSUER`、`LC_BUYER_*`。
   跨服务的值**只能**写在这里。
2. 编辑 `/etc/live-commerce/env/*.env`。这些文件只放各服务自己的旋钮；compose.yml 里 `environment:` 已接好的变量不得重复定义（preflight P06 会拦截）：
   - `api.env`：OIDC client id、`COMMERCE_SESSION_TTL`、`COMMERCE_PAYMENT_PROFILE=SANDBOX`、`COMMERCE_STUDIO_ENABLED=0`（必须为 0）。
   - `caddy.env`：真实的 `ACME_EMAIL`。`LC_ACME_CA` 要么保持注释，要么填 https URL，**不能留空**。
3. 生成密钥（只补缺失的文件，不覆盖、不打印值）：
   ```sh
   deploy/scripts/secrets-init.sh
   ```
4. owner 提供的密钥：用真实值替换文件内容 `__UNSET__`，权限保持 `0440 root:10500`。
   - `commerce_oidc_client_secret`：如果保持 `__UNSET__`，表示使用公共 PKCE 客户端，P09 给出 WARN。
   - `commerce_meta_apps_json`：仅在启用 Meta 时需要。
5. 校验：
   ```sh
   deploy/scripts/preflight.sh --online    # 只输出规则号、PASS/FAIL 和变量名
   ```
   因为 cmd/api 启动失败时只打印 `api stopped`，所以 preflight 必须全绿后才能继续。

## 3. 首次部署

```sh
deploy/scripts/build-images.sh           # 输出 IMAGE_TAG=<sha12>，写入 compose.env（-dirty 标签禁止用于生产）
deploy/scripts/deploy.sh first           # preflight → 镜像检查 → postgres 健康 → migrate → provision-logins → up -d → 部署后检查 → 记录
```
- 首次签发 ACME 证书：DNS 必须先指向本机（P14）。演练时在 `caddy.env` 中打开 staging CA，正式签发前再注释掉。
- OIDC 回调地址：`https://<LC_ADMIN_HOST>/api/auth/callback`（`cmd/api/identity.go:59`），需要在 IdP 注册。
- 部署后检查（脚本会自动执行）：
  - 所有常驻服务 running/healthy。
  - worker 就绪标记在 60 s 内出现：`expiry_worker_ready`、`payment_worker_ready`、`meta_worker_ready`。
  - `https://<api>/healthz` 返回 200。
- `deployments.log`（`/var/lib/live-commerce/`）会追加一行：时间、动作、tag、ledger 行数、操作人。回滚判断依赖这一行。

## 4. 升级（只能前向）

```sh
deploy/scripts/build-images.sh           # 新 tag
deploy/scripts/deploy.sh upgrade <tag>
```
步骤：
1. preflight（新 tag）。
2. **强制备份**（`pg-ops.sh backup --tag pre-upgrade-<tag>`）。备份失败会立即中止，此时没有任何改动。
3. 停止 api/admin/storefront/workers。这段时间 Caddy 返回 503 + `Retry-After: 60`，即维护窗口。
4. 用新镜像运行 migrate。先执行业务 SQL，再执行 River，最后执行 post_river，全部在同一个 advisory lock 下完成。
5. 运行 provision-logins。
6. `up -d`。
7. 部署后检查并写入日志。

要点：
- 迁移失败时修复代码后重新执行。**绝不修改已经应用的 SQL 或 checksum**。
- 退出码 75 表示另一个迁移持有锁 718020260920。**不要循环重试**：先确认没有其他迁移在运行，再人工重试一次（见 incident.md §migrate）。
- 已知风险 R3：`migrations.Apply` 内部超时 30 s，大数据量迁移前需要先由集成者调整（I4）。

## 5. 回滚决策树

1. **与该 tag 上次部署时相比，ledger 行数没有变化** → `deploy/scripts/deploy.sh app-rollback <旧tag>`。
   如果 ledger 有变化，脚本会拒绝，并提示 "forward-fix only"。
2. ledger 已变化，**且**备份之后**没有任何外部业务事实**（恢复流量后没有新订单或支付）→ 由 owner 决定是否按 backup-restore.md 恢复数据库，再做 app-rollback。
3. 其他情况 → **前向修复 + 对账**（架构.md §22.1）。**禁止在真实支付之上恢复数据库**。
- 应用回滚不等于数据库回滚。脚本永远不会自动回滚或自动恢复。

## 6. 功能上线顺序

- **Phase A**：identity + accounts + buyer + payments **SANDBOX**（`COMPOSE_PROFILES=db,app,payments-sandbox`）。
- **Phase B**：**LIVE** 需要 owner 批准和 PAYUNi 资质。切换 profile 为 `payments-live`，把 `COMMERCE_PAYMENT_PROFILE` 改为 `LIVE`。
  在历史 sandbox job 处理完之前保留 `payments-sandbox`。preflight 会对 `payments-live` 给出 WARN 提醒。
- Meta：在 meta-runtime 门禁重新通过之前保持关闭（`COMMERCE_META_WEBHOOK_ENABLED=0`，不启用 `meta` profile）。
- Studio/直播：保持关闭（P06 强制 `COMMERCE_STUDIO_ENABLED=0`，media worker 不部署）。

## 7. 密钥轮换（按 deploy/secrets.manifest.tsv 的 rotation 列）

| 密钥 | 做法 |
|---|---|
| `pw_<login>` | 写入新值（`openssl rand -hex 32`，不带换行）→ `secrets-init.sh --rederive` → `docker compose run --rm -T --no-deps provision-logins` → 重启该服务 |
| `pg_superuser_password` | 在 pg-ops 中执行 `ALTER ROLE postgres PASSWORD ...`（先写临时文件，禁止放进 argv）→ 替换文件 → `--rederive` 更新 `dsn_migrate_owner` |
| `commerce_bff_key` | 替换后 api 和 admin **同时**重启 |
| `commerce_buyer_bff_key` | 替换后 api 和 storefront 同时重启（值必须不同于商家 BFF key） |
| `commerce_buyer_cookie_key` | 替换后所有买家会话失效 |
| `commerce_account_keys_json` / `commerce_meta_payload_keys_json` | **只能追加**新 key，再修改 active id 文件并重启相关服务。**禁止删除**仍被存储凭据引用的 key |
| `commerce_account_replay_key` | 只能走 owner 批准的流程 |

所有密钥文件权限保持 `0440 root:10500`。改完后执行 `preflight.sh`。

## 8. 证书与域名

- Caddy 自动申请和续期证书，数据在 `caddy-data` 卷（/data），需要随主机备份。
- 看门狗 W5 在证书剩余不足 14 天时告警。
- 更换域名的步骤：改 compose.env → preflight --online → `docker compose up -d caddy api admin`。origin 由域名推导。

## 9. 迁移到两台主机 / Kubernetes

- 两台主机（NOT_RUN，风险 R6）：
  - DB 主机：`LC_TWO_HOST=1`，`COMPOSE_PROFILES=db,ops`，设置 `LC_PG_BIND_ADDR=<私网IP>`，并放置 `pg_server_crt`/`pg_server_key`。
    启用 pg_hba 中的 `hostssl` 行。migrate、provision 和 pg-ops 都在 DB 主机上运行。
  - 应用主机：`LC_TWO_HOST=1`，`COMPOSE_PROFILES=app,...`，`LC_PG_HOST=<DB IP>`，`LC_PG_SSLMODE=verify-full`，放置 `pg_ca_crt`，然后执行 `secrets-init.sh --rederive`。
  - 代价：两边的 `backend` 网络都变为非 internal，需要用主机防火墙限制出站。
- Kubernetes：暂不提供 manifest（架构.md 规定首发不上 K8s）。映射关系见 deploy-design §19：
  - web Pod = 边缘 sidecar + api + admin + storefront。
  - migrate/provision 改为 Job，pg-ops 改为 CronJob。
  - Secret 挂载到 `/run/secrets`，lcentry 的约定不变。

## 10. 发布检查清单

- [ ] owner 批准记录（范围、tag、窗口）
- [ ] `deploy/scripts/smoke.sh static` 和 `smoke.sh full` 均 PASS，证据位于 `deploy/.evidence/<run>/result.json`，由集成者复制到 `evidence/release/`（I6）
- [ ] 独立验收：test_worker 复跑 smoke；security_reviewer 审查 hba、密钥、加固和 Caddy（作者不能是唯一验收人）
- [ ] `preflight.sh --online` 全部 PASS
- [ ] 升级前备份已经成功，且最近 30 天内做过恢复演练（backup-restore.md）
- [ ] 密钥离线加密副本已经更新（backup-restore.md §密钥备份）
- [ ] `deployments.log` 已经记录本次部署
