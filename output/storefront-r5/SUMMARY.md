# storefront-r5 — BLOCKED（G1 方案 A 失败，按指示停止）

日期：2026-10-02。**本单未完成 UI 交付，不可据此合并功能或发布。**

## 结论

已实际尝试 `next/image` + 已安装的 Sharp，并对同一 fixture 做前后测量。
**构建成功，但运行失败：原图 200，优化图片 400，首屏 4 张商品图全部破图。**
触发派单中「A 不行就停下来报告，不做 B」的停止条件。已撤回全部试验性应用改动，并重建、复测恢复后的原图版本；没有把破图代码留给集成者。

## 工作区与提交

- 唯一修改目录：`/Volumes/data/live_commerce_architecture_v1/.worktrees/storefront-r5`
- 分支：`unit/storefront-r5`
- 基线：`aca14d7b7794c22950ffffe9f1714117235b2675`
- 证据提交：`41e2be869db270a165d41984e260a5d4c417556c` — `test(storefront): record blocked next image host probe`
- 本 SUMMARY 单独作为交接文档提交；其提交由 `git log -1 -- output/storefront-r5/SUMMARY.md` 定位。
- 保留的改动仅为 `output/storefront-r5/` 内的重现脚本、试验补丁、日志、JSON 和截图。`apps/storefront` 相对基线无 diff。
- 没有改 Go、SQL、依赖或锁文件；没有改主 checkout、其他 worktree、OrderFlow.tsx；没有 push、merge、部署或访问生产主机/凭据。

## 各项状态

| 项目 | 状态 | 结果 |
|---|---|---|
| G1 图片按尺寸输出 | **FAIL / BLOCKED** | A 已试验，运行失败；源代码已恢复。没有尝试 B。 |
| G2 ProductAssurance | NOT_STARTED | 遵守 G1 失败停止条件，未新增配送/付款/退货说明。 |
| G3 同系列商品 | NOT_STARTED | 未实现；排除当前商品/草稿/别店商品的红绿断言 NOT_RUN。 |
| G4 分类 chips | NOT_STARTED | 未实现；390px 分类交互验收 NOT_RUN。 |
| G5 三语防诈骗 | NOT_STARTED | 未改 legal/footer/BankTransfer，未动 OrderFlow。 |
| G8 图库放大 | NOT_STARTED | 只试验过图库图片组件转换，已撤回；未实现放大。 |
| 视觉稿 09、橙色/NT$、44px、三语 | NOT_IMPLEMENTED | 已读取并核对 README 的规则优先级；截图是基线/失败诊断，不是新视觉稿落地。 |
| 收藏计数、虚拟账号 | NOT_IMPLEMENTED | 保持明确排除。 |

## G1 根因与证据

1. `apps/storefront/lib/media-proxy.ts:27–28` 用 `request.headers.get("host")` 选店；没有有效 Host 就返回非图片的 404。直接请求 fixture 原图：本店 **200**，其他 Host **404**。
2. 已安装 Next **16.3.5** 的 `dist/server/image-optimizer.js:1034–1044`，相对路径走 `fetchInternalImage()`。创建内部 request mock 时传了 url/method/socket，但**没有传 headers**。`dist/server/lib/mock-request.js:424` 的默认值是 `headers = {}`。
3. 因而 `/media/p/...` 内部请求没有原 storefront Host，选店失败；优化器收到非图片响应，再对外返回 **400**：`The requested resource isn't a valid image.`
4. Sharp **0.35.4** 已从 Next 的依赖范围成功 resolve 且 require；这不是“未安装 Sharp”问题。Node 实跑版本 **v24.15.0**。
5. 另一个需要未来设计覆盖的 **SOURCE 风险**：Next 的优化缓存键在 `image-optimizer.js:690–697` 只含版本、href、width、quality、mimeType，没有 Host。**本轮没有复现跨店缓存泄漏**：本店优化本身已失败，无法形成有效 warm-cache，所以缓存隔离实测为 NOT_RUN。不能靠补一个 Host 就直接放行。

这里证明的是**本单所指定的相对 `/media/**` 方案 A 不成立**，不推断所有纯前端方案都不可能。是否另行评估其他选店/缓存设计或方案 B，由集成者裁决后重新定界；本单没有增加后端工作。

### 同 fixture、390px 首屏对比

对象：`https://shop.example/zh-TW/products`（本机合成 TLS edge）；390×844、DPR 2、每阶段 3 个新 browser context。
fixture SHA256：`3634d2aea38ce21150a00946c082e299f76234b576a5a3f0d31c8cff29d2f764`。

