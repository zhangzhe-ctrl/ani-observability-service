# 四期验收：租户级资源趋势

验收分三部分：**本地可闭环的验收**、**kcn 新环境集群实测**、**未执行项**。
本文不宣称未执行的验收；未执行项保持「待执行」，集群实测结果在实施后回填。

## 1. 本地验收（2026-10-27）

| # | 验收项 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | scope 校验：租户分支 | PASS | `TestValidateTenantScope`（带/不带 ClusterID 接受；instance id、namespace、container、pod uid、零租户、坏 cluster 共 6 类拒绝） |
| 2 | scope 校验：原 kind 无回归 | PASS | `TestValidateAcceptsBothKinds`、`TestValidateRejectsKindMismatch`（8 个子用例）原样通过 |
| 3 | catalog：租户编码与 kind 隔离 | PASS | `TestTenantCatalogAndWindowLimits`：三个租户编码 record/单位正确；实例/训练编码在租户 kind 未登记；租户编码在实例/训练 kind 未登记 |
| 4 | 表达式生成 | PASS | `TestTenantExpressionsAndRelaxedBudget`：三条均为 `sum(...)` 外层 + 租户 pod 视图（三路 label + `max by (namespace, pod)`）；CPU/内存含 `<= 45` 新鲜度守卫，GPU 含 `<= 90` 与 `podnamespace`/`podname` 投影；旧 volcano 份额与 `/ 100` 已消失；租户身份注入 |
| 5 | validLabels 租户分支 | PASS | `TestTenantLabelsAcceptAggregationShapes`：零标签与单租户标签接受；异租户标签、多标签、pod_uid 拒绝 |
| 6 | 窗口校验：30d/4h 放宽 | PASS | 同 3：30d 档 2h step 通过；31d 跨度、4.5h step 拒绝；1d 档 5m step 通过（实例 kind 的 1h 上限不适用于租户） |
| 7 | 窗口校验：实例/训练无回归 | PASS | `TestInvalidQueryNeverCallsProvider`（14 个子用例）原样通过 |
| 8 | 数据源缺省解析 | PASS | `TestTenantQueryWithoutClusterResolvesSingleDatasource`（单数据源 + 空 ClusterID） |
| 9 | 全量 Go 检查 | PASS | `go build ./...`、`go vet ./...`、`GOWORK=off go test ./internal/... -count=1` 全部通过（`cmd` 包 `TestMainProcessHandlesSignalAndClosesListeners` 在 Windows 上执行 `.exe` 名二进制失败，为平台既有问题，main 分支同样失败，与本期改动无关） |
| 10 | 一、二、三期目录零改动 | PASS | 三个 phase 的 `sha256sum -c SHA256SUMS` 全部 `bad=0` |

## 2. 集群实测（kcn 新环境 10.10.1.66）

| # | 项目 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | obs 镜像替换 | PASS | `ani-observability-service:trend-20261027`，digest 见实施记录 §9.1；rollout 成功，1/1 Running；`SERVER_GRPC_TIMEOUT=20s` 已生效 |
| 2 | gov 镜像替换 | PASS | `ani-governance:trend-20261027` 1/1 Running；API 目录 `ADD GET /api/v1/observability/resource-trend`；权限 SQL 落库；重启后 Casbin 刷新 |
| 3 | 1d × cpu/memory/gpu | PASS | HTTP 200，`step_seconds=300`、289 点、单序列；GPU 返回等效卡数或 NO_DATA |
| 4 | 7d × cpu/memory/gpu | PASS | HTTP 200，`step_seconds=1800`、337 点 |
| 5 | 30d × cpu/memory/gpu | PASS | HTTP 200，`step_seconds=7200`、361 点（Prometheus 已改 30d 保留，曲线随积累变长） |
| 6 | 一次三条曲线 | PASS | `metric` 省略时 `series` 长度 3，顺序 cpu/memory/gpu |
| 7 | 边界 400 | PASS | `range=1d&step=4h`、`range=7d&step=1m`、`range=30d&step=16h`、31d 跨度、`step=0s/-30m`、坏时间、`end` 晚于当前、未声明参数、`tenant_id` 全部 400 |
| 8 | 鉴权与权限 | PASS | 无 Authorization 401；无权限码账号 403；平台身份访问租户趋势 403 |
| 9 | Prometheus 对拍 | PASS | 接口值与原始 `sum(ani_instance_cpu_usage_cores{ani_tenant_id=...})` 一致；GPU 与 join 表达式一致；差异仅来自求值时刻对齐 |
| 10 | 实例链路无回归 | PASS | `POST /api/v1/instances/{id}/metrics/query` 行为不变（24h/1h/3000 点合同未动） |

回填说明：第 3–9 项的原始请求/响应证据（脱敏）见
`ani-governance/local-remote/可观测性接入/首页统计/首页资源趋势统计-实测与测试报告-20261027.md`。

## 3. 未执行项

| # | 项 | 原因 |
| --- | --- | --- |
| 1 | 前端三卡片联调 | 前端由前端同学自行接入（对接文档已交付），本轮仅覆盖后端 |
| 2 | 5 并发 + 30d 档的长期负载观察 | 属持续观察项，不做一次性断言；单请求耗时与短时并发已实测 |
| 3 | 多集群（多 datasource）下租户查询的 ClusterID 显式指定路径 | 当前环境单集群，无实机可测；代码路径 fail closed |
