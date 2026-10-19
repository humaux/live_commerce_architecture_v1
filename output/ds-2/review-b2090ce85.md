**结论：REQUEST CHANGES**

# PR #69（DS-2）第 2 轮独立复核 · head b2090ce85

- 复核范围：`git diff 2806c0f9f..b2090ce85`（共 4 个文件，+118/-2），并对整个 PR（`origin/r3/integration...b2090ce85`）只查 P0/P1。
- 复核方式：只读。没有运行测试（NOT_RUN）。证据来自 RED 运行 39230357823 的失败日志（`gh run view --log-failed`）、GREEN 账本 `output/ds-2/gallery-39240249918/playwright/run.v3BkxEa1/ui-gallery/click-ledger.json`（1080 行），以及 `output/visual-review/gallery-39240249918/findings.md`。
- 证据级：E2（代码阅读 + 对照 CI 原始日志）。这次复核不能替代一次运行。

## RED → GREEN 对账（39230357823，56 条）

| 类别 | RED 条数 | 本轮修法 |
|---|---|---|
| 禁用主按钮对比度不足（disabled primary too faint） | 36 | 改产品：加了新令牌 `--ds-disabled-primary-bg` |
| 截图帧里吸顶/固定元素压住控件（sticky/fixed chrome over controls） | 9 | **改截图方式**：截图前把视口放大到文档高度 |
| 页签文字被截断（tab labels clipped） | 3 | 改产品：页签改为 `flex-wrap: wrap` |
| 箭头图标墨迹只有约 9.4–9.8px（chevron ink） | 8 | 改产品：箭头放大 1.5 倍 |

这 56 条全部来自新加的断言。round 2 没有删除断言，也没有放宽阈值。唯一被替换的一行是 `page.screenshot`，它挪进了 try 块，行为不变。

其中 9 条遮挡的分布是：components 页 1440 和 390 各 3 条（三个语言各 1 条），form-released 390 共 3 条。

## 逐项核对

1. **截图改动有没有掩盖真实缺陷**：只核对通过了一部分，见 P1-1 和 P1-2。
   - 滚轮检查确实在放大视口之前跑，用的是真实的 390×844 视口（tests/admin/ui-gallery-clicks.mjs:794 在 :803–820 的放大之前）。
   - `finally` 会在 axe 和 lint 之前恢复视口（:820）。
   - 问题是滚轮检查只覆盖手机宽度的 form*/settings* 页面。RED 里 9 条遮挡中，有 6 条在 components 页（两种宽度都有）。这 6 条现在通过，**只是因为视口被放大了**，没有任何真实视口下的检查替它们作证。
2. **断言是否真实、够严**：readFrame 和 wheelReach 的逻辑都成立。
   - 遮挡判断要求控件的中心点加四条边的中点，都能用 elementFromPoint 命中控件本身。
   - 滚轮走到页面底部时有断言，顶栏必须一直钉在顶部。
   - 没有放宽任何阈值。
   - 但 wheelReach 在 RED 和 GREEN 两次运行里都是 PASS，没有一次「注错→红」的记录，见 P1-2。
3. **R1–R7（只许用令牌）**：本轮新加的 CSS 都只用 `var(--ds-*)`。新的字面量只出现在 tokens.css 里（`#6d8cab`、`1.5`），符合 R1–R3。
4. **对比度复算**（WCAG 相对亮度）：
   - `#6d8cab` 对白色 = **3.51:1**，白字放在 `#6d8cab` 上也是 **3.51:1**。
   - 对 `--ds-page #f8f8f8` = 3.30:1，对 `--ds-surface-muted #f3f3f3` = 3.16:1。
   - 都 ≥ 3:1，和 tokens.css:164 的注释一致。
   - 作为对照，旧的 `--ds-disabled-bg #dae5ef` 对白色只有 2.74:1，所以 RED 那 36 条是真实缺陷，这次修对了。
5. **文件头注释（Purpose / Depends on / Used by）**：controls.module.css、shell2.module.css、tokens.css、ui-gallery-clicks.mjs 四个文件都有。
6. **DS-2 范围**：apps/ 下除了 ui-gallery 之外，没有文件引入 `ds/` 或 `data-ds="2"`。tokens.css 只新增了 `--ds-*` 令牌，没有改动 `--ui-*`。shell-registry 测试把 404 闸门扩展到了 `/ui-gallery/templates/[name]`，这是收紧，不是放松。
7. **Gemini/Antigravity 给的 PASS（39240249918）**：它第 9 项「移动端操作栏不遮挡」引用的是放大视口后拍的截图。按这次的截图方式，截图里**本来就不可能出现遮挡**，所以这一项不能当成独立证据，只能算 E0。真实视口下能作证的只有 wheelReach。Jev 只因为 button_scale_inconsistent 判 FIX，已在 2026-10-15 裁定为误报，我认可这个裁定，不计入本次结论。

