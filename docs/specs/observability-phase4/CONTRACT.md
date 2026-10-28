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
| PrometheusRule | 不新增预聚合。租户级 join 固定写在 obs 代码里（§4），三条曲线全部直查 cAdvisor / HAMi |
| 采集配置 | **KSM 白名单需扩容一次**（§2.1）：新增 pod label `ani.kubercloud.com/tenant-id` 与 ns label `ani.io/resource-tenant-id`，用于覆盖 `ani-system` 推理 pod 与训练命名空间 |
| 数据库 | obs 无库；governance 侧仅新增权限码与 API 目录数据，无表结构变更 |

### 1.1 口径（2026-10-28 修订）

| 曲线 | 旧口径 | 现口径 |
| --- | --- | --- |
| CPU | 租户**容器实例**的 `container="workspace"` 使用核数 | 租户**全部工作负载**的全部业务容器使用核数（cAdvisor `rate`） |
| 内存 | 租户**容器实例**的 `container="workspace"` working set | 租户**全部工作负载**的全部业务容器 working set |
| GPU | vGPU 分配份额 / 100 = 等效卡数（**分配量**） | 租户**全部工作负载**的 HAMi 显存**实际用量**（bytes） |

修订依据：旧 CPU/内存读 `ani_instance_*` 规则，源头硬编码
`container="workspace"`，实测同租户旧口径 19.3 MiB、真实全部工作负载 1.91 GiB，
相差约 101 倍；旧 GPU 是分配量，租户不跑负载曲线也不变，与 CPU/内存口径相反。

### 1.2 KSM 白名单扩容（前置条件）

`--metric-labels-allowlist` 是单个参数，只能整体 `replace`：

```
--metric-labels-allowlist=pods=[ani.io/tenant-id,ani.io/instance-id,ani.io/workload-kind,ani.kubercloud.com/tenant-id],namespaces=[ani.io/resource-tenant-id]
```

补丁对：`deployments/observability/collection/patches/ksm-tenant-scope.patch.json`
与 `restore-ksm-tenant-scope.patch.json`。三期补丁对保持可重放（每对只覆盖一次状态迁移）。

实测（kcn，2026-10-28）：扩容后 `kube_namespace_labels` 28 条、每 ns 恒 1 条
（无 one-to-many 放大）；`label_ani_kubercloud_com_tenant_id` 与
`label_ani_io_tenant_id` 两类 pod label 互斥；训练 ns 的 pod 完全不带
`ani.io/*` 租户 label，只能靠 ns 级 label 归属。

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
实例/训练编码在租户 kind 下未登记，租户编码在实例/训练 kind 下未登记（kind 隔离）。
三条编码的 `Record` 仅作文档用途，完整表达式全部由 `query_builder.tenantExpression` 生成：

| 编码 | record（文档） | 单位 | 口径 |
| --- | --- | --- | --- |
| `tenant.cpu.usage_cores` | `container_cpu_usage_seconds_total` | `cores` | 该租户全部工作负载的业务容器 CPU 使用核数之和 |
| `tenant.memory.working_set_bytes` | `container_memory_working_set_bytes` | `bytes` | 该租户全部工作负载的业务容器 working set 之和 |
| `tenant.gpu.memory_usage_bytes` | `vGPU_device_memory_usage_in_bytes` | `bytes` | 该租户全部工作负载的 HAMi 显存实际用量之和 |

GPU 采用**显存实际用量**而非分配份额：`HostCoreUtilization` 是设备级指标且无租户维度，
共享 vGPU 下无法归属到租户；volcano 的 `..._core_allocation_for_a_certain_pod` 是分配量，
不随实际使用变化。三条曲线统一为「实际用量」口径。

## 4. 固定表达式（query_builder）

`<t>` 由服务端从 `scope.TenantID` 以 `strconv.Quote` 注入，不接受上游任何 label 或
表达式片段。

### 4.0 租户 pod 视图（三条曲线共用）

```promql
max by (namespace, pod) (
  kube_pod_labels{label_ani_io_tenant_id="<t>"}
  or label_replace(
       kube_pod_labels{label_ani_kubercloud_com_tenant_id="<t>"},
       "label_ani_io_tenant_id", "$1", "label_ani_kubercloud_com_tenant_id", "(.*)")
  or (kube_pod_labels * on(namespace) group_left()
        max by (namespace) (kube_namespace_labels{label_ani_io_resource_tenant_id="<t>"}))
)
```

| 路 | 来源 | 覆盖对象 |
| --- | --- | --- |
| ① | pod label `ani.io/tenant-id` | 租户容器实例 |
| ② | pod label `ani.kubercloud.com/tenant-id` | `ani-system` 内租户推理 / 模型任务 |
| ③ | ns label `ani.io/resource-tenant-id` | 训练命名空间（其 pod 无租户 label） |

外层 `max by (namespace, pod)` **不可省**：三段标签集各不相同（各带不同的
`label_ani_*`），`or` 天然不去重，实测同一批 pod 经两段 `or` 后条数翻倍。
同样地，`sum(A) or sum(B)` 会让右侧被静默丢弃（两侧 `sum()` 都零标签），禁止使用。

### 4.1 `tenant.cpu.usage_cores`

```promql
sum(
  (
    rate(container_cpu_usage_seconds_total{container!="",container!="POD",namespace!="",pod!=""}[5m])
    and (time() - timestamp(container_cpu_usage_seconds_total{container!="",container!="POD",namespace!="",pod!=""}) <= 45)
  ) * on(namespace, pod) group_left() <租户 pod 视图>
)
```

### 4.2 `tenant.memory.working_set_bytes`

同 4.1，源指标换为 `container_memory_working_set_bytes`（不带 `rate`）。

45 秒新鲜度过滤**必须保留**：已删除 pod 的历史样本不再计入租户曲线（与一期语义一致）。

### 4.3 `tenant.gpu.memory_usage_bytes`

```promql
sum(
  label_replace(
    label_replace(
      max by (podnamespace, podname, ctrname, vdeviceid) (
        vGPU_device_memory_usage_in_bytes
        and (time() - timestamp(vGPU_device_memory_usage_in_bytes) <= 90)
      ),
      "namespace", "$1", "podnamespace", "(.*)"
    ),
    "pod", "$1", "podname", "(.*)"
  ) * on(namespace, pod) group_left() <租户 pod 视图>
)
```

实现要点（已实测约束）：

1. HAMi 自带的 `namespace` 是 device-plugin 所在的 `kube-system`，业务 ns 在
   `podnamespace`，必须先 `label_replace` 覆盖；`pod` 同理来自 `podname`。
2. `max by (podnamespace, podname, ctrname, vdeviceid)` 去重新旧两个 ServiceMonitor
   的重复采集（实测 4 条 series → 2 条，值不变）。
3. GPU 新鲜度用 `<= 90`（HAMi 30s 采集，留 3 个周期），而非 CPU/内存的 45s。
4. CPU/内存的 `container!=""` / `container!="POD"` 是排除 pod 级 cgroup 与 pause 容器；
   HAMi 指标没有这两个 label，不适用。

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
- `validLabels` 租户分支：三条表达式外层都是 `sum(...)`，聚合后序列无标签；因此租户
  分支只接受**零标签**或**恰好一个等于目标租户的 `ani_tenant_id` 标签**的序列，其余
  （含多余标签、异租户标签）一律 `MONITORING_SOURCE_RESPONSE_INVALID`。租户锚定由
  服务端固定表达式保证，不依赖回包标签。运行时身份标签（`pod_uid` /
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
