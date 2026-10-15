<!-- Purpose: the admin design specification developers build to (owner 2026-10-15「在开发的时候就需要进行规范」); review checks against it, it is not invented at review time.
Depends on: docs/design/page-inventory/reference-shopline-admin.md (measured SHOPLINE values), NEEDS.md §1/§3, the DS-2 audit (output/integrator/briefs/ds-2-rework.md).
Used by: every admin UI unit card (Codex/agents read it first), scripts/dev/check-ds-rules.mjs (static rules R1–R8), jev-vr-bank.json (visual checklist), integrator audit. -->
# 商家后台设计规范 v1（开发时强制）

**这份规范管三件事：开发时照着做 → 提交前脚本自动拦 → 评审按同一份清单判。** 三处用同一套数字，不在评审时临时发明标准。
改规范 = 改这个文件 + 同步改检查脚本和 Jev 题库，并重新校准；不能只改其中一处。

## 0. 总原则
1. **页面只组合组件**：页面文件里不写颜色、不写间距数字、不画图标；缺组件就先补组件（带展示页和测试），再用。
2. **一个页面一个主操作**：每个区域最多一个蓝色主按钮；危险操作用红色，并且必须二次确认。
3. **先结构后装饰**：标题行 → 筛选行 → 内容卡片 → 操作条，顺序固定（§4 页面模板）。
4. **三种语言同时成立**：zh-TW 为主，zh-CN、en 文字变长也不能换行错乱、截断或溢出。
5. **桌面 1440 与手机 390 都必须验收**：手机端不允许整页横向滚动（表格可在自己的框内滚动）。

## 1. 设计令牌（唯一来源 `packages/ui/src/tokens.css`）
| 类 | 令牌 | 值 |
|---|---|---|
| 侧栏 | `--ds-nav-bg` / `--ds-nav-active` | #373D4C / #494F5D |
| 页面底 / 卡片 | `--ds-page` / `--ds-surface` | #F8F8F8 / #FFFFFF |
| 主色（主操作、链接、卡片标题） | `--ds-primary` / `--ds-link` | 蓝；对白字对比度 ≥ 4.5:1 |
| 文字 | `--ds-text-1/2/3` | #333 / #5B5B5B / #777（text-3 只用于辅助说明） |
| 边线 | `--ds-border` / `--ds-border-input` | #DDD / #CCC |
| 语义 | success / warning / danger / info 各一组 `*-bg` `*-text` `*-border` | 只表达状态，不做装饰 |
| 间距 | `--ds-space-1..8` | 4 / 8 / 12 / 16 / 20 / 24 / 32 / 48 |
| 字号 | `--ds-text-xs..2xl` | 12 / 13 / 14 / 16 / 18 / 22 / 28，每档配固定行高 |
| 字体 | `--ds-font` | system-ui, "PingFang TC", "Noto Sans TC", "Microsoft JhengHei", sans-serif（zh-CN 换成对应的 SC 字体） |
| 圆角 | `--ds-radius-sm/md/lg` | 2 / 4 / 8 |
| 控件高度 | `--ds-control-sm/md/lg` | 28 / 36 / 44 |
| 输入框宽度 | `--ds-field-sm/md/lg` | 160 / 320 / 480，或 100% |
| 卡片内边距 | 头 16×24、体 24、脚 16 | 不设最小高度 |

## 2. 图标
- 只能通过 `<Icon name=… size={16|20|24} />` 使用；线性风格、描边 1.75、颜色继承 currentColor；同一行里的图标尺寸一致。
- **禁止**用文字字符当图标（⌂ ≡ ✉ ▣ ✦ ⚙ • ▸ ▾ ⌕ ✓ ✕ × ○ ■ 等），也禁止在组件外手写 `<svg>`。
- 图标旁边必须有文字或 `aria-label`。

