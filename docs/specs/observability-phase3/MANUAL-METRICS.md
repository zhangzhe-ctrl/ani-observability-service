# 三期手工采集增量：训练执行 Pod 资源指标

本文是 `docs/specs/observability-phase1/MANUAL-METRICS.md` 的**增量补充**，不替代一期文件。
一期的人工操作、promtool 方式、`obs_root` / `obs_collection` / `obs_kubectl` 变量继续有效。
本期新增一组训练规则、一对 KSM 补丁，**不改**一期规则组、不改一期补丁、不加 Installer 开关。

## 1. KSM 补丁对（新增，先于规则）

一期 `patches/ksm-workload.patch.json` 的 `test` 锁基线 args（两项），已于 2026-10-19 应用；
直接改它会同时破坏一期历史可重放性与当前集群的恢复路径。本期新增一对补丁，各自只覆盖一段
状态迁移（理由见 CONTRACT §7.1）。

```bash
# 应用前先取当前对象
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get deployment ani-metrics-kube-state-metrics -o json > "$obs_root/collection/ksm-workload.phase3.before.json"

# dry-run
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch deployment ani-metrics-kube-state-metrics --type=json --patch-file "$obs_collection/patches/ksm-workload-phase3.patch.json" --dry-run=server -o json > "$obs_root/collection/ksm-workload.phase3.dry-run.json"

# 应用
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" patch deployment ani-metrics-kube-state-metrics --type=json --patch-file "$obs_collection/patches/ksm-workload-phase3.patch.json"
```

补丁要求：

- `test /metadata/uid` 锁当前对象 UID；
- `test /spec/template/spec/containers/0/args` 锁**一期状态**的三项值（`--port`、`--resources`、
  两项标签 allowlist）；
- `replace /spec/template/spec/containers/0/args/2` 为
  `--metric-labels-allowlist=pods=[ani.io/tenant-id,ani.io/instance-id,ani.io/workload-kind]`。

回滚用 `restore-ksm-workload-phase3.patch.json`：同样 `test` 锁三期状态，`replace /args/2`
回两项标签 allowlist，回到一期状态（不是基线）。

**易错点**：`--metric-labels-allowlist` 是单个参数，不能 append 第二个；必须 `replace`
数组元素。`test` 失败时先取当前对象、逐字段核对，不删 test、不绕过。

**空窗**：单副本滚动 → 30–45s 无新样本，叠加 45s 守卫 → 实例与训练一起 45–90s `NO_DATA`。
放部署窗口、避开活跃执行。该空窗类型登记到 `source-profile.yaml`。

## 2. 训练规则组

新文件 `rules/ani-training-metrics-v1.yaml`（PrometheusRule 对象）与
`rules/ani-training.rules.yaml`（纯 groups 文件）。组名 `ani-training-metrics-v1`，
`interval: 30s`，选择标签沿用 `release: ani-metrics`，放入 `ani-observability`。

**不得**并入一期组 `ani-instance-metrics-v1`：一期文件只读，且组内求值顺序与 helper 位置耦合。

```text
正确顺序（组内按文件顺序求值）：
  ani:training:cpu_source
  ani:training:memory_source
  ani:training:info_source
  ani:training:running_source
  ani:training:labels_source
  ani:training:binding
  ani_training_cpu_usage_cores
  ani_training_memory_working_set_bytes
```

**易错点 1（helper 必须在 binding 之前）**：helper 追加到组末尾时，canonical 在同一周期只能
读到上一周期的 helper；因 canonical 带 `timestamp(helper) == time()` 快照守卫，训练 canonical
会恒为空，且规则 `health` 仍为 `ok`、无 `lastError`，极难排查。

