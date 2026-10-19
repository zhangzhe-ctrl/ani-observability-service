# ani-observability-service

ANI 容器实例 `workspace` 的 CPU、内存范围及最新值查询服务。完整链路为
Gov 鉴权 → Res 持久化可信范围 → Observability → Prometheus → Gov DTO。
根模块与同仓独立 API 子模块均通过正式发布依赖构建，不需要模板仓库或本地 replace。

一期业务仅 `cpu.usage_cores`（5m rate，cores）与
`memory.working_set_bytes`（working set，bytes）。不提供任意 PromQL、前端、GPU 指标、日志查询或告警接口；
`internal/{biz,data,service}/{logs,alerts}` 留作后续业务边界。

公共入口：

- `POST /api/v1/instances/{instance_id}/metrics/query`
- `GET /api/v1/instances/{instance_id}/metrics/latest`

只经 Gov 使用正常租户身份与 `instance:read`。客户端不能选择 tenant、cluster、namespace 或数据源。
字段、编号、路由、错误和限制以 [固定合同](docs/specs/observability-phase1/CONTRACT.md) 为准；
[用户确认的调整](docs/specs/observability-phase1/EXECUTION-OVERRIDES.md) 单独记录。
历史记录启用前没有回填；缺失输出 null，测量0仍是0；每个时间点按 UID/CID 裁决，多来源不求和。
CPU 不是百分比，负载恢复按5分钟窗口解释。

在获准的 Fedora 环境使用 Go 1.26.9，分别执行根模块与 API 门禁：

```sh
GOWORK=off make generate verify build
GOWORK=off make supply-chain-tools audit
```

`make verify` 显式检查 API tidy-diff/test/vet/mod verify 与两条生成链。
`make audit` 保留漏洞、全历史泄密、完整 SBOM、来源及许可证证据检查。
自有 API 未声明开源许可证，不以模板 MIT notice 代替；参见
[许可证与来源复核](docs/scaffold/license-review.md) 和 `LICENSE`。

运行必须提供受信 Gov 的 mTLS 证书、唯一集群静态 datasource 和固定查询限制；
非法配置启动失败。`/healthz`、`/readyz`、`/metrics` 是独立 admin 监听，Ready 不保证 Prometheus 在线。
数据源故障通过查询明确返回503/504，不转换为空数组。
部署、证书引用和启动输入见 [部署说明](deployments/observability/README.md)，
人工采集及字段级恢复见 [collection](deployments/observability/collection/README.md)。不修改 ani-installer。

按错误定位：401/403先查 Gov 身份和权限；409查 Res 持久化范围；503查 datasource 配置/连接；
502查响应、标签与 rule；504查本次4秒批次预算及上游取消。
成功但 null 时核对同一求值时间的 record、45秒新鲜度、运行身份和规则启用时间。
不得为排障删除 TSDB、PVC 或其他规则。

[任务报告](docs/specs/observability-phase1/reports/) 区分代码、fixture、真实采集和真实业务验收，
并记录真实版本与尚未执行项。原始凭据不入仓。
