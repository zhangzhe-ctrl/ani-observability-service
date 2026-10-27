# 三期公共实施约定：训练执行 Pod 资源指标

本文件是训练执行 Pod 的 CPU、内存指标三期实施合同，按「逐条覆盖，不做删除式改写」的原则对
一期 `docs/specs/observability-phase1/CONTRACT.md`（下称「一期合同」）作增量覆盖。对本文已列
差异项，本文优先于一期合同；一期合同未列差异的条款继续有效。二期
`docs/specs/observability-phase2/CONTRACT.md` 已覆盖的条款（指标数 1–4 等）继续有效，
本文不重述。一期、二期目录全部文件（含各自 `SHA256SUMS`）保持原样，实施后在各自目录内执行
`sha256sum -c SHA256SUMS` 必须仍全 PASS。

依据：`ani-governance/local-remote/可观测性接入/observability-训练监控target-对接需求-20261026.md`
v4（2026-10-27，提出方 ModelDev）。v4 §0 的四个决策为定稿输入：scope 增加 `pod_uid`、
第三标签 `ani.io/workload-kind=trainer`、step 分页懒加载、新起 phase3 合同。

v4 §12 四个待确认项的本合同答复，逐条落在 §3、§6、§7、§9：

| v4 提问 | 本合同结论 | 落点 |
| --- | --- | --- |
| §7.1 不加 kind 分支是否可接受 | 接受，不加 | §5 |
| §10.2 系数 8 / 15 是否认可 | 认可；附部署前复验项 | §6.6、§10.2 |
| §9.1 §4 覆盖行措辞 | 采纳并补两点 | §1、§9 |
| §10.3 上限 8k 还是 13k | 8k | §8.3 |

## 1. 对一期条款的逐条覆盖

| 一期条款 | 一期原文 | 三期生效为 |
| --- | --- | --- |
| §1 交付范围（第 13 行） | 排除项含「训练」 | 「训练」自本期起生效为：仅接入训练执行 Pod 的 `cpu.usage_cores` 与 `memory.working_set_bytes` 两个指标域；GPU/共享 GPU、磁盘、推理、平台服务、日志、告警等其余排除项不变 |
| §3 规范 recording 结果（2 行表） | 仅 `ani_instance_cpu_usage_cores`、`ani_instance_memory_working_set_bytes` | 追加 2 行训练规范结果，见 §2。同一业务编码按 TargetKind 映射到不同 record |
| §3 录制时的绑定（首段） | 原始指标固定为 5 个 | **不新增原始指标**。训练侧复用同一 5 个数据源，仅选择器与 KSM 标签过滤不同，见 §6.2 |
| §3 规范结果标签（第 91–99 行段） | `container` 固定 `workspace` | 训练 kind 下 `container` 为真实容器名（`node` / `main` / `wait`），属于对外契约，见 §3.3 |
| §4 可信范围与内部 RPC（第 102–138 行） | scope 由 `Res.ResolveInstanceMetricScope` 解析，Gov 校验后透传；`TargetKind` 仅 `CONTAINER_INSTANCE=1`；`TargetScope` 6 字段；`container_name=workspace` | **训练 target 的 scope 由 Gov 基于模型侧权威映射组装，不调用 Res**。UUID / namespace / container / `pod_uid` 的精确匹配校验不变；`TargetKind` 追加 `TRAINING_EXECUTION=2`，`TargetScope` 追加 `pod_uid=7`；`container_name` 在训练 kind 下允许任意合法容器名。其余条款继续有效。Gov 侧义务见 §9 |
| §6 第 179 行 | 「只构造两种固定 record 表达式；scope 五个 matcher 精确等值……Pod UID/CID 不从当前业务对象查询，而由已录制时序返回」 | 训练 kind 构造 2 种固定 record 表达式、**六个** matcher（追加 `pod_uid`）。`pod_uid` 在训练 kind 下**由调用方 scope 提供**并经精确等值匹配，不再「由已录制时序返回」；实例 kind 保持原语义 |
| §6 第 187 行 | 同一时间点存在多个 UID/CID 来源 → `AMBIGUOUS_SOURCE` | 训练 kind 下该分支**不可达**：scope 已固定 `pod_uid`，同一容器同刻两个 CID 会被 binding 的唯一性守卫拒绝，canonical 直接为空点；实例 kind 语义不变 |
| §6 第 196 行 | 「实例授权仍成立、持久化范围可解析、时序尚在保留期内时，可以查询其已停止或已删除 Pod 的历史」 | 训练侧**不适用**：训练 Pod 无持久化授权对象，`pod_uid` 一旦随 Pod 删除而不可再获取，已删除 Pod 的历史只能由调用方在 Pod 存活期间保存的 `pod_uid` 重放。见 §9.3 |
| §7 第 202 行 | 「每请求实例数 1」 | 保持 1。训练侧「一个实例」= **一个 Pod**，必须由 `pod_uid` 定位；一个 Pod 内多个容器分别请求 |
| §7 第 203 行 | 「指标数 1–2」 | 已由二期第 21 行覆盖为 1–4，本期不改。训练侧实际只登记 2 个 code，见 §5 |
| §7 第 213 行 | 「一次请求包含的两个 Prom 查询共用 Prom 4s 批次预算」 | 「一次请求包含的**最多 4 个** Prom 查询共用同一 Prom 4s 批次预算，不给每项重置完整预算」（二期已放宽到 4 code，该句此前过时，本期改写） |
| §7 第 207 行 | 「每指标最多 128；请求 limit=129，收到 129 即超限」 | 不变。训练侧单 Pod 单容器，series 恒为 1，不触发上限，见 §8.2 |

