# OBS-05 监控用例与规范化

状态：已完成代码与业务行为验证。前置 API 已真实发布，适配器产物已存在；实际后端数据验收另见 OBS-08。

起始提交 `4827e664ee7a5dc91ec4707f407838045bec718b`，分支 `codex/observability-phase1-20261019`。`internal/biz/monitoring/` 固定两指标、完整原始网格、逐点来源裁决、零值 presence、NO_DATA/AMBIGUOUS_SOURCE/INVALID_VALUE、顺序和聚合状态。latest 一次取整秒时钟；4s 整批预算、失败取消兄弟、每副本16/租户4请求并发立即拒绝。`internal/service/monitoring/` 只转换固定 DTO 与标准 ErrorInfo，不把业务放到生成代码。

根模块首次业务 import 消费已发布 API v0.1.0，安全补丁后正式锁定 v0.1.1（见 OBS-01），无本地 replace、不依赖 go.work。

Fedora `go test -count=1 -v ./internal/biz/monitoring ./internal/data/monitoring ./internal/service/monitoring` 中 biz 行为首轮已通过；修正外部 HTTP 取消 fixture 后 data/service 全部通过，日志任务根 `logs/bootstrap/monitoring-behavior.log`、`monitoring-data-fixture-corrected.log`。后续联合 `make verify` 通过日志 `service-integrated-verify.log`，安全补丁后最终门禁见 OBS-09。

覆盖 N01–N09：测量0和缺失分开、全空/部分空、先后运行身份/重叠、非法值、非对齐end、同一latest时间、参数拒绝、批次失败/取消、短父deadline、16/4并发及释放。服务测试以 protobuf marshal/unmarshal 验证 present0/absent，标准机器错误可区分，拒绝nil/非法时间。

没有以 Prom fixture 声称真实采集或 Gov 链路完成；运行 TLS 和清理职责见 OBS-06。
