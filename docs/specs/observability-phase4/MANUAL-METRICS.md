# 四期运维手册：租户级资源趋势

## 1. 采集与规则：本期无新增

- **不新增 ServiceMonitor / 采集补丁**：volcano device-plugin 的
  `ServiceMonitor ani-observability/ani-gpu-volcano-monitor`（15s）已在采，
  三节点 target up，`metricRelabelings` keep `HostCoreUtilization | HostGPUMemoryUsage`，
  并已注入 `ani_cluster_id`。
- **不新增 PrometheusRule**：租户级 CPU/内存直接聚合一期
  `ani-instance-metrics-v1` 组的 `ani_instance_cpu_usage_cores` /
  `ani_instance_memory_working_set_bytes`（已投影 `ani_tenant_id`，12 条规则
  `health=ok`）；GPU 的租户 join 固定写在 obs 代码里，不做预聚合。
- GPU 指标实测口径（2026-10-27，kcn 新环境）：
  `count(HostCoreUtilization)=6`（3 节点 × 2 卡，单位 percent）、
  `HostGPUMemoryUsage` 单位 bytes、
  `volcano_vgpu_device_core_allocation_for_a_certain_pod` 为 0–100 的 vGPU 份额。
  **结论：不需要 DCGM。**

## 2. Prometheus 保留期与容量（2026-10-27 已调整）

| 项 | 调整前 | 调整后 |
| --- | --- | --- |
| `spec.retention` | 24h | **30d** |
| `spec.retentionSize` | 4GiB | **54GiB** |
| PVC | 5Gi | **64Gi**（在线扩容，`FileSystemResizeSuccessful`） |

- 执行序：先扩 PVC，再 patch CR 的 retention/retentionSize，StatefulSet 重建 Pod；
  备份 `node1:/tmp/prom-before-20261027.yaml`，回滚 `kubectl replace -f`。
- **隐患**：CR 的 `spec.storage.volumeClaimTemplate` 仍是 5Gi（StatefulSet 模板不可变，
  未直接改）。当前运行无问题；但**重建环境或删除 PVC 会回到 5Gi**，届时必须先改 CR
  模板再重建。
- **历史不回补**：调整前 TSDB 只有 ~24h 数据，30 天档曲线从 2026-10-27 起逐步积累；
  上线初期长档位显示的历史短于所选窗口是预期行为，不是接口缺陷。
- 容量观察：23 万 series / 24h 接近 4GiB；30 天实际占用看
  `prometheus_tsdb_storage_blocks_bytes`，必要时再放大 PVC 或对租户级序列做预聚合。

## 3. GPU 指标与租户 join 复现命令

```bash
kubectl port-forward -n ani-observability svc/ani-metrics-prometheus 19098:9090 &

# 设备级指标（无租户维度）
curl -s -G 'http://127.0.0.1:19098/api/v1/query' --data-urlencode 'query=HostCoreUtilization'

# per-pod vGPU 份额（namespace/pod 是 scheduler 自身的，业务 pod 名在 podName）
curl -s -G 'http://127.0.0.1:19098/api/v1/query' \
  --data-urlencode 'query=volcano_vgpu_device_core_allocation_for_a_certain_pod'

# 租户级 GPU（obs 代码内固定表达式的等价形式）
curl -s -G 'http://127.0.0.1:19098/api/v1/query' --data-urlencode 'query=sum(
  label_replace(volcano_vgpu_device_core_allocation_for_a_certain_pod,
    "pod", "$1", "podName", "(.*)")
  * on(pod) group_left(label_ani_io_tenant_id)
    kube_pod_labels{label_ani_io_tenant_id="<tenant_uuid>"}
) / 100'

# 租户级 CPU 对拍
curl -s -G 'http://127.0.0.1:19098/api/v1/query' \
  --data-urlencode 'query=sum(ani_instance_cpu_usage_cores{ani_tenant_id="<tenant_uuid>"})'
```

## 4. 部署顺序与超时配置

1. **先 obs 后 gov，同批完成**：旧 gov 不感知新编码（无影响）；新 gov 对旧 obs 发
   kind=3 会被拒（invalid target kind），因此 obs 必须先就位。
2. obs ConfigMap / Deployment 环境变量：**`SERVER_GRPC_TIMEOUT` ≥ 20s**
   （默认 6s 会先于 15s 批次预算切断 30d 档请求）。
3. 换镜像必须用新 tag（节点 `imagePullPolicy: IfNotPresent`）。
4. governance 部署后按序执行：`admin sync-apis --dry-run` → `sync-apis` →
   `register-observability-permissions.sql` → `rollout restart` 刷新 Casbin。
5. 回滚：`kubectl rollout undo` 两个 Deployment；权限数据回滚见 governance 仓
   `scripts/ops/sql/register-observability-permissions.sql` 头注（仅在确认接口
   弃用时执行），回滚后同样需重启刷新策略。
