# ANI 可观测服务一期任务包 v1.2 审核全文

本文件把公共约定、手工采集专项、十个任务和验收资料合并，方便一次通读。实施时按各原文件及任务依赖执行；本包尚未实施项目或验证真实环境。

所有 JSON 示例为合成契约示意。API proto 为本包确定的生成输入。真实配置 patch 和完整 recording 表达式由 OBS-02 基于实际样本交付。


源文件：[README.md](README.md)

## ANI 可观测服务一期任务包 v1.2

本包供执行 Agent 分项实施和验收。交付目标是容器实例 CPU、内存监控的完整后端链路：Gov 先向 Res 解析可信实例范围，再调用 ani-observability-service 查询 Prometheus。范围查询和最新值查询均通过 Gov 验收，本期不开发前端。

本版沿用已确认方案 v1.1 的业务接口、指标口径、recording rules、API 子模块和权限模型；唯一范围调整是：**暂不接入 ani-installer，采集配置由人工应用并归档到新服务仓库。** 手工部署和真实数据验收仍须完成。

### 1. 阅读与使用

1. 先读本文件、`CONTRACT.md` 和 `ACCEPTANCE.md`。这些文件是全部任务的公共约定。
2. 将整包作为执行 Agent 可读取的工作资料。OBS-00 将其归档到新服务的 `docs/specs/observability-phase1/`。跨仓库任务继续引用同一份资料，不各自维护协议副本。
3. 将 `START.md` 中的启动指令交给协调 Agent，或单独发送 `tasks/` 下某个任务文件。每个任务文件的全部字符数控制在 4000 以内；公共资料必须一并可读。
4. 每个任务完成后按 `TASK-REPORT.md` 返回真实产物、版本、验证结果和下一项任务可使用的输入。报告写入 `docs/specs/observability-phase1/reports/`。
5. `REVIEW.md` 是为审核合并的阅读视图；修改时以公共约定及各任务原文件为准，再重新生成阅读视图。

此包交付时的任务状态均为 **未执行**。包内 JSON 是契约示例，proto 是确定的接口输入，均不代表服务已部署或真实集群已验证。环境字段由执行时读取的真实值填写，不能以示例值代替。

### 2. 仓库范围与设计基线

| 仓库 | 设计基线 | 本期用途 |
| --- | --- | --- |
| ani-kratos-layout | `fd18422211c741dd5242d2992c3d307d522aa28c` | 初始化模板；不在模板内实现监控业务 |
| ani-resource-service | `f7459f7ae71c5573f53a50c5d20908b129c17e0c` | 新增只读范围解析 RPC |
| ani-governance | `90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812` | 新增两个查询入口及受信调用 |
| ani-observability-service | 新项目，实际创建后记录提交 | API 子模块、监控实现、手工采集材料、服务部署 |

以上是方案所依据的提交，不声称执行时仍是最新 main。开始任务时记录实际基线及相关差异，保留用户既有工作；不得为了对齐基线覆盖现有改动。`ani-installer` 不在本包修改仓库与完成条件中。

### 3. 固定任务与依赖

| ID | 任务 | 前置 | 主要完成条件 |
| --- | --- | --- | --- |
| OBS-00 | 项目初始化与公共约定落库 | 模板、工作目录 | 新项目可构建，目录与约定就位 |
| OBS-01 | 独立 API Go 子模块 | OBS-00 | API 独立验证且存在真实可引用版本 |
| OBS-02 | 手工补齐采集与 recording rules | OBS-00、实际集群与 Res 集群 ID | 真实规范时序、规则及身份验证通过 |
| OBS-03 | Res 可信范围解析 | Res 实际基线 | 新 RPC 只读且形成真实可引用版本 |
| OBS-04 | Prometheus 查询适配 | OBS-01 | 固定查询、数据源、解析和上限生效 |
| OBS-05 | 监控业务与结果规范化 | OBS-04 | 时间网格、零值、缺失、冲突、并发生效 |
| OBS-06 | 受信 gRPC 与服务部署 | OBS-05 | mTLS、配置、启动清理、私网部署通过 |
| OBS-07 | Gov 查询接入 | OBS-01、03、06 | 两个 HTTP 接口、权限与真实依赖通过 |
| OBS-08 | 真实后端链路验收 | OBS-02、07 | Gov → Obs → Prometheus 的证据完整 |
| OBS-09 | 版本、部署与交付收口 | OBS-00 至 08 | 从真实依赖可重建，验收材料可复核 |

OBS-03 可以先行；OBS-00 后，OBS-01 与 OBS-02 可以并行。OBS-04 至 06 顺序推进。Gov 在依赖版本明确后可提前准备代码，但 OBS-07 完成必须有 OBS-06 的服务端验证。OBS-08 必须同时等到采集与 Gov 链路完成。

多个 Agent 不同时写同一工作树内的 `go.mod`、生成配置、装配文件。并行时采用独立分支或 checkout，由协调 Agent 顺序整合；避免为此创建复杂的项目管理机制。

### 4. 本包内容

| 文件 | 用途 |
| --- | --- |
| `START.md` | 可复制的总启动指令 |
| `CONTRACT.md` | 范围、接口、字段、错误、限制、模块和身份约定 |
| `MANUAL-METRICS.md` | 人工应用采集配置、recording rules 和验证的固定流程 |
| `ACCEPTANCE.md` | 按任务分配的验收用例与真实验证条件 |
| `TASK-REPORT.md` | 每项任务的完成报告模板 |
| `tasks/OBS-*.md` | 10 个可独立派发的执行任务 |
| `contracts/api/` | 两份正式接口 proto 输入 |
| `contracts/res-scope.md` | Res 增量 RPC 与字段定义 |
| `examples/` | Gov 请求响应与环境、版本记录模板 |
| `MANIFEST.json` | 任务编号、依赖、基线与文件字符数 |
| `SHA256SUMS` | 交付文件校验值 |

### 5. 执行边界

执行固定任务，不扩展平台服务监控、GPU 指标、日志查询或告警实现。日志和告警只保留模块边界。现有实例创建、启停、调度、配额语义不在本期变更范围内。

记录规则继续使用，集群 ID 继续贯通。手工配置集中在 `deployments/observability/collection/`，服务部署集中在 `deployments/observability/`。本期不新增安装器代码、自动安装系统、业务数据库、消息队列或通用 PromQL 接口。

正常实现选择按本包直接推进。只有具体环境缺失、接口冲突或已有访问权限阻挡某一步时，记录证据并完成其余独立任务；不得用 mock、忽略失败或伪造版本将整包标记完成。

### 6. 依据

仓库事实来自已核实基线；规范性字段和参数是本任务包的实施决定。

