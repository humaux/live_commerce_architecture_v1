---
name: "商品对账簿"
description: "面向商家高频商品与库存操作的清晰、可审计数字账簿"
colors:
  navigation-navy: "#193c61"
  ledger-ink: "#142942"
  muted-text: "#64758a"
  ledger-line: "#dde5ee"
  canvas: "#f5f7fa"
  surface: "#ffffff"
  action-teal: "#247965"
  action-teal-hover: "#1c6554"
  focus-blue: "#1264ba"
  selected-row: "#e5f2ff"
  selection-mint: "#c9e6de"
  active-fill: "#d9f2e9"
  active-text: "#126953"
typography:
  headline:
    fontFamily: "Arial, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "29px"
    fontWeight: 700
    lineHeight: 1.4
    letterSpacing: "-0.5px"
  title:
    fontFamily: "Arial, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "14px"
    fontWeight: 700
  body:
    fontFamily: "Arial, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "14px"
    fontWeight: 400
  data:
    fontFamily: "Arial, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "13px"
    fontWeight: 400
    fontFeature: "tabular-nums"
  table-header:
    fontFamily: "Arial, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "12px"
    fontWeight: 700
  field-label:
    fontFamily: "Arial, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "12px"
    fontWeight: 400
  action:
    fontFamily: "Arial, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "15px"
    fontWeight: 600
rounded:
  control: "5px"
  surface: "7px"
  chip: "20px"
  round: "50%"
spacing:
  tight: "5px"
  control: "8px"
  cluster: "12px"
  section: "24px"
components:
  button-primary:
    backgroundColor: "{colors.action-teal}"
    textColor: "{colors.surface}"
    typography: "{typography.action}"
    rounded: "{rounded.control}"
    padding: "10px 24px"
    height: "45px"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ledger-ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "8px 16px"
    height: "38px"
  field:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ledger-ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "8px 11px"
    height: "39px"
  status-active:
    backgroundColor: "{colors.active-fill}"
    textColor: "{colors.active-text}"
    typography: "{typography.field-label}"
    rounded: "{rounded.chip}"
    padding: "3px 13px"
  ledger-surface:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ledger-ink}"
    rounded: "{rounded.surface}"
    padding: "0"
  selected-row:
    backgroundColor: "{colors.selected-row}"
    textColor: "{colors.ledger-ink}"
    typography: "{typography.data}"
---

# Design System: 商品对账簿

## Overview

**Creative North Star: "The Auditable Ledger"**

商品对账簿是一套 Operate-first 的商家工作台系统：像一张经过整理的数字账页，优先让人定位 SKU、核对现有/预留/可售关系，再留下可追溯的库存调整。它不追求展示型视觉声量；可信度来自对齐、密度、真实状态和稳定的任务顺序。

该世界由用户在 decision `6ecd43a2` 选择，FORM seed 为 `3046f272`，并以 decision `a30e0e29` 批准 wide-ledger 组合。桌面以全宽账簿和下方上下文托盘为签名关系；移动端保留同一任务顺序而不是压缩全部列。已实现的登录与首店入口复用此世界的控件和状态语言；其获准的 C 三步构图、尺寸与行为单独记录在 `docs/implementation/2026-09-20-entry-visual-brief.md`。已实现的 Merchant Settings A 表面也复用此世界；其四步设置流程、状态列和表面局部样式记录在 `.impeccable/merchant-settings-brief.md`。这里不代表其他产品模块已经完成。

**Key Characteristics:**

- 冷静的深海军蓝框架与克制的青绿色操作色。
- 细冷色规则、紧凑行高、右对齐且等宽的数字列。
- 单行选择持续保留列表上下文，下方托盘完成调整。
- 演示、未连接、错误、等待与成功状态均用文字说明，不靠颜色暗示。
- zh-CN、zh-TW、en 是平等选择；语言不改变店铺、币种或权限。

## Colors

色彩承担导航、工作面、动作和状态四种职责；大面积保持冷白与浅灰，深色与强调色只出现在有明确语义的位置。

