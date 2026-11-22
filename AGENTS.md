# 直播电商 SaaS 仓库工作入口

本页为各端共用的项目导航。用户级规则与 Humaux 七项细则在其原处维护；本仓的工程与验收依据如下。

| 我要做什么 | 从哪里开始 |
|---|---|
| 确认任务范围、前置依赖与允许修改的路径 | 当前任务卡、[单元简报](docs/delivery/units/)及[任务依赖](contracts/tasks.json) |
| 定位买家或商家的实际业务入口 | [核心功能地图](docs/FEATURE_MAP.md)、[商家应用](apps/admin/)、[买家应用](apps/storefront/) |
| 修改领域逻辑、接口或数据库 | [架构基线](架构.md)第 0、1、2 节、[ADR](docs/adr/)、[领域契约](contracts/)与 [P0 不变量](contracts/invariants.json) |
| 查工程分工、租户边界、外部动作授权与交付证据 | [项目交付流程](docs/delivery/PROCESS.md)；架构第 26、27 节保留初版工作区与任务 DAG 的设计背景 |
| 修改或验收 UI | [核心功能地图](docs/FEATURE_MAP.md)、[项目交付流程](docs/delivery/PROCESS.md)中的真实点击要求及[浏览器门禁](docs/delivery/GATES.md) |
| 选择受影响的验证、记录未验证范围 | [定向验证 CLI](scripts/dev/verify)、[门禁与运行模式](docs/delivery/GATES.md)、[CI 工作流](.github/workflows/gates.yml) |

跨域细则按任务在 Humaux 取原文：长线任务、编排、额度与缓存、开发流程、视觉、文件与磁盘、权威。