一期合同 §3 七标签投影、45s 新鲜度守卫（含端点边界）、§5 proto 与 JSON 编码、§6 网格与
缺失/冲突语义、§7 其余限制与错误码表、§8 身份与 mTLS、§10 部署等条款**继续有效**，不因本文
件存在而重述或放宽。

## 2. 指标口径（三期新增）

| 业务编码 | record 名称 | 单位 | 定义 |
| --- | --- | --- | --- |
| `cpu.usage_cores` | `ani_training_cpu_usage_cores` | `cores` | 训练 Pod 内该容器的整容器 CPU counter 的 `rate(...[5m])` |
| `memory.working_set_bytes` | `ani_training_memory_working_set_bytes` | `bytes` | 训练 Pod 内该容器的 working set 字节数 |

口径细则（与一期 CPU/内存同构，仅三处差异）：

1. **同一业务编码映射两条 record**。`cpu.usage_cores` 在 `CONTAINER_INSTANCE` 下是
   `ani_instance_cpu_usage_cores`，在 `TRAINING_EXECUTION` 下是 `ani_training_cpu_usage_cores`。
   单位、数值语义、七标签投影完全一致；分流由 §5 的按 kind `Lookup` 完成。
2. **录制粒度是「每个常规容器」**。训练 Pod 的 `main` 与 `wait` 各自产出规范序列；
   展示侧只查询调用方指定的容器（step 只查 `main`），见 §3.3。这不是「sidecar 计入」，
   而是训练 Pod 的容器集合全部属于录制范围；查询侧由 `container` matcher 收敛。
3. **init 容器不形成规范值**。KSM 的 `kube_pod_container_info` 只遍历 `Spec.Containers`，
   init 容器走独立的 `kube_pod_init_container_info`；`info_source` 只查前者，因此 init 容器
   无法通过 binding 形成 canonical，只能在运行期间产生 `cpu_source` / `memory_source`
   瞬时中间序列。该结论依据 KSM 上游源码，部署前须按 §6.6 复验。
4. **训练 kind 下 `AMBIGUOUS_SOURCE` 不可达**。scope 已含 `pod_uid` 精确 matcher，同一容器在
   同一求值时刻出现两个 CID 会被 binding 的唯一性守卫
   `count by(ani_cluster_id,namespace,pod_uid,container) == 1` 拒绝，canonical 直接为空点。
   `normalize.go` 的多来源分支只在实例 kind 生效。这是定位维度变化的结果，不是语义放宽。