## 3. 组件用法（行为就是规范的一部分）
| 组件 | 规则 |
|---|---|
| Button | 三档高度 28/36/44；主按钮 / 次按钮 / 危险 / 幽灵四种；加载中显示 SVG 转圈并禁止重复点击；禁用态文字对比度 ≥ 3:1 |
| 页签 Tabs | 下划线式 = 页面级视图；分段式 = 表单分区。面板有内边距和容器；支持方向键切换 |
| Badge | 纯文字胶囊，不加符号前缀；颜色按状态语义映射（付款 / 出货 / 退款 / 连接） |
| Alert | 浅色底或左边条二选一；图标与标题同一行；关闭 × 与标题对齐；最多一个操作按钮，靠右 |
| 空白 / 错误 / 无权限状态 | 卡片内用紧凑模式（约 200px 高），整页用完整模式；48px 浅色圆底 + SVG 图标；最多一个主操作 |
| Field | 标签 + 必填星号 + 帮助文字 + 错误文字 + 绑定在输入框上的字数计数；宽度只能用 sm/md/lg/full |
| 表单网格 | 桌面两列、手机一列；标签在上方 |
| FormActionBar | 桌面：顶部一条 + 底部吸附一条；手机：只有底部吸附条；显示“未保存 / 保存中 / 已保存”状态 |
| DataTable | 首列复选框 44px；数字和金额右对齐、等宽数字；操作列靠右、不换行；空 / 加载 / 错误状态在表格内显示；手机端在自己的框内横向滚动 |
| FilterBar | 不含敏感信息的筛选写进 URL；搜索词 `q` 不进 URL（隐私约束） |
| KPI 卡 | 固定栅格（图标 / 数字 / 标签 / 可选涨跌）；多张卡片的数字在同一基线；整张卡可点击 |
| Dialog | 打开时焦点锁在弹窗内，Esc 关闭；危险操作要输入确认词；请求进行中不能关闭 |
| 侧栏 | 宽 270；一级菜单行高 52；图标列宽度固定；折叠箭头随展开旋转；二级菜单缩进、无图标 |
| 顶栏 | 高 50；桌面：语言下拉、帮助、通知、进入商店（外链图标）、账号；手机：只有一行（菜单、店名、通知、账号），其余收进账号菜单 |

## 4. 页面模板（每个页面都必须套用其中一个）
- **列表页**：标题行（标题 + 按钮组，最多一个主按钮）→ 说明横幅（可选）→ 主卡片（筛选行 + 批量菜单 + DataTable + 分页）
- **详情页**：标题行（标题 + 状态 Badge + 操作组）→ 两栏（主栏放卡片，侧栏放摘要卡）→ 时间线
- **表单页**：标题行 → 分段式页签分区 → Field 网格 → FormActionBar
- **设置页**：卡片内左侧二级导航（当前项左侧蓝色竖条 + 浅蓝底）→ 右侧表单 → 右下保存条

## 5. 文案与格式
- 金额 `NT$ 1,234`（en 显示 `NT$1,234`）；日期 `YYYY-MM-DD HH:mm`（台北时区）
- 按钮用动词（「儲存」「匯出」）；错误提示写清楚发生了什么、下一步怎么做
- 文案不硬编码在页面里，全部走三语 copy 文件

## 6. 开发时的自动检查（`scripts/dev/check-ds-rules.mjs`，进入 prepush 和 CI，违规直接失败）
| 规则 | 检查内容 |
|---|---|
| R1 | DS 文件和 DS 页面里不出现颜色字面量（hex / rgb / hsl），只用 `var(--ds-*)`（tokens.css 除外） |
| R2 | CSS 的 margin / padding / gap / inset 只能是 `var(--ds-space-*)`、0、1px、auto 或百分比 |
| R3 | `font-size` 只能是 `var(--ds-text-*)` |
| R4 | TSX 里不出现单个文字符号当图标（§2 列出的字符） |
| R5 | `icons.tsx` 之外不出现 `<svg` |
| R6 | 不用内联 `style={{…}}`，只允许传 `--` 开头的 CSS 变量 |
| R7 | DS 页面不引用老的全局 class（`globals.css` 里的 `.section-bar`、`.purchase-entry` 等） |
| R8 | 每个 DS 组件和 DS 页面都有文件头注释（用途 / 依赖 / 被谁使用） |
「DS 页面」= 文件头写了 `@ds-page` 的页面文件；改版后的页面必须写上这个标记，加上标记后上面的规则自动生效。

## 7. 验收顺序（与 ORCHESTRATION「VR-PAGE」一致）
开发自检（R1–R8 + 本规范）→ CI（axe 0 / visual-lint 0 / 点击全覆盖 / 三语 × 1440、390 截图）→ Jev 清单评审（`jev-vr-bank.json`，判定规则见 `JEV-REVIEW-SPEC.md`）→ Antigravity Pro 整页评审 → 统筹逐项核对 → owner 看截图确认。
