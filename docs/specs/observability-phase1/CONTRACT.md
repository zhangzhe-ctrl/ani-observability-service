# 一期公共实施约定

本文件是 OBS-00 至 OBS-09 共用的实施合同。用户后续明确确认的修改优先；执行 Agent 不自行调整本文件中的编码、字段、路由、单位、算法、边界或错误含义。

## 1. 一期交付范围

- 新服务：`ani-observability-service`，基于 `ani-kratos-layout`，组织 monitoring、logs、alerts 三个业务模块。
- 只实现容器实例 `workspace` 容器的 CPU、内存范围与最新值查询；CPU 规格和 GPU 规格实例均适用这两项资源指标。
- Gov 负责公共入口、租户和 `instance:read` 鉴权、Res 范围解析及调用协调。
- Res 只提供持久化实例的可信查询范围。
- Observability 负责指标定义、数据源、固定查询、限制和结果规范化；查询路径不调用 Res、Modeldev 或 Kubernetes。
- Prometheus 负责采集、recording rules 和时序存储。本期手工补齐配置，不改 ani-installer。
- 不开发前端；不实现 GPU/共享 GPU、网络、磁盘、平台服务、训练或推理指标；不实现日志查询和告警。logs、alerts 仅目录与说明，不提供假成功接口。
- 不新增业务数据库、队列、调度系统、PVC、数据源管理页面、动态指标管理或任意 PromQL API。

一次请求的顺序固定为：Gov 鉴权 → Res.ResolveInstanceMetricScope → Gov 校验返回范围 → Observability.QueryRange/QueryLatest → Prometheus → 规范化 → Gov DTO。

## 2. 项目和 Go 模块边界

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

### 工具与初始版本

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

## 3. 集群和采集身份

同一真实集群使用 Res 登记的稳定、非零、规范 UUID：

- Res：`Spec.ClusterID`。
- 人工采集配置：本集群唯一 `cluster_id` 输入。
- Prometheus 本地原始与规范时序：`ani_cluster_id` 标签。
- Observability datasource：`cluster_id`。

它是 ANI 的集群身份；不是 Kubernetes 内置 cluster UID、Prometheus Pod ID 或鉴权凭证。重启 Pod、容器、Prometheus 不更换该 ID。每个集群配置一次，各项指标共享；不对每个指标新增开关或集群配置。本期验证一个真实集群，配置结构可列多个 datasource；只按可信 scope 精确定位一个，不跨源搜索或汇总。

`external_labels` 不能代替本地时序所需的标签注入。不得依赖前端传入 cluster_id、tenant_id、namespace 或 PromQL 来形成可信范围。

### 规范 recording 结果

| 业务编码 | record 名称 | 单位 | 定义 |
| --- | --- | --- | --- |
| `cpu.usage_cores` | `ani_instance_cpu_usage_cores` | `cores` | workspace 整容器 CPU counter 的 `rate(...[5m])` |
| `memory.working_set_bytes` | `ani_instance_memory_working_set_bytes` | `bytes` | workspace 的 working set 字节数 |

两个 record 都只保留以下业务/运行身份标签：`ani_cluster_id`、`ani_tenant_id`、`ani_instance_id`、`namespace`、`pod_uid`、`runtime_container_id`、`container`。Prometheus 自身的 `__name__` 不计入这七项。

`pod_uid` 使用规范 UUID。`runtime_container_id` 为去除运行时前后缀后的完整 64 位小写十六进制 CID，原始 cAdvisor 与 KSM 两侧一致。`container` 固定 `workspace`。

CPU 不是百分比，不除以 request/limit，不压到 1；不将 sidecar 或 POD sandbox 计入。CPU 启动时至少两条有效样本才能得到 rate，5 分钟窗口未填满时仍按原算法作估计。内存不是 RSS、申请额度或限制值，不转 MiB/百分比。

### 录制时的绑定

原始指标固定为 `container_cpu_usage_seconds_total`、`container_memory_working_set_bytes`、`kube_pod_container_info`、`kube_pod_container_status_running`、`kube_pod_labels`。KSM Pod label allowlist 包含 `ani.io/tenant-id` 与 `ani.io/instance-id`，保留已有项。

