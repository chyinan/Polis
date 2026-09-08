# 开始阅读 · Draft 0.4.5

先读[产品章程与分层交付](product/PRODUCT_CHARTER.md)，再读[主规范](ARCHITECTURE.md)。

首要目标仍是面向希望虚拟公司长期执行复杂目标、突破单个顶尖Agent局限的人。软件是第一验证场景，不是用户职业限定。

R1先用一个完整参考组合证明直接协作、接班与交付；工作区、选定Skills、stdio MCP和QQ主动通知不被全数延期。R2增加多天、真实反馈及更多合格组合。目标/权限/持久性对开放能力不打折。

`product/RELEASE_SCOPE.json`逐需求记录覆盖；`product/REVIEW_DISPOSITIONS.json`逐项回应17项审查；`product/EXPERIMENTS.json`记录3项尚待确认、默认禁用的实验。

原220项计划保留，新增12项PP，总计232项未执行。产品假设仍待证据；阈值与真实预算不是本包授予的授权。

本包的HTML是离线文档阅读器，不是Agent工作台产品。可运行 `python build_readers.py` 重生成、`python validate_design_pack.py --json DOCUMENT_CHECKS.json` 只做静态校验；不会启动数据库、模型、MCP或QQ。
