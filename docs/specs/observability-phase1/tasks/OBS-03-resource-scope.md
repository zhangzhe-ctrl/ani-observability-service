# OBS-03：Res 实例监控范围解析

## 仓库 / 前置

- 仓库：`ani-resource-service`；基线 `f7459f7ae71c5573f53a50c5d20908b129c17e0c`。
- 前置：本包公共契约已冻结且可读取；本任务可先行，不要求新服务已部署。

## 必读

- `../README.md`
- `../CONTRACT.md`
- `../contracts/res-scope.md`
- `../ACCEPTANCE.md`（R01–R05）
- Res `api/instance/v1/instance.proto`、`internal/service/instance/service.go`、`internal/biz/instance/types.go`、`internal/server/instance_identity.go`。

## 目标

为 Gov 提供一次只读、受租户身份约束的实例范围解析。返回持久化业务位置，支持读取已有历史，不依赖实例当前 Pod 或运行状态。

## 固定改动

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

## 验收

- 合法租户读取自己实例，返回持久化 cluster/namespace 和 workspace。
- 无身份、身份租户与请求不一致、其他租户实例均无法获取范围。
- 不存在实例沿用现有 NotFound；正常删除但 Spec 完整可解析；缺范围返回指定错误。
- 验证该 RPC 只有持久化读取，无运行时访问或业务写入；实例不在 Running/Ready 时仍可读取合法范围。
- 新方法经过真实 gRPC 身份中间件，不仅直调 Go 方法测试。
- 本仓生成一致性、相关服务/身份测试和既有构建检查通过。

## 交付

交付源 proto、生成文件、只读实现、允许方法表、文档及测试。报告新 RPC 所在完整提交及 Gov 应引用的真实 module version，不仅给本地分支名。

按 `../TASK-REPORT.md` 的格式提交任务报告。