七标签投影（`ani_cluster_id`、`ani_tenant_id`、`ani_instance_id`、`namespace`、`pod_uid`、
`runtime_container_id`、`container`）与一期逐字一致，加 `__name__` 共 8 个标签。

## 3. TargetScope 与校验

### 3.1 proto

`api/observability/target/v1/target.proto`：

```proto
enum TargetKind {
  TARGET_KIND_UNSPECIFIED = 0;
  TARGET_KIND_CONTAINER_INSTANCE = 1;
  TARGET_KIND_TRAINING_EXECUTION = 2;
}

message TargetScope {
  TargetKind kind = 1;
  string tenant_id = 2;
  string instance_id = 3;
  string cluster_id = 4;
  string namespace = 5;
  string container_name = 6;
  // Required for TARGET_KIND_TRAINING_EXECUTION; must be empty for
  // TARGET_KIND_CONTAINER_INSTANCE.
  // TRAINING_EXECUTION: instance_id carries the execution id (worker) or the
  // KFP run id (step); both are canonical UUIDs but neither is a container
  // instance. container_name is part of the external contract
  // (worker="node", step="main") — renaming a container silently breaks the
  // metric chain.
  string pod_uid = 7;
}
```

字段编号 1–6 不得改动；`pod_uid=7` 为新增。API 子模块与根模块同批重新生成。

### 3.2 领域校验

`internal/biz/target/scope.go`：

```go
const (
	ContainerInstance = 1
	TrainingExecution = 2
)

var containerPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

type Scope struct {
	Kind          int32
	TenantID      string
	InstanceID    string
	ClusterID     string
	Namespace     string
	ContainerName string
	PodUID        string
}
```

`Validate()` 按 kind 分支：两种 kind 都要求 `TenantID` / `InstanceID` / `ClusterID` 为规范
UUID，`Namespace` 长度 ≤ 63 且匹配 `namespacePattern`，`ContainerName` 长度 ≤ 63 且匹配
`containerPattern`；`CONTAINER_INSTANCE` 额外要求 `ContainerName == "workspace"` 且
`PodUID == ""`；`TRAINING_EXECUTION` 额外要求 `PodUID` 为规范 UUID。未知 kind 拒绝。

`containerPattern` 必须是 DNS-1123 label 形态，**不得**放宽成 `.+`：训练容器名仍是精确
等值 matcher，放宽只增加注入面。

### 3.3 容器名是对外契约

`worker="node"`、`step="main"`。Obs 不硬编码这两个值（校验只保证合法容器名），但传错会得到
空曲线且不报错（`NO_DATA`），属于调用方错误。容器名变更须同时改 ModelDev 准入侧与调用方，
并由 ModelDev 侧约束测试钉住；Obs 侧在 proto 注释与本文件登记该契约。

### 3.4 映射入口

`internal/service/monitoring/service.go` 的 `scope()` 必须逐字段映射 `PodUid`：

```go
return target.Scope{Kind: int32(v.Kind), TenantID: v.TenantId, InstanceID: v.InstanceId,
	ClusterID: v.ClusterId, Namespace: v.Namespace, ContainerName: v.ContainerName, PodUID: v.PodUid}
```

漏这一处，新字段进不了领域层，训练请求必被 `Validate()` 以「pod uid is required」拒绝。

## 4. 固定表达式与 matcher

`internal/data/monitoring/query_builder.go` 抽出只返回 matcher 串、不带 `{}` 的 `selector()`，
主体与 45s 守卫两处统一拼装：

```go
func selector(scope target.Scope) string {
	s := fmt.Sprintf(`ani_cluster_id=%s,ani_tenant_id=%s,ani_instance_id=%s,namespace=%s,container=%s`,
		strconv.Quote(scope.ClusterID), strconv.Quote(scope.TenantID),
		strconv.Quote(scope.InstanceID), strconv.Quote(scope.Namespace), strconv.Quote(scope.ContainerName))
	if scope.Kind == target.TrainingExecution {
		s += fmt.Sprintf(`,pod_uid=%s`, strconv.Quote(scope.PodUID))
	}
	return s
}

body := metric.Record + "{" + selector(scope) + "}"
return body + " and (time() - timestamp(" + body + ") <= 45)", nil
```