- [模板 Makefile](https://github.com/zhangzhe-ctrl/ani-kratos-layout/blob/fd18422211c741dd5242d2992c3d307d522aa28c/Makefile)
- [Res 实例接口](https://github.com/zhangzhe-ctrl/ani-resource-service/blob/f7459f7ae71c5573f53a50c5d20908b129c17e0c/api/instance/v1/instance.proto)
- [Gov 实例超时过滤器](https://github.com/zhangzhe-ctrl/ani-governance/blob/90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812/app/admin/service/internal/server/instance_timeout.go)
- [Prometheus recording rules](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
- [Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [Go 子目录模块发布](https://go.dev/doc/modules/managing-source)


源文件：[START.md](START.md)

## 执行启动指令

请执行这份 ANI 可观测服务一期任务包。先完整阅读 `README.md`、`CONTRACT.md`、`ACCEPTANCE.md`，再按任务依赖执行 `tasks/OBS-00` 至 `OBS-09`。单项任务只在其前置产物存在时标记完成。

固定目标：基于 ani-kratos-layout 初始化 ani-observability-service，组织 monitoring、logs、alerts 三个业务模块，建立同仓库独立 API Go 子模块。本期仅实现容器实例 CPU、内存范围和最新值查询，闭合 Gov → ani-observability-service → Prometheus；Gov 通过 Res 的只读 RPC 解析可信实例范围。不开发前端。

本期不修改 ani-installer。采集、标签及 recording rules 由 OBS-02 手工应用，文件归档到新服务 `deployments/observability/collection/`。必须完成真实配置和真实数据验证，不以说明文档或 mock 代替。

先核对实际仓库位置、提交、工作树状态、相关项目规范与现有环境。模板使用命名分支及规定初始化脚本。不要覆盖已有工作，不要编造 UUID、地址、证书、提交、模块版本或测试结果。

将本包归档到新服务的 `docs/specs/observability-phase1/`，其他仓库任务引用同一份契约。API proto 使用 `contracts/api/` 中确定的字段和编号。固定指标、路由、错误、限制和缺失语义不得自行改名或扩展。

先发布或形成真实可获取的 API 子模块版本，再让根服务和 Gov 消费；Res 的新 RPC 也要形成真实可引用版本。本地 go.work 可以辅助开发，正式验收必须 GOWORK=off，且不能依赖本地 replace。

OBS-03 可先行，OBS-00 后 OBS-01 与 OBS-02 可并行；OBS-04、05、06 顺序推进；OBS-07 完成后与 OBS-02 汇合做 OBS-08；最后完成 OBS-09。并行任务避免争写共享文件。

每项任务按 `TASK-REPORT.md` 汇报改动、提交/版本、真实命令和结果、证据路径、尚未执行项。遇到具体阻碍时保留证据，继续无依赖工作。只有代码验证、手工采集验证和真实后端链路验收全部满足时，才能宣布本期完成。


源文件：[CONTRACT.md](CONTRACT.md)

## 一期公共实施约定

本文件是 OBS-00 至 OBS-09 共用的实施合同。用户后续明确确认的修改优先；执行 Agent 不自行调整本文件中的编码、字段、路由、单位、算法、边界或错误含义。

### 1. 一期交付范围

- 新服务：`ani-observability-service`，基于 `ani-kratos-layout`，组织 monitoring、logs、alerts 三个业务模块。
- 只实现容器实例 `workspace` 容器的 CPU、内存范围与最新值查询；CPU 规格和 GPU 规格实例均适用这两项资源指标。
- Gov 负责公共入口、租户和 `instance:read` 鉴权、Res 范围解析及调用协调。
- Res 只提供持久化实例的可信查询范围。
- Observability 负责指标定义、数据源、固定查询、限制和结果规范化；查询路径不调用 Res、Modeldev 或 Kubernetes。
- Prometheus 负责采集、recording rules 和时序存储。本期手工补齐配置，不改 ani-installer。
- 不开发前端；不实现 GPU/共享 GPU、网络、磁盘、平台服务、训练或推理指标；不实现日志查询和告警。logs、alerts 仅目录与说明，不提供假成功接口。
- 不新增业务数据库、队列、调度系统、PVC、数据源管理页面、动态指标管理或任意 PromQL API。

一次请求的顺序固定为：Gov 鉴权 → Res.ResolveInstanceMetricScope → Gov 校验返回范围 → Observability.QueryRange/QueryLatest → Prometheus → 规范化 → Gov DTO。

### 2. 项目和 Go 模块边界

根模块：`github.com/zhangzhe-ctrl/ani-observability-service`。

API 子模块：`github.com/zhangzhe-ctrl/ani-observability-service/api`，与根模块同一仓库、独立 `go.mod/go.sum`。业务模块不再拆成独立 Go module。

| 目录 | 固定职责 |
| --- | --- |
| `api/observability/target/v1/` | TargetKind、TargetScope |
| `api/observability/monitoring/v1/` | MonitoringService、查询与时序 DTO |
| `internal/biz/target/` | 领域范围及校验 |
| `internal/biz/monitoring/` | 指标定义、查询用例、规范化与限流 |
| `internal/data/monitoring/` | 固定 PromQL 构造、HTTP 数据源适配 |
| `internal/service/monitoring/` | gRPC DTO 与领域类型转换 |
| `internal/{biz,data,service}/{logs,alerts}/doc.go` | 后续模块边界说明 |
| `internal/server/governance.go` | 受信 Gov 监听与元数据校验 |
| `internal/server/observability.go` | 保留模板已有进程自身遥测职责 |
| `cmd/ani-observability-service/monitoring.go` | openMonitoring 装配与关闭清理 |
| `deployments/observability/collection/` | 手工采集配置、规则、验证与恢复资料 |
| `deployments/observability/` | 新服务部署配置 |
| `docs/specs/observability-phase1/` | 本包、任务报告与脱敏验收资料 |

沿用模板手工装配方式，不引入 Wire。API 只含 proto、标准 protobuf/gRPC 生成代码及契约说明，不依赖根模块、Kratos 服务实现、Prometheus adapter、Kubernetes、日志或告警 SDK。根服务与 Gov 导入 API，API 不反向导入根服务。对 Gov 的隔离检查只评估本次新增 API 依赖，不要求删除 Gov 已有其他业务依赖。

#### 工具与初始版本

| 项目 | 固定初值 |
| --- | --- |
| 模板 Go / API Go | `1.26.7` |
| API gRPC runtime | `v1.82.1` |
| API protobuf runtime 与 protoc-gen-go | `v1.36.11` |
| protoc-gen-go-grpc | `v1.5.1` |
| Buf | `v1.60.0` |
| Kratos 初始化 CLI | `v3.0.0-20260626125723-668db92c2c00` |

API 的 `go.mod` 一开始写入固定 runtime 版本，不使用空模块 `tidy` 自动选择最新版本。Gov 保留其现有运行时版本，由 Go 模块选择规则决定共同依赖版本，并以实际构建验证兼容性，不强迫 Gov 降级。

根 `buf.yaml` 为 v2，两个 module path 分别为 `internal` 和 `api`。保留原 `buf.gen.yaml` 生成配置；新增 `buf.gen.api.yaml`，input=`api`、out=`api`、`paths=source_relative`，两个插件使用以上固定版本。不得设置 `clean: true` 清空 API 输入与 go.mod。Makefile 和 `scripts/verify-source` 显式执行两份模板；根 `go generate ./...` 不能代替 API 子模块生成检查。

API 先形成真实可获取版本，再由消费者引用。API tag 使用 `api/v0.1.0`，消费者版本为 `v0.1.0`；也可记录已经推送、可获取的真实 pseudo-version。不得编造版本。OBS-01 不为尚无 import 的根模块强塞会被 tidy 删除的 require；首次真实导入 API 时固定已验证版本。根服务 tag 使用 `v0.1.0`，VERSION 使用 `git describe --tags --match 'v[0-9]*' --always --dirty` 排除 API tag。

正式验收分别对 API、根服务、Gov 执行 `GOWORK=off` 的相关检查，无本地 replace。API 单独执行 tidy-diff、test、vet、mod verify；根服务保留模板 verify/build/audit；Gov 保留现有生成与质量门槛。API 子模块不被根目录 `go test ./...` 自动覆盖。

### 3. 集群和采集身份

同一真实集群使用 Res 登记的稳定、非零、规范 UUID：

- Res：`Spec.ClusterID`。
- 人工采集配置：本集群唯一 `cluster_id` 输入。
- Prometheus 本地原始与规范时序：`ani_cluster_id` 标签。
- Observability datasource：`cluster_id`。

它是 ANI 的集群身份；不是 Kubernetes 内置 cluster UID、Prometheus Pod ID 或鉴权凭证。重启 Pod、容器、Prometheus 不更换该 ID。每个集群配置一次，各项指标共享；不对每个指标新增开关或集群配置。本期验证一个真实集群，配置结构可列多个 datasource；只按可信 scope 精确定位一个，不跨源搜索或汇总。

`external_labels` 不能代替本地时序所需的标签注入。不得依赖前端传入 cluster_id、tenant_id、namespace 或 PromQL 来形成可信范围。

#### 规范 recording 结果

| 业务编码 | record 名称 | 单位 | 定义 |
| --- | --- | --- | --- |
| `cpu.usage_cores` | `ani_instance_cpu_usage_cores` | `cores` | workspace 整容器 CPU counter 的 `rate(...[5m])` |
| `memory.working_set_bytes` | `ani_instance_memory_working_set_bytes` | `bytes` | workspace 的 working set 字节数 |

两个 record 都只保留以下业务/运行身份标签：`ani_cluster_id`、`ani_tenant_id`、`ani_instance_id`、`namespace`、`pod_uid`、`runtime_container_id`、`container`。Prometheus 自身的 `__name__` 不计入这七项。

`pod_uid` 使用规范 UUID。`runtime_container_id` 为去除运行时前后缀后的完整 64 位小写十六进制 CID，原始 cAdvisor 与 KSM 两侧一致。`container` 固定 `workspace`。

CPU 不是百分比，不除以 request/limit，不压到 1；不将 sidecar 或 POD sandbox 计入。CPU 启动时至少两条有效样本才能得到 rate，5 分钟窗口未填满时仍按原算法作估计。内存不是 RSS、申请额度或限制值，不转 MiB/百分比。

#### 录制时的绑定

原始指标固定为 `container_cpu_usage_seconds_total`、`container_memory_working_set_bytes`、`kube_pod_container_info`、`kube_pod_container_status_running`、`kube_pod_labels`。KSM Pod label allowlist 包含 `ani.io/tenant-id` 与 `ani.io/instance-id`，保留已有项。

1. 根据真实样本核实 cAdvisor CPU 维度，选择唯一整容器 counter，禁止把 total 和 per-CPU 数据同时计算。
2. 从 cAdvisor 原始 `id` 解析 Pod UID 与 CID，只支持经 fixture 核实的 Linux/containerd 的 systemd/cgroupfs 路径；systemd UID 下划线还原。保留原始 `id` 与 job/instance/endpoint 等来源标签直到唯一性判断完成。
3. 先对每条原始 CPU counter 作 rate，再与 KSM 的同 UID、当前 CID、workspace、running 状态匹配；再绑定同 UID 的可信 ANI 租户/实例标签。不能仅按 Pod 名、当前 UID 或租户标签关联。
4. 录制求值时原始样本、KSM 输入均必须新鲜，且原始当前序列仍存在。旧 CID 在 5 分钟窗口残留 counter 不能继续产生规范值。
5. 先验证唯一来源，再投影成七个规范标签。重复采集不能靠任意 max、sum、topk 或 labeldrop 隐藏。重复导致不产出规范值时，部署验证必须失败并提供原始冲突证据；之后修正配置。
6. 允许为匹配和诊断使用少量内部辅助 record，但它们不是第三个业务指标。辅助 current/snapshot 自身刷新时间不等于底层样本新鲜度；每条辅助规则都要验证输入年龄，禁止层层刷新延长过期数据寿命。

采集间隔 15s；规则组 `ani-instance-metrics-v1`，间隔 30s；原始/KSM 输入最大年龄 45s。PrometheusRule 对象同名，放入实际被选中的监控 namespace；基线选择标签是 `release: ani-metrics`，应用前核实真实 ruleSelector，部署名称不同时按真实选择标签记录，不改全局 selector 来迁就本规则。

完整人工操作、字段 patch、fixtures、promtool 与恢复要求见 `MANUAL-METRICS.md`。

### 4. 可信范围与内部 RPC

#### Res

在 `api/instance/v1/instance.proto` 的 InstanceService 增加：

`ResolveInstanceMetricScope(GetInstanceRequest) returns (InstanceMetricScope)`。

| 字段 | 编号 | 类型 | 数据来源 |
| --- | --- | --- | --- |
| tenant_id | 1 | string | 持久化 TenantID |
| instance_id | 2 | string | 持久化 ID |
| cluster_id | 3 | string | Spec.ClusterID |
| namespace | 4 | string | Spec.Namespace |
| container_name | 5 | string | 现有 biz.Container，值为 workspace |

固定逻辑：`RequireTenant → s.repo.Get(tenantID, instanceID) → 校验持久化范围 → 返回`。不调用 Observe/runtime，不查询当前 Pod，不以 Running 为条件，不刷新数据库、不用 UpdatedAt 推算历史区间。已停止或正常删除但持久化范围仍在的实例可解析；不存在按既有 NotFound；delete-first tombstone 等缺少范围的记录返回 `INSTANCE_METRIC_SCOPE_UNAVAILABLE`。

新增 `internal/service/instance/metric_scope.go`，把精确 RPC 加入 `internal/server/instance_identity.go` 的受信允许范围，更新 `docs/specs/governance-calls.md`。复用现有请求及租户语义，不扩展生命周期和配额。

#### Observability

包内 `contracts/api/` 的两份 proto 是字段编号和类型的确定输入，复制至新仓库 `api/` 对应位置生成，不重写编号。

- package：`observability.target.v1`；TargetKind 仅 UNSPECIFIED=0、CONTAINER_INSTANCE=1。
- TargetScope：kind=1、tenant_id=2、instance_id=3、cluster_id=4、namespace=5、container_name=6。
- package：`observability.monitoring.v1`；服务名 `MonitoringService`，仅 QueryRange、QueryLatest。
- QueryRangeRequest：target=1、metric_codes=2、start=3、end=4、step_seconds=5。
- QueryLatestRequest：target=1、metric_codes=2。
- QueryRangeResponse：start=1、end=2、step_seconds=3、series=4。
- QueryLatestResponse：evaluated_at=1、series=2。
- MetricSeries：metric_code=1、unit=2、status=3、points=4。
- MetricPoint：Timestamp timestamp=1、**optional double value=2**、GapReason gap_reason=3。

Target 的 tenant/instance/cluster 都是非零规范 UUID，namespace 是合法 Kubernetes namespace，container_name=workspace；tenant 与受信元数据相同。内部 uint32 step_seconds 不承担省略默认逻辑，必须由 Gov 提供已确定的合法值。

内部枚举保留完整前缀：SERIES_STATUS_UNSPECIFIED=0、OK=1、PARTIAL=2、NO_DATA=3；GAP_REASON_UNSPECIFIED=0、NO_DATA=1、AMBIGUOUS_SOURCE=2、INVALID_VALUE=3。正式 proto 中每个值均带其枚举前缀。

### 5. Gov 公共 HTTP 合同

#### 范围查询

`POST /api/v1/instances/{instance_id}/metrics/query`，成功 HTTP 200。

body 仅为 `{"data":{"metric_codes":[...],"start":"RFC3339","end":"RFC3339","step_seconds":60}}`。

- instance_id 只来自路径，body 即使重复同一个 ID 也拒绝。
- metric_codes 必填 1 至 2 项、不能重复、不能未知。
- start/end 必填，使用整秒 RFC3339；允许时区偏移，统一转换 UTC。
- step_seconds 公共 DTO 使用可感知 presence 的字段：省略时 60；显式 0 非法。
- 只接受以上字段和层次，拒绝未知字段、重复 JSON 字段、额外 query 参数以及 scope、PromQL、数据源地址。公共 JSON 字段使用 snake_case。
- 同步查询不走 202、创建幂等、版本提交或配额分支。

响应：`data { instance_id, start, end, step_seconds, series[] }`；series 元素：`{ metric_code, unit, status, points[] }`；point 元素：`{ timestamp, value, gap_reason? }`。series 与请求 metric_codes 顺序相同。

#### 最新值

`GET /api/v1/instances/{instance_id}/metrics/latest`，无 body/query 参数，成功 HTTP 200。固定依次查询 CPU、内存。

响应：`data { instance_id, evaluated_at, series[] }`。每项 series 恰好一个点。Observability 在本批次开始时取得一个整秒 evaluated_at，两项指标使用同一求值时刻；point.timestamp 也为该求值时间，而不是宣称原始抓取恰在该时刻。

#### Gov proto 与转换

在 `api/protos/instance/service/v1/instance.proto` 增加 QueryInstanceMetricsRequest、QueryInstanceMetricsData、QueryInstanceMetricsResponse、InstanceMetricsRangeData、GetInstanceLatestMetricsResponse、InstanceMetricsLatestData、InstanceMetricSeries、InstanceMetricPoint。latest 请求复用现有 GetInstanceRequest。公共 service 方法名固定 QueryInstanceMetrics、GetInstanceLatestMetrics。

data 包装必须是实际 proto 字段。snake_case 用显式 json_name；时间用 Timestamp；公共 value 使用 `google.protobuf.DoubleValue`；gap_reason 为 optional string。短状态字符串由 Gov 显式映射：OK、PARTIAL、NO_DATA；缺失原因映射为 NO_DATA、AMBIGUOUS_SOURCE、INVALID_VALUE。

读取内部 optional double 时先检查 presence，正常零值转为包装的 0，缺失转为 nil wrapper。不能只用 GetValue 后把缺失变成 0。真实 HTTP 编码必须分别输出 `"value":0` 与 `"value":null`；正常点不输出 gap_reason。OpenAPI nullable 与真实编码一致。

完整合法示例见 `examples/gov-range-request.json`、`gov-range-response.json`、`gov-latest-response.json`。这些样例为协议示意，不是实测记录。

### 6. 时间、历史、缺失与冲突

范围网格是 `t_n = start + n*step` 且 `t_n <= end`。点数为 `floor((end-start)/step)+1`，start=end 合法。返回 UTC 整秒时间；不自行对齐边界、不插值、不用前值或 0 补空洞。end 不要求恰为网格点。

历史只查询规范 record，不现场 join 当前 KSM，不以当前 Pod/Running 状态过滤。record 最大年龄 45s，相对 **每个历史求值时刻** 检验。例如固定查询等价于 `record{可信范围} and (time() - timestamp(record{同一可信范围}) <= 45)`。不得拿当前墙钟和旧记录比较，也不得把矩阵的求值时间误当底层 record 样本时间。

查询适配使用 range 的 `/api/v1/query_range` 和 latest 的 `/api/v1/query`。只构造两种固定 record 表达式；scope 五个 matcher 精确等值：ani_cluster_id、ani_tenant_id、ani_instance_id、namespace、container。Pod UID/CID 不从当前业务对象查询，而由已录制时序返回。

每项指标、每个求值点独立处理：

| 合格来源 | 结果 |
| --- | --- |
| 没有 | value=null，gap_reason=NO_DATA |
| 恰一个且为有限非负值 | 输出真实数值，包括 0 |
| 同时存在多个 UID/CID 来源 | value=null，gap_reason=AMBIGUOUS_SOURCE；不相加、不任选来源 |
| 恰一个但值为 NaN、±Inf 或负数 | value=null，gap_reason=INVALID_VALUE |

多来源判断先于数值裁决；不同时间点分别来自先后 Pod/CID 是正常历史。结构损坏、scope 标签不符、缺少必要标签、重复点等属于上游响应错误，不按 NO_DATA 吞掉。

所有点有效为 OK，部分有效为 PARTIAL，无有效点为 NO_DATA。即使整段没数据，仍返回请求的 series 和完整 null 网格。NO_DATA 不能证明实例已停止或采集正常。

录制前发现重复原始采集并抑制结果时，查询只能看见空点，不能凭空还原 AMBIGUOUS_SOURCE；此类问题由手工配置验收识别和修正。查询 AMBIGUOUS_SOURCE 用于规范结果仍有多个运行来源的情形。

只从规则启用时开始保存规范历史，不回填更早 raw 历史。当前规则健康检查属于部署验收，不作为每次历史查询的前置条件。实例授权仍成立、持久化范围可解析、时序尚在保留期内时，可以查询其已停止或已删除 Pod 的历史。

### 7. 查询限制和错误

| 限制 | 固定值 |
| --- | --- |
| 每请求实例数 | 1 |
| 指标数 | 1–2，去重前不得重复 |
| 范围 | 最大 24h，start ≤ end，end 不晚于服务当前时刻 |
| step | 公共省略 60s；合法 30–3600s，30 的整数倍 |
| 点数 | 每指标最大 3000 |
| Prometheus 返回 series | 每指标最多 128；请求 limit=129，收到 129 即超限 |
| Gov body | 最大 64 KiB |
| 单个 Prometheus 响应体 | 解码前最大 8 MiB；有界读取，不依赖 Content-Length |
| 每副本请求并发 | 16 |
| 每副本每租户请求并发 | 4 |

一次请求包含的两个 Prom 查询共用 Prom 4s 批次预算，不给每项重置完整预算。超额请求立即失败，不排无限队列。无自动重试、缓存或分布式限流。必须传播请求取消，并释放所有并发令牌及空闲连接。

超时链：Gov 两个精确路由 10s；Res 范围 RPC 2s；Observability RPC 6s；Prometheus 批次 4s。所有子调用服从更短的父 deadline，不能用 background 脱离父请求。

| 公开 reason | HTTP | gRPC code/处理 |
| --- | --- | --- |
| MONITORING_INVALID_QUERY | 400 | InvalidArgument |
| 既有身份与权限错误 | 401/403 | 复用现有 Unauthenticated/PermissionDenied |
| 既有实例不存在错误 | 404 | NotFound |
| INSTANCE_METRIC_SCOPE_UNAVAILABLE | 409 | FailedPrecondition |
| MONITORING_REQUEST_TOO_LARGE | 413 | Gov body 边界直接拒绝 |
| MONITORING_RESULT_TOO_LARGE | 422 | ResourceExhausted + 对应 reason |
| MONITORING_CONCURRENCY_LIMIT_EXCEEDED | 429 | ResourceExhausted + 对应 reason |
| MONITORING_SOURCE_NOT_CONFIGURED | 503 | Unavailable + 对应 reason |
| MONITORING_SOURCE_UNAVAILABLE | 503 | Unavailable + 对应 reason |
| MONITORING_SOURCE_RESPONSE_INVALID | 502 | DataLoss + 对应 reason |
| MONITORING_QUERY_FAILED | 500 | Internal |
| MONITORING_QUERY_TIMEOUT | 504 | DeadlineExceeded |

沿用 Gov 既有错误外壳。新增 Observability 错误及 Res 的 INSTANCE_METRIC_SCOPE_UNAVAILABLE 统一通过标准 `google.rpc.ErrorInfo.reason` 携带上述业务 reason；不新增私有错误 proto。Gov 按 gRPC code 和该字段明确转换，不靠错误文本或 ErrorInfo 其他字段分派。既有 Res 错误沿用原有处理；不能复用日志接口对所有 ResourceExhausted 的单一映射。请求已取消则终止下游；不要在客户端已断开后伪造成功响应。

任一指标发生上游请求失败，整批失败；合法缺失数据继续用 200 + null 表示。连接失败、上游 5xx 为 SOURCE_UNAVAILABLE；非预期 resultType、畸形 JSON、损坏点、身份不符为 SOURCE_RESPONSE_INVALID；固定表达式被拒绝执行且非超时为 QUERY_FAILED。超时按 TIMEOUT 优先分类，不能归成空数组。

为固定解析边界，一期对 Prometheus 成功响应中的非空 warnings 按 SOURCE_RESPONSE_INVALID 处理并保留诊断，避免把可能不完整的结果宣称为正常；infos 可记日志。limit 超出与响应体超限优先返回 RESULT_TOO_LARGE。不得向公共响应输出 PromQL、原始标签、内部地址或 dev_profile。

### 8. Gov 与内部调用身份

两个公共接口均为 `instance:read`。Gov 先完成用户身份、选定租户和实例读权限，再调用 Res。Gov 验证 Res 返回 tenant_id、instance_id 与本次上下文一致，其他范围字段也满足合同，再调用 Observability。外部请求不能覆盖此范围。

Observability 受信 gRPC 端口使用 TLS ≥1.3、校验 CA 和服务身份。Gov 验证服务端身份 `ani-observability-service`；服务端只允许受信 `ani-governance` 客户端身份。按证书 SAN 等现有受信机制校验，不只接受“任意同 CA 证书”。不得复制 Res 旧监听的 `ani-network-service` ServerName。

Gov 重建以下单值元数据：`x-ani-tenant-id`、`x-ani-actor`、`x-ani-request-id`。不把外部同名 header 原样透传。Obs 拒绝缺失、多值、格式不合法或与 target 不一致的身份。只开放两个精确方法：

- `/observability.monitoring.v1.MonitoringService/QueryRange`
- `/observability.monitoring.v1.MonitoringService/QueryLatest`

一期仅 tenant scope，不自动放宽为跨租户管理员全局查询；reflection 关闭。实例工作负载不得以自行拼接 scope 绕过受信入口。Kubernetes 网络范围与 mTLS 配合验证，不因 ClusterIP 就假设租户完全不能访问底层 Prometheus。

Gov 增加专用薄 HTTP handler，同步 200，设置 Operation 并执行原有 `ctx.Middleware`。body 使用 `"*"` 映射 data 包装；防止自动生成路由和手工路由重复注册。仅这两个精确路由使用 10s 超时，其余 Gov 路由行为保持现有配置。

必须覆盖公共 proto、`api/protos/admin/service/v1/i_instance.proto`、service/server 各自的 instance_metrics.go、Res client、observability_client.go、observability_client_security.go、instance_timeout.go、instance_authorization.go、wiring_ent.go、权限 SQL 与实际 API 同步/权限注册步骤。

### 9. 实现落点与服务配置

| 文件 | 固定行为 |
| --- | --- |
| biz/monitoring/catalog.go | 两个代码、record 名、单位、口径 v1 |
| biz/monitoring/query.go | QueryRange/QueryLatest 校验与批次执行 |
| biz/monitoring/normalize.go | 网格、来源冲突、零/缺失/异常值 |
| data/monitoring/query_builder.go | 两种固定 record + 可信 matcher + 45s freshness |
| data/monitoring/prometheus.go | 按集群选源、固定 API、超时/体积/series 上限、解析 |
| service/monitoring/service.go | gRPC 转换，optional presence 和错误 reason |
| server/governance.go | 受信身份、方法允许范围、请求元数据 |
| cmd/.../monitoring.go | 创建与注入资源、返回 cleanup |

路径省略的前缀为新服务 `internal/`。领域对象不靠前端提供 Prometheus 标签。字段、方法实现可复用模板现有工具，不能为两项指标新建通用插件框架。

配置示例中的占位必须换成环境实值：

```yaml
server:
  grpc:
    addr: 0.0.0.0:19090
    timeout: 6s
  admin:
    addr: 0.0.0.0:19091
    timeout: 1s
  shutdown_timeout: 10s
governance:
  tls_cert_file: /etc/ani-observability/tls/tls.crt
  tls_key_file: /etc/ani-observability/tls/tls.key
  client_ca_file: /etc/ani-observability/tls/client-ca.crt
monitoring:
  prometheus_timeout: 4s
  max_concurrent_requests: 16
  max_concurrent_requests_per_tenant: 4
  datasources:
    - cluster_id: "<Res真实集群UUID>"
      address: "http://<内部Prometheus服务>:9090"
      bearer_token_file: ""
      tls_ca_file: ""
      tls_server_name: ""
```

沿模板现有配置 proto 的字段编号扩展，不重新编号既有字段。配置启动时加载，修改后重启生效。不合法/重复 cluster_id、缺少地址、不可读取凭据等配置错误启动失败；已配置数据源暂不可用按查询级错误处理，不把整个服务 readyz 永久打掉。

Gov 环境输入固定 ANI_OBSERVABILITY_GRPC_ADDR、ANI_OBSERVABILITY_CA_FILE、ANI_OBSERVABILITY_CLIENT_CERT_FILE、ANI_OBSERVABILITY_CLIENT_KEY_FILE；客户端正常关闭。根服务保留模板自身 metrics/log/health，业务查询不得误写到模板进程遥测模块。

### 10. 部署与后续扩展

交付 Dockerfile、deployments/observability/README.md、configmap.yaml、deployment.yaml、service.yaml、secret.example.yaml，以及 collection/ 手工资料。一期 1 副本 Deployment、私网 ClusterIP、admin 健康探针、已有日志与遥测；无业务 DB/队列/PVC/Kubernetes API 查询 RBAC。若 S06 发现租户可直连底层查询入口，基于现有网络能力补齐仅针对本次 Obs/Prom 查询面的最小访问策略，归档为 deployments/observability/networkpolicy.yaml；保留已有采集、平台访问和探针，不扩展成全平台网络改造。

手工采集使用现有 Prometheus/KSM/cAdvisor 与 Operator 管理对象，按字段应用最小 patch，不升级或重装整个监控栈。保存相关变更前字段、实际 patch、规范规则、原始 fixture、验证命令与恢复说明。日后 Helm 等自动流程可能覆盖手工字段，需将本次变更资料交给后续自动化任务；本期不因此修改 Installer。

新增日志或告警时复用可信目标与服务身份，但采用各自数据源和 RPC 模型。告警后续使用稳定指标定义、规则评估和通知能力，不轮询公共 Gov HTTP 接口。新增业务指标通常扩展目录定义与查询/recording；只有新数据源才补相应采集，不按每个指标新增安装器开关。

### 11. 依据与验证状态

本文件规定实现目标，真实集群适配和后端完成状态只能由任务报告证明。当前交付不包含已运行环境的测试结论。

- [Prometheus recording rules](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)：预计算时序、规则组与规则检查。
- [Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/)：instant/range 参数、结果类型和 series limit。
- [Prometheus 查询与 staleness](https://prometheus.io/docs/prometheus/latest/querying/basics/)：选择器、求值点和 lookback。
- [Prometheus rate](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate)：counter 的每秒速率与边界估计。
- [Prometheus external_labels](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)：对外系统通信标签，不等同本地采集标签。
- [Go 模块源与子目录 tag](https://go.dev/doc/modules/managing-source)：API 子模块版本发布方式。
- 仓库文件依据见 `README.md` 的基线链接。


源文件：[MANUAL-METRICS.md](MANUAL-METRICS.md)

## OBS-02 人工补齐实例指标采集与录制

### 1. 目的、适用范围与交付状态

本专项将 v1.1 的 Installer 工作替换为现有监控环境的人工字段变更。公共约定见 [CONTRACT.md](./CONTRACT.md)，验收见 [ACCEPTANCE.md](./ACCEPTANCE.md)，执行任务为 [OBS-02-manual-collection.md](./tasks/OBS-02-manual-collection.md)。

**当前没有连接真实集群或取得真实指标样本。** 本文件规定实施要求，不提供号称已经验证、可以直接 apply 的完整 PromQL/YAML。OBS-02 先取真实样本，再交付对应配置、规则和验证证据。

仅复用现有 Prometheus、KSM、kubelet/cAdvisor，为 Res 实例的 `workspace` 容器补齐 CPU 核数和 memory working set 字节数。Gov 授权并解析 Res 可信 scope，Obs 查询 Prometheus；该链路保持原方案。静态多 cluster 映射继续支持，真实验收只要求一个 cluster，标识与 Res/Obs 一致。

**不改 `ani-installer` Go、模板、values 或验收逻辑。** 不执行 Helm install/upgrade/rollback，不改全局 selector；通过手工命令、实际 patch、一个只读验证脚本完成工作，不建设安装或回滚工具。不扩展其他指标、日志、告警或新的状态机。

### 2. 固定输出和时间语义

#### 2.1 两条业务 recording 指标

| 指标 | 数值口径 |
| --- | --- |
| `ani_instance_cpu_usage_cores` | 对同一实际运行容器的原始 CPU counter 执行 `rate(...[5m])`，单位为核 |
| `ani_instance_memory_working_set_bytes` | 同一实际运行容器的 `container_memory_working_set_bytes`，单位为字节 |

两条最终业务 record 必须仅保留以下七个标签；原始诊断信息在投影前处理，不混入最终规范记录。Gov 不向外返回这些底层标签：

```text
ani_cluster_id
ani_tenant_id
ani_instance_id
namespace
pod_uid
runtime_container_id
container
```

`container` 固定为 `workspace`。不得纳入 sidecar、基础设施容器、Pod 聚合值、其他业务容器或百分比。`pod_uid` 与 `runtime_container_id` 一起区分同名 Pod 重建和同一 Pod 中容器重启；业务实例身份保持稳定。

Kubernetes 对象名和 rule group 名均为 `ani-instance-metrics-v1`。基线选择标签为 `release: ani-metrics`，但应用前必须读取真实 Prometheus 的 `ruleSelector` 和 `ruleNamespaceSelector`。若真实部署使用不同选择标签，只给本对象生成实际需要的标签并记录依据，不修改全局 selector 来迁就新规则。规则命名空间由真实 selector 决定，不猜测部署名称。

#### 2.2 固定参数

| 参数 | 本方案值 | 实现要求 |
| --- | --- | --- |
| cAdvisor / KSM scrape interval | 15 s | 对选定采集端点作局部调整，保留其他端点配置 |
| recording interval | 30 s | 同一规则组按依赖顺序评估 |
| 原始 CPU / memory 样本新鲜度 | ≤45 s | 在原始输入上判断，不能用中间 record 的新时间戳代替 |
| KSM 样本新鲜度 | ≤45 s | 对参与关联的每类原始 KSM 输入分别判断 |
| 查询 record 新鲜度 | ≤45 s | 相对于每个查询评估时刻判断 |
| CPU rate window | 5 min | 至少两个有效原始样本；保留 counter 的物理与采集来源身份 |

CPU 先对同一原始 counter 执行 rate，再严格关联身份；不跨 UID、CID 或抓取源聚合 counter。五分钟窗口至少两个有效样本即可参与，启动阶段无需填满五分钟。它是包含 Prometheus 边界外推的窗口估计，不是完整五分钟的严格平均；样本不足时缺失，不补零。

45 秒 freshness 约束本点最新有效原始样本，不要求五分钟窗口里所有样本年龄都 ≤45 秒。窗口内缺采沿用既定 rate 语义，不补零或换窗口。Memory 是近期 gauge，不是 30 秒区间平均；采集时差和生命周期过渡缺口如实保留。

### 3. 实现归档目录与文件清单

实施文件固定归档在 **`ani-observability-service/deployments/observability/collection/`**。只要求以下资料和一个只读脚本：

| 文件 | 必须交付的内容 |
| --- | --- |
| `README.md` | 真实环境参数、取证/备份/应用/验证的逐步手工命令、字段差异、预期输出、恢复说明与恢复命令 |
| `source-profile.yaml` | 实际 cluster ID、对象/selector、runtime/cgroup、来源标签、UID/CID 规范化与时间戳行为；不含凭据 |
| `patches/ksm-workload.patch.json` | 合并 allowlist 的实际字段 patch |
| `patches/ksm-servicemonitor.patch.json` | KSM 实际采集端点的字段 patch |
| `patches/cadvisor-servicemonitor.patch.json` | cAdvisor 实际采集端点的字段 patch |
| `patches/restore-*.patch.json` | 按本次改动对象保存的逆向字段 patch；执行前仍须核对当前值 |
| `rules/ani-instance.rules.yaml` | 两条规范 record 及实际必要的内部辅助规则，完整可运行表达式 |
| `rules/ani-instance-metrics-v1.yaml` | 实际 PrometheusRule，使用上述已验证 groups、真实 namespace 与 selector 标签 |
| `diagnostics.promql` | 当前重复来源、关联冲突、解析失败、新鲜度和 CID 不匹配的只读诊断表达式及注释 |
| `tests/ani-instance.rules.test.yaml`、`tests/fixtures/` | promtool 时间轴、输入、预期标签/数值/缺失；真实脱敏样本与合成负例分别标注 |
| `verify.sh` | 唯一要求实现的脚本，只读验证并输出证据，不修改集群、不操作业务实例 |

无需修改的对象不制造空 patch，README 记录“原配置已满足”。取证、patch 编写、应用和恢复均由手工命令完成，不要求专用生成器或程序。两份规则文件中的 groups 必须一致，不能人工复制后各自漂移。

备份和执行输出保存在 `collection/artifacts/manual-metrics/<run-id>/`，忽略提交。保留原始对象及 UID/resourceVersion、修改字段的原值和应用后值、实际 patch、targets/样本、promtool 输出、验证与恢复结果；无需另建固定目录框架。Git 中只提交必要的脱敏样本和配置资料，凭据、未脱敏对象及私有运行备份不提交。

### 4. 阶段 A：真实环境取证与变更前备份

先记录 Res 的稳定 `cluster_id`、kubectl context、API server 身份、Prometheus 地址和验收实例的可信 scope。cluster ID 不使用 context/release/显示名。实例必须来自正常 Res 流程，读取其真实 Pod UID、主容器 CID 和 owner 标签，不用手工打标签的任意 Pod 冒充。

按以下顺序取证，将实际 `kubectl get` 和 Prometheus 只读请求写入 README：

1. 保存 Prometheus CR/版本、规则与采集 selector、现有规则组；保存 KSM workload/args、KSM 与 cAdvisor ServiceMonitor。记录对象 kind、namespace、name、UID、resourceVersion、Helm 所有权和拟改字段原值。
2. 查看实际 scrape targets，确认 job/instance/endpoint/metrics path 和重复发现。既要保存 exposition，也要保存 TSDB 落库样本；单个 endpoint 返回正常不证明 Prometheus 已正确存储。
3. 保存 CPU、memory、Pod labels、container info、running 五类样本，核实 allowlist 转换名称、UID/CID、runtime/cgroup、CPU 整容器维度、时间戳和数个采集周期的变化。
4. 把不满足项列成字段差异，填入 source profile，编写实际 patch 和逆向恢复 patch。后续 Helm 操作可能覆盖这些人工字段，报告记录这一点，留给后续部署工作接收。

### 5. 阶段 B：保留配置的人工补采集

#### 5.1 允许修改的字段

先生成、审查并保存字段差异，再逐项执行 JSON Patch。每个 patch 包含对象身份和原值的前置检查，防止在对象已被替换或同一字段已变化时继续套用旧计划。

- **KSM allowlist：** 在现有 `--metric-labels-allowlist` 配置中合并 `pods=[ani.io/tenant-id,ani.io/instance-id]`。保留其他资源类型、已有 Pod 标签、其他启动参数及其行为；不得用这两项覆盖整份 allowlist，不启用通配符。
- **KSM 和 cAdvisor 采集：** 对实际相关端点设定 15 秒间隔，合并该 profile 需要的 relabel/metric relabel。保留已有 metric 路径、认证/TLS、已有筛选规则和其他端点。不能替换整个 `endpoints` 或 relabel 数组而丢掉原项。
- **Cluster 标签：** 在两条采集链路进入本地 TSDB 前注入同一权威 `ani_cluster_id`，不信任 exporter 自报的冲突值。只设置 Prometheus `external_labels` 无法作为本地关联已经具备该标签的证明。
- **时间戳：** 核实 cAdvisor 原始样本的显式时间戳是否由 Prometheus 保留。原始 freshness 承诺依赖该行为；若必须调整相关端点的 `honorTimestamps` 或 staleness 配置，应单列字段差异与测试依据。不得把 exporter 重复返回的旧统计值重新赋予 scrape 时间，再声称统计值是新的。

KSM 样本新鲜度表示该 KSM 指标样本近期可用，不代表 Kubernetes API watch 缓存与 cAdvisor 在现实世界中原子同步。该边界由严格 UID/CID 匹配和缺失处理兜住，不增加新的同步状态机。

修改 KSM 参数通常会引起其 workload rollout。操作前在字段差异报告中写清影响对象；完成后等 KSM 恢复可抓取并核验标签，再进行规则加载。不得为验证而重启或删除业务 Pod。

#### 5.2 来源 profile 与规范化

本期只接收经实际样本验证的 Linux/containerd profile；systemd 与 cgroupfs 分别按样本实现，不用一个宽泛正则猜所有格式。

| 来源 | 必须得到的规范值 |
| --- | --- |
| cAdvisor `id` | 提取真实 Pod UID 和运行容器 CID；原始 `id` 留到 rate 与唯一性检查结束 |
| KSM `uid` | 标准 UUID，五组格式 `8-4-4-4-12`，与原始来源规范值完全相同 |
| KSM `container_id` | 验证 `containerd://` 前缀后提取 64 位小写十六进制 CID |
| systemd cgroup 路径 | 仅对已识别的 Pod UID 段把下划线恢复为连字符；识别并去除 CID 段的 `cri-containerd-` 与 `.scope` |
| cgroupfs cgroup 路径 | 从已验证的 `pod<UID>/<CID>` 结构提取 UID 和完整 CID |

两个输入都必须执行同一规范化验证，不能只有 cAdvisor 一边转换。CID 不截短，不接受带前后缀的最终值；Pod UID 不允许用 Pod 名称、Notebook 名称或 intent version 代替。未知格式、缺字段、解析不唯一或实际 CPU profile 不符时，明确拒绝该 profile 的验收，不自由发挥为“按 namespace + pod 关联”或“把所有 CPU 分量相加”。

KSM 必须提供 `kube_pod_labels` 中的 `uid` 和 owner 标签，以及 `kube_pod_container_info{container="workspace"}` 中的同一 `uid`、当前 `container_id`，再以 `kube_pod_container_status_running == 1` 约束运行状态。标签 allowlist 转换结果预计为 `label_ani_io_tenant_id`、`label_ani_io_instance_id`；实际名称、冲突后缀必须在真实样本中核实。owner 标签应来自 Res 写入的 Pod metadata，不能假定 cAdvisor 自动带出这些标签。

### 6. 阶段 C：完整 recording rules 的生成契约

OBS-02 按以下顺序生成完整表达式。这里规定算法和可验收不变量，不提供未经真实样本验证的“通用可 apply 规则”。任何内部 current/snapshot 或 diagnostic 辅助 record 都必须与最终规则同包交付、同样测试，不得藏在手工终端中。

1. **筛选与规范化原始输入。** 仅选中 `workspace` 的目标 CPU total counter、memory gauge 和三类 KSM 输入。保留原始 cgroup ID、来源 job/instance/endpoint 等标签；注入权威 cluster ID，规范化 UID/CID。
2. **检查原始 freshness。** 在原始样本上，以本次规则评估时刻减去输入样本时间戳，检查 ≤45 秒。KSM 的 Pod labels、container info 和 running 状态分别检查，不能通过一个新写入的 snapshot 时间戳替代所有输入检查。
3. **检查来源唯一性。** 对每类输入按对应物理身份检查有效来源数量；同一 UID/CID 被多个抓取源重复提供，或同一 UID 的业务身份/CID 关联存在冲突，先诊断并抑制该身份的输出。必须在删除 job/instance/endpoint 等标签之前完成，避免 many-to-many join 或落库标签冲突影响整个规则组。
4. **建立当前容器身份关联。** 通过 cluster、namespace、Pod、Pod UID、container 名称以及规范化的当前 CID 建立严格关联条件。Pod 名只作为一致性检查字段，不能替代 UID。已有同名 Pod 或历史 CID 不因名字相同而获得关联。本步构建匹配条件，不把关联后的 counter 重新录制、聚合后再执行 rate。
5. **计算 CPU 与 memory。** CPU 对原始、未混合的 counter 执行 5 分钟 rate，至少两个有效样本，并与当前仍然存在且 fresh 的同一条原始来源相交；相交键必须保留完整采集来源身份。只按 UID/CID 相交可能把已经消失的旧抓取源 rate 重新带入。Memory 使用同一合格身份的近期 working set gauge。
6. **写入最终 record。** 只有来源唯一、身份匹配且原始输入合格时，投影到固定七标签并写入两条最终 record。内部辅助规则按依赖顺序位于同一 `ani-instance-metrics-v1` 规则组，统一 30 秒评估。

#### 6.1 Current / snapshot 必须避免的错误

`rate(raw[5m])` 在原始容器已消失后仍可能存在，不能把“窗口里还有 counter”当作当前运行身份。只要求其原始样本年龄 ≤45 秒，也不能替代当前 CID 匹配。

如果先录制 current/snapshot，再让最终规则读取它，必须证明：snapshot 只由本轮合格的原始输入生成，保留真实 UID/CID，并且下游不会通过默认 lookback 读回旧 snapshot 来继续录值。实现需对本轮 snapshot 的评估时间或等价的原始来源时间实施明确约束，并用同 UID 换 CID、消失与重复源 fixture 证明。**`timestamp(snapshot)` 刚刚刷新，不代表 snapshot 内部采用的原始 CPU、memory 或 KSM 身份也刚刚刷新。**

同组顺序评估只能统一规则的评估时刻，不能让 KSM 与 cAdvisor 的异步采集变成现实世界中的原子快照。过渡时刻允许出现缺失；不通过补零、延长窗口、复用上一 CID 或选最新 Pod 把缺口藏起来。

#### 6.2 重复采集与多运行身份是两种裁决

| 情况 | recording / 验收处理 | Obs 仅查询最终 record 时能表达的结果 |
| --- | --- | --- |
| 同一物理 UID/CID 由多个原始采集源提供 | 在投影前识别并抑制受影响输出；验收失败，记录当前冲突并修正重复抓取 | 该点没有合格 record 时为 `NO_DATA`，不能反推出 `AMBIGUOUS_SOURCE` |
| 规范 record 在同一查询时刻存在多个不同 UID/CID | 保留实际已经写入的各身份，不能求和或选择一份 | `null` 与 `AMBIGUOUS_SOURCE` |
| 输入 stale、标签缺失、CID 不匹配、CPU 样本不足 | 不写入该点业务值，诊断记录实际证据 | `NO_DATA`，不擅自判为停止或采集故障 |

原始冲突经抑制后，两条业务数值 record 无法事后还原原因。本期仅要求 `diagnostics.promql` 能诊断当前冲突，并保存本次验证证据，**不要求建设历史冲突存储**。只有实际实现并验证了内部 diagnostic record 等保存方式，才可承诺相应历史诊断能力；该项可选。内部记录不构成第三个公开业务指标，不新增 HTTP 指标接口或前端状态码。

不得用全局 `labeldrop`、`max`、`sum`、只取最新源或任意丢弃副本标签消除重复。规范标签仅在唯一性已经证明的结果上投影，不改变其他业务 metrics 的标签和采集行为。

### 7. 阶段 D：离线验证与应用顺序

#### 7.1 离线验证先于规则应用

使用与目标 Prometheus 版本相容的 promtool，保存以下命令结果：

```bash
promtool check rules deployments/observability/collection/rules/ani-instance.rules.yaml
promtool test rules deployments/observability/collection/tests/ani-instance.rules.test.yaml
```

语法通过不代表关联正确。固定输入时间、评估点和预期标签，至少覆盖：路径有效/拒绝样本；同名不同 UID 与同 UID 新 CID；旧 counter 残留、reset、一个/两个样本及 memory 独立可用；45 秒边界与 snapshot 重刷；同身份多个 job、KSM 重复或身份冲突；跨 cluster/tenant 防误关联；删除后历史；旧新 CID 的查询重叠。原始重复应抑制输出，当前 diagnostics 可核验；最终多身份不汇总。合成故障样本与真实脱敏样本分开标注，不冒充真实测试。

#### 7.2 人工应用顺序

1. 检查 context、cluster ID、对象 UID 和涉及字段仍与基线一致；变化时重新读取并修订 patch，保留他人变更。
2. 审查 README 字段差异与恢复值，对 `patches/` 中的实际 patch 和规则对象执行服务端 dry-run。
3. 手工应用 KSM allowlist patch，等采集组件恢复，再应用两个 ServiceMonitor patch；保存各步应用后值。
4. 用 `verify.sh` 检查 targets、TSDB 样本、15 秒间隔、cluster/owner 标签、UID/CID、时间戳和唯一性。失败则修复对应配置，暂停规则加载。
5. 应用 `ani-instance-metrics-v1`，确认被实际 selector 选中、完整规则加载且连续数次评估成功，再做真实联调。不得修改全局 selector。

README 使用实际对象参数给出逐步命令。例如字段 patch 先执行下面的 dry-run，再在该步骤明确移除 `--dry-run=server` 应用；变量从本次 profile 人工填写，不要求配置渲染程序：

```bash
kubectl --context "$obs_context" -n "$obs_namespace" patch "$obs_kind" "$obs_name" --type=json --patch-file "$obs_patch_file" --dry-run=server -o yaml
```

每步失败即停。

### 8. 阶段 E：真实验收与历史查询约束

在一个真实 cluster 上，用正常 Res 实例证明可信 scope 与七标签一致，UID/CID 对应实际 Pod 和主容器，owner 标签来源正确，CPU 为 cores、内存为 bytes；在同一时间点对照原始表达式和规范 record。返回 `mode: real` 不能作为证据。

通过正常 Res 生命周期验证停止/重启/重建及其缺失点，`verify.sh` 自身不操作业务实例。某个场景未在真实环境执行时，明确标为仅离线通过或待实测。授权和持久化范围仍有效、数据在保留期内时，已删除 Pod 的历史仍可查询；历史只读已录制数据，不 join 当前 KSM，不回填规则上线前的原始历史。

**查询 freshness 按每个历史评估时刻 `t` 检查。** 用 `time() - timestamp(record) <= 45` 或严格等价的过滤保留合格 record，不能与当前墙钟比较；`query_range` 矩阵点时间是查询评估时间，不能代替实际 record 样本时间。

每个指标、每个点独立裁决：0 个来源为 `null/NO_DATA`；1 个且有限非负则返回数值；多个不同 UID/CID 为 `null/AMBIGUOUS_SOURCE`。仅一个来源但 NaN、无穷或负数时为 `null/INVALID_VALUE`，多来源裁决优先。CPU warmup 不强迫有效 memory 同时缺失。

旧 CID 最后一个 record 与新 CID 首个 record 可能同时满足 45 秒 freshness，按多来源处理，不相加两个 trailing rate、不选最新，也不借当前 KSM 改写历史。当前规则健康属于部署验收；不为历史查询新增每请求 `/api/v1/rules` 检查，不让 Obs `/readyz` 依赖各数据源即时健康。

### 9. 恢复步骤与并发修改保护

README 逐对象列出恢复命令和 `patches/restore-*.patch.json` 的适用前提，人工恢复仅撤销本次字段：

1. 核对当前对象 UID，以及字段原值、本任务应用值、当前值。对象重建或相关字段被他人修改时暂停该项，先核对差异；保留其他字段的并发改动。
2. 优先恢复规则：本次新建的规则只在对象名与记录 UID 一致时删除；原先已有则恢复本次改动字段。
3. 恢复两类 ServiceMonitor 的 interval/relabel/时间戳字段，再恢复 KSM allowlist/args。数组按精确差异恢复，不能整个旧数组覆盖回去；等待采集组件恢复。
4. 验证原有 targets、采集与规则，记录已恢复、无需恢复和因并发修改待处理的项。

不将整个旧 YAML 重新 apply，不执行 Helm rollback，不删除 namespace、PVC、Prometheus 数据目录或他人的规则，不重启业务实例。移除新规则不删除已有 TSDB 历史，本任务不清理历史数据。恢复说明和 patch 是交付物，不要求实现回滚程序。

### 10. OBS-02 完成报告

报告链接手工命令、实际 patch、规则、fixture、promtool 和真实验证结果，逐项标注已实测、仅离线或待真实验证。写清 cluster 映射、对象/selector、来源 profile、两侧 UID/CID 规范化、owner 标签、字段差异，以及 freshness/current CID/重复来源的验收结果。恢复部分区分“说明和 patch 已准备”与“已执行演练”，有演练才写实测结论。

来源若不能提供严格匹配的 UID/CID 或原始 freshness 依据，应明确该 profile 未通过，提出使其满足契约的最小配置追加；不能退化为弱关联后标记完成。

### 11. 核对依据

以下定义不构成真实部署证据：[recording rules](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)、[rate 与 timestamp](https://prometheus.io/docs/prometheus/latest/querying/functions/)、[lookback/staleness](https://prometheus.io/docs/prometheus/latest/querying/basics/)、[relabel 与时间戳配置](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)、[KSM Pod 指标](https://github.com/kubernetes/kube-state-metrics/blob/v2.19.0/docs/metrics/workload/pod-metrics.md)、[kubelet cAdvisor 标签](https://github.com/kubernetes/kubernetes/blob/v1.35.8/pkg/kubelet/server/server.go#L1218-L1252)。运行环境版本和完整样本仍须由 OBS-02 核实。



源文件：[tasks/OBS-00-bootstrap.md](tasks/OBS-00-bootstrap.md)

## OBS-00：初始化项目与归档任务包

### 仓库 / 前置

- 来源：`zhangzhe-ctrl/ani-kratos-layout`，基线 `fd18422211c741dd5242d2992c3d307d522aa28c`。
- 目标：`zhangzhe-ctrl/ani-observability-service`；前置为可用的 Go、Git 和项目父目录。
- 本任务不修改 Gov、Res、Installer 或模板业务源码。

### 必读

- `../README.md`
- `../CONTRACT.md`
- `../ACCEPTANCE.md`（B01）
- 模板 `README.md`、`AGENTS.md`、`scripts/new-service`。

### 目标

按真实模板创建可独立构建的新服务，保留模板工程能力，将本任务包归档进新仓库，供后续任务使用。

### 固定改动

1. 使用模板基线的干净 checkout，建立指向该提交的专用命名分支；不要在 detached HEAD 下执行初始化。记录模板提交、分支和实际项目父目录。目标目录须不存在；已有项目时先核对初始化来源，不重复生成覆盖。
2. 先把任务专用环境变量 `OBS_WORKSPACE_DIR` 设置为实际项目父目录的非空绝对路径，再从模板目录执行：

```bash
go install github.com/go-kratos/kratos/cmd/kratos/v3@v3.0.0-20260626125723-668db92c2c00
make tools
./scripts/new-service github.com/zhangzhe-ctrl/ani-observability-service --parent "$OBS_WORKSPACE_DIR"
```

`make tools` 安装模板固定版本的 Buf；Kratos CLI 单独安装。父目录使用执行环境提供的工作区真实路径，不照抄其他机器路径。
3. 新仓库根 Go module 固定为 `github.com/zhangzhe-ctrl/ani-observability-service`；服务名固定为 `ani-observability-service`。沿用脚本的命名、来源记录和独立 Git 初始化结果。
4. 将本包目录内容完整复制到新仓库 `docs/specs/observability-phase1/`，保留相对目录结构。必须包含公共约定、任务文件、contracts、examples 和报告模板，不能仅放外部路径或链接。后续任务以该副本为入口。
5. 保留模板配置、Kratos 生命周期、admin 健康检查、自身 metrics、脱敏日志、生成验证和供应链检查。采用手工依赖组装，不增加 Wire、ProviderSet、数据库、缓存或队列。`internal/server/observability.go` 仍表示进程自身遥测。
6. 建立 `internal/{biz,data,service}/{monitoring,logs,alerts}` 模块目录。日志、告警只放 `doc.go` 职责说明，不注册空 RPC；监控业务实现留给后续任务。先不建立假的 API 依赖或 import。
7. 核对 `cmd/ani-observability-service` 和构建产物名称。OBS-01 的 API 真实发布前，根模块保持可独立构建，不 require 一个尚不存在的 API 版本。

### 验收

- 初始化来源与目标 module、服务名正确，生成文件中无错误模板 import。
- 新服务执行 `make generate`、`make verify`、`make build` 成功；记录实际命令与退出码。
- 模板 admin 健康检查可访问；不声称本任务已经接通业务指标。
- 本包在仓库中完整归档，相对引用有效；日志、告警无伪实现。
- 后续发布 API 使用新仓库的真实独立远端，不能把提交推到模板远端。

### 交付

交付初始化提交、来源记录、模块目录、归档清单及验证结果。说明当前根模块尚未消费 API，供 OBS-01 发布后继续实施。

按 `../TASK-REPORT.md` 的格式提交任务报告。


源文件：[tasks/OBS-01-api-module.md](tasks/OBS-01-api-module.md)

## OBS-01：建立并发布独立 API 子模块

### 仓库 / 前置

- 仓库：`ani-observability-service`。
- 前置：OBS-00 完成；已具备新仓库真实远端。

### 必读

- `../README.md`
- `../CONTRACT.md`
- `../contracts/api/` 中的两份固定协议。
- `../ACCEPTANCE.md`（B02–B04）

### 目标

在同仓库建立独立、可消费的 API Go 子模块。Gov 和服务实现共享契约，API 不依赖服务实现。

### 固定改动

1. 新增 `api/go.mod`、`api/go.sum`，module 固定为 `github.com/zhangzhe-ctrl/ani-observability-service/api`，Go 基线 `1.26.7`。直接 runtime 依赖固定为 `google.golang.org/grpc v1.82.1`、`google.golang.org/protobuf v1.36.11`；保留 tidy 得出的必要间接依赖，不复制根模块依赖清单。
2. 按固定协议落地 `api/observability/target/v1/target.proto`、`api/observability/monitoring/v1/monitoring.proto` 及生成代码。package、字段号、枚举和 `go_package` 以 CONTRACT/contracts 为准；MonitoringService 只注册 QueryRange、QueryLatest。
3. API 只含 proto、标准 Go protobuf/gRPC 生成代码及必要契约测试。不加入 Kratos HTTP/error/client 包装、Prometheus 查询、配置加载或根服务 import。配置 protobuf 继续属于根模块。
4. 根 `buf.yaml` 保留 `path: internal`，增加 `path: api`。保留现有 `buf.gen.yaml` 只生成内部配置，新增 `buf.gen.api.yaml`：

```yaml
version: v2
inputs:
  - directory: api
plugins:
  - local: [go, run, google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11]
    out: api
    opt: paths=source_relative
  - local: [go, run, google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1]
    out: api
    opt: paths=source_relative
```

不得设置 `clean: true`，以免清空同目录 proto/go.mod。不得关闭 Buf lint 规避目录或枚举问题。
5. `Makefile` 增加 phony API 生成入口，`make generate` 同时生成配置与 API。`scripts/verify-source` 显式执行两份生成模板，并把 API 纳入现有全树生成前后比对。根 `go generate ./...` 不代表已覆盖嵌套 module。
6. 扩展 make verify/CI，显式对 API 与根模块分别执行 tidy-diff、test、vet、mod verify；生成检查覆盖两者。修改服务 VERSION 的 tag 选择，使用 `git describe --tags --match 'v[0-9]*' --always --dirty`，避免 `api/*` tag 被当作服务版本。
7. 先提交并发布 API 可引用版本。首个 tag 使用 `api/v0.1.0`，Go require 版本为 `v0.1.0`；禁止手造版本、移动已有 tag。记录完整提交、tag/module version。
8. 从工作区外、`GOWORK=off` 且无本地 replace 的消费者验证 API 可下载和编译。本地联调可使用 go.work，但不能替代发布验收。
9. 本任务结束时根模块尚未实际 import API，不强加会被 tidy 移除的 require。把真实 API 版本交给 OBS-05、OBS-07；OBS-05 首次实现 gRPC service 并真实导入时，再固定根 go.mod/go.sum。OBS-04 只实现 domain/data，不通过无意义空白 import 保住依赖。

### 验收

- 在 API 目录使用 GOWORK=off，分别执行 `go mod tidy -diff`、`go test ./...`、`go vet ./...`、`go mod verify` 通过；根模块原有验证仍通过。
- 重新生成无差异；API 不导入根模块或服务实现。
- 干净消费者通过真实版本导入两个 API 包；包所属 module 为 `.../ani-observability-service/api`。
- 不要求 Gov 整个依赖图移除其既有 Kubernetes/OTel 依赖；只验证本 API 边界。
- 发布版本可获取后才标记完成；仅在 go.work 中编译成功不算发布。

### 交付

交付 proto、生成代码、两个 Go module 的验证结果、生成脚本改动和可消费 API 版本记录。向后续任务提供精确 `go get .../api@<版本>` 参数，不使用 `@latest`。

按 `../TASK-REPORT.md` 的格式提交任务报告。


源文件：[tasks/OBS-02-manual-collection.md](tasks/OBS-02-manual-collection.md)

## OBS-02 手工补齐采集与 recording rules

### 仓库与前置

仓库：ani-observability-service。前置：OBS-00、可使用的实际集群、Res 登记的集群 ID。可以与 OBS-01、OBS-03 并行，不修改 ani-installer。

必读：`../README.md`、`../CONTRACT.md` 第 3、6 节、`../MANUAL-METRICS.md`、`../ACCEPTANCE.md` 的 C 类用例。

### 目标

人工补齐现有 Prometheus、cAdvisor、KSM 的采集字段与规则，产出两个身份可靠的真实规范时序，并保留可再次应用、验证和恢复的文件。

### 固定改动

1. 在 `deployments/observability/collection/` 按 MANUAL-METRICS 的文件清单交付。记录真实 Prometheus、Operator、KSM 版本、namespace、资源名、selector、containerd/cgroup 模式和 Res cluster_id。原始样本必须来自环境，不能用虚构样本宣称适配成功。
2. 保存本次要改的原字段；生成可查看的字段 patch。只补 KSM ANI label allowlist、cAdvisor/KSM 采集身份标签与 15s 间隔及独立 PrometheusRule，保留已有 args、relabel 和其他采集项。手工应用到实际资源，不升级/重装整套栈，不创建新安装子系统。
3. 全部原始输入的 ani_cluster_id 与 Res、Obs datasource 一致。KSM 同时导出可信租户和实例标签。身份取自同 Pod UID 的 KSM 信息；从 cAdvisor id 解析 UID/CID，与 KSM 当前 CID、workspace、running 匹配，按固定规范化规则处理，不退化为名称关联。
4. 核实 CPU 维度，选择唯一整容器 counter，逐原始 counter 执行 rate(...[5m])；内存使用 working set。原始当前序列和 KSM 输入在录制时均须不超过 45s，旧 CID 的 5m 残留不能继续记录。
5. 去掉原始 id/job/instance/endpoint 前验证唯一性，重复源要可定位并使验收失败，不能 max/sum/labeldrop 掩盖。若过滤重复后无规范值，查询只能 NO_DATA；运行来源在查询点实际重叠才是 AMBIGUOUS_SOURCE。少量辅助规则及诊断要随包测试，不增加公开指标。
6. 固定两个 record：ani_instance_cpu_usage_cores、ani_instance_memory_working_set_bytes；最终七个标签按合同。PrometheusRule 与规则组均为 ani-instance-metrics-v1，30s 求值。按实际 ruleSelector 放置标签与 namespace。
7. 完整表达式先用真实样本制作 fixture，再用与目标 Prometheus 匹配的 promtool 检查和测试；从 CR 的 spec 提取纯规则文件后运行，不能把 Kubernetes 外壳当 promtool 规则输入。通过后人工应用、验证规则加载与实际数值，归档恢复本次字段的步骤。

### 验收

- C01–C10 按 ACCEPTANCE 执行：原始指标与两个 record 实际存在、七标签正确、字段 patch 保留旧配置、规则健康、有界负载可解释变化。
- fixture 覆盖两种已支持路径、非法 id、不足两样本、45s 边界、旧 CID 残留、重复源、同名 Pod 换 UID、KSM 信息过期和辅助 record 新鲜度。
- 不以 /metrics 200、YAML 可解析、Prom 查询空数组或 fixture 通过代替真实采集成功。
- 没有实际集群访问时可交付准备好的资料和离线测试，但此任务不能标记完成；列出所缺输入，其他独立任务继续。

### 交付

提交实际 patch/规则、脱敏原始样本、promtool 结果、手工应用记录、真实时序查询、冲突定位和恢复说明。环境密钥只记录引用。结果按 `../TASK-REPORT.md` 汇报，向 OBS-08 交付真实 datasource/cluster_id、规则启用时间、测试实例和证据路径。


源文件：[tasks/OBS-03-resource-scope.md](tasks/OBS-03-resource-scope.md)

## OBS-03：Res 实例监控范围解析

### 仓库 / 前置

- 仓库：`ani-resource-service`；基线 `f7459f7ae71c5573f53a50c5d20908b129c17e0c`。
- 前置：本包公共契约已冻结且可读取；本任务可先行，不要求新服务已部署。

### 必读

- `../README.md`
- `../CONTRACT.md`
- `../contracts/res-scope.md`
- `../ACCEPTANCE.md`（R01–R05）
- Res `api/instance/v1/instance.proto`、`internal/service/instance/service.go`、`internal/biz/instance/types.go`、`internal/server/instance_identity.go`。

### 目标

为 Gov 提供一次只读、受租户身份约束的实例范围解析。返回持久化业务位置，支持读取已有历史，不依赖实例当前 Pod 或运行状态。

### 固定改动

1. 在现有 `api/instance/v1/instance.proto` 增加：

```proto
rpc ResolveInstanceMetricScope(GetInstanceRequest)
    returns (InstanceMetricScope);

message InstanceMetricScope {
  string tenant_id = 1;
  string instance_id = 2;
  string cluster_id = 3;
  string namespace = 4;
  string container_name = 5;
}
```

复用现有 GetInstanceRequest，使用本仓已有生成方式更新生成文件，不修改既有字段号和 RPC 行为。
2. 新增 `internal/service/instance/metric_scope.go`，在已有 `Service` 上实现 ResolveInstanceMetricScope。该类型实际定义于 `internal/service/instance/service.go`，持有 `repo biz.Repository`；不要新建第二个实例 service 或寻找不存在的 `instance.go`。
3. 实现顺序：校验请求非空及 ID 格式 → `biz.RequireTenant(ctx, req.TenantId)` → `s.repo.Get(ctx, req.TenantId, req.InstanceId)` → 校验持久化范围 → 返回结果。复用现有错误映射与租户约束。
4. 从记录的 TenantID、ID、Spec.ClusterID、Spec.Namespace 取值，container_name 使用 `biz.Container`（workspace）。不得从当前服务配置重建丢失的位置，不接受调用者提交 cluster、namespace 或 Pod。
5. 不要求 Running/Ready、不调用 Observe、终端、日志、运行时 target 或 Kubernetes。普通已删除记录只要保存完整 Spec，仍返回范围；delete-first tombstone 或范围缺失返回 FailedPrecondition，reason 为 `INSTANCE_METRIC_SCOPE_UNAVAILABLE`。新 reason 使用可机器读取的状态详情，不仅写入 message 供 Gov 解析文本。
6. 不新增数据库字段、SQL 查询、写入、状态机、配额或删除行为。不使用 UpdatedAt 作为历史终点，不返回当前 PodName/PodUID。
7. 在 `internal/server/instance_identity.go` 精确允许方法 `/instance.v1.InstanceService/ResolveInstanceMetricScope`，保持现有 Gov mTLS、单值身份元数据和请求租户匹配检查。
8. 更新 `docs/specs/governance-calls.md`，写清请求、返回来源、历史范围语义与新 reason。形成包含新 RPC 的真实可消费 Res 提交版本，交给 OBS-07；不拆迁 Res 现有根/API 模块布局。

### 验收

- 合法租户读取自己实例，返回持久化 cluster/namespace 和 workspace。
- 无身份、身份租户与请求不一致、其他租户实例均无法获取范围。
- 不存在实例沿用现有 NotFound；正常删除但 Spec 完整可解析；缺范围返回指定错误。
- 验证该 RPC 只有持久化读取，无运行时访问或业务写入；实例不在 Running/Ready 时仍可读取合法范围。
- 新方法经过真实 gRPC 身份中间件，不仅直调 Go 方法测试。
- 本仓生成一致性、相关服务/身份测试和既有构建检查通过。

### 交付

交付源 proto、生成文件、只读实现、允许方法表、文档及测试。报告新 RPC 所在完整提交及 Gov 应引用的真实 module version，不仅给本地分支名。

按 `../TASK-REPORT.md` 的格式提交任务报告。


源文件：[tasks/OBS-04-prometheus-adapter.md](tasks/OBS-04-prometheus-adapter.md)

## OBS-04 Prometheus 查询适配

### 仓库与前置

仓库：ani-observability-service。前置：OBS-01 已形成可消费 API 契约；实现可使用 HTTP fixture，不以此替代 OBS-02 的真实采集验收。

必读：`../README.md`、`../CONTRACT.md` 第 3、6、7、9 节，以及 `../ACCEPTANCE.md` 的 D 类用例。

### 目标

实现只按可信范围查询两个规范 record 的数据源层，限定地址、表达式、返回规模和超时；不暴露原始 PromQL 或访问 Kubernetes。

### 固定改动

1. 在 `internal/biz/target/` 定义所需领域范围，在 `internal/biz/monitoring/` 定义数据层需要的查询/原始时序接口；仅支撑本期两个用例，不建通用插件框架。此任务不为了添加依赖而无意义导入 API；根模块首次真实 API import 与 pin 在 OBS-05 完成。
2. 实现 `internal/data/monitoring/query_builder.go`：metric code 只能映射到固定两个 record；scope matcher 精确包含 cluster、tenant、instance、namespace、workspace；保持 UID/CID 供后续来源裁决。对每一求值点限制 record 年龄 45s，等价于 record and (time()-timestamp(record)<=45)，不得使用现在的 KSM/Pod 或只对最后一点判断年龄。
3. 实现 `internal/data/monitoring/prometheus.go`：通过静态配置中 cluster_id 唯一定位 datasource，未知源返回 SOURCE_NOT_CONFIGURED，不回退、不遍历全部源。address、证书、token 仅来自服务配置；HTTP 请求传入父 context。
4. range 使用 /api/v1/query_range、latest 使用 /api/v1/query；明确传时间、step、timeout 与 limit=129。两个查询共用一次 4s 批次预算，不重试。复用有界 HTTP client，正确关闭 response body 和生命周期资源。
5. HTTP 层有界读取单响应最多 8MiB，不能先无界读再检查。每指标最多 128 条原始返回 series，检测 129 即 RESULT_TOO_LARGE，不能静默截断。只接受所需 matrix/vector 类型、合法 timestamp 和数值表示。
6. 保留七项身份与值的 presence；验证 scope 标签、不缺 UID/CID；返回损坏数据、重复点或越界点时明确错误。NaN/Inf/负值交由业务层输出 INVALID_VALUE，不直接把它们编码为 JSON 数字。
7. 按合同分类非 2xx、Prom status=error、超时、warnings、畸形 JSON、响应过大等；不能将请求失败变成空结果。结构正确的空结果是合法输入，交由 OBS-05 补 null 网格。

### 验收

- D01–D07：固定表达式可信过滤、历史 freshness、按集群选源、matrix/vector、limit、体积上限、错误和取消。
- HTTP 级测试验证端点、参数、deadline 与 reader 上限，覆盖恶意/损坏上游；范围/最新值的无数据作为成功空输入传出。
- 构造跨租户标签、未知 datasource、重复点和超大响应，均有确定错误，不泄漏或忽略。
- 真实 Prom 测试留给 OBS-08；本任务只声明完成适配器及其可重复验证。

### 交付

提交接口、adapter、固定 query builder、必要测试和 datasource 配置字段说明；给 OBS-05 明确输入/输出、错误与 cleanup。按 `../TASK-REPORT.md` 交付实际命令和结果。


源文件：[tasks/OBS-05-monitoring-usecases.md](tasks/OBS-05-monitoring-usecases.md)

## OBS-05 监控用例与结果规范化

### 仓库与前置

仓库：ani-observability-service。前置：OBS-04 数据层、OBS-01 API 真实版本。

必读：`../README.md`、`../CONTRACT.md` 第 4–7、9 节、`../contracts/api/`、`../ACCEPTANCE.md` 的 N 类用例。

### 目标

实现 QueryRange/QueryLatest、固定指标定义、完整时间网格和逐点缺失语义，提供可注册的 gRPC service。本任务首次实际使用已发布 API，并固定根模块依赖；网络信任与部署在 OBS-06 完成。

### 固定改动

1. catalog.go 仅登记 cpu.usage_cores/cores 和 memory.working_set_bytes/bytes、对应 record、口径 v1 与 workspace 范围，不新增目录查询 API 或动态配置。
2. query.go 校验 target、指标数量/重复/未知、时间顺序/整秒/未来、24h、step 30–3600 且为 30 倍数、3000 点上限。内部 step 不允许 0；公共省略默认由 Gov 处理。重复 code 拒绝，不能自行去重。
3. QueryRange 保持 start+n*step≤end 的网格，不对齐、不插值；同请求不同指标的点时间完全一致。QueryLatest 一次捕获整秒 evaluated_at，两项调用复用同一时刻，每项返回一个点。
4. normalize.go 对每指标每点执行来源裁决：0 个为 null/NO_DATA；1 个有限非负值直接返回，0 保留；多 UID/CID 来源为 null/AMBIGUOUS_SOURCE；1 个 NaN、Inf 或负数为 null/INVALID_VALUE。先后来源不在同一时刻重叠时正常拼接。
5. series 顺序按请求；正常点不带缺失原因，所有点有效 OK、部分有效 PARTIAL、无有效值 NO_DATA。空 Prom 输入仍产出全部所需 series 与 null 网格，不能省略指标或伪装成 0。
6. 一次请求最多并发两条 Prom 查询，共用 4s 预算；一项上游失败取消另一项并整批失败。每副本总并发16、每租户4，按请求计数，超限立即429对应错误。所有成功/失败/取消路径释放令牌，不增加重试或排队系统。
7. 实现 `internal/service/monitoring/service.go` 的两个 RPC 转换，检查 optional double presence，按合同处理枚举与错误 reason。根 go.mod/go.sum 此时引用 OBS-01 真实 API 版本，不用本地 replace。不得把领域逻辑塞进生成文件。

### 验收

- N01–N09：零值/缺失、全部空、部分空、来源前后更换/重叠、NaN/Inf/负值、未对齐 end、同一 latest 时刻、非法查询、批次失败、租户并发和取消资源释放。
- 用固定 clock/受控上游验证时间和并发行为；不依赖长时间 sleep 或仅对实现细节作镜像断言。
- gRPC 返回中 present 0 与 absent value 可区分，未知枚举/无效 scope 被拒绝；全空网格点数准确。
- GOWORK=off 的根模块验证通过，证明第一次 API pin 可独立消费。

### 交付

提交 usecase、catalog、normalize、gRPC service 和必要测试；向 OBS-06 交付构造依赖、已固定 API 版本和关闭职责。按 `../TASK-REPORT.md` 汇报。


源文件：[tasks/OBS-06-trust-and-deployment.md](tasks/OBS-06-trust-and-deployment.md)

## OBS-06 受信 gRPC 与服务部署

### 仓库与前置

仓库：ani-observability-service。前置：OBS-05 监控 service。实际部署需要真实 namespace、datasource、镜像和证书引用。

必读：`../README.md`、`../CONTRACT.md` 第 8–10 节、`../ACCEPTANCE.md` 的 S 类用例。

### 目标

将监控模块接入模板的配置、启动与关闭链路，仅向受信 Gov 提供两个 gRPC 方法，并交付可运行服务。

### 固定改动

1. 扩展 `internal/conf/v1/` 的 monitoring、governance 配置，保持原有字段编号与生成链。cluster_id 重复/非法、address 缺失、证书或凭据不可读时启动失败；数据源临时离线按请求失败，不把全部 readiness 绑定单源健康。
2. 在 `internal/server/governance.go` 构造 TLS≥1.3 监听，验证客户端 CA 与精确 ani-governance 身份；校验三个单值受信 metadata 和 target tenant 一致。允许方法仅 MonitoringService 的 QueryRange/QueryLatest，reflection 关闭。
3. 服务端证书身份固定 ani-observability-service；文档明确 Gov 必须验证该身份。不要复用 Res 旧的 ani-network-service 名称，也不允许任意同 CA 客户端。
4. 在 `cmd/ani-observability-service/monitoring.go` 实现 openMonitoring，接入现有手工 wiring/cleanup；注册监控 service，传播6s父RPC期限、4s Prom预算、请求取消；退出时关闭连接/监听。保留模板 admin、健康、自身 metrics/log，业务逻辑不写入进程遥测文件。
5. 交付 Dockerfile 和 `deployments/observability/{README.md,configmap.yaml,deployment.yaml,service.yaml,secret.example.yaml}`，配置键/端口遵循合同。一期1副本、私网ClusterIP、admin探针；无业务数据库、队列、PVC或查询Kubernetes的额外RBAC。
6. secret.example 只提供键名和挂载方式；实际凭据使用已有证书与Secret流程。真实配置、镜像tag/digest、发布命令和应用结果写入报告。普通租户工作负载不能绕过受信查询入口；检查实际网络与mTLS边界。若发现直连底层查询入口的路径，以现有网络能力补最小访问策略，归档networkpolicy.yaml；只覆盖本次入口，保留采集、平台访问和探针。
7. logs/alerts 各层仅 doc.go，无空壳业务RPC、数据库表或后台任务。

### 验收

- S01–S06：有效Gov证书可调用；无证书、错CA、同CA错身份、伪造/多值/缺失元数据、租户不一致和未知方法拒绝。
- 配置错误可读且不泄密，单源查询故障不会将其他能力误判为全服务故障。
- 根服务构建、生成、audit按既有门槛执行，真实部署启动和探针通过；受信客户端调用可达配置的Prometheus，缺失业务时序仍按正常NO_DATA语义处理。
- 实际部署或证书输入缺失时明确未完成项；只构建镜像/manifest不等于部署验证完成。

### 交付

提交配置proto与生成物、监听/装配/cleanup、部署文件和安全验证。向OBS-07交付实际gRPC地址、CA与客户端证书引用、服务端身份及两个完整方法名。按 `../TASK-REPORT.md` 汇报。


源文件：[tasks/OBS-07-governance.md](tasks/OBS-07-governance.md)

## OBS-07：Gov 监控入口与后端调用

### 仓库 / 前置

- 仓库：`ani-governance`；基线 `90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812`。
- 前置：OBS-01 API 已可消费、OBS-03 Res RPC 已可消费、OBS-06 已完成可观测查询服务和 Gov mTLS 验证。沿用本包约定，不开发前端。

### 必读

- `../README.md`
- `../CONTRACT.md`
- `../contracts/gov-http.md`、`../contracts/res-scope.md`
- `../ACCEPTANCE.md`（H01–H09、B04–B05）
- `../examples/`
- 现有实例 HTTP、service、授权、超时与 Res client 实现。

### 目标

完成 Gov 认证授权 → Res 可信范围解析 → Observability gRPC 查询 → 公共 HTTP 结果的完整协调，保持固定公共合同。

### 固定改动

1. 在 `api/protos/instance/service/v1/instance.proto` 增加公共 DTO；在 `api/protos/admin/service/v1/i_instance.proto` 增加 QueryInstanceMetrics、GetInstanceLatestMetrics：

```text
POST /api/v1/instances/{instance_id}/metrics/query
GET  /api/v1/instances/{instance_id}/metrics/latest
```

消息名与字段号逐项采用 contracts/gov-http.md；POST body 保留 data 外壳，annotation 使用 `body: "*"`，成功固定 200。Latest 请求复用现有 GetInstanceRequest，无 body/查询参数，固定 CPU 后内存。
2. instance_id 只取路径；公共 body 拒绝 instance_id、租户、cluster、namespace、Pod、PromQL、数据源地址，以及未知字段、重复 JSON 字段和额外查询参数。body 超过 64 KiB 返回 413。区分 step 省略与显式 0，默认值及限制按 CONTRACT。
3. 下列清单相对 `app/admin/service/internal/`：
   - 新增 `service/instance_metrics.go`，实现两个方法；修改已有 `service/instance_service.go` 的字段与构造注入。
   - 新增 `server/instance_metrics.go`，注册专用同步查询 handler，并接入现有 RegisterInstanceHTTPServer。
   - 新增 `data/observability_client.go`、`data/observability_client_security.go`。
   - 修改已有 `data/instance_client.go`，调用 ResolveInstanceMetricScope；同步现有 InstanceOwnerClient 接口及测试替身，保留原方法行为。
   - 修改 `data/instance_authorization.go`、`server/instance_timeout.go`。
4. handler 设置生成的 Operation，调用现有 `ctx.Middleware`，显式返回 200；确保生成注册器与薄 handler 不重复注册路由。不得使用通用非 GET 的 202 行为；不要求幂等键、expected_version，不进入操作受理、配额或写入流程。
5. service 先调用现有实例 scope/权限逻辑，再带可信租户调用 Res。核对返回的 tenant_id、instance_id 与请求一致，其他范围字段符合 CONTRACT，再构造内部 TargetScope 调用 Observability。禁止可观测服务反向解析业务对象，禁止信任外部同名身份 header。
6. 新客户端复用现有 Gov 身份约定，重新构造单值 tenant/actor/request-id 元数据，使用独立 `ANI_OBSERVABILITY_*` 配置。TLS 最低 1.3，验证新服务身份 `ani-observability-service`，客户端身份 `ani-governance`。不得复制 Res 旧服务 SAN 或 `ANI_INSTANCE_*` 配置。
7. 修改 `app/admin/service/cmd/server/wiring_ent.go` 的实际手工组装入口，构造、注入新客户端并登记关闭清理。父 context 取消向下传播；Gov HTTP 总预算 10 秒、Res 2 秒、Observability 6 秒。只调整上述精确方法/路由，保留其他接口的既有预算。
8. 公共响应保持 data → series → points，显式固定下划线 json_name。公共点 value 使用 google.protobuf.DoubleValue；转换内部 optional double 时检查 presence：缺失为 null，有效零为 0。有效点省略 gap_reason。status/原因映射按 CONTRACT，OpenAPI 标记 nullable，不手改生成文件。
9. 两接口精确绑定 `instance:read`。更新 `scripts/ops/sql/register-instance-permissions.sql`；按现有 `admin sync-apis --dry-run`、`admin sync-apis` 和权限注册脚本完成目标环境同步，记录结果；不扩大既有角色权限。
10. 监控查询 reason 使用专门错误映射，不能把 ResourceExhausted 映射为日志连接上限，也不能把超时/上游故障转为空曲线。已有认证和实例错误沿用现有 envelope。
11. go.mod/go.sum 固定 OBS-03 的真实 Res 版本及 OBS-01 的 `.../ani-observability-service/api` 版本。Gov 不新增依赖可观测根模块，不复制内部 proto，不靠本地 replace 验收，也不为匹配 API 而降级 Gov 已有 runtime。按本仓 make api/生成检查更新公共生成物和 OpenAPI。

### 验收

- 两接口经真实 HTTP handler 执行认证、中间件、Operation、instance:read，成功为 200。
- 验证 Gov 先调用 Res 解析范围，再由 Gov 调用 Observability；两指标编码、单位、顺序、时间及状态满足 CONTRACT。
- HTTP 编码测试验证 null、0、普通值、data 外壳、下划线字段及有效点无 gap_reason；不能只测试 DTO。
- 覆盖非法/重复字段、跨租户、无读取权限、范围不可用、并发限流和查询超时；错误映射正确，取消下游调用。
- GOWORK=off、无本地 replace，真实固定版本可下载并通过本仓生成、相关测试和构建。
- API 权限同步已执行并有结果；未同步不能声称已完成对外授权接入。最终真实 Prometheus 负载验收按后续端到端任务执行。

### 交付

交付公共 proto/生成物、handler、服务协调、客户端安全与清理、权限和超时改动。报告依赖版本、HTTP 样例、测试结果及权限同步记录，不提交真实凭据。

按 `../TASK-REPORT.md` 的格式提交任务报告。


源文件：[tasks/OBS-08-backend-e2e.md](tasks/OBS-08-backend-e2e.md)

## OBS-08 真实后端链路验收

### 仓库与前置

仓库：ani-observability-service、ani-governance、ani-resource-service；不修改 ani-installer。前置：OBS-02真实采集通过、OBS-07 Gov接入完成、实际认证和专用验收实例可用。

必读：`../README.md`、`../CONTRACT.md`、`../ACCEPTANCE.md` 的E类及全部遗留实测项、`../MANUAL-METRICS.md`。

### 目标

从Gov入口查询真实实例，证明范围、最新值、权限、运行身份与失败语义完整闭合。无需前端参与。

### 固定操作

1. 记录三个实现仓库的实际commit、API/Res依赖版本、部署镜像、Res cluster_id、Prom地址引用、规则启用时刻和数据保留范围。先确认真实实例持久化范围与Pod标签一致。
2. 通过真实Gov身份执行两个公共HTTP接口，保存完整脱敏请求/响应、时间范围、step和request_id。CPU规格实例必须验证；有已可用GPU规格实例时复用它验证CPU/内存，不扩展GPU指标或为本任务新增调度逻辑。
3. 对专用验收实例施加有界CPU、内存负载，观察基线、负载、恢复。内存触碰实际页并控制在现有限额内；CPU按5m rate解释过渡，不要求瞬间恢复。用完全相同scope、求值时间和step的固定Prom查询对照Gov结果，允许合理浮点容差，禁止拿不同求值时刻比较。
4. 用同一业务实例覆盖容器CID更换和Pod UID更换；记录更换前后UID/CID和时间。新旧时序可在一条业务曲线中按时间解释，重叠时不能相加。实例停止/正常删除后持久化范围仍可解析时，旧时间段仍可查询；不要求返回已物理清理的业务记录。
5. 验证跨租户、无instance:read、伪造scope/header、错误内部证书、未知/重复code、body/path冲突、未来或过大区间等。前端提供PromQL/namespace/datasource字段必须被拒绝。
6. 故障验证包括可控数据缺失、Prom连接失败/超时、客户端取消、并发与规模上限。服务错误不能伪装成200空数组；真实0与缺失null的HTTP编码由H类测试及本次样本核对。
7. 当前规则异常时查询已有历史不得被额外健康检查阻断。破坏性故障只作用于专用验证资源或受控连接，保留变更和恢复记录，完成后恢复验证现场；不要停整个共享Prometheus来制造错误。
8. 难以稳定在真实环境制造的Nan、精确多源重叠、重复HTTP点等由既定fixture测试证明，并明确测试层级；核心数据、Gov链路、授权和运行身份更换不能全部以mock替代。

### 验收

- E01–E07和所有标记“真实”的相关条目完成，证据能沿request_id/实例/时间追到规范record。
- 记录CPU窗口、规则间隔、缺口、规则启用前无历史等合理边界，不能把“有曲线”当唯一通过条件。
- 专用实例操作遵循既有生命周期和配额，不新增政策或绕开限制。
- 缺少环境/证书/真实实例时列出已完成与剩余实测，不能宣布整包完成；继续收齐无依赖的验证。

### 交付

归档真实范围和最新请求响应、Prom对照、负载记录、UID/CID更换与历史证据、授权和故障结果、现场恢复记录。只保存脱敏材料。按 `../TASK-REPORT.md` 汇报，并将未通过项交给OBS-09明确处理。


源文件：[tasks/OBS-09-delivery.md](tasks/OBS-09-delivery.md)

## OBS-09 版本与交付收口

### 仓库与前置

仓库：ani-observability-service、ani-resource-service、ani-governance。前置：OBS-00至OBS-08的实际产物与报告。

必读：`../README.md`、`../CONTRACT.md`、`../ACCEPTANCE.md`、所有任务报告。

### 目标

使本期代码、依赖、手工采集配置和真实验收结果可重新获取与复核，明确可以交付的完整后端能力。

### 固定操作

1. 核对任务产物与固定合同：两个指标、两个监控RPC、两个GovHTTP、一个Res范围RPC，日志/告警只有边界。确认没有把前端、任意PromQL、Installer改造、配额/调度或额外存储混入本期。
2. 填写 `examples/version-lock.example.yaml` 对应的实际版本记录，归档到报告目录。每个版本能远端获取，API tag和服务tag规则正确；根服务与Gov实际引用已发布API，Gov引用含范围RPC的Res版本。不能保留本地replace或只在go.work生效的依赖。
3. API子模块与根模块分别执行GOWORK=off的tidy-diff/test/vet/mod verify及相应生成和构建；Gov、Res执行现有受影响范围质量门槛。完整audit按各仓库要求执行；不为了通过删除门槛或忽略真实失败。核对API没有反向依赖根模块。
4. 在干净可复现的工作环境验证生成物一致、部署文档输入完整、镜像digest/版本对应提交。已充分验证的项复用报告，不无目的重复全套压测或新建测试框架。
5. 核对手工采集交付物包含实际应用的字段patch、recording rules、原始fixture、配置/版本清单、验证命令、变更前字段与恢复说明。资料能说明日后如何同步至自动化部署，但本期不修改ani-installer。
6. 核对Gov API同步和权限注册在实际部署完成，不只检查代码。核对TLS身份、配置引用与关闭清理、缺失/错误/超时语义和真实数据证据。
7. 形成一期交付说明：如何启动、如何人工应用采集、如何执行Gov请求、怎样解释单位/时间/缺失、怎样定位故障、后续日志/告警模块接入位置。环境数据和敏感凭据分离，示例不能冒充实测。
8. 汇总ACCEPTANCE每项通过/失败/未执行及证据。具体阻碍未解决时明确部分完成，不能将“全部代码已写”写成后端链路已闭合。

### 验收

- B01–B06及所有必需真实验收项通过，版本与证据互相一致。
- 另一位执行者按文档、真实版本与现有凭据引用可以重建服务并复核同一数据范围。
- 当前用户可审阅提交、文件、部署和测试结果；不擅自把任务报告发到外部聊天、邮件或无关仓库。

### 交付

按 `../TASK-REPORT.md` 给出最终报告，附三个实现仓库commit、API/Res真实版本、服务镜像、实际手工配置、真实Gov/Prom对照和未完成项。只有真实采集、后端调用、权限与历史语义全部满足合同，才标记本期完成。


源文件：[ACCEPTANCE.md](ACCEPTANCE.md)

## 一期验收清单

本清单的状态在任务报告中填写。本任务包交付时所有项目均未执行；示例数据、源码审阅和离线 fixture 不能替代要求真实环境的条目。

测试用于证明可信范围、历史身份、数据语义和边界，不为目录占位等低影响改动增加镜像测试。已完成的有效验证可复用，不为收口无目的重跑所有场景。

### 1. 构建与交付 B 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| B01 | OBS-00 | 从固定模板以实际命名分支初始化，模块/命令名正确；手工装配与现有健康能力正常；根模块在 API 首次发布前可独立构建。 |
| B02 | OBS-01 | API 子模块独立 tidy-diff、test、vet、mod verify；生成代码存在；API 版本已真实可获取，记录真实 tag/commit 与消费者版本。 |
| B03 | OBS-01 | 原配置与 API 两份生成链均执行，verify-source 能发现生成漂移；没有 clean:true 删除输入；API 不依赖根服务。 |
| B04 | OBS-01、05、07 | API tag 不污染服务 VERSION；根服务/Gov 从实际 API 版本构建，不依赖本地 replace/go.work；Gov 的既有其他业务依赖不作为新增 API 隔离失败依据。 |
| B05 | OBS-06、07、09 | GOWORK=off 下 API、根服务、Gov/Res 相关构建验证通过；既有 audit、生成和依赖检查结果明确，不能忽略真实失败。 |
| B06 | OBS-09 | 三个实现仓库的提交、API/Res版本、镜像digest、实际手工patch/规则、部署与验证材料对应；全部必需真实项通过才宣告一期完成。 |

### 2. Res 范围解析 R 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| R01 | OBS-03 | 请求合法时从持久化记录返回 tenant/instance/cluster/namespace/workspace，字段与存储相同。 |
| R02 | OBS-03 | 跨租户、伪造身份或不受信调用按既有机制拒绝；body tenant 与可信租户不一致不能继续读记录。 |
| R03 | OBS-03、08 | 实例停止或正常删除后，只要持久化范围存在，仍可解析；真正不存在按 NotFound。真实历史按 E04 验证。 |
| R04 | OBS-03 | delete-first tombstone 等缺少可靠位置返回 INSTANCE_METRIC_SCOPE_UNAVAILABLE，Gov 明确409；不猜namespace或cluster。 |
| R05 | OBS-03 | 查询没有 Kubernetes/runtime/Observe 请求或持久化更新，不扩展启停、配额、调度语义；精确 RPC 受信允许项与文档/生成物完整。 |

### 3. 手工采集 C 类

| ID | 验证层级 | 验收内容与通过条件 |
| --- | --- | --- |
| C01 | 真实 | 记录实际Kubernetes context、Res cluster_id、Prometheus/Operator/KSM版本、采集对象与选择器，变更前字段可复核。 |
| C02 | 真实 | 五类所需原始指标确有样本；所选CPU为唯一整容器counter，workspace、UID/CID、KSM ANI业务标签来源可验证。 |
| C03 | 真实 | 原始时序与record都含同一ani_cluster_id，与Res、Obs datasource一致；KSM allowlist保留旧项，不能仅配置external_labels。 |
| C04 | fixture + 真实 | 每种声称支持的systemd/cgroupfs格式有可追溯依据与fixture；实际环境格式被验证；UID/CID规范化一致，非法/未知路径不回退Pod名。 |
| C05 | fixture | CPU至少2有效样本、5m rate、counter reset、启动窗口、总量维度处理符合合同；内存为working set。不能用任意sum掩盖多来源。 |
| C06 | fixture | 原始当前样本、KSM info/running/labels分别检查45s；过期current/snapshot不能经lookback或下游重录而续命；边界含45s、不含超过45s。 |
| C07 | fixture + 真实 | 同UID换CID及同名换UID的旧counter不被当前身份重新带入；旧CID在5m内残留且已有新CID时仍能排除旧运行身份。真实更换见E03。 |
| C08 | fixture + 现场诊断 | 原始重复采集在删除transport标签前发现、抑制并报告冲突；禁止max/sum/topk/labeldrop掩盖。抑制后NO_DATA不伪称可反推历史AMBIG。 |
| C09 | 真实 | PrometheusRule实际被选中、无lastError、持续求值；两个record及七标签真实存在；有界负载后有可解释变化。仅promtool或/metrics200不够。 |
| C10 | 文件 + 真实变更记录 | 实际patch保留既有参数、端点、过滤和allowlist；验证脚本可执行；保存应用差异、恢复本次字段的方法；不修改installer或回滚整个release/PVC/TSDB。 |

OBS-02负责C类。C05–C08的fixture必须能稳定重现失败条件；C01–C03、C09不得用模拟环境冒充。若只在一个实际cgroup模式上测试，报告清楚范围，不宣称两个模式都经过实机测试。

### 4. Prometheus 适配 D 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| D01 | OBS-04 | 固定metric code到record映射；五个可信scope matcher不可省略或被外部输入扩展；未知code与非法范围不形成PromQL。 |
| D02 | OBS-04 | 历史查询逐求值点限制record年龄45s，既不过滤所有历史，也不把矩阵求值时间当原始record时间；查询不依赖当前KSM。 |
| D03 | OBS-04 | 已知cluster精确选源，未知source503，重复配置启动拒绝；无跨源回退、用户URL或动态发现。 |
| D04 | OBS-04 | 正常matrix/vector、空结果、整秒时间、scope标签校验正确；损坏结构、重复点、缺失身份、意外resultType明确502。 |
| D05 | OBS-04 | limit129检测128条上限；有界读取8MiB；超限明确422，不能截断后返回200；边界值可通过。 |
| D06 | OBS-04 | Prom status=error、连接失败、5xx、超时、非空warnings、畸形JSON按合同映射；无错误吞成空数组。 |
| D07 | OBS-04、05 | 批次两查询共用4s与更短父deadline，无自动重试；请求取消会终止HTTP读写，关闭body和资源。 |

### 5. 业务规范化 N 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| N01 | OBS-05 | 有效0保留为present0，缺失是absent；不从GetValue默认值推断presence。 |
| N02 | OBS-05 | 全空/部分空返回完整series和网格，状态NO_DATA/PARTIAL正确；正常点无gap_reason。 |
| N03 | OBS-05 | 不重叠的UID/CID历史正常拼接；同一指标同一点多来源为null/AMBIGUOUS_SOURCE，不相加或任选。 |
| N04 | OBS-05 | NaN、±Inf、负数单来源为null/INVALID_VALUE；不能输出非法JSON数值。多来源裁决优先。 |
| N05 | OBS-05 | start=end、end不在网格、时区转换、整秒、point数正确；不擅自对齐、不插值、不前值填充。 |
| N06 | OBS-05 | 未知/重复指标、非法ID/namespace/container、未来end、倒置时间、超24h、step0/非30倍数/越界和点数上限有确定400。 |
| N07 | OBS-05 | latest批次只捕获一次evaluated_at，两个指标同一时刻且各恰1点；输出顺序按请求。 |
| N08 | OBS-05 | 任一上游失败整批失败，并取消另一项；合法单指标数据缺口仍为成功加null。 |
| N09 | OBS-05 | 总并发16、每租户4按请求而非指标计数；超限立即429；成功/失败/取消释放令牌，不同租户互不误占。 |

### 6. 信任与部署 S 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| S01 | OBS-06、07 | 真实受信Gov证书+正确scope可调用；Gov验证ani-observability-service服务身份，TLS≥1.3。 |
| S02 | OBS-06 | 无客户端证书、错CA、同CA错误客户端身份拒绝；不能照搬旧Res服务SAN。 |
| S03 | OBS-06、07 | 缺失/多值/不合法metadata、伪造actor/tenant、target tenant不一致拒绝；Gov重建可信header，不透传外部同名值。 |
| S04 | OBS-06 | 仅两个精确MonitoringService RPC允许；reflection关闭，任意方法或跨租户管理员scope不能放行。 |
| S05 | OBS-06 | 非法配置启动失败；单源临时不可用按查询错误；真实Deployment/ClusterIP/探针正常，关闭清理能结束连接和监听。 |
| S06 | OBS-06、08 | 从实际租户工作负载验证无法绕过受信入口获得其他实例指标；核实Obs与底层Prom查询面的网络/认证边界，不能只以ClusterIP推断隔离。 |

### 7. Gov HTTP H 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| H01 | OBS-07 | 两个精确/api/v1路由返回200；POST同步查询不进入202、写入幂等或配额逻辑。data为实际结构，路由不重复注册。 |
| H02 | OBS-07 | body缺少data、未知/重复JSON字段、同名path/body ID、PromQL/scope/datasource、额外query参数拒绝；超过64KiB是413。 |
| H03 | OBS-07 | step省略为60、显式0拒绝，使用真实HTTP decoder验证presence；0、null、正常值在实际codec中有区别。 |
| H04 | OBS-07、08 | instance:read权限和现有选租户机制生效；API同步及权限注册实际执行，只有代码声明不算完成。 |
| H05 | OBS-07 | Res返回范围与本次tenant/instance一致才调用Obs；ID来自路径，无客户端选择集群能力；只读OwnerClient与fake同步更新。 |
| H06 | OBS-07 | handler设置Operation并执行现有Middleware；仅两个精确路由10s、Res2s、Obs6s与Prom4s；更短deadline与取消不被抹掉。 |
| H07 | OBS-07 | 各reason对应HTTP码；ResourceExhausted区分422/429；错误使用既有外壳，不按日志接口统一映射。 |
| H08 | OBS-07 | OpenAPI和实际响应同为snake_case、value可null、状态短字符串，正常点不含gap_reason；不泄露原始labels/PromQL/dev_profile。 |
| H09 | OBS-07 | latest拒绝body与query参数，固定CPU后内存且各1点，evaluated_at与points时间相同。 |

### 8. 真实链路 E 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| E01 | OBS-08 | 经真实Gov认证查询CPU与内存范围/最新值，可对应到真实Res范围、Obs调用和Prom记录；不是仅grpc fixture成功。 |
| E02 | OBS-08 | 专用实例施加有界CPU/内存负载，基线与变化可解释；同scope、同求值点、同step对比Prom结果，在记录的浮点容差内一致。 |
| E03 | OBS-08 | 保留实际CID更换和Pod UID更换证据，业务实例ID不变；历史可解释，旧counter不串到新对象，多源重叠不求和。 |
| E04 | OBS-08 | 停止/正常删除Pod后，在业务范围仍可解析、数据保留期内，查询旧时间段成功；当前规则健康不作为旧历史前置；规则启用前不伪造回填。 |
| E05 | OBS-08 | 实际跨租户、无权限、伪造内部身份失败；真实接口拒绝任意PromQL和scope控制。 |
| E06 | OBS-08 | 可控连接故障/超时/取消按约定失败，无伪正常空数组；负载和故障操作不破坏共享组件，完成后恢复验证资源。 |
| E07 | OBS-08、09 | 真实request/response、request_id、Prom对照、运行身份时间线、版本、手工配置和恢复记录归档；未实际完成项明确，不写“全链路完成”。 |

核心数据、真实认证、真实链路和运行对象更换必须有环境证据。极端NaN、精确并发重叠、上游重复点等允许由确定的fixture验证，但报告要注明其测试层级。

### 9. 证据归档

脱敏、可提交的报告放 `docs/specs/observability-phase1/reports/`；原始采集/资源对象与运行记录按手工专项放忽略提交的 artifacts 目录，报告指向其实际位置。归档请求时移除Authorization、cookie、私钥、完整kubeconfig等凭据。

所有用例记录通过/失败/未执行、真实命令、关键输出、时间和证据位置。不要伪造成功标记；环境缺失只阻挡相应任务，其他独立工作继续完成。


源文件：[TASK-REPORT.md](TASK-REPORT.md)

## 任务完成报告模板

每个执行任务提交一份报告，路径为 `docs/specs/observability-phase1/reports/OBS-XX.md`。本模板中的状态和占位内容由执行 Agent 用真实结果填写；未执行的检查明确写“未执行”。

### 任务与结论

- 任务 ID：
- 状态：未开始 / 进行中 / 已完成 / 有阻碍
- 完成的具体行为：
- 尚未满足的完成条件：

### 改动与版本

| 仓库 | 实际起始提交 | 当前提交或分支 | 主要文件与目的 |
| --- | --- | --- | --- |
| 待填写 | 待填写 | 待填写 | 待填写 |

尚未提交时写明工作树状态，不编造提交。附真实 API 发布版本、Res 引用版本、根服务/Gov 锁定版本及获取验证结果；不适用时注明原因。

### 验证

| 用例 ID | 实际命令或操作 | 结果 | 证据文件 |
| --- | --- | --- | --- |
| 待填写 | 待填写 | 通过 / 失败 / 未执行 | 待填写 |

代码测试、规则 fixture 和真实集群验证分别列明。保留关键输入、输出、时间和退出码；不倾倒无关日志。

### 环境与配置

涉及真实环境时，记录目标集群 ID、Kubernetes context、监控 namespace、Prometheus/Operator/KSM 版本、实际应用文件及变更摘要。证书、令牌和 kubeconfig 只记录引用位置，不归档密钥内容。

### 阻碍与下一项输入

- 具体阻碍、证据、影响哪些任务：
- 已完成且可继续使用的产物：
- 下一项任务需要的提交、版本、接口、配置与证据路径：
- 是否修改了固定约定：没有 / 明确列出用户已确认的变更。


源文件：[contracts/res-scope.md](contracts/res-scope.md)

## Res 范围解析接口增量

在既有 `api/instance/v1/instance.proto` 的 `instance.v1.InstanceService` 内增加以下方法；复用原 GetInstanceRequest，不新建同名 service 或改动已有字段编号。

```proto
rpc ResolveInstanceMetricScope(GetInstanceRequest)
    returns (InstanceMetricScope);
```

在同一 proto 文件增加独立消息：

```proto
message InstanceMetricScope {
  string tenant_id = 1;
  string instance_id = 2;
  string cluster_id = 3;
  string namespace = 4;
  string container_name = 5;
}
```

完整内部方法：`/instance.v1.InstanceService/ResolveInstanceMetricScope`。

服务实现置于 `internal/service/instance/metric_scope.go`，复用现有 service 的 `repo` 与受信身份机制。执行 `RequireTenant → s.repo.Get → 校验持久化范围 → 返回`；不观察当前运行状态，不改变实例记录。返回普通已删除记录的可用历史范围；记录不存在复用 NotFound；缺少集群/namespace 等可靠范围时返回 `INSTANCE_METRIC_SCOPE_UNAVAILABLE`。

Gov 的 InstanceOwnerClient 和实际 client、mock/fake 均添加该方法，并引用确实包含增量 RPC 的 Res 版本。其余字段与错误按 `../CONTRACT.md` 执行。


源文件：[contracts/gov-http.md](contracts/gov-http.md)

## Gov 公共 DTO 增量

下列消息追加到既有 `api/protos/instance/service/v1/instance.proto`，沿用其 package、go_package 和生成链。新增消息的字段编号固定如下；不要重建现有文件或改既有字段。

```proto
message QueryInstanceMetricsRequest {
  string instance_id = 1 [json_name = "instance_id"];
  QueryInstanceMetricsData data = 2;
}

message QueryInstanceMetricsData {
  repeated string metric_codes = 1 [json_name = "metric_codes"];
  google.protobuf.Timestamp start = 2;
  google.protobuf.Timestamp end = 3;
  optional uint32 step_seconds = 4 [json_name = "step_seconds"];
}

message QueryInstanceMetricsResponse {
  InstanceMetricsRangeData data = 1;
}

message InstanceMetricsRangeData {
  string instance_id = 1 [json_name = "instance_id"];
  google.protobuf.Timestamp start = 2;
  google.protobuf.Timestamp end = 3;
  uint32 step_seconds = 4 [json_name = "step_seconds"];
  repeated InstanceMetricSeries series = 5;
}

message GetInstanceLatestMetricsResponse {
  InstanceMetricsLatestData data = 1;
}

message InstanceMetricsLatestData {
  string instance_id = 1 [json_name = "instance_id"];
  google.protobuf.Timestamp evaluated_at = 2 [json_name = "evaluated_at"];
  repeated InstanceMetricSeries series = 3;
}

message InstanceMetricSeries {
  string metric_code = 1 [json_name = "metric_code"];
  string unit = 2;
  string status = 3;
  repeated InstanceMetricPoint points = 4;
}

message InstanceMetricPoint {
  google.protobuf.Timestamp timestamp = 1;
  google.protobuf.DoubleValue value = 2;
  optional string gap_reason = 3 [json_name = "gap_reason"];
}
```

复用/补齐 Timestamp 与 wrappers.proto import。latest 请求复用既有 GetInstanceRequest。

在 `api/protos/admin/service/v1/i_instance.proto` 的既有公共 service 增加 QueryInstanceMetrics、GetInstanceLatestMetrics，使用所在文件既有 import 和类型限定方式。HTTP annotation 为：

```proto
// QueryInstanceMetrics
option (google.api.http) = {
  post: "/api/v1/instances/{instance_id}/metrics/query"
  body: "*"
};

// GetInstanceLatestMetrics
option (google.api.http) = {
  get: "/api/v1/instances/{instance_id}/metrics/latest"
};
```

annotation 生成文档；运行时薄 handler 保证HTTP200、数据包装、严格JSON输入、Operation和Middleware。若生成注册器已注册相同路径，需采用现有模式使每条路由只注册一次，不改变其他实例接口。

虽然请求 proto 有路径字段 instance_id，公共body中该字段必须拒绝，由路径绑定；不能仅凭proto结构接受它。normal point的gap_reason缺省，value为包装的0时输出0、nil时输出null；OpenAPI生成结果须标记nullable并由真实codec验证。

这些字段映射为本包实施定义，示例见 `../examples/`，限制/错误见 `../CONTRACT.md`。


## 接口源文件 contracts/api/observability/monitoring/v1/monitoring.proto

```proto
syntax = "proto3";

package observability.monitoring.v1;

import "google/protobuf/timestamp.proto";
import "observability/target/v1/target.proto";

option go_package = "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/monitoring/v1;monitoringv1";

// MonitoringService is exposed only on the trusted Gov listener.
service MonitoringService {
  rpc QueryRange(QueryRangeRequest) returns (QueryRangeResponse);
  rpc QueryLatest(QueryLatestRequest) returns (QueryLatestResponse);
}

message QueryRangeRequest {
  observability.target.v1.TargetScope target = 1;
  repeated string metric_codes = 2;
  google.protobuf.Timestamp start = 3;
  google.protobuf.Timestamp end = 4;
  uint32 step_seconds = 5;
}

message QueryLatestRequest {
  observability.target.v1.TargetScope target = 1;
  repeated string metric_codes = 2;
}

message QueryRangeResponse {
  google.protobuf.Timestamp start = 1;
  google.protobuf.Timestamp end = 2;
  uint32 step_seconds = 3;
  repeated MetricSeries series = 4;
}

message QueryLatestResponse {
  google.protobuf.Timestamp evaluated_at = 1;
  repeated MetricSeries series = 2;
}

enum SeriesStatus {
  SERIES_STATUS_UNSPECIFIED = 0;
  SERIES_STATUS_OK = 1;
  SERIES_STATUS_PARTIAL = 2;
  SERIES_STATUS_NO_DATA = 3;
}

enum GapReason {
  GAP_REASON_UNSPECIFIED = 0;
  GAP_REASON_NO_DATA = 1;
  GAP_REASON_AMBIGUOUS_SOURCE = 2;
  GAP_REASON_INVALID_VALUE = 3;
}

message MetricSeries {
  string metric_code = 1;
  string unit = 2;
  SeriesStatus status = 3;
  repeated MetricPoint points = 4;
}

message MetricPoint {
  google.protobuf.Timestamp timestamp = 1;
  // Absent is missing data. Present zero is a valid measured value.
  optional double value = 2;
  // GAP_REASON_UNSPECIFIED for a valid value; otherwise a concrete reason.
  GapReason gap_reason = 3;
}
```


## 接口源文件 contracts/api/observability/target/v1/target.proto

```proto
syntax = "proto3";

package observability.target.v1;

option go_package = "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/target/v1;targetv1";

// TargetKind is the governed business object category.
enum TargetKind {
  TARGET_KIND_UNSPECIFIED = 0;
  TARGET_KIND_CONTAINER_INSTANCE = 1;
}

// TargetScope is resolved by Gov through the resource owner.
// It is never accepted as a public HTTP request body.
message TargetScope {
  TargetKind kind = 1;
  string tenant_id = 2;
  string instance_id = 3;
  string cluster_id = 4;
  string namespace = 5;
  string container_name = 6;
}
```


## 示例 environment.example.yaml

```yaml
# 非部署配置；执行时读取实际环境后填写，不能直接 apply。
# 所有 null 表示未知输入，不是可使用的默认值。
workspace:
  template_checkout: null
  observability_checkout: null
  resource_checkout: null
  governance_checkout: null
cluster:
  kubernetes_context: null
  cluster_id_from_res: null
  tenant_id: null
  validation_instance_id: null
  validation_namespace: null
  container_name: workspace
collection:
  monitoring_namespace: null
  prometheus_resource_name: null
  prometheus_version: null
  operator_version: null
  kube_state_metrics_version: null
  cadvisor_service_monitor: null
  kube_state_metrics_service_monitor: null
  kube_state_metrics_deployment: null
  rule_selector: null
  rule_namespace_selector: null
  prometheus_internal_address: null
  existing_configuration_backup_path: null
service:
  deployment_namespace: null
  image_registry: null
  observability_server_certificate_ref: null
  gov_client_certificate_ref: null
  ca_ref: null
  gov_api_address: null
# 仅填引用或路径。不得在此文件放令牌、私钥、kubeconfig 正文。
```


## 示例 gov-latest-response.json

```json
{
  "data": {
    "instance_id": "5a01c947-5f3e-4de5-b070-818fde83d6a1",
    "evaluated_at": "2026-10-19T03:02:00Z",
    "series": [
      {
        "metric_code": "cpu.usage_cores",
        "unit": "cores",
        "status": "OK",
        "points": [
          { "timestamp": "2026-10-19T03:02:00Z", "value": 0.4 }
        ]
      },
      {
        "metric_code": "memory.working_set_bytes",
        "unit": "bytes",
        "status": "NO_DATA",
        "points": [
          { "timestamp": "2026-10-19T03:02:00Z", "value": null, "gap_reason": "NO_DATA" }
        ]
      }
    ]
  }
}
```


## 示例 gov-range-request.json

```json
{
  "data": {
    "metric_codes": [
      "cpu.usage_cores",
      "memory.working_set_bytes"
    ],
    "start": "2026-10-19T11:00:00+08:00",
    "end": "2026-10-19T11:02:00+08:00",
    "step_seconds": 60
  }
}
```


## 示例 gov-range-response.json

```json
{
  "data": {
    "instance_id": "5a01c947-5f3e-4de5-b070-818fde83d6a1",
    "start": "2026-10-19T03:00:00Z",
    "end": "2026-10-19T03:02:00Z",
    "step_seconds": 60,
    "series": [
      {
        "metric_code": "cpu.usage_cores",
        "unit": "cores",
        "status": "OK",
        "points": [
          { "timestamp": "2026-10-19T03:00:00Z", "value": 0.25 },
          { "timestamp": "2026-10-19T03:01:00Z", "value": 0 },
          { "timestamp": "2026-10-19T03:02:00Z", "value": 0.4 }
        ]
      },
      {
        "metric_code": "memory.working_set_bytes",
        "unit": "bytes",
        "status": "PARTIAL",
        "points": [
          { "timestamp": "2026-10-19T03:00:00Z", "value": 268435456 },
          { "timestamp": "2026-10-19T03:01:00Z", "value": null, "gap_reason": "NO_DATA" },
          { "timestamp": "2026-10-19T03:02:00Z", "value": 402653184 }
        ]
      }
    ]
  }
}
```


## 示例 version-lock.example.yaml

```yaml
# 实施后按真实提交和可下载版本填写；当前没有发布声明。
task_pack_version: "1.2"
template_baseline: fd18422211c741dd5242d2992c3d307d522aa28c
resource_design_baseline: f7459f7ae71c5573f53a50c5d20908b129c17e0c
governance_design_baseline: 90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812
observability:
  commit: null
  api_tag: null
  api_consumer_version: null
  api_download_verified: false
  root_api_required_version: null
  service_tag: null
  image_digest: null
resource:
  scope_rpc_commit: null
  consumer_version: null
  download_verified: false
governance:
  commit: null
  observability_api_required_version: null
  resource_required_version: null
  api_sync_verified: false
  permission_registration_verified: false
validation:
  gowork_off_verified: false
  real_collection_verified: false
  backend_e2e_verified: false
```
