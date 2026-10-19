**结论：APPROVE**

PR #81 `perf(ci): setup-go cache only on shard:gNN legs; exact -run/-skip assertions` · head 5e930e4ed · base r3/integration · 独立复核（只读，未跑测试）· 2026-10-19

## P0
无。

## P1
无。

## P2
- **P2-1** `.github/workflows/gates.yml:147,151-153`：非 `shard:gNN` 的 Go 腿（`shard:unit`、各 `--browser-*`/`--meta-*` 模式，约 65 条）现在完全不恢复缓存，所以每条都要从 proxy.golang.org 重新 `go mod download`。这些腿本身确实编译 Go，因此严格说它们「失去了」#78 带来的 37 MB 模块缓存。只是那份缓存是不完整的 go.sum-only 条目，编译收益本来就接近 0。风险在于第 150 行注释记录过的 proxy INTERNAL_ERROR 暴露面会回到 #78 之前的水平。目前有 3 次重试兜底，PR 运行 39204047209（76 job 全绿）是一次正样本。browser 腿的耗时变化没有测量。建议：如果 proxy 报错复现，就加一个只含模块、按 go.sum 做 key 的 `actions/cache`，不要恢复 `cache: true`。
- **P2-2** `tests/ci/go-build-cache.test.mjs:20-22` 对 `gates.yml:231`：缓存安全的前提是「所有 gNN 腿编译同一张图」，而这个前提靠的是 gates.yml 的分派行 `shard:g[0-9][0-9]) LC_FOUNDATION_PKGS=./tests/foundation …`。守卫测试只锁住了 `cache:` 表达式，没锁这一行。将来如果有人给某个 gNN 改 PKGS 或 build tag，第一个写入者可能留下不完整的缓存，测试也不会变红。这只影响性能，不影响正确性。可以在 `cacheContract` 里顺手断言这一行。
- **P2-3** `output/release-gate-ci/go-cache-81/RECEIPTS.md:24`：只有一冷一热两个样本，墙钟时间的效果和 real-clock 方差在同一量级，作者自己也写明了这一点。稳定的信号是 pre-test 列（89–130 s 降到 3–5 s）。不阻塞。

## 逐项核对
1. **缓存范围**：`cache: ${{ startsWith(matrix.mode, 'shard:g') }}`（gates.yml:147）。matrix 由 ci-plan.mjs:29 生成 `shard:unit` + `shard:g01..g10`，其中只有 gNN 命中。gNN 腿统一走 `LC_FOUNDATION_PKGS=./tests/foundation`，使用同一条 `go test -p 1 -race -count=1` 命令（test-local.sh:1588），`-run`/`-skip` 不改变 build ID，所以第一个写入者写进去的是完整的 foundation 编译。ui-gallery 和 shard:unit 已不能抢先写入这个不可变 key。key 加入 go.mod 后，#78 遗留的不完整条目退役。plan job（gates.yml:94）仍是 `cache: false`。第 1 点的风险见 P2-1。
2. **YAML 与守卫一致，没有放宽**：旧断言 `cache: true` 换成了更严的精确表达式，并有 `cache: true` / `false` / `'shard:'` 三种注错（test:45）。旧的 `go.sum` 单行改为 `deepEqual(["go.mod","go.sum"])`，两个方向的删项都有注错（test:48-49）。selector 从「包含变量名」收紧为「包含完整展开」（test:31），新增 skip↔run 互换、丢 `-skip`、`-run`→`-skip`、PKGS 默认值被写死等注错（test:53-54），每个注错都有 `notEqual` 保证真正生效。`-race`/`-count=1`/`-timeout=4500s` 与 `go vet ./...` 断言保留。说明：这里锁住的是 test-local.sh 的展开语义；shard-plan.json 的成员划分由 tests/ci/shard-plan.test.mjs 负责，不属于本 PR。测试在 scripts/dev/test-node.sh:64 中执行。
3. **测量**：RECEIPTS 中 4 个 run id 已用 `gh run view` 逐一核实存在，且都是 success：38947914663@735cb2ce2（PR）、39204046804@09a3944b8（dispatch）、39209457669@09a3944b8（dispatch）、39204047209@09a3944b8（PR）。09a3944b8 到 5e930e4ed 之间 gates.yml 只改了注释，test 只改了 1 行，收据仍然有效。证据级：GitHub 原始 jobs JSON + 日志摘录，样本量 n=1/1（见 P2-3）。
4. **required/gate**：`required` job（gates.yml:315）及其判定没有改动。没有新增 `if:`、`continue-on-error` 或 skip，`-count=1` 不变，没有测试被跳过或收窄。
5. **头注释**：gates.yml（1-17 行，Depends on 已同步更新）、go-build-cache.test.mjs（1-5 行）、RECEIPTS.md（1-5 行）都有 Purpose / Depends on / Used by。jobs-*.json 和 pretest-*.tsv 是原始数据，无法加注释，由 RECEIPTS.md 的头说明它们的来源。
