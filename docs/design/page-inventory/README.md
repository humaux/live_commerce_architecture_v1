<!-- Purpose: entry point of the 2026-10-15 page redesign (owner charter: inventory every page's functions, then redo and redesign all pages).
Depends on: INVENTORY.md (existing functions, file:line), NEEDS.md (required-function gaps, IA, patterns, priority), AUDIT-product-editor.md.
Used by: integrator and design/implementation units of the redesign. -->
# 页面重做：功能梳理

- `INVENTORY.md` — 51 个页面现有功能 459 项（逐条 file:line；7 个只读会话产出，脚本校验 51/51 覆盖、0 条无效位置，抽查 6/6 属实；E1）。
- `NEEDS.md` — 需求差距：新导航与信息架构、每页缺失的必需功能（附需求出处）、跨页通用模式、模块重做优先级（E1，读代码与需求文档）。
- `AUDIT-product-editor.md` — 商品编辑页的功能与设计审计（触发本次重做的页面）。

基线：r3/integration `92d73661`。下一步：owner 确认导航结构与设计方向 → 设计系统（令牌 → 组件 → 模式 → 模板）→ 按优先级逐模块重做。
