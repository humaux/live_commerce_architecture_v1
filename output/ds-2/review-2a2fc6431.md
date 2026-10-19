**结论：REQUEST CHANGES**

# PR #69（DS-2）复核 · head 2a2fc6431（相对 b2090ce85 的增量）

- 复核范围：`git diff b2090ce85..2a2fc6431`，涉及 4 个文件（+54/-22）。
- 复核方式：只读，没有运行测试（NOT_RUN）。
- 证据来源：`output/ds-2/gallery-39248846370/playwright/run.9tmmnQxp/ui-gallery/click-ledger.json`。账本里 wheel reach 共 67 行，全部 pass，其中 1 行是自检。
- 证据级：E2（读代码 + 对照账本）。

## 上轮问题逐条核对

| 上轮 | 状态 | 依据 |
|---|---|---|
| P1-1 真实视口滚轮检查覆盖不全 | **已修** | tests/admin/ui-gallery-clicks.mjs:806–823。除 form-released 外，每个截图用例都跑：3 个语言 × 2 种宽度 × 11 个用例 = 66 行，加 1 行自检，共 67 行。检查在 :829 之后放大视口之前执行，所以用的是真实视口。范围改为 `#ds-main`，components 页也包括在内。步数上限从 80 提到 400，走到页面底部的断言还在。 |
| P1-2 没有注错变红的记录 | **已修** | :807–818。注入一个全视口的 fixed 遮罩后，账本显示 `occluder: 12/12 controls blocked`。wheelReach 现在只返回结果，由调用方断言，所以能看到失败，finally 里会移除遮罩。 |
| P2-1 放大视口没有收敛断言 | 已修 | :839–840 |
| P2-2 页签没进遮挡检查 | 已修 | 已加入 `[role="tab"]`，不再排除 tablist |
| P2-3 箭头描边变粗 | **实际没变（no-op）** | 见下面的 P2-B |
| P2-4 禁用主按钮配色不在静态测试里 | 已修 | ds-tokens.test.mjs 的 UI_PAIRS 加了 4 对，:57 要求 ≥ 3:1，复算是 3.16–3.51 |
| P2-5 页签同时设了换行和横向滚动 | 已修 | 去掉了 `overflow-x: auto`。wrap 后的圆角按 worker 的说明保留，仍记为 P2 |

整个增量没有删除断言，阈值也没有放宽，只有下面 P1 说的那一处收窄。新代码都用令牌，文件头注释也都在。

## 对「sideways」收窄的裁定

**有条件可以接受。** 理由如下：

- 这个 gate 量的是纵向滚动时，吸顶或固定元素会不会遮住控件。表格在 390px 宽度下横向滚动是正常的设计，横向能不能够到不属于它的职责。
- 账本里所有 sideways 都是 `.scroll`（data.module.css:20，`overflow-x: auto`）里的行内操作按钮：components 每个语言 10 个，list 每个语言 5 个，和 worker 报的一致。
- 这只是 gallery（MOCK）范围，不会进正式路由。

接受的条件：

1. **必须做到（P1）**：只有真正能被用户滚动的容器，才算「内滚动容器」。具体要求见 P1-A。
2. **建议（P2-A）**：把 sideways 限定在白名单里，即控件必须在表格的横向滚动容器内。超出白名单就直接 fail，不要只把它列出来。
3. **之后再做**：DS-2 表格用到正式路由（W3）之前，补一次横向走查。把内滚动容器拉到 scrollLeft 最大，对这些按钮再做一次 elementFromPoint 检查。fullRow 的 sticky（data.module.css:49）在横向滚动时可能压住按钮，这次的检查看不到。这不阻塞本 PR。

## P0

无。

## P1

- **P1-A 内滚动容器的判定太宽：被裁切、用户滚不到的控件也会被当成通过**
  - 位置：tests/admin/ui-gallery-clicks.mjs:704。
  - 现在的写法是 `overflowX === "visible" || scrollWidth <= clientWidth` 时跳过，其余都算内滚动容器。这样 `overflow: hidden` 或 `clip` 的祖先也会被当成内滚动容器，可用户根本没法滚动它们。
  - 后果有两个：被裁掉一半的控件，只检查可见的那部分就判 clear；被整个裁掉的控件，会被判成 sideways 而不是 out。
  - 这比上一轮「控件必须完整在视口内」放宽了，而且超出了本次要放宽的范围（只为表格滚动容器）。仓库里就有这样的祖先：controls.module.css:70 的分段开关容器是 `overflow: hidden`。
  - 修法（一行）：只有 `overflowX` 是 `auto` 或 `scroll` 时才算内滚动容器，`hidden`、`clip`、`visible` 一律按以前的规则检查。

## P2

- **P2-A 没有锁定 sideways 的范围**：现在 sideways 只是写进账本。以后任何新出现的 sideways 控件都会悄悄通过。建议断言每个 sideways 控件都在表格的横向滚动容器内，或者把 390 宽度下的个数固定为 components 10 个、list 5 个。
- **P2-B 箭头描边的改动是 no-op**（shell2.module.css:63）。
  - Icon 用的是 `viewBox 0 0 24 24`、`strokeWidth 1.75`。16px 的图标描边约为 1.17px。
  - 上一轮 `scale(1.5)` 之后，描边是 1.17 × 1.5 ≈ 1.75px。这次加了 `non-scaling-stroke`，描边按屏幕像素算，还是 1.75px。
  - 渲染结果没变，注释的说法也不对。我上一轮说「描边约 2.6」，那是我算错了，在这里更正。
  - 和 20px 图标（约 1.46px）差得不多，可以接受。建议删掉这行，或者改正注释。
- **P2-C 自检只用了全屏遮罩**：这只能证明 gate 能失败。更有代表性的失败场景是「底部固定条只压住一部分控件」。可以再加一个只盖住底部 64px 的遮罩，断言被挡的数量 > 0 且 < total。
- **P2-D section 页签换成两行后，只有顶部是圆角**（controls.module.css:85）。worker 声明不修，不阻塞。

## 可以合并的条件

修完 P1-A，拿到一次新的 CI 运行，满足以下几点后可以 APPROVE，不需要再完整复核一轮：

- 390 宽度下 sideways 仍然只有表格按钮。
- 自检仍然是 12/12。
- 其余 66 行仍然 pass。