- 实例 kind：5 个精确等值 matcher，与一期逐字一致，不追加 `pod_uid`。
- 训练 kind：6 个精确等值 matcher（追加 `pod_uid`）。
- 两种 kind 都保留 45s 新鲜度守卫，边界含端点。
- `Expression()` 内的登记一致性校验必须传 kind：`biz.Lookup(metric.Code, scope.Kind)`，
  随后仍要求返回值与传入 `metric` 逐字段相等。只改 selector 而不改这处校验，训练 kind 会拿
  实例 record 名比对而被 `InvalidQuery` 拒绝，或更糟——落回实例 record。
- 仍只构造固定表达式，不接受任何调用方 PromQL 片段。

## 5. catalog 按 kind 分流

`internal/biz/monitoring/catalog.go`：

```go
func Lookup(code string, kind int32) (Metric, bool)
```

| kind | 已登记 code | record |
| --- | --- | --- |
| `ContainerInstance` | 4 个（CPU、内存、网络收、网络发） | `ani_instance_*` |
| `TrainingExecution` | 2 个（CPU、内存） | `ani_training_*` |
| 其他 | 无 | 未登记 |

训练 kind 的网络码**必须 fail-closed**：返回未登记 → `MONITORING_INVALID_QUERY` 400。
不得回落实例 record；回落会在 `validLabels()` 的 `__name__` 比对处变成
`MONITORING_SOURCE_RESPONSE_INVALID` 502，把调用方错误伪装成上游故障。

三个调用点同批改：`internal/biz/monitoring/query.go` 的 `Validate()`（code 白名单）与
`Execute()`（取 metric），`internal/data/monitoring/query_builder.go` 的 `Expression()`
（取 record 名并做一致性校验）。

`Query.Validate()` **不加 kind 分支**（v4 §7.1，本合同认可）：服务侧指标数上限保持全局 1–4，
训练侧实际只登记 2 个 code，白名单职责唯一落在 `Lookup(code, kind)`，避免两处维护同一份
列表。加分支不会带来任何边界保护，只会产生第二份白名单。

## 6. 训练 recording rules

### 6.1 交付物与组

新规则组 `ani-training-metrics-v1`，`interval: 30s`，独立 PrometheusRule 对象，选择标签沿用
`release: ani-metrics`，放入 `ani-observability`。**不得**把训练规则并入一期组
`ani-instance-metrics-v1`：一期文件只读，且同组内求值顺序与 helper 位置耦合。

| 文件 | 作用 |
| --- | --- |
| `deployments/observability/collection/rules/ani-training-metrics-v1.yaml`（新） | 部署对象 |
| `deployments/observability/collection/rules/ani-training.rules.yaml`（新） | promtool 可检纯 YAML |
| `deployments/observability/collection/tests/ani-training.rules.test.yaml`（新） | promtool 用例 |
| `deployments/observability/collection/source-profile.yaml`（改） | 登记规则组、第三标签、空窗类型 |
| `deployments/observability/collection/README.md`（改） | 说明新规则组与补丁对 |

### 6.2 过滤条件

中间规则的选择条件（第三标签方案，v4 §5.2）：

```promql
container!="", container!="POD"
and on(ani_cluster_id, namespace, pod) kube_pod_labels{
  label_ani_io_instance_id=~"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}",
  label_ani_io_instance_id!="00000000-0000-0000-0000-000000000000",
  label_ani_io_workload_kind="trainer"
}
```

