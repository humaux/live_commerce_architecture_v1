<!--
File: docs/runbooks/deploy.md
Purpose: 部署运行手册 — 主机准备、配置、首次部署、升级、回滚决策、功能上线顺序、密钥轮换、证书、迁移到两台主机/K8s、发布检查清单。
Runs as/in: 文档（运维人员在部署主机上以 root 执行其中命令）。
Reads env / secrets: 无（命令读取 /etc/live-commerce/compose.env 与 secrets 目录，本文不含任何密钥值）。
Used by: 运维/owner/集成者；deploy/README.md 链接到此。
Depends on: deploy/scripts/*.sh, deploy/compose.yml, deploy/env/*.env.example。
Status: DESIGN。未在生产执行过；smoke full 目前 BLOCKED（缺 cmd/migrate，I1）。
  本地（scratch 克隆，加入 I1 提案）：45 PASS / 1 BLOCKED（S29m，I8），包括 first/upgrade/app-rollback 全部路径。
  2026-09-28 评审 P1 修复：deploy.sh 把部署的 tag 写回 compose.env（§3–§5，smoke S43、看门狗 W10）；
  超级用户口令轮换改为 `pg-ops.sh rotate-superuser`（§7，smoke S41）。
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
   - `COMPOSE_PROFILES`。
   - `IMAGE_TAG` **不要手工改**：`deploy.sh first|upgrade|app-rollback` 在 `up -d` 之前把要启动的 tag 原子写入这里（临时文件 + 改名，保留属主和权限）。
     所有其他 Compose 入口（`dc up`、`dc run migrate`、重跑 `first`、§8 换域名）都从这个文件取 tag，所以它必须始终等于正在运行的 tag。
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
deploy/scripts/build-images.sh           # 输出 IMAGE_TAG=<sha12>（-dirty 标签禁止用于生产）
deploy/scripts/deploy.sh first <sha12>   # preflight → 镜像检查 → postgres 健康 → migrate → provision-logins → 写 compose.env IMAGE_TAG → up -d → 部署后检查 → 记录
```
- 不带 `<tag>` 时使用 compose.env 里已有的 `IMAGE_TAG`。
- 首次签发 ACME 证书：DNS 必须先指向本机（P14）。演练时在 `caddy.env` 中打开 staging CA，正式签发前再注释掉。
- OIDC 回调地址：`https://<LC_ADMIN_HOST>/api/auth/callback`（`cmd/api/identity.go:59`），需要在 IdP 注册。
- 部署后检查（脚本会自动执行）：
  - 所有常驻服务 running/healthy。
  - 所有 `lc-*` 镜像的容器都运行 `:$IMAGE_TAG`。发现旧容器时直接失败。
  - worker 就绪标记要出现在**该容器本次启动之后**的日志里，也就是 `docker logs --since <.State.StartedAt>`，最多等 60 s。标记包括 `expiry_worker_ready`、`payment_worker_ready`、`meta_worker_ready`。
    `up -d` 不会重建配置没变的容器，所以重复执行 `first`，或回滚到正在运行的 tag，都是合法的空操作，检查会通过。
  - `https://<api>/healthz` 返回 200。
- 构建主机的代理在本机回环地址（`127.0.0.1`/`localhost`/`[::1]`）时，BuildKit 的 RUN 步骤访问不到它。
  `build-images.sh` 默认（`LC_BUILD_NETWORK=auto`）会自动改用 `--network host` 并给出 WARN。只影响构建，不影响镜像。
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
6. 把新 tag 写入 compose.env 的 `IMAGE_TAG`（日志：`compose.env IMAGE_TAG <旧> -> <新>`）。
   时机是 `up -d` 之前、迁移成功之后：从这一刻起容器运行新 tag，所以即使部署后检查失败，compose.env 也和实际运行的镜像一致，
   之后任何普通的 `dc up -d` 都不会悄悄换回旧镜像（旧镜像跑在已迁移的 schema 上，旧 migrate 还会报 `database migration unknown to this binary`）。
7. `up -d`。
8. 部署后检查；通过后才写入 `deployments.log`（它只记录验证通过的部署，是回滚判断的依据）。

要点：
- 迁移失败时修复代码后重新执行。**绝不修改已经应用的 SQL 或 checksum**。
- 退出码 75 表示另一个迁移持有锁 718020260920。**不要循环重试**：先确认没有其他迁移在运行，再人工重试一次（见 incident.md §migrate）。
- 已知风险 R3：`migrations.Apply` 内部超时 30 s，大数据量迁移前需要先由集成者调整（I4）。

## 5. 回滚决策树

1. **与该 tag 上次部署时相比，ledger 行数没有变化** → `deploy/scripts/deploy.sh app-rollback <旧tag>`。
   如果 ledger 有变化，脚本会拒绝，并提示 "forward-fix only"（此时 compose.env 不会被改动）。
   允许回滚时，脚本同样在 `up -d` 之前把 `<旧tag>` 写入 compose.env，否则之后的 `dc up` 会把坏版本装回来。
   回滚到正在运行的 tag 是空操作，会通过部署后检查。smoke S39 覆盖四条路径：空操作、换 tag（容器重建）、换回原 tag、ledger 变化时拒绝；
   S43 验证 compose.env 跟随回滚、普通 `up -d` 不改变 tag、看门狗 W10 能发现漂移。
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
| `pg_superuser_password` | **只用** `deploy/scripts/pg-ops.sh rotate-superuser`（见下文）。**禁止**手工执行 `ALTER ROLE postgres PASSWORD ...`：服务器 `log_statement='ddl'`，新口令会以明文进入 postgres 日志（Docker json-file）和诊断包 |
| `commerce_bff_key` | 替换后 api 和 admin **同时**重启 |
| `commerce_buyer_bff_key` | 替换后 api 和 storefront 同时重启（值必须不同于商家 BFF key） |
| `commerce_buyer_cookie_key` | 替换后所有买家会话失效 |
| `commerce_account_keys_json` / `commerce_meta_payload_keys_json` | **只能追加**新 key，再修改 active id 文件并重启相关服务。**禁止删除**仍被存储凭据引用的 key |
| `commerce_account_replay_key` | 只能走 owner 批准的流程 |

所有密钥文件权限保持 `0440 root:10500`。改完后执行 `preflight.sh`。

`pg-ops.sh rotate-superuser`（在 DB 主机上以 root 执行，postgres 必须在运行；smoke S41 验证）：
1. 生成新值（`openssl rand -hex 32`），先持久化到 `secrets/.pg_superuser_password.rotating`（0400），再改数据库。
2. 在 pg-ops 中用**同一个 psql 会话**：`SET log_statement='none'; SET log_min_error_statement='panic'; SET log_min_duration_statement=-1;`，
   psql 自己读取新值（`` \set pw `cat …` ``，新值经 stdin 进入 pg-ops 的 /tmp tmpfs），`SELECT format('ALTER ROLE postgres PASSWORD %L', :'pw') \gexec`。
   然后验证新口令能登录、旧口令被拒绝。做法与 provision-logins.sh 相同（`deploy/postgres/ops/rotate-superuser.sh`）。
3. **原地**写入 `pg_superuser_password`（同一个 inode，属主和权限不变）：Compose 把文件密钥按单文件 bind mount 挂载，
   原地写入后正在运行的 postgres 容器立即看到新值（`lc_psql`、看门狗、deploy.sh 都依赖它）；改名替换会让容器一直看到旧值，直到重启。
4. `secrets-init.sh --rederive` 更新 `dsn_migrate_owner`。
5. 验证 `lc_psql` 可用，并对 `docker compose logs postgres` 做 `lc_secret_scan`，命中即失败。
- 中途失败：`.pg_superuser_password.rotating` 会保留，**重新执行同一命令即可继续**（若新值已生效则跳过 ALTER）。不要删除这个文件。
- 完成后更新离线加密的密钥副本（backup-restore.md §7）。PITR 不受影响：临时集群用 peer 认证，`--promote` 会把超级用户口令设为当前值。

## 8. 证书与域名

- Caddy 自动申请和续期证书，数据在 `caddy-data` 卷（/data），需要随主机备份。
- 看门狗 W5 在证书剩余不足 14 天时告警。
- 更换域名的步骤：改 compose.env → preflight --online → `dc up -d caddy api admin`（`dc` 见 incident.md 开头）。origin 由域名推导。
  compose.env 的 `IMAGE_TAG` 由 deploy.sh 维护，等于正在运行的 tag，所以这一步不会换镜像；执行前可用 `watchdog.sh` 的 W10 确认没有漂移。

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
