# 四期阅读视图：租户级资源趋势（首页三曲线）

本期在一、二、三期实例/训练指标链路上**加法式**新增租户级聚合 target：`TENANT=3`，
配三个租户级编码（CPU 用量、内存 working set、GPU 显存实际用量）。方案 A：**不改 proto、
不新增采集与规则**，租户 pod 视图与三条表达式固定写在 obs 代码里；时间窗按 kind 分支
放宽到 30d / 4h（点数 ≤3000），批次预算 15s。

| 文件 | 读什么 |
| --- | --- |
| [CONTRACT.md](CONTRACT.md) | 合同主体：范围与不改项（§1）、口径修订与 KSM 白名单扩容（§1.1/§1.2）、TENANT scope（§2）、三个编码（§3）、租户 pod 视图与三条表达式（§4）、按档位时间窗（§5）、超时与响应标签（§6）、Gov 契约摘要（§7）、部署约束（§8） |
| [MANUAL-METRICS.md](MANUAL-METRICS.md) | KSM 白名单扩容步骤、Prometheus 保留期与容量变更（30d/54GiB/64Gi）、租户 join 复现命令、部署顺序 |
| [ACCEPTANCE.md](ACCEPTANCE.md) | 本地 10 项（全 PASS）、集群实测 10 项、未执行 3 项 |
| [SHA256SUMS](SHA256SUMS) | 本目录四份文档的校验值（照三期实际格式，含本文件则另计） |

## 快速事实

- scope：`TENANT=3` 只携带租户身份；`ClusterID` 可选（单数据源缺省解析，多数据源 fail closed）；
  `InstanceID/Namespace/ContainerName/PodUID` 必须为空。
- 编码：`tenant.cpu.usage_cores`（cores）、`tenant.memory.working_set_bytes`（bytes）、
  `tenant.gpu.memory_usage_bytes`（bytes，HAMi 显存实际用量）。
  **不需要 DCGM**（HAMi 指标已实测可用）。
- 口径（2026-10-28 修订）：三条曲线统一为「租户全部工作负载的实际用量」，
  覆盖容器实例 + `ani-system` 推理/模型 pod + 训练命名空间。旧口径只含容器实例
  （`container="workspace"`），实测差约 101 倍；旧 GPU 是分配份额，不随实际使用变化。
- 租户 pod 视图：三路 label（pod `ani.io/tenant-id`、pod `ani.kubercloud.com/tenant-id`、
  ns `ani.io/resource-tenant-id`）`or` 后用 `max by (namespace, pod)` 收敛，
  外层 `max by` 不可省（`or` 不去重）；禁止 `sum(A) or sum(B)`（右侧会被静默丢弃）。
- 新鲜度：CPU/内存 45s（cAdvisor 15s）；GPU 90s（HAMi 30s）。
- 时间窗：实例/训练 24h/1h 原合同不变；租户 30d/4h、点数 ≤3000；批次 15s、
  Prometheus 求值 14s、HTTP 客户端 15s。**部署时 `SERVER_GRPC_TIMEOUT` 须 ≥20s。**
- `validLabels` 租户分支：只接受零标签或单租户标签；异租户/多余标签 502。
- 信任链：mTLS SAN、`x-ani-tenant-id == target.tenant_id`、方法白名单全部沿用；
  `QueryRange` 复用，无新 RPC。
- 一、二、三期目录零改动（含各自 `SHA256SUMS` 全 PASS）。

## 实施状态

代码已在分支 `feat/tenant-resource-trend-20261027` 落地，并已在 kcn 新环境部署实测；
2026-10-28 口径统一改造（三路 label join + GPU 改显存实际用量）在
`feat/tenant-scope-unified-20261028` 上实施。

> 过程记录与详细测试数据在协作目录：
> `ani-governance/local-remote/可观测性接入/首页统计/首页资源趋势统计-实施过程-20261027.md`
> 与 `首页资源趋势统计-实测与测试报告-20261027.md`。本目录只保留合同五件套。

- 已改：`scope.go`、`catalog.go`、`query.go`、`query_builder.go`、`prometheus.go` 及单测。
- 本地已通过：`go build` / `go vet` / `go test ./internal/...`；一、二、三期哈希校验。
- 集群已实测：obs `trend-20261027` 与 gov `trend-20261027` 上线；API 目录同步 + 权限
  SQL + Casbin 刷新；1d/7d/30d × cpu/memory/gpu 全 200（step/点数符合三档）；
  边界 400、401/403、Prometheus 对拍一致；实例链路无回归。
- 仍待：前端三卡片联调（前端自行接入）、多集群路径无实机。

governance 侧改动与交付（路由/权限码/SQL/前端对接文档）见 governance 仓
`feat/resource-trend-20261027` 分支；obs 与 gov 必须同批部署。
