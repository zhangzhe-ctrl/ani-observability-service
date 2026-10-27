# 四期合同：租户级资源趋势（首页 GPU / CPU / 内存）

- 编制日期：2026-10-27
- 性质：对一期/二期/三期条款的**增量覆盖**。沿用「逐条覆盖，不做删除式改写」原则；
  一期、二期、三期合同全部继续有效，本文只新增租户级聚合形态。
- 关联方案：`ani-governance/local-remote/可观测性接入/首页统计/首页资源趋势统计-技术方案分析-20261027.md`
  与同目录实施手册。过程记录与测试报告在 governance 的 `local-remote/可观测性接入/首页统计/`，
  不进本仓。

## 1. 范围与不改项

本期**不新增采集、不新增 recording rule、不改 proto（方案 A）**：

| 项 | 结论 |
| --- | --- |
| `api` 子模块 | 零改动，不打新 tag。`TargetKind` 枚举不新增命名值；租户级 kind 以数值 `3` 透传（proto3 未知枚举值原样保留），obs 侧按 `int32(Kind)==3` 识别 |
| PrometheusRule | 不新增预聚合。GPU 的租户 join 固定写在 obs 代码里（§4.3，已实测可行） |
| 采集配置 | 不变。volcano device-plugin 的 ServiceMonitor 已在采（`ani-gpu-volcano-monitor`，15s，三节点 target up） |
| 数据库 | obs 无库；governance 侧仅新增权限码与 API 目录数据，无表结构变更 |

## 2. 新增 TargetKind：TENANT

- `internal/biz/target/scope.go` 新增常量 `TENANT = 3`。
- `Scope.Validate()` 租户分支：`TenantID` 必须为 canonical UUID；`ClusterID` 可选
  （非空时必须为 canonical UUID）；`InstanceID / Namespace / ContainerName / PodUID`
  必须为空，出现即拒绝。
- 租户级 target 由 governance 从 JWT 主体组装，obs 侧信任链与一期一致：
  `governanceIdentity` 的 mTLS SAN、`x-ani-tenant-id` 与 `target.tenant_id` 一致性校验
  不变，租户级请求走 `QueryRange`，方法白名单不变。

## 3. 租户级指标编码（catalog）

`internal/biz/monitoring/catalog.go` 新增 `lookupTenant`，仅登记以下三个编码；
实例/训练编码在租户 kind 下未登记，租户编码在实例/训练 kind 下未登记（kind 隔离）：

| 编码 | record（用于表达式） | 单位 | 口径 |
| --- | --- | --- | --- |
| `tenant.cpu.usage_cores` | `ani_instance_cpu_usage_cores` | `cores` | 该租户全部运行中容器实例的 CPU 用量之和 |
| `tenant.memory.working_set_bytes` | `ani_instance_memory_working_set_bytes` | `bytes` | 该租户全部运行中容器实例的内存 working set 之和 |
| `tenant.gpu.allocated_cores` | （无 record，§4.3 整体表达式） | `cores` | vGPU 份额 / 100 = 等效占用卡数（与 CPU/内存同为用量口径） |

GPU 采用「占用份额」而不是设备利用率：`HostCoreUtilization` 是设备级指标且无租户维度，
共享 vGPU 下无法归属到租户；首页三条曲线统一为「用量」口径。

## 4. 固定表达式（query_builder）

`<t>` 由服务端从 `scope.TenantID` 以 `strconv.Quote` 注入，不接受上游任何 label 或
表达式片段。

### 4.1 `tenant.cpu.usage_cores`

```promql
sum by(ani_tenant_id)(
  ani_instance_cpu_usage_cores{ani_tenant_id="<t>"}
  and (time() - timestamp(ani_instance_cpu_usage_cores{ani_tenant_id="<t>"}) <= 45)
)
```

### 4.2 `tenant.memory.working_set_bytes`

同 4.1，record 换为 `ani_instance_memory_working_set_bytes`。

45 秒新鲜度过滤**必须保留**：已删除实例的历史样本不再计入租户曲线（与一期语义一致）。

### 4.3 `tenant.gpu.allocated_cores`

```promql
sum(
  label_replace(
    volcano_vgpu_device_core_allocation_for_a_certain_pod,
    "pod", "$1", "podName", "(.*)"
  ) * on(pod) group_left(label_ani_io_tenant_id)
    kube_pod_labels{label_ani_io_tenant_id="<t>"}
) / 100
```

实现要点（已实测约束）：