- instance-id 用 UUID 正则，与一期 `labels_source` 一致，不接受 `.+`。
- `workload_kind="trainer"` 消除与实例 `workspace` 的重叠。实例 Pod 无此标签，不进训练 record。
- `container!="POD"` 排除 cAdvisor pause 容器；`container!=""` 排除无容器名序列。
- 不用 namespace 正则。训练 namespace 形态不稳定，正则收缩会静默漏采。
- 该 join 依赖 `kube_pod_labels` 带 `ani_cluster_id`（由 ServiceMonitor metricRelabeling 写入），
  **夹具必须带该标签**，否则 join 恒空、用例测不出真假。

### 6.3 规则命名与结构

中间规则 `ani:instance:*` → `ani:training:*`；canonical `ani_instance_*` → `ani_training_*`。
共 8 条，顺序与一期同构：

```text
ani:training:cpu_source
ani:training:memory_source
ani:training:info_source
ani:training:running_source
ani:training:labels_source
ani:training:binding
ani_training_cpu_usage_cores
ani_training_memory_working_set_bytes
```

其余照抄一期：systemd 与 cgroupfs 两套 id 正则都要保留；`and (timestamp(x) == time())` 快照
守卫；`time() - timestamp(...) <= 45` 新鲜度；`count by(...) == 1` 唯一性守卫；
`info_source` / `running_source` 同样按 §6.2 过滤；canonical `sum by` 投影同上七标签。

### 6.4 两份文件等价

`ani-training.rules.yaml` 与 `ani-training-metrics-v1.yaml` 的 `spec.groups` 必须逐字等价
（含末尾换行）。一期 `MANUAL-METRICS.md` 的转义规则（YAML 块标量 `\\.`、JSON 再转义
`\\\\.`）同样适用。

### 6.5 promtool 用例

除一期已有形态外，必须覆盖：

1. worker 单容器正常产出，恰好 8 个标签。
2. 两个 step Pod 同时有点，各自独立成条，互不产生 `AMBIGUOUS_SOURCE`。
3. 缺 `ani.io/instance-id` → 不产出。
4. 实例 `workspace` 容器（无 `workload-kind=trainer`）不得进 `ani_training_*`。
5. 同一 Pod 容器重启产生两个 `runtime_container_id`：不同时段正常拼接；同一时刻由
   `ani:training:binding` 的唯一性守卫拒绝，canonical 为空点（不产出两条，也不会被查询层
   判为 `AMBIGUOUS_SOURCE`）。
6. `container="POD"` 被过滤。
7. step 存活短于一个求值周期 → 该点为空。
8. **init 容器不形成 canonical**：带 `kube_pod_init_container_info` 样本、不带
   `kube_pod_container_info` 样本时，只有对应瞬时中间序列，无 canonical。此用例固化 §2 差异 3。
9. `kube_pod_labels` 缺 `ani_cluster_id` → 4 键 join 不命中，不产出。

### 6.6 部署前必须复验的两条事实

本节两个前提直接决定 §10.2 的系数与容量结论，落地前须用只读 PromQL 复验并记录结果。
未复验前，本合同只把这些数字当作**待确认输入**，不作为已验收事实。

1. **init 容器不进 `kube_pod_container_info`**。依据是 KSM 上游源码
   `internal/store/pod.go` 的 `createPodContainerInfoFamilyGenerator` 只遍历 `Spec.Containers`，
   init 容器由 `kube_pod_init_container_info` 独立族提供；当前集群运行 KSM 2.19.0，与上游
   main 分支存在版本差。复验：

   ```promql
   count by (container) (kube_pod_container_info{namespace="<训练 namespace>"})
   count by (container) (kube_pod_init_container_info{namespace="<训练 namespace>"})
   ```

   判据：`init` / `kfp-launcher` 只出现在第二个结果，不出现在第一个结果。

2. **worker Pod 只有 1 个常规容器、无 init 容器**。依据 ModelDev v4 §1 实测（`node` 一项，
   `runtime_plan.go:70-77` 无 `initContainers`）。该文件不在本仓库，Obs 无法独立核对。
   复验：首个真实 worker Pod 上执行
   `kube_pod_container_info{pod=~"<worker pod>"}`，判据为仅 `node` 一条。
   若 worker 容器数变化，§10.2 的系数 8 立即失效，容量需重算。