1. 根据真实样本核实 cAdvisor CPU 维度，选择唯一整容器 counter，禁止把 total 和 per-CPU 数据同时计算。
2. 从 cAdvisor 原始 `id` 解析 Pod UID 与 CID，只支持经 fixture 核实的 Linux/containerd 的 systemd/cgroupfs 路径；systemd UID 下划线还原。保留原始 `id` 与 job/instance/endpoint 等来源标签直到唯一性判断完成。
3. 先对每条原始 CPU counter 作 rate，再与 KSM 的同 UID、当前 CID、workspace、running 状态匹配；再绑定同 UID 的可信 ANI 租户/实例标签。不能仅按 Pod 名、当前 UID 或租户标签关联。
4. 录制求值时原始样本、KSM 输入均必须新鲜，且原始当前序列仍存在。旧 CID 在 5 分钟窗口残留 counter 不能继续产生规范值。
5. 先验证唯一来源，再投影成七个规范标签。重复采集不能靠任意 max、sum、topk 或 labeldrop 隐藏。重复导致不产出规范值时，部署验证必须失败并提供原始冲突证据；之后修正配置。
6. 允许为匹配和诊断使用少量内部辅助 record，但它们不是第三个业务指标。辅助 current/snapshot 自身刷新时间不等于底层样本新鲜度；每条辅助规则都要验证输入年龄，禁止层层刷新延长过期数据寿命。

采集间隔 15s；规则组 `ani-instance-metrics-v1`，间隔 30s；原始/KSM 输入最大年龄 45s。PrometheusRule 对象同名，放入实际被选中的监控 namespace；基线选择标签是 `release: ani-metrics`，应用前核实真实 ruleSelector，部署名称不同时按真实选择标签记录，不改全局 selector 来迁就本规则。

完整人工操作、字段 patch、fixtures、promtool 与恢复要求见 `MANUAL-METRICS.md`。

## 4. 可信范围与内部 RPC

### Res

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

### Observability

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

## 5. Gov 公共 HTTP 合同

### 范围查询

`POST /api/v1/instances/{instance_id}/metrics/query`，成功 HTTP 200。

body 仅为 `{"data":{"metric_codes":[...],"start":"RFC3339","end":"RFC3339","step_seconds":60}}`。

- instance_id 只来自路径，body 即使重复同一个 ID 也拒绝。
- metric_codes 必填 1 至 2 项、不能重复、不能未知。
- start/end 必填，使用整秒 RFC3339；允许时区偏移，统一转换 UTC。
- step_seconds 公共 DTO 使用可感知 presence 的字段：省略时 60；显式 0 非法。
- 只接受以上字段和层次，拒绝未知字段、重复 JSON 字段、额外 query 参数以及 scope、PromQL、数据源地址。公共 JSON 字段使用 snake_case。
- 同步查询不走 202、创建幂等、版本提交或配额分支。

响应：`data { instance_id, start, end, step_seconds, series[] }`；series 元素：`{ metric_code, unit, status, points[] }`；point 元素：`{ timestamp, value, gap_reason? }`。series 与请求 metric_codes 顺序相同。

### 最新值

`GET /api/v1/instances/{instance_id}/metrics/latest`，无 body/query 参数，成功 HTTP 200。固定依次查询 CPU、内存。

响应：`data { instance_id, evaluated_at, series[] }`。每项 series 恰好一个点。Observability 在本批次开始时取得一个整秒 evaluated_at，两项指标使用同一求值时刻；point.timestamp 也为该求值时间，而不是宣称原始抓取恰在该时刻。

### Gov proto 与转换

在 `api/protos/instance/service/v1/instance.proto` 增加 QueryInstanceMetricsRequest、QueryInstanceMetricsData、QueryInstanceMetricsResponse、InstanceMetricsRangeData、GetInstanceLatestMetricsResponse、InstanceMetricsLatestData、InstanceMetricSeries、InstanceMetricPoint。latest 请求复用现有 GetInstanceRequest。公共 service 方法名固定 QueryInstanceMetrics、GetInstanceLatestMetrics。

data 包装必须是实际 proto 字段。snake_case 用显式 json_name；时间用 Timestamp；公共 value 使用 `google.protobuf.DoubleValue`；gap_reason 为 optional string。短状态字符串由 Gov 显式映射：OK、PARTIAL、NO_DATA；缺失原因映射为 NO_DATA、AMBIGUOUS_SOURCE、INVALID_VALUE。

读取内部 optional double 时先检查 presence，正常零值转为包装的 0，缺失转为 nil wrapper。不能只用 GetValue 后把缺失变成 0。真实 HTTP 编码必须分别输出 `"value":0` 与 `"value":null`；正常点不输出 gap_reason。OpenAPI nullable 与真实编码一致。

