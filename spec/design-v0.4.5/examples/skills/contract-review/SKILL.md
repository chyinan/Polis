---
name: contract-review
description: "在接口合同发生变化时，检查调用方、边界用例和验收证据。本文件为设计样本，不授予执行权限。"
metadata:
  example-version: "1"
  execution-policy: "read-only-instructions"
---

# 接口合同检查（设计样本）

先读取当前已批准的接口 revision、目标产物和调用方引用，记录不确定之处。不要用作者说“已通过”代替实际证据。

核对字段、错误码、兼容要求以及未闭合发现。按照任务批准的规则提交结果，不修改目标、权限或验收下限。

附属资料：[验收记录要素](references/acceptance.md)。本样本没有脚本，不需要安装包、不触发网络请求；实际执行必须经工作环境资格与授权。