**易错点 2（两份文件等价）**：YAML 块标量里的 `\` 写作 `\\.`；JSON 版在其上再做 JSON 转义
写作 `\\\\.`；JSON 版 expr 末尾带 `\n`。逐字比对 `spec.groups`。

**易错点 3（join 的键）**：主序列与 `kube_pod_labels` 的 join 用
`and on(ani_cluster_id, namespace, pod)`。`kube_pod_labels` 的 `ani_cluster_id` 来自
KSM ServiceMonitor 的 metricRelabeling（一期已写入）；夹具缺该标签时 join 恒空，用例会「假
通过」成「不产出」。

**易错点 4（不要 namespace 正则）**：训练 namespace 形态不稳定
（如 `ani-modeldev-tenant-a-v21`），用 `namespace=~"ani-modeldev-.*"` 会静默漏采。隔离靠
`label_ani_io_workload_kind="trainer"`，不靠命名。

**易错点 5（init 容器）**：`info_source` / `running_source` 只查 `kube_pod_container_info` /
`kube_pod_container_status_running`，init 容器在独立 `kube_pod_init_container_*` 族里，
因此 init 容器只有瞬时 `cpu_source` / `memory_source`，永远形不成 canonical。这不是漏配，
是数据源语义；用例 8 固化该行为。

## 3. promtool

```bash
podman run --rm --security-opt label=disable -v "$obs_collection:/work:ro" --workdir /work --entrypoint /bin/promtool 172.16.101.10:5000/prometheus/prometheus:v3.11.3-distroless check rules rules/ani-training.rules.yaml
podman run --rm --security-opt label=disable -v "$obs_collection:/work:ro" --workdir /work --entrypoint /bin/promtool 172.16.101.10:5000/prometheus/prometheus:v3.11.3-distroless test rules tests/ani-training.rules.test.yaml
```

判据：`SUCCESS: 8 rules found`；用例全 PASS（至少覆盖 CONTRACT §6.5 的 9 类；夹具必须带
`ani_cluster_id` 的 `kube_pod_labels` 样本）。

## 4. 应用后线上判据

```bash
# 规则健康：训练组 8 条全 ok 且无 lastError
"$obs_kubectl" --context "$obs_context" -n "$obs_namespace" get prometheusrule ani-training-metrics-v1 -o json
# 通过 Prometheus API 复核
#   /api/v1/rules 中 group=ani-training-metrics-v1，8 条 health=ok
```

只读 PromQL 判据（在集群 Prometheus 上执行）：

```promql
# 1. 第三标签是否已暴露（无训练 Pod 时为 0，属正常）
count(kube_pod_labels{label_ani_io_workload_kind="trainer"})

# 2. init 不进 info（部署前事实复验，CONTRACT §6.6）
count by (container) (kube_pod_container_info{namespace="<训练 ns>"})
count by (container) (kube_pod_init_container_info{namespace="<训练 ns>"})

# 3. 中间序列与 canonical 数量（有训练 Pod 时应 > 0）
count(ani:training:cpu_source)
count(ani:training:binding)
count(ani_training_cpu_usage_cores)

# 4. 新鲜度：最大样本年龄应 < 30s（组间隔）
max(time() - timestamp(ani:training:binding))

# 5. 实例隔离：训练 canonical 不得出现 workspace 容器
count(ani_training_cpu_usage_cores{container="workspace"})
```

诊断口径：

- `kube_pod_labels{label_ani_io_workload_kind="trainer"}` 为 0 且确有训练 Pod → 先查 KSM args
  第三项、再查 Pod 标签是否真被打上。
- `ani:training:cpu_source` 有值而 `binding` 为空 → `info_source` / `running_source` 的 join 键
  或标签过滤不一致。
- `binding` 有值而 canonical 为空 → helper 位置在组末尾，或 `timestamp(helper) == time()` 不成立。
- canonical 有值但查询为空 → `pod_uid` / `container` / `ani_instance_id` 与调用方 scope 不一致。

## 5. 部署顺序

1. 应用 KSM 补丁（阶段 1'），等滚动完成，确认 args 三项、KSM 恢复抓取。
2. 模型侧 webhook 打三标签（阶段 1，可与 1' 并行）。
3. 应用训练规则组（阶段 2），等一个 30s 周期，确认 8 条 `health=ok`。
4. 有训练 Pod 后按 `ACCEPTANCE.md` §2 验收；无训练 Pod 则按 §1.1 决定是否用临时 Pod 验证。
5. 若第 1 步之后需要回退，只能先回滚训练规则（否则规则恒空但 health 仍 ok，掩盖问题），
   再回滚 KSM 补丁。

## 6. source-profile 登记项

`source-profile.yaml` 增加：

- 规则组 `ani-training-metrics-v1`、PrometheusRule 对象名与选择标签；
- KSM allowlist 第三项 `ani.io/workload-kind` 及其进入 `kube_pod_labels` 的条件
  （Pod 真实带标签时才输出）；
- 训练 namespace 实际形态与「不用 namespace 正则」的理由；
- KSM 滚动空窗类型（30–45s 无样本 / 45–90s `NO_DATA`），标注同样影响 `ani_instance_*`；
- init 容器不进 `kube_pod_container_info` 的事实与复验结论（含复验日期与命令）。
