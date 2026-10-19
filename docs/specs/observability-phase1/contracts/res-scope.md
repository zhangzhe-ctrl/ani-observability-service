# Res 范围解析接口增量

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