完整合法示例见 `examples/gov-range-request.json`、`gov-range-response.json`、`gov-latest-response.json`。这些样例为协议示意，不是实测记录。

## 6. 时间、历史、缺失与冲突

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

## 7. 查询限制和错误

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

## 8. Gov 与内部调用身份

两个公共接口均为 `instance:read`。Gov 先完成用户身份、选定租户和实例读权限，再调用 Res。Gov 验证 Res 返回 tenant_id、instance_id 与本次上下文一致，其他范围字段也满足合同，再调用 Observability。外部请求不能覆盖此范围。

Observability 受信 gRPC 端口使用 TLS ≥1.3、校验 CA 和服务身份。Gov 验证服务端身份 `ani-observability-service`；服务端只允许受信 `ani-governance` 客户端身份。按证书 SAN 等现有受信机制校验，不只接受“任意同 CA 证书”。不得复制 Res 旧监听的 `ani-network-service` ServerName。

Gov 重建以下单值元数据：`x-ani-tenant-id`、`x-ani-actor`、`x-ani-request-id`。不把外部同名 header 原样透传。Obs 拒绝缺失、多值、格式不合法或与 target 不一致的身份。只开放两个精确方法：

- `/observability.monitoring.v1.MonitoringService/QueryRange`
- `/observability.monitoring.v1.MonitoringService/QueryLatest`

一期仅 tenant scope，不自动放宽为跨租户管理员全局查询；reflection 关闭。实例工作负载不得以自行拼接 scope 绕过受信入口。Kubernetes 网络范围与 mTLS 配合验证，不因 ClusterIP 就假设租户完全不能访问底层 Prometheus。

Gov 增加专用薄 HTTP handler，同步 200，设置 Operation 并执行原有 `ctx.Middleware`。body 使用 `"*"` 映射 data 包装；防止自动生成路由和手工路由重复注册。仅这两个精确路由使用 10s 超时，其余 Gov 路由行为保持现有配置。

必须覆盖公共 proto、`api/protos/admin/service/v1/i_instance.proto`、service/server 各自的 instance_metrics.go、Res client、observability_client.go、observability_client_security.go、instance_timeout.go、instance_authorization.go、wiring_ent.go、权限 SQL 与实际 API 同步/权限注册步骤。

## 9. 实现落点与服务配置

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

## 10. 部署与后续扩展

交付 Dockerfile、deployments/observability/README.md、configmap.yaml、deployment.yaml、service.yaml、secret.example.yaml，以及 collection/ 手工资料。一期 1 副本 Deployment、私网 ClusterIP、admin 健康探针、已有日志与遥测；无业务 DB/队列/PVC/Kubernetes API 查询 RBAC。若 S06 发现租户可直连底层查询入口，基于现有网络能力补齐仅针对本次 Obs/Prom 查询面的最小访问策略，归档为 deployments/observability/networkpolicy.yaml；保留已有采集、平台访问和探针，不扩展成全平台网络改造。

手工采集使用现有 Prometheus/KSM/cAdvisor 与 Operator 管理对象，按字段应用最小 patch，不升级或重装整个监控栈。保存相关变更前字段、实际 patch、规范规则、原始 fixture、验证命令与恢复说明。日后 Helm 等自动流程可能覆盖手工字段，需将本次变更资料交给后续自动化任务；本期不因此修改 Installer。

新增日志或告警时复用可信目标与服务身份，但采用各自数据源和 RPC 模型。告警后续使用稳定指标定义、规则评估和通知能力，不轮询公共 Gov HTTP 接口。新增业务指标通常扩展目录定义与查询/recording；只有新数据源才补相应采集，不按每个指标新增安装器开关。

## 11. 依据与验证状态

本文件规定实现目标，真实集群适配和后端完成状态只能由任务报告证明。当前交付不包含已运行环境的测试结论。

- [Prometheus recording rules](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)：预计算时序、规则组与规则检查。
- [Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/)：instant/range 参数、结果类型和 series limit。
- [Prometheus 查询与 staleness](https://prometheus.io/docs/prometheus/latest/querying/basics/)：选择器、求值点和 lookback。
- [Prometheus rate](https://prometheus.io/docs/prometheus/latest/querying/functions/#rate)：counter 的每秒速率与边界估计。
- [Prometheus external_labels](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)：对外系统通信标签，不等同本地采集标签。
- [Go 模块源与子目录 tag](https://go.dev/doc/modules/managing-source)：API 子模块版本发布方式。
- 仓库文件依据见 `README.md` 的基线链接。
