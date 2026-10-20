# 二期公共实施约定：实例网络出入速率指标

本文件是实例网络速率指标的二期实施合同，按「逐条覆盖，不做删除式改写」的原则对一期
`docs/specs/observability-phase1/CONTRACT.md`（下称「一期合同」）作增量覆盖。对本文已列
差异项，本文优先于一期合同；一期合同未列差异的条款继续有效。一期目录全部文件（含其
`SHA256SUMS`）保持原样，实施后在一期目录内执行 `sha256sum -c SHA256SUMS` 必须仍全 PASS。

依据：一期合同 §10 第 307 行「新增业务指标通常扩展目录定义与查询/recording；只有新数据源
才补相应采集，不按每个指标新增安装器开关」。本期复用现有 cAdvisor 数据源
（`container_network_*`），不新增数据源，因此不需要新的安装器开关。

## 1. 对一期条款的逐条覆盖

| 一期条款 | 一期原文 | 二期生效为 |
| --- | --- | --- |
| §1 交付范围（第 13 行） | 「不实现 GPU/共享 GPU、**网络**、磁盘……指标」 | 排除项中的「网络」自本期起生效为：仅接入 `network.receive_bytes_per_second` 与 `network.transmit_bytes_per_second` 两个 Pod 级网络速率指标；GPU/共享 GPU、磁盘、平台服务、训练/推理等其余排除项不变 |
| §3 规范 recording 结果（第 76–79 行表） | 仅 2 行 | 追加 2 行，见本文 §2 表 |
| §3 录制时的绑定（第 89 行） | 原始指标固定为 5 个 | 追加 `container_network_receive_bytes_total`、`container_network_transmit_bytes_total`（均带 `interface="eth0"`），共 7 个 |
| §5 范围查询（第 149 行） | 「metric_codes 必填 1 至 2 项、不能重复、不能未知」 | 「必填 1 至 4 项、不能重复、不能未知」 |
| §5 最新值（第 159 行） | 「固定依次查询 CPU、内存」 | 「固定依次查询 CPU、内存、网络收、网络发」 |
| §7 查询限制（第 203 行） | 「指标数 1–2，去重前不得重复」 | 「指标数 1–4，去重前不得重复」 |

一期合同其余条款（§3 七标签投影与身份语义、§4 可信范围与内部 RPC、§5 proto 与 JSON 编码、
§6 时间/历史/缺失/冲突、§7 限制表其余行与错误码表、§8 身份与 mTLS、§9 实现落点与配置、
§10 部署）**继续有效**，不因本文件存在而重述或放宽。

## 2. 指标口径（二期新增）

| 业务编码 | record 名称 | 单位 | 定义 |
| --- | --- | --- | --- |
| `network.receive_bytes_per_second` | `ani_instance_network_receive_bytes_per_second` | `bytes_per_second` | `container_network_receive_bytes_total{interface="eth0"}` 的 `rate(...[5m])` |
| `network.transmit_bytes_per_second` | `ani_instance_network_transmit_bytes_per_second` | `bytes_per_second` | `container_network_transmit_bytes_total{interface="eth0"}` 的 `rate(...[5m])` |

口径细则（与一期 CPU/内存同构，仅两处差异）：

1. 七标签投影（`ani_cluster_id`、`ani_tenant_id`、`ani_instance_id`、`namespace`、`pod_uid`、
   `runtime_container_id`、`container`）与一期一致。差异一：网络原始序列**不含** `container`
   标签，且其 `id` 指向 pause 沙箱 CID，因此 `pod_uid` 从 cgroup `id` 提取，而
   `runtime_container_id` 与 `container` 必须经 `ani:instance:binding` 以 4 键
   `ani_cluster_id,namespace,pod,pod_uid` join 投影，**不得**从网络序列自身 `id` 提取。
2. 差异二：唯一性守卫按 Pod 级 4 键计数（CPU/内存为 6 键）。同一 4 键出现多于 1 条 eth0
   序列时不产出规范值，不得相加。

45s 新鲜度守卫、helper 快照守卫（`timestamp(helper) == time()`）、systemd/cgroupfs 双分支
id 解析、45s 含端点边界均与一期完全一致。

## 3. 语义与契约声明（用户 2026-10-20 确认）

1. **Pod 级语义（强声明）**：`container_network_*` 为 **Pod netns 级**指标。实例 Pod 内
   sidecar（如 Istio、日志采集等）产生的流量会一并计入实例网络值。这是数据源固有语义，
   不是实现缺陷；本指标不是容器级精确值。
2. **条件式覆盖措辞**：本期不做全量覆盖率承诺。仅当实例 Pod 具备 `eth0` 接口时提供网络
   指标；hostNetwork Pod 不存在 `eth0`，网络指标恒为 NO_DATA（CPU/内存不受影响）。
3. **CNI 依赖契约**：`interface="eth0"` 的收敛能力基于当前 CNI 环境；`eth0` 为当前 CNI 下的
   Pod 主接口名，**更换 CNI 需重新验证**。失败模式：`eth0` 命中 0 条 → 网络指标整体
   NO_DATA，CPU/内存不受影响；本期不做 interface 白名单化。该契约同时记录于
   `deployments/observability/collection/source-profile.yaml` 的 `network:` 段。

## 4. 实现落点（二期增量）

| 仓库 | 文件 | 行为 |
| --- | --- | --- |
| ani-observability-service | `deployments/observability/collection/rules/ani-instance.rules.yaml` 与 `rules/ani-instance-metrics-v1.yaml` | 组内新增 4 条规则；2 个 helper 必须插在 `ani:instance:binding` **之前**（组内按文件顺序求值），2 个 canonical 追加到末尾；两份文件 expr 逐字等价 |
| ani-observability-service | `internal/biz/monitoring/catalog.go` | 登记 2 个 code/record/单位 |
| ani-observability-service | `internal/biz/monitoring/query.go` | 请求数上限 2→4 |
| ani-governance | `app/admin/service/internal/service/instance_metrics.go` | `metricCatalog` 完整映射表（白名单 + 单位）；上限 2→4；latest 固定 4 个 code |
| ani-governance | `app/admin/service/internal/server/instance_metrics_test.go` | latest 顺序断言 4 code；4 通过 / 5 拒绝用例 |

**明确不改**：`internal/data/monitoring/*`（按 `Metric` 结构数据驱动）、
`normalize.go`、`query_builder.go`、`internal/service/monitoring/service.go`、所有 proto
（`metric_codes` 为 `repeated string`）、`deployments/observability/collection/patches/*`。

## 5. 部署约束（跨仓同批）

Gov 与 Obs 各自持有**独立的硬编码上限**，无共享配置，**必须同批上线**：

| 场景 | 结果 |
| --- | --- |
| Gov 新、Obs 旧 | 3–4 个 code 请求被 Obs 以 `MONITORING_INVALID_QUERY` 拒绝 |
| Obs 新、Gov 旧 | 请求在 Gov 侧直接 400，Obs 不参与 |

单独部署 Obs 新镜像对存量 1–2 指标请求向后兼容；单独部署 Gov 新镜像则使 3–4 code 请求
到达 Obs 后被旧 Obs 拒绝。两种中间态都不影响既有 1–2 指标请求，但均属未验收状态。

## 6. 边界（本期不做）

- 前端对接（`bytes_per_second` 渲染方式另行确认后单独出前端对接文档）。
- 日志查询、告警、GPU/共享 GPU/磁盘/平台服务/训练/推理指标。
- DNAT/CNI 变更、interface 白名单化、覆盖率承诺。
- 新增 Installer 开关；不修改 `deployments/observability/collection/patches/*`；不改一期目录。
