# Facebook 直播电商 SaaS

商家运营直播、处理留言与代客单，买家从直播或店面下单；库存、收款与履约按店铺隔离。

## 项目入口

- 开发与协作：[仓库指令](AGENTS.md)、[交付流程](docs/delivery/PROCESS.md)、[任务与依赖](contracts/tasks.json)。
- 产品与架构：[核心功能地图](docs/FEATURE_MAP.md)、[架构基线](架构.md)、[业务边界与事务](docs/architecture/merchant-workflows.md)、[领域契约](contracts/)、[架构裁决](docs/adr/)。
- 源码：[商家后台](apps/admin/)、[买家店面](apps/storefront/)、[服务入口](cmd/)、[Go 领域与适配器](internal/)、[数据库迁移](migrations/)。
- 验证：[定向命令](scripts/dev/verify)（Node 24）、[隔离本地测试入口](scripts/dev/test-local.sh)、[正式门禁](docs/delivery/GATES.md)与[CI](.github/workflows/gates.yml)。
- 运行与发布：[部署手册](docs/runbooks/deploy.md)。验收状态以对应提交与运行回执为准。

## 按改动验证

对工作区改动运行 `scripts/dev/verify --changed`，或指定比较基线。模块可用 `scripts/dev/verify --module storefront`、`admin` 或 `infra`。CLI 会报告已选测试或 NOT_RUN。Go、PostgreSQL、浏览器及外部服务分别按[门禁说明](docs/delivery/GATES.md)验收。

初版架构包可用 `python3 scripts/check_packet.py` 检查。原模型、阶段记录与历史命令保存在[最初交付的哈希清单](MANIFEST.sha256)、[规格模型](experiments/)及[改版前的 README](https://github.com/humaux/live_commerce_architecture_v1/blob/88d4737977500b43f601c3f4372ab31a3c2904c3/README.md)。
