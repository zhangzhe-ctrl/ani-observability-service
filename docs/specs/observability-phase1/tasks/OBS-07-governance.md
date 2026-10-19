# OBS-07：Gov 监控入口与后端调用

## 仓库 / 前置

- 仓库：`ani-governance`；基线 `90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812`。
- 前置：OBS-01 API 已可消费、OBS-03 Res RPC 已可消费、OBS-06 已完成可观测查询服务和 Gov mTLS 验证。沿用本包约定，不开发前端。

## 必读

- `../README.md`
- `../CONTRACT.md`
- `../contracts/gov-http.md`、`../contracts/res-scope.md`
- `../ACCEPTANCE.md`（H01–H09、B04–B05）
- `../examples/`
- 现有实例 HTTP、service、授权、超时与 Res client 实现。

## 目标

完成 Gov 认证授权 → Res 可信范围解析 → Observability gRPC 查询 → 公共 HTTP 结果的完整协调，保持固定公共合同。

## 固定改动

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

## 验收

- 两接口经真实 HTTP handler 执行认证、中间件、Operation、instance:read，成功为 200。
- 验证 Gov 先调用 Res 解析范围，再由 Gov 调用 Observability；两指标编码、单位、顺序、时间及状态满足 CONTRACT。
- HTTP 编码测试验证 null、0、普通值、data 外壳、下划线字段及有效点无 gap_reason；不能只测试 DTO。
- 覆盖非法/重复字段、跨租户、无读取权限、范围不可用、并发限流和查询超时；错误映射正确，取消下游调用。
- GOWORK=off、无本地 replace，真实固定版本可下载并通过本仓生成、相关测试和构建。
- API 权限同步已执行并有结果；未同步不能声称已完成对外授权接入。最终真实 Prometheus 负载验收按后续端到端任务执行。

## 交付

交付公共 proto/生成物、handler、服务协调、客户端安全与清理、权限和超时改动。报告依赖版本、HTTP 样例、测试结果及权限同步记录，不提交真实凭据。

按 `../TASK-REPORT.md` 的格式提交任务报告。
