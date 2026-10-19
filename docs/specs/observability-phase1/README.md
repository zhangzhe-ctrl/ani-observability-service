# ANI 可观测服务一期任务包 v1.2

本包供执行 Agent 分项实施和验收。交付目标是容器实例 CPU、内存监控的完整后端链路：Gov 先向 Res 解析可信实例范围，再调用 ani-observability-service 查询 Prometheus。范围查询和最新值查询均通过 Gov 验收，本期不开发前端。

本版沿用已确认方案 v1.1 的业务接口、指标口径、recording rules、API 子模块和权限模型；唯一范围调整是：**暂不接入 ani-installer，采集配置由人工应用并归档到新服务仓库。** 手工部署和真实数据验收仍须完成。

## 1. 阅读与使用

1. 先读本文件、`CONTRACT.md` 和 `ACCEPTANCE.md`。这些文件是全部任务的公共约定。
2. 将整包作为执行 Agent 可读取的工作资料。OBS-00 将其归档到新服务的 `docs/specs/observability-phase1/`。跨仓库任务继续引用同一份资料，不各自维护协议副本。
3. 将 `START.md` 中的启动指令交给协调 Agent，或单独发送 `tasks/` 下某个任务文件。每个任务文件的全部字符数控制在 4000 以内；公共资料必须一并可读。
4. 每个任务完成后按 `TASK-REPORT.md` 返回真实产物、版本、验证结果和下一项任务可使用的输入。报告写入 `docs/specs/observability-phase1/reports/`。
5. `REVIEW.md` 是为审核合并的阅读视图；修改时以公共约定及各任务原文件为准，再重新生成阅读视图。

此包交付时的任务状态均为 **未执行**。包内 JSON 是契约示例，proto 是确定的接口输入，均不代表服务已部署或真实集群已验证。环境字段由执行时读取的真实值填写，不能以示例值代替。

## 2. 仓库范围与设计基线

| 仓库 | 设计基线 | 本期用途 |
| --- | --- | --- |
| ani-kratos-layout | `fd18422211c741dd5242d2992c3d307d522aa28c` | 初始化模板；不在模板内实现监控业务 |
| ani-resource-service | `f7459f7ae71c5573f53a50c5d20908b129c17e0c` | 新增只读范围解析 RPC |
| ani-governance | `90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812` | 新增两个查询入口及受信调用 |
| ani-observability-service | 新项目，实际创建后记录提交 | API 子模块、监控实现、手工采集材料、服务部署 |

以上是方案所依据的提交，不声称执行时仍是最新 main。开始任务时记录实际基线及相关差异，保留用户既有工作；不得为了对齐基线覆盖现有改动。`ani-installer` 不在本包修改仓库与完成条件中。

## 3. 固定任务与依赖

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

## 4. 本包内容

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

## 5. 执行边界

执行固定任务，不扩展平台服务监控、GPU 指标、日志查询或告警实现。日志和告警只保留模块边界。现有实例创建、启停、调度、配额语义不在本期变更范围内。

记录规则继续使用，集群 ID 继续贯通。手工配置集中在 `deployments/observability/collection/`，服务部署集中在 `deployments/observability/`。本期不新增安装器代码、自动安装系统、业务数据库、消息队列或通用 PromQL 接口。

正常实现选择按本包直接推进。只有具体环境缺失、接口冲突或已有访问权限阻挡某一步时，记录证据并完成其余独立任务；不得用 mock、忽略失败或伪造版本将整包标记完成。

## 6. 依据

仓库事实来自已核实基线；规范性字段和参数是本任务包的实施决定。

- [模板 Makefile](https://github.com/zhangzhe-ctrl/ani-kratos-layout/blob/fd18422211c741dd5242d2992c3d307d522aa28c/Makefile)
- [Res 实例接口](https://github.com/zhangzhe-ctrl/ani-resource-service/blob/f7459f7ae71c5573f53a50c5d20908b129c17e0c/api/instance/v1/instance.proto)
- [Gov 实例超时过滤器](https://github.com/zhangzhe-ctrl/ani-governance/blob/90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812/app/admin/service/internal/server/instance_timeout.go)
- [Prometheus recording rules](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)
- [Prometheus HTTP API](https://prometheus.io/docs/prometheus/latest/querying/api/)
- [Go 子目录模块发布](https://go.dev/doc/modules/managing-source)
