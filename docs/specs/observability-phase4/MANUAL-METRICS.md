# 四期运维手册：租户级资源趋势

## 1. 采集与规则：本期无新增 rule，但需一次 KSM 白名单扩容

- **不新增 ServiceMonitor / 采集补丁**：volcano device-plugin 的
  `ServiceMonitor ani-observability/ani-gpu-volcano-monitor`（15s）仍在采，
  三节点 target up。GPU 曲线已改为直查 HAMi
  `vGPU_device_memory_usage_in_bytes`（30s，见 `deploy/gpu/hami-test-environment.json`
  与 `docs/contracts/gpu-monitoring-v1.md`）。
- **不新增 PrometheusRule**：三条租户曲线全部直查 cAdvisor / HAMi 原始指标，
  不再引用 `ani_instance_*` 预聚合（其源头硬编码 `container="workspace"`，只覆盖容器实例）。
- **KSM 白名单扩容一次**（2026-10-28 新增）：

  ```
  --metric-labels-allowlist=pods=[ani.io/tenant-id,ani.io/instance-id,ani.io/workload-kind,ani.kubercloud.com/tenant-id],namespaces=[ani.io/resource-tenant-id]
  ```

  补丁对 `deployments/observability/collection/patches/ksm-tenant-scope.patch.json`
  与 `restore-ksm-tenant-scope.patch.json`。该参数是**单个参数**，只能整体 `replace`；
  `test` 失败时先取当前对象逐字段核对，不删 test、不绕过。
- GPU 指标实测口径（2026-10-28，kcn 新环境）：
  HAMi `vGPU_device_memory_usage_in_bytes` 为容器级显存实际用量（bytes），
  `podnamespace` / `podname` / `ctrname` / `vdeviceid` 为业务维度。
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

# HAMi 容器级显存用量（podnamespace/podname 是业务维度，namespace 是 device-plugin 的）
curl -s -G 'http://127.0.0.1:19098/api/v1/query' \
  --data-urlencode 'query=vGPU_device_memory_usage_in_bytes'

# 三路 label 是否落地（KSM 白名单扩容后）
curl -s -G 'http://127.0.0.1:19098/api/v1/query' \
  --data-urlencode 'query=count(kube_namespace_labels{label_ani_io_resource_tenant_id=~".+"})'
curl -s -G 'http://127.0.0.1:19098/api/v1/query' \
  --data-urlencode 'query=count(kube_pod_labels{label_ani_kubercloud_com_tenant_id=~".+"})'

# 租户 pod 视图（obs 代码内固定表达式的等价形式）
curl -s -G 'http://127.0.0.1:19098/api/v1/query' --data-urlencode 'query=max by (namespace, pod) (
  kube_pod_labels{label_ani_io_tenant_id="<tenant_uuid>"}
  or label_replace(kube_pod_labels{label_ani_kubercloud_com_tenant_id="<tenant_uuid>"},
       "label_ani_io_tenant_id", "$1", "label_ani_kubercloud_com_tenant_id", "(.*)")
  or (kube_pod_labels * on(namespace) group_left()
        max by (namespace) (kube_namespace_labels{label_ani_io_resource_tenant_id="<tenant_uuid>"}))
)'

# 租户级 GPU 显存（obs 代码内固定表达式的等价形式）
curl -s -G 'http://127.0.0.1:19098/api/v1/query' --data-urlencode 'query=sum(
  label_replace(label_replace(max by (podnamespace, podname, ctrname, vdeviceid) (
    vGPU_device_memory_usage_in_bytes and (time() - timestamp(vGPU_device_memory_usage_in_bytes) <= 90)),
    "namespace", "$1", "podnamespace", "(.*)"), "pod", "$1", "podname", "(.*)")
  * on(namespace, pod) group_left() <租户 pod 视图>
)'
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