## 7. KSM allowlist 第三标签

### 7.1 补丁对必须新增，不得改写一期补丁

一期 `patches/ksm-workload.patch.json` 的 `test` 锁的是**基线** args（只有 `--port` 与
`--resources` 两项），`restore-ksm-workload.patch.json` 锁的是**一期应用后**状态（三项，含
两项标签的 allowlist）。该补丁对已于 2026-10-19 应用到集群。现在把第三标签加进这两个文件
会出现两个问题：

- 对当前已应用集群重放 `ksm-workload.patch.json` 时，`test` 比对的原始 args 已不存在，
  patch 必失败；
- 改写一期 restore 会让「回到基线」的历史恢复路径失去可重放性。

因此新增一对补丁，与一期补丁并存，各自只覆盖一段状态迁移：

| 文件 | 迁移 | 关键操作 |
| --- | --- | --- |
| `patches/ksm-workload-phase3.patch.json`（新） | 一期状态 → 三期状态 | `test /metadata/uid`、`test args` 为三项（含两项标签 allowlist），`replace /args/2` 为三项标签 allowlist |
| `patches/restore-ksm-workload-phase3.patch.json`（新） | 三期状态 → 一期状态 | 同样 `test` 锁三项，`replace /args/2` 回两项标签 allowlist |

值固定为
`--metric-labels-allowlist=pods=[ani.io/tenant-id,ani.io/instance-id,ani.io/workload-kind]`。
`test` 不得删除；test 失败时保存当前对象、逐字段核对，不绕过。

### 7.2 时机与副作用

KSM allowlist 提前到阶段 1'（与阶段 1 并行）：在没有任何规则消费第三标签时，多暴露一个标签
是零行为变化，无副作用。若延后，§6.2 的过滤恒不命中，worker 曲线在阶段 3 必然为空。

### 7.3 滚动空窗

KSM 单副本滚动 → 进程重启加首次 15s 抓取 ≈ **30–45s 无新样本**；叠加规则侧 45s 新鲜度守卫，
`ani_instance_*` 与 `ani_training_*` 一起出现 **45–90s `NO_DATA`**。实例指标同样受影响，
不只是训练。要求：放部署窗口、避开活跃执行、单副本滚动；在 `source-profile.yaml` 登记该
空窗类型，避免以后被当故障排查。

## 8. 查询限制与容量

### 8.1 复用一期限制

时间窗 24h、step 30–3600s 且 30s 整数倍、每指标 3000 点、单响应 8 MiB、单指标 128 series
（`limit=129` 超限即 `RESULT_TOO_LARGE`）、每副本 16 并发、每副本每租户 4 并发，全部沿用。
并发超限**立即拒绝**（`MONITORING_CONCURRENCY_LIMIT_EXCEEDED`），不排队——排队语义由调用方
BFF 承担（v4 §7.3），本服务不加队列。

### 8.2 单次 RPC 的形状

训练 kind 单次请求最多 2 个 code；两个 Prom 查询共享同一 4s 批次预算与同一父 deadline。
单 Pod 单容器，每个 code 的 series 恒为 1，`limit=129` 不触发。`latest` 语义不变：本批次开始
时取一个整秒 `evaluated_at`，两点同值。

### 8.3 序列上限与容量

合同写入：`ani:training:*` 中间序列 + canonical 序列**稳态 ≤ 8k 条**。

推导（按实测系数，见 v4 §10.2 与 §6.6）：

| 项 | 条数 |
| --- | --- |
| 单个 worker Pod | 8：每容器规则 7 条 × 1 个容器（`cpu_source`、`memory_source`、`info_source`、`running_source`、`binding`、`cpu`、`memory`）+ `labels_source` 1 |
| 单个 step Pod | 15：每容器规则 7 条 × 2 个常规容器（`main`、`wait`）+ `labels_source` 1 |
| 一次执行（1 worker + 9 step） | 8 + 9 × 15 = 143 |