| 阶段 | 首屏成功图片 body 字节/次 | 成功加载 | 结论 |
|---|---:|---:|---|
| 原生 img，初始基线 | 64,754 / 64,754 / 64,754 | 4/4 | `before-initial.json` |
| next/image 方案 A | 0 / 0 / 0 | 0/4 | 每张返回 400，另有 172 字节错误正文/次；**不是节省 100%** |
| 撤回后恢复确认 | 64,754 / 64,754 / 64,754 | 4/4 | `before.json`，恢复正常 |

测量只统计初始 viewport 内实际图片的成功 HTTP 响应正文，并按 currentSrc 去重；不统计整页预加载或 HTTP header。原始 JSON 保留每个资源的 URL、状态、字节和 SHA256。
**70% 降幅 gate：FAIL；有效降幅为 null。LCP 非劣化：NOT_VERIFIED**，破图时的文字/空框 paint 不能与原图 LCP 比较。三轮时间原值见 `COMMANDS.md` 和 JSON；没有夸大噪声样本。

## 门禁实跑

| 命令 | 退出码 | 判定 |
|---|---:|---|
| `bash scripts/dev/test-node.sh` | **0** | 270 PASS / 0 FAIL / 0 SKIP；另行声明的 R04 二进制测试因变量未配置 NOT_RUN。 |
| `pnpm --filter storefront exec tsc --noEmit` | **0** | 恢复后的源代码通过。 |
| `bash scripts/dev/check-gates.sh` | **0** | 55 个模式有登记，现有跟踪测试有执行入口。 |
| `bash scripts/dev/test-local.sh --browser-storefront` | **NOT_RUN** | G1 失败后按停止条件中止，不以自制 MOCK probe 替代。 |
| `bash scripts/dev/test-local.sh --browser-catalog-media` | **NOT_RUN** | 同上。 |
| `bash scripts/dev/test-local.sh --browser-storefront-publish` | **NOT_RUN** | 同上。 |
| 原始 / 方案 A / 恢复后 `pnpm run build:storefront` | **0 / 0 / 0** | 构建通过不等于图片工作。 |
| 原图 / A / 恢复后 image-spike | **0 / 1 / 0** | 真实本地浏览器 + Next，Go 为 MOCK；不是 PG/生产通过。 |
| `node --check output/storefront-r5/image-spike.mjs` | **0** | 包含最后加入的 phase/LCP guard 的最终脚本语法检查。 |

全部命令、证据文件、边界和前置 harness 失败详见 [COMMANDS.md](./COMMANDS.md)。最终脚本新增的 phase/LCP 防误判断言仅语法检查，未为了该 guard 再重启已停止的方案 A；不隐瞒这一 NOT_RUN。

## 截图

以下 **4 张 after 截图是失败诊断**，不代表已实现视觉稿 09。尺寸为 PNG 实际 CSS 像素（非 2× DPR 导出），已检查：

- `after-products-zh-TW-390x844.png`
- `after-products-en-390x844.png`
- `after-products-zh-TW-1586x992.png`
- `after-products-en-1586x992.png`
- `before-products-zh-TW-390x844.png`：撤回并重建后的正常原图对照。

没有为了凑视觉验收改 fixture 的商家名称/商品内容、CSS 或主题；原 fixture 的绿色主题不是本单承诺落地的橙色稿。

## 审计、独立复核与清理

- impeccable audit-first：先读现有 Card/Gallery、shop.css 和模板真实数据关系；商品网格重复是目录本身的语义，不据此新增卡片或重复 CTA。发现原图无尺寸变体的性能缺口；本轮新问题 P1 为优化后破图。G1 失败后未展开 G2–G8 的全页面 A11y/视觉评分。
- ponytail / frontend-architect：复用现有 fixture、原生 Next 路由与已装 Sharp，没有引入转换服务、loader 框架或依赖。试验失败后未留下补丁绕路。
- 机械 detector 退出 0（`ui-detector-trial.json`），但它不能证明网络图片可用；不作为放行依据。
- 主执行：Codex root；运行时没有向本单暴露可核实的 root 精确模型名/推理档位，不臆填。只读独立审查：`r5_image_audit`，explorer，明确指定 **gpt-6-luna / medium**，同一基线，只读、无递归、无进程或文件写入。
- 独立证据复核 Humaux：`3132b690-abeb-4238-a36e-821feb2cc534`，标题 `R5 G1 Option A runtime failure and independent probe review`。早期 Sharp 误判已更正并软覆盖。
- 子进程/端口/browser/API/edge/自建 TLS 目录均由 probe 的 finally 关闭回收；保留失败记录。没有启动 PG 容器、没有占用/绕过机器级 PG gate 锁，没有清共享缓存。
- Humaux 已存失败结论，画布 `agent:codex-storefront-r5` 明确 BLOCKED。该单由 Claude 决定后续范围；不能把本报告当“六项 UI 全部完成”。