### Primary

- **Navigation Navy** (`navigation-navy`): 固定导航框架和高密度工作区的稳定背景。
- **Action Teal** (`action-teal`): 提交型主操作与输入光标；hover 使用 `action-teal-hover`。

### Secondary

- **Focus Blue** (`focus-blue`): 键盘焦点、选择控件和可操作文字，不与主提交动作争夺层级。
- **Selection Mint** (`selection-mint`): 浏览器文本选区，配合深色正文保持可读。

### Neutral

- **Ledger Ink** (`ledger-ink`): 标题、正文、数据与高价值数字。
- **Muted Text** (`muted-text`): 辅助标签、审计提示和次要元数据；Merchant Settings 的局部对比度调整见其表面 brief。
- **Canvas / Surface** (`canvas`, `surface`): 冷灰页面底与白色工作面。
- **Ledger Line** (`ledger-line`): 表格、输入和容器的细分隔。
- **Selected Row** (`selected-row`): 当前 SKU 的整行上下文强调。

### Named Rules

**The Earned Accent Rule.** 青绿色用于已实现的提交与推进动作、输入光标，以及关联任务的当前和已完成进度状态；选择、焦点与链接使用蓝色语义，不把所有可点击元素染成同一种强调色。

**The State Has Words Rule.** 在售、归档、未连接、错误、等待与成功必须保留可读文字；颜色永远不是唯一状态载体。

## Typography

- **Headline Font:** Arial with PingFang SC / Microsoft YaHei fallback
- **Body Font:** Arial with PingFang SC / Microsoft YaHei fallback
- **Data Font:** Same UI stack with tabular numerals

**Character:** 这是操作界面字体而不是品牌展示字体。层级通过字号、字重、对齐和留白建立；中文与 Latin/SKU 在同一紧凑节奏中工作。

### Hierarchy

- **Headline** (700, 29px, 1.4): 页面任务标题；移动端收敛至 23px。
- **Title** (700, 14px): 托盘、区块和产品名称。
- **Body** (400, 14px): 常规控件文案；段落单独使用 1.6 行高。
- **Data** (400, 13px, tabular figures): 表格与金额；关键库存数字提高至 25px/700。
- **Table Header** (700, 12px): 表头。
- **Field / Status Label** (400, 12px): 字段名、状态与元数据；状态 badge 单独使用 1.3 行高。
- **Primary Action** (600, 15px): 页面主创建动作；普通按钮继承 14px 基础字号。

### Named Rules

**The Ledger Numbers Align Rule.** 金额和库存值右对齐并使用 tabular numerals；不要用比例字体或居中排版破坏纵向核对。

## Layout

桌面是固定导航轨道加弹性工作区：导航宽 214px，顶部工具条高 66px，主内容留 18px 内边距。产品账簿占满工作区宽度，53px 数据行保持九行可扫读；选中 SKU 的 180px 上下文托盘紧接表格下方。1586×992 是批准稿与最终桌面验收的原生尺寸，不是全产品的固定画布。

响应层级来自实际断点：1500px 以上放宽产品列；1280px 以下导航收至 185px、托盘调整区换行；900px 以下导航收至 165px并隐藏独立 SKU 列；680px 以下导航变为抽屉，顶栏高 58px，只保留产品、售价、可售等优先列，托盘按产品 → 库存 → 调整纵向排列。表格仍允许自然横向滚动，不伪造常显滚动条。

**The List Before Action Rule.** 无论宽屏或窄屏，先保留可扫描列表，再呈现所选对象与调整动作；不要用 modal 替换这条上下文链。

**The Priority Collapse Rule.** 响应式先隐藏低优先列和次级操作，再重排托盘；不要把完整桌面表格等比缩小到手机。

## Elevation & Depth

系统是 flat-by-default。工作面通过白色表面、1px 冷色规则、浅色行状态和间距建立深度；已实现表面没有卡片阴影词汇，也不以玻璃、模糊或立体拟物制造层级。