「500 个同时存活 Pod」按**全部是 step Pod**折算：500 × 15 = 7,500 条；同一时刻存在
同一批执行的 worker，按 9 step/执行 折算约 56 个 worker × 8 ≈ 448 条，合计 ≈ 7,948 条 < 8k。
此处不得把「500 Pod」直接写成「500 并发执行」——两者差约 9 倍，是 v4 §10.3 唯一容易被产品
侧误读的地方，合同把度量单位钉死为**同时存活的 Pod 数**。

**init 容器瞬时注脚**：step Pod 初始化期间每个 init 容器额外产生 2 条
（`cpu_source` + `memory_source`），随初始化结束消失，不进 canonical。同一时刻通常仅 1 个
init 容器运行，故瞬时峰值额外 ≤ 2 × 存活 step Pod 数；该增量不计入 8k 稳态预算，但须在
容量盘点时观察 `prometheus_tsdb_head_series` 是否出现对应尖峰。

日增样本估算：8k × 2 样本/分钟（30s 间隔）× 1440 ≈ 23M 样本/日。retention `1d` / `4GiB`
可容纳。复盘点：`prometheus_tsdb_head_series` 增长率、规则组求值时长、磁盘写放大。

「500 Pod 容量回归」**单列为上线前动作，不在一期验收**：8k 是外推值，未在并发训练下压测。

## 9. 信任边界与 Gov 义务

### 9.1 Obs 只做格式与精确匹配

Obs 不判断训练归属、不读 Kubernetes、不调用 Res 或 ModelDev。训练 kind 的隔离由
`ani_cluster_id`、`ani_tenant_id`、`ani_instance_id`、`namespace`、`container`、`pod_uid`
六个精确等值 matcher 合起来实现。六个 matcher 里 `pod_uid` 是全局唯一标识，实际定位能力由它
提供；其余五个保证记录归属与调用方 scope 一致。伪造 scope 需要同时猜对六项，且 `pod_uid`
不可枚举。

### 9.2 Gov 的校验义务（v4 §9.1 §4 覆盖行必须显式写入）

一期 §4 的前提是「scope 来自资源服务的持久化对象」。训练侧改为 Gov 组装后，Gov 必须承担
以下义务，Obs 无法代偿：

1. `pod_uid`、容器名、`namespace`、`instance_id`、`tenant_id` **全部来自权威映射**
   （`ListExecutionPods` 与 execution 的 workspace 绑定），**不接受调用方传值**。
2. 归属链校验通过后才组装 scope：worker 走 `Pod → Job → JobSet → TrainJob` UID 全等链，
   step 按 `RunAuthority.WorkflowUID`。
3. 调用方租户鉴权在 Gov 完成（既有 `instance:read` 语义扩展到训练执行读取），Obs 只看
   `x-ani-tenant-id` 与 scope 是否一致。
4. `container_name` 由 Gov 按 Pod 类型选择（worker `node`、step `main`），不取自请求体。

任一义务缺失，Obs 侧六个 matcher 仍会「精确命中某个真实存在的 Pod」，构成越权读取。
因此 §1 的 §4 覆盖行不是措辞问题，而是信任来源的实质变更，须在 phase3 的
`REVIEW.md` 与 Gov 侧合同中同时留痕。

### 9.3 已删除 Pod 的历史（一期 §6 第 196 行的例外）

一期允许查询已停止或已删除实例的历史，前提是「实例授权仍成立、持久化范围可解析」。训练侧
不成立：训练 Pod 没有持久化授权对象，Pod 删除后 `pod_uid` 不再可从任何权威服务获得。因此：

- 训练事件结束后，若要回看曲线，调用方必须在 Pod 存活期间保存 `pod_uid` 与容器名，并用其
  重放查询；Obs 不提供「按执行 id 反查已删除 Pod」的能力。
- 记录仍在 retention 内时，保存了 `pod_uid` 的调用方可以查到历史；retention 到期即不可查。
- 该限制写入合同与 `ACCEPTANCE.md`，不当作缺陷。