1. volcano per-pod 指标的 `namespace/pod` 是 **volcano-scheduler 自身**的，业务 pod 名在
   `podName`；必须先 `label_replace` 改写成 `pod` 再 `on(pod)` join，`on(namespace,pod)`
   会 join 到 scheduler 自身。
2. 平台任务（如 `image-registry-...-predictor-*`）没有 `label_ani_io_tenant_id`，
   join 后自然被过滤。
3. GPU **不保留** 45 秒新鲜度过滤：该指标是 scheduler 分配快照，直接取当前值。

## 5. 按档位的时间窗校验（query.go）

实例/训练 kind 的原 24h / 1h / 3000 点合同**不变**；租户 kind 独立分支：

| 项 | 实例 / 训练（沿用） | 租户（新增） |
| --- | --- | --- |
| 跨度上限 | 24h | 30d（720h） |
| step 范围 | 30s ~ 1h，且为 30s 整数倍 | 30s ~ 4h，且为 30s 整数倍 |
| 点数 | ≤ 3000 | ≤ 3000 |
| `end` | 不得晚于当前（截断到秒） | 同左 |
| 时间戳精度 | `Nanos == 0` | 同左 |

governance 侧按首页三档施加更严的档位约束（1d 档 step 30s–1h、7d 档 5m–1h、
30d 档 15m–4h），obs 侧以上表为后置闸门。两份白名单（obs catalog 与 gov 的
`trendMetricCodes`）独立维护，编码不一致时 gov 先拒绝（400）。

## 6. 超时与响应标签

- 批次预算：实例/训练 4s 不变；租户 kind 放宽到 **15s**（`query.go Execute`）。
- Prometheus 求值超时：实例/训练请求 `timeout=4s` 不变；租户 kind 请求
  `timeout=14s`（留 1s 余量）。Prometheus HTTP 客户端超时从 4s 放宽到 15s
  （实例链路的实际截止仍由 4s 批次 context 决定，行为不变）。
- 数据源解析：租户 kind 的 `ClusterID` 为空且恰好配置一个 datasource 时直接解析；
  多 datasource 且未指定时 fail closed（`MONITORING_SOURCE_NOT_CONFIGURED`）。
- `validLabels` 租户分支：CPU/内存按 `ani_tenant_id` 聚合后序列只剩该标签，GPU 的
  `sum()` 归一无标签；因此租户分支只接受**零标签**或**恰好一个等于目标租户的
  `ani_tenant_id` 标签**的序列，其余（含多余标签、异租户标签）一律
  `MONITORING_SOURCE_RESPONSE_INVALID`。运行时身份标签（`pod_uid` /
  `runtime_container_id`）在租户级聚合结果中不存在，不做校验。
- 归一化：租户级查询期望单序列；`Normalize` 以空 `PodUID + "/" + ContainerID` 作
  身份去重，不冲突。无数据返回 `status=NO_DATA`、点 `value=null` +
  `gap_reason=NO_DATA`，不返回 0 冒充。

## 7. Gov 侧契约（governance 仓，摘要）

- 路由：`GET /api/v1/observability/resource-trend`，权限码
  `observability:resource-trend:read`，业务模块 `INSTANCE`（2026-10-27 定案：
  现有套餐 plan2/plan3 均含 INSTANCE、均无 DASHBOARD，不改套餐数据）。
- 请求仅接受 `metric`（`cpu|memory|gpu`，省略返回三条，顺序固定）、`range`
  （`1d|7d|30d`，必填）、可选 `start/end`（RFC3339 整秒，成对，跨度 ≤ 档位）、
  `step`（Go duration，落在档位且为 30s 整数倍）；其余参数一律 400。
- 租户只取自 JWT；gov 客户端超时放大到 20s；HTTP 预算 25s。
- 权限 SQL：`scripts/ops/sql/register-observability-permissions.sql`（governance 仓）。

## 8. 部署约束

1. obs 与 gov **必须同批部署**：gov 发送 kind=3 请求，旧 obs 按 invalid target kind 拒绝；
   旧 gov 不感知新编码，无影响。
2. obs 的 gRPC 服务端超时（`SERVER_GRPC_TIMEOUT`，默认 6s）必须放大到 ≥20s，
   否则 30d 档会被 RPC 中间件先切断。
3. Prometheus 保留期必须 ≥30d（本环境 2026-10-27 已调整为 30d/54GiB/PVC 64Gi，
   见 MANUAL-METRICS §2）；保留期调整不回补历史，长档位初期曲线短于窗口属预期。
