# 执行启动指令

请执行这份 ANI 可观测服务一期任务包。先完整阅读 `README.md`、`CONTRACT.md`、`ACCEPTANCE.md`，再按任务依赖执行 `tasks/OBS-00` 至 `OBS-09`。单项任务只在其前置产物存在时标记完成。

固定目标：基于 ani-kratos-layout 初始化 ani-observability-service，组织 monitoring、logs、alerts 三个业务模块，建立同仓库独立 API Go 子模块。本期仅实现容器实例 CPU、内存范围和最新值查询，闭合 Gov → ani-observability-service → Prometheus；Gov 通过 Res 的只读 RPC 解析可信实例范围。不开发前端。

本期不修改 ani-installer。采集、标签及 recording rules 由 OBS-02 手工应用，文件归档到新服务 `deployments/observability/collection/`。必须完成真实配置和真实数据验证，不以说明文档或 mock 代替。

先核对实际仓库位置、提交、工作树状态、相关项目规范与现有环境。模板使用命名分支及规定初始化脚本。不要覆盖已有工作，不要编造 UUID、地址、证书、提交、模块版本或测试结果。

将本包归档到新服务的 `docs/specs/observability-phase1/`，其他仓库任务引用同一份契约。API proto 使用 `contracts/api/` 中确定的字段和编号。固定指标、路由、错误、限制和缺失语义不得自行改名或扩展。

先发布或形成真实可获取的 API 子模块版本，再让根服务和 Gov 消费；Res 的新 RPC 也要形成真实可引用版本。本地 go.work 可以辅助开发，正式验收必须 GOWORK=off，且不能依赖本地 replace。

OBS-03 可先行，OBS-00 后 OBS-01 与 OBS-02 可并行；OBS-04、05、06 顺序推进；OBS-07 完成后与 OBS-02 汇合做 OBS-08；最后完成 OBS-09。并行任务避免争写共享文件。

每项任务按 `TASK-REPORT.md` 汇报改动、提交/版本、真实命令和结果、证据路径、尚未执行项。遇到具体阻碍时保留证据，继续无依赖工作。只有代码验证、手工采集验证和真实后端链路验收全部满足时，才能宣布本期完成。