## 10. 实现落点

| 仓库 | 文件 | 行为 |
| --- | --- | --- |
| ani-observability-service | `api/observability/target/v1/target.proto` | `TargetKind` 加 2；`TargetScope` 加 `pod_uid=7` 与契约注释 |
| ani-observability-service | `internal/biz/target/scope.go` | `TrainingExecution` 常量、`PodUID` 字段、`containerPattern`、按 kind 校验 |
| ani-observability-service | `internal/service/monitoring/service.go` | `scope()` 映射 `PodUid` |
| ani-observability-service | `internal/biz/monitoring/catalog.go` | `Lookup(code, kind)` 分流，训练只 2 个 code |
| ani-observability-service | `internal/biz/monitoring/query.go` | `Validate()` / `Execute()` 传 `q.Target.Kind`；不加 kind 分支 |
| ani-observability-service | `internal/data/monitoring/query_builder.go` | `selector(scope)` 抽函数；训练加 `pod_uid`；一致性校验传 kind |
| ani-observability-service | `internal/data/monitoring/prometheus.go` | `validLabels()` 在训练 kind 下比对 `labels["pod_uid"] == scope.PodUID` |
| ani-observability-service | `deployments/observability/collection/rules/ani-training-*.yaml`、`tests/ani-training.rules.test.yaml` | 新规则组 8 条 + 用例 |
| ani-observability-service | `deployments/observability/collection/patches/ksm-workload-phase3.patch.json`、`restore-ksm-workload-phase3.patch.json` | 新增补丁对 |
| ani-observability-service | `deployments/observability/collection/source-profile.yaml`、`README.md` | 登记规则组、第三标签、空窗类型 |
| ani-governance | （ModelDev 提出方实施）BFF 训练查询链路、按租户串行队列、obs API 依赖升级 | 见 §11 |

`validLabels()` 的 `len(labels) != 8` 判定不变：第三标签只用于规则过滤与 KSM allowlist，
不进入七标签投影。

## 11. 跨仓版本与实施顺序

`TargetScope` 新增字段与 `TargetKind` 新增枚举值属于 API 子模块变更，Gov 必须升级对
`ani-observability-service/api` 的依赖版本，且与 Obs 同批上线。中间态不验收：

| 场景 | 结果 |
| --- | --- |
| Gov 新、Obs 旧 | 训练请求被旧 Obs 以 `invalid target kind` 拒绝（400/`InvalidArgument`） |
| Obs 新、Gov 旧 | 训练请求在 Gov 侧无法组装 `pod_uid`，功能不可用；实例请求不受影响 |

| 阶段 | ModelDev | Obs | 可并行 |
| --- | --- | --- | --- |
| 1 | webhook 给 worker 与 step Pod 打三个标签 | proto / scope / service / catalog / query_builder / prometheus | 是 |
| 1' | — | KSM allowlist 补丁 + 滚动重启（部署窗口） | 与阶段 1 并行 |
| 2 | BFF 单 Pod 查询链路 + 按租户串行队列 | 训练规则组 + promtool 用例 | 是 |
| 3 | 验 worker 曲线（需 1、1'、2 全就绪） | 同左 | — |
| 4 | step 分页懒加载 | phase3 合同定稿 | 是 |

阶段 2 的规则在阶段 1' 之前落地时，`label_ani_io_workload_kind` 尚未暴露，训练 record 恒空；
这不是规则缺陷，验收必须等 1' 完成。

## 12. 边界（本期不做）

- 判断训练归属、判断 GPU 请求、读 Kubernetes API、调用 Res 或 ModelDev。
- 新增标签族、新增 `GapReason` 枚举值（GPU 无数据由 BFF 合并 `device_mode` 判定）。
- 新增原始指标或数据源、新增 Installer 开关。
- 查询侧排队、缓存、重试、分布式限流；分页与串行队列由 BFF 承担。
- 修改一期、二期目录内任何文件（含各自 `SHA256SUMS`）。
- 前端对接。