**The Rules Over Shadows Rule.** 账簿与托盘使用边框、分隔线和色面表达层级；不要为普通容器添加阴影来补偿结构不清。

## Shapes

控件使用轻微曲边（5px），账簿和托盘使用稍大的 7px 圆角，状态标签使用 20px 胶囊，头像和连接状态点才使用圆形。方形账簿结构始终主导，圆角只软化交互边界。

**The Small Radius Rule.** 大容器保持 7px、控件保持 5px；除短状态标签外，不把操作面做成药丸。

## Components

### Buttons

- **Primary:** 青绿色填充、白字、5px 圆角；主创建动作高 45px，普通提交动作保持至少 38px。
- **Secondary:** 白底、细冷色边框、深色文字；hover 切换为浅冷灰。
- **Focus / Disabled:** 所有按钮使用 3px 蓝色外轮廓并偏移 3px；disabled 降至 55% 不透明度并显示禁止光标。

### Status Chips

- **Active:** 淡绿底、深绿文字、20px 胶囊；状态文字必须保留。
- **Archived:** 冷灰底与深灰文字；不使用仅靠色相区分的圆点。

### Cards / Containers

- **Ledger Surface:** 白底、1px 冷色边框、7px 圆角、无阴影。
- **Inspector Tray:** 桌面为横向三段，内部用 1px 规则分隔；窄屏转为纵向并移除无意义的左分隔。
- **Messages:** 错误、成功和待重试状态使用浅色面、文字与 `role` 状态，不使用侧边强调条。

### Inputs / Fields

- **Style:** 白底、1px 冷色边框、5px 圆角、39px 最小高度。
- **Focus:** 3px 蓝色可见轮廓；文本 caret 使用青绿色。
- **Browser Surfaces:** 文本选区使用 mint/ink 配色；表格横向滚动条使用细灰蓝轨道和 thumb，并接受 macOS overlay 自动隐藏。

### Navigation

导航使用深海军蓝固定轨道、统一线性 SVG 图标和左侧 4px 激活标记；移动端在 680px 以下变为 150ms ease 的水平抽屉。`prefers-reduced-motion: reduce` 时移除全部 transition 与平滑滚动。

### SKU Ledger and Context Tray

语义表格、整行选择、真实金额与库存字段构成签名组件。合成商品 atlas 只用于明确标注的隔离演示 fixture；真实环境没有商品图时使用文字回退。托盘必须保留选中产品、现有/预留/可售、整数调整量、必填原因、一个主提交动作及明确反馈。

### Shared Control Reuse

登录与首店入口实际复用了账簿的青绿色主动作及 hover、白底细线输入、5px 控件圆角、3px 蓝色可见焦点和文字状态反馈。新任务表面可继续复用这些已实现的基础控件；共享语义不要求复制账簿构图。入口的大号字段、步骤进度、局部辅助色和响应尺寸只属于入口 brief，不扩展上方 token 表或账簿规则。源码依据为 `apps/admin/app/globals.css` 的基础控件及入口局部样式、`apps/admin/components/Entry.tsx`。

## Do's and Don'ts

### Do:

- **Do** 保持网站客服、Meta 消息和平台支持为独立导航与授权域。
- **Do** 在每个数据密集表面使用稳定列对齐、tabular numerals 和文字状态。
- **Do** 让 zh-CN、zh-TW、en 切换保留当前路径与查询，同时保持店铺和币种不变。
- **Do** 为真实加载、空、错误、等待、成功、disabled 和键盘焦点提供明确状态。
- **Do** 在 680px 以下按任务优先级重排，而不是缩小桌面构图。

### Don't:

- **Don't** 把演示 fixture、合成商品 atlas 或示例价格当作客户目录资产或商业事实。
- **Don't** 为未实现模块制造看似可用的 tab、通知、渠道同步或安全库存控制。
- **Don't** 合并聊天域、从语言推导市场/币种，或用乐观界面覆盖交易与权限真相。
- **Don't** 引入 KPI 卡片墙、装饰性 hero、重阴影、玻璃效果、渐变文字或无语义动画。