## P0

无。

## P1

- **P1-1 真实视口下的遮挡检查覆盖不全；6 条 RED 遮挡只是被截图方式消掉了**
  - 位置：tests/admin/ui-gallery-clicks.mjs:794（条件是 `width < 768 && /^(form|settings)/`）。
  - components 页（1440 和 390）、list 和 detail（两种宽度），以及所有 1440 的 form/settings，都没有在真实视口下滚一遍。
  - RED 里 components 页 1440 和 390 共 6 条遮挡，现在靠 :806–809 把视口放大到文档高度才变绿。readFrame 是在放大后的视口里读的，按构造就看不到「滚动时吸顶条压住控件」。
  - 要求：去掉宽度和模板的限制，对每个截图用例都先在真实视口里跑 wheelReach（form-released 可以复用 form 的结果）。80 格滚轮（:711）放不下长页面时，按页面高度提高上限，不要跳过。

- **P1-2 新闸门 wheelReach 没有「注错→红」记录**
  - 位置：tests/admin/ui-gallery-clicks.mjs:683–723。
  - 「手机端遮挡是截图假象」这个裁定，唯一的依据就是 wheelReach PASS。可它在 RED（39230357823）和 GREEN 里都通过，从来没被观察到失败过。
  - 按项目规则，没有红过的闸门不算存在。
  - 要求：照 :781–783 control scale 的做法，加一个不改真实 DOM 的负向自检。例如在一次性用例里注入一个 `position: fixed` 的条盖在某个控件上，断言 wheelReach 一定会把这个控件列进 blocked。

## P2

- **P2-1 放大视口的循环没有收敛断言**
  - 位置：tests/admin/ui-gallery-clicks.mjs:806–809。
  - 循环 4 次后，如果 `scrollHeight` 仍然大于视口高度，代码会照常拍照和读帧，等于悄悄退回到会出现假象的状态。
  - 建议：循环结束后断言 `scrollHeight <= viewport.height`。
- **P2-2 页签行没有做真实视口下的遮挡检查**
  - 位置：tests/admin/ui-gallery-clicks.mjs:690。
  - wheelReach 排除了 `[role="tablist"]`，readFrame 只在放大后的视口里检查页签。所以「吸顶条压住页签」这种情况在真实视口下没人查。
  - 建议：页签的截断仍由 readFrame 查，但 wheelReach 要把页签也算进遮挡检查。
- **P2-3 箭头放大后描边也跟着变粗**
  - 位置：packages/ui/src/ds/shell2.module.css:62 和 tokens.css:168。
  - 用 `scale(1.5)` 放大 svg，描边从 1.75 变成约 2.6，比同一套里其他图标重。图形也会溢出 16px 的槽位（墨迹 14px 在范围内，但描边盒子大约 24px）。
  - 建议：在 Icon 里给 chevron 换一个裁过 viewBox 的路径，或者用 `vector-effect: non-scaling-stroke`，这样就不需要这个缩放令牌。
- **P2-4 禁用主按钮的配色对没进静态对比度测试**
  - 位置：tests/ui/ds-tokens.test.mjs:38。
  - `disabled-primary-text / disabled-primary-bg`（3:1 档）只在浏览器矩阵里查。建议在静态测试里也加上这一对，这样不用开浏览器就能挡住回退。
- **P2-5 页签同时设了换行和横向滚动**
  - 位置：packages/ui/src/ds/controls.module.css:83。
  - `flex-wrap: wrap` 和 `overflow-x: auto` 并存，第二条规则基本已经失效。section 变体换成两行后，只有顶部是圆角。Gemini 的 P2 也提到了这一点。这是视觉层面的问题，不阻塞合并。

## 可以合并的条件

P1-1 和 P1-2 修好之后，需要给出新的 CI 运行号：滚轮检查要覆盖全部用例，注入的负向自检要红、真实页面要绿。然后再交一次独立复核。round 2 里改产品的三项（禁用主按钮、页签换行、箭头）本身没有问题。
