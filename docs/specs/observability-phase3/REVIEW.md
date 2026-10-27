# 三期阅读视图：训练执行 Pod 资源指标

本期在一期 CPU/内存实例指标链路上**加法式**新增训练执行 Pod 的同类指标：同一业务编码按
`TargetKind` 映射到 `ani_training_*` record，`scope` 增加 `pod_uid` 精确定位单个 Pod，第三标签
`ani.io/workload-kind=trainer` 把训练 record 与实例 record 彻底分开。七标签投影、45s 新鲜度
守卫、唯一性守卫、`AMBIGUOUS_SOURCE` 语义与 4s 批次预算全部沿用。

| 文件 | 读什么 |
| --- | --- |
| [CONTRACT.md](CONTRACT.md) | 合同主体：对一期条款的逐条覆盖（§1，含 v4 §12 四问的答复落点）、指标口径（§2）、scope 与校验（§3）、固定表达式（§4）、catalog 分流（§5）、训练规则与部署前复验（§6）、KSM 补丁对（§7）、限制与容量（§8）、信任边界与 Gov 义务（§9） |
| [MANUAL-METRICS.md](MANUAL-METRICS.md) | KSM 补丁对的应用/回滚、训练规则组的五个易错点、promtool、线上判据与诊断、部署顺序 |
| [ACCEPTANCE.md](ACCEPTANCE.md) | 本地 10 项、集群实测 10 项、待真实训练 Pod 6 项、部署前事实复验 2 项 |
| [SHA256SUMS](SHA256SUMS) | 本目录自身文件的校验值（不含本文件） |

## 快速事实

- 指标：`cpu.usage_cores` / `memory.working_set_bytes`；训练 kind 的 record 是
  `ani_training_cpu_usage_cores` / `ani_training_memory_working_set_bytes`，单位与实例侧一致。
- scope：`TargetKind` 增 `TRAINING_EXECUTION=2`，`TargetScope` 增 `pod_uid=7`。训练 kind 必须带
  `pod_uid`，实例 kind 必须留空；训练 `container_name` 放开为任意合法容器名（契约值：
  worker `node`、step `main`）。
- 表达式：实例 5 个 matcher，训练 6 个（追加 `pod_uid`）；两者都保留
  `and (time() - timestamp(...) <= 45)`。
- 规则：新组 `ani-training-metrics-v1`（8 条），**不并入**一期组；一份 YAML、一份 JSON，逐字等价。
- KSM：新增补丁对（一期补丁不改），allowlist 加第三项；提前到阶段 1'，无消费方时零行为变化。
- 容量：稳态 ≤ 8k 条；「500」是同时存活 Pod 数（≈ 56 执行），不是 500 并发执行。系数 8 / 15，
  单次执行 143 条。
- 信任边界：训练 scope 由 Gov 基于权威映射组装，不调 Res；Obs 只做格式与六项精确匹配。
  Gov 四项义务见 CONTRACT §9.2。
- 一期、二期目录零改动（含各自 `SHA256SUMS`）。

## 实施状态

代码、规则与补丁已在分支 `feat/training-execution-metrics` 落地，并已在 kcn 新环境部署实测：

> 过程记录与详细测试数据在协作目录：`ani-governance/local-remote/可观测性接入/训练监控target-实施过程-20261027.md`、`训练监控target-部署实测与测试报告-20261027.md`。本目录只保留合同五件套。

- 已改：`target.proto`、`scope.go`、`service.go`、`catalog.go`、`query.go`、`query_builder.go`、
  `prometheus.go`，以及新增的单测与训练规则三件套。
- 本地已通过：promtool `check rules` 8 条、`test rules` 9 用例、两份规则逐字等价、
  `go build` / `go test` / `go vet`、一期与二期 `SHA256SUMS`。
- 集群已实测：obs 镜像 `training-20261027` 上线；KSM allowlist 三项生效；训练规则组 8/8
  `health=ok`；带三标签探针 Pod 产出 8 标签 canonical；gRPC 训练查询返回 2 条 21 点序列，
  缺 `pod_uid` / 网络码 / 零 UUID 均 400 语义拒绝；删除探针后记录归零。
- 仍待：真实 worker/step 曲线（需 ModelDev webhook 打标签）、Gov 端到端（需 Gov 升级
  `api` 依赖）、500 Pod 容量回归。

根模块通过 `api` 子模块的已发布版本引用 API；本地编译用 `go.work`，镜像构建时在构建机
副本注入 `replace` 指向本地 `api`。交付给 Gov 前必须先为 `api` 打新 tag（含 `pod_uid=7`
与 `TARGET_KIND_TRAINING_EXECUTION=2`），再由 Gov 升级依赖。
