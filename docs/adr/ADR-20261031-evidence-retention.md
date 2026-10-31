# ADR-20261031 证据保留：原始 CI 产物不进 git，只留指针和哈希

状态：已决定（integrator，2026-10-31，owner 指派「Design an evidence retention policy for output/」）。
证据级：本文的测量为一手 git 读数（base f927c3f5f）。gate 的红绿用例见 `tests/ci/evidence-retention.test.mjs`，由 CI 运行。
规范性内容只在 `scripts/dev/check-evidence-retention.mjs` 与 `.gitignore`；本文只写理由，不复制规则。

## 权威
- 架构.md §27.3：`evidence/` 存放「不包含 PII/secret 的可追溯证据」。
- 架构.md §27.4 与 AGENTS.md「任务收尾」：保留失败证据，不清理他人或 owner 的 KEEP 文件。
- AGENTS.md「完成与失败」：每个任务交付实现、测试命令、退出码、证据文件和风险。
- 以上都要求证据**可追溯**，没有要求把原始 CI 产物的字节**提交进 git**。
- GitHub 仓库设置（2026-10-31 读取）：artifact 和日志保留 90 天，这是平台允许的最大值。

## 测量（origin/r3/integration f927c3f5f，2026-10-31）
- 已跟踪 15,269 个文件，共 600.9 MB；其中 `output/` 有 11,904 个文件，514.2 MB，占 86%。
- `output/` 按类型：png 352.7 MB（2,785 个），log 96.2 MB（5,147 个），json 31.8 MB，zip/gz 9.0 MB。
- 增长：10-10 为 234 MB → 10-17 为 342 MB → 10-30 为 353 MB → 现在 514 MB。
- 一次合并占了大头：#351（37e097399）新增 160.7 MB、1,543 个文件，其中 156.6 MB 是下载回来的 CI artifact 副本，例如 `fix1-artifacts/`、`relay2-artifact/playwright/run.*`。
- 正常合并的规模：#369、#362、#361、#354、#350、#358、#371，每次 `output/` 新增 0.2–1.1 MB，单个文件最大 213 KB（生成的 browser-ownership.json）。
- 全仓已跟踪路径清单为 965,112 B；#363 会推到 1,072,894 B，超过 Node execFileSync 默认的 1 MiB，导致 check-browser-evidence 报 ENOBUFS（由 #425 修复）。
- 每个 `.worktrees/*` 都会检出这 514 MB，并发开 lane 时磁盘按 N 倍放大。
- 在本 gate 下，开放 PR 的情况：
  - #363 新增 55.2 MB（原始产物 44.4 MB）；
  - #365 新增 12.1 MB，#366 新增 6.1 MB，主要是 Figma 参考截图；
  - #355 的 `.log.gz`、#375 的 `playwright/run.*` 属于原始副本；
  - 其余开放 PR 通过。

## 决定
1. **原始 CI 产物不进 git。** 包括下载的 artifact 目录（`*-artifact(s)/`）、`playwright/run.*`、`test-results/`、trace/video/test-failed 截图，以及 zip/gz/tgz/tar/webm。
   - 它们保留在 GitHub Actions（90 天）和 lane 的本地忽略路径里。
   - DELIVERY.md 写 run id、job id、artifact 名，以及关键文件的 sha256。
2. **单个新增文件不超过 1 MiB；每个 PR 在 `output/` 下新增和修改的总量不超过 4 MiB。**
   - 已签名的小证据照常提交：DELIVERY.md、ledger、parity 对照图、生成的 ownership 文件。
3. **Figma 参考截图不整批提交。** 原因是 Figma 本身就是权威（owner：「figma的UI设计是最终上线的」），截图可以用文件 key + 节点 id + 版本重新取。
   - 只提交节点 id、版本或 lastModified、截图 sha256 的清单；需要放进对照表的少量截图计入 4 MiB 预算。
4. **只判本 PR 新增或修改的文件，不追溯 base 上已有的历史。** 推送到 main 时没有 PR base，gate 报 NOT_RUN，不猜 base。
5. `.gitignore` 加入上述原始产物路径：`git add -A` 时不会误加，强行 `-f` 加入的由 gate 拦截。

## 放弃的方案
- **只靠 `.gitignore`**：拦不住 `git add -f`，也拦不住超大单文件。
- **Git LFS**：仍会占用存储和带宽配额，而且每个 worktree 仍要检出；原始产物本来就在 Actions 保留 90 天，没有必要再存一份。
- **重写历史或从 HEAD 删除旧产物**：属于删除既有证据，需要 owner 明确批准（硬边界）。本 ADR 不做，列为待 owner 决定。
- **按 PR 豁免清单**：豁免会被 lane 自行添加。需要提高阈值时，由 integrator 修改 gate 常量，并经过独立复核。

## 开放 PR 的迁移（合并 base 后会被 gate 拦下的 5 个）
- 把原始产物从索引移除、保留本地副本：`git rm -r --cached <路径>`（文件仍留在磁盘，被 `.gitignore` 忽略）。
- 在 DELIVERY.md 写入 run id、job、artifact 名和 sha256。
- Figma 截图改为清单，只留对照表需要的少量图。
- 以上要求写入 integrator 的 lane 卡片（known-base-issues、parity-common），lane 修复时照做。

## 验收 gate
- `node scripts/dev/check-evidence-retention.mjs`：由 `scripts/dev/check-gates.sh` 调用，在 foundation 工作流的 pull_request 中执行，base 用 `origin/$GITHUB_BASE_REF`。
- 「注错 → 红 → 修 → 绿」记录：
  - `tests/ci/evidence-retention.test.mjs` 在临时仓库中注入 5 类原始副本、1 个超 1 MiB 的文件、超 4 MiB 预算，期望退出码为 1；
  - 干净交付、base 上已有的历史、无 base 的情况，期望退出码为 0。
- 真实数据静态复核（2026-10-31）：#351 合并为 FAIL（1,482 项）；#363 为 FAIL（580 项）；#369 合并和本分支为 PASS。

## 已知局限与升级信号
- 历史里的 514 MB 仍在，clone 和 worktree 的体积不会变小。若磁盘或 clone 时间成为瓶颈，可选：
  - 新 worktree 用 sparse-checkout 排除 `output/`；
  - 经 owner 批准后，从 HEAD 移除原始产物（历史仍可恢复）。
- Actions 只保留 90 天；超过 90 天仍需要的原始证据（例如生产事故复盘），由 owner 指定外部存档位置。
- 阈值是按 2026-10-31 的分布定的（非原始文件 p99 为 363 KB）。若正常交付开始频繁触顶，应先查看是否在提交可再生的内容，再考虑调整阈值。
