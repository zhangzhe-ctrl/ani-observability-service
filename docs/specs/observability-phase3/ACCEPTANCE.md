# 三期验收：训练执行 Pod 资源指标

验收分四部分：**本地可闭环的验收**、**kcn 新环境集群实测**、**仍需真实训练 Pod 的验收**、
**部署前事实复验**。本文不宣称未执行的验收；未执行的项保持「待执行」。

## 1. 已完成（2026-10-27，本地）

| # | 验收项 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | promtool `check rules` 训练组 | PASS | `SUCCESS: 8 rules found`（promtool 3.11.3，windows-amd64，与集群版本一致） |
| 2 | promtool `test rules` 训练用例 | PASS | `SUCCESS`（9 个用例，见 §1.1 覆盖清单） |
| 3 | 两份训练规则文件 expr 逐字等价 | PASS | 8/8 条，`mismatch=0`；JSON 版 expr 末尾带 `\n`（与一期惯例一致） |
| 4 | 领域校验按 kind | PASS | `TestValidateAcceptsBothKinds`、`TestValidateRejectsKindMismatch`（8 个子用例） |
| 5 | `scope()` 映射 | PASS | `TestScopeMapsTrainingPodUID` 直接断言领域字段；实例 kind 断言 `PodUID` 为空 |
| 6 | catalog 分流 | PASS | `TestTrainingCatalogAndKindIsolation`：同一 code 两 kind 各自 record；训练网络码未登记；未知 kind 不解析 |
| 7 | selector 形状 | PASS | `TestTrainingExpressionAndLabelsPinPodUID`：训练表达式 `pod_uid=` 出现 2 次，实例表达式不含 |
| 8 | `validLabels()` | PASS | 同上：训练 kind 下 `pod_uid` 与外层 scope 不符判为响应非法 |
| 9 | 一期、二期包完整性 | PASS | 一期 27 文件、二期 4 文件哈希全部匹配（`bad=0`） |
| 10 | 全量 Go 检查 | PASS | `go build ./...`、`go test ./internal/... -count=1`、`go vet ./...`、api 子模块 `GOWORK=off` build/vet 全部通过 |

### 1.1 promtool 用例覆盖清单

| # | 用例 | 断言 |
| --- | --- | --- |
| 1 | `training-worker-node-normal-eight-labels` | worker `node` 产出 CPU 与内存 canonical，标签恰为 8 个 |
| 2 | `instance-workspace-container-never-enters-training-records` | 实例 `workspace` 不进 `ani_training_*`；训练 canonical 总数仍为 1 |
| 3 | `missing-instance-label-produces-nothing` | 缺 `label_ani_io_instance_id` 时 canonical 为空 |
| 4 | `pause-sandbox-container-is-filtered` | `container="POD"` 的中间序列被过滤 |
| 5 | `init-container-transient-source-only` | init 容器有瞬时 `cpu_source`，但无 canonical |
| 6 | `two-step-pods-stay-independent` | 两个 step Pod 各自成条，总数为 2 |
| 7 | `labels-without-cluster-id-never-join` | `kube_pod_labels` 缺 `ani_cluster_id` 时 4 键 join 不命中 |
| 8 | `same-pod-two-container-ids-rejected-by-singleton-guard` | 同 Pod 同容器同刻两个 CID → binding 唯一性守卫拒绝 |
| 9 | `short-lived-step-has-no-rate-but-has-memory` | 单样本 step：CPU 无 rate、内存有值 |

## 2. 集群实测（2026-10-27，kcn 新环境 10.10.1.66）

| # | 项目 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | obs 镜像替换 | PASS | `ani-observability-service:training-20261027`，digest `sha256:2fc0d9561209…`；rollout 成功，新 Pod `Running` |
| 2 | KSM allowlist 补丁 | PASS | Deployment UID `5a334163-8bdc-4216-8f97-df0553d917c9`；dry-run 与实际 args 均为 `pods=[ani.io/tenant-id,ani.io/instance-id,ani.io/workload-kind]`；KSM 单次滚动完成 |
| 3 | 训练规则加载 | PASS | `ani-training-metrics-v1` 已出现在 Prometheus rules API；8 条全 `health=ok`、无 `lastError` |
| 4 | 第三标签暴露 | PASS | 带三标签探针 Pod 出现后 `count(kube_pod_labels{label_ani_io_workload_kind="trainer"}) == 1` |
| 5 | 规则产出 | PASS | 同一时刻 `ani:training:cpu_source=1`、`ani:training:binding=1`、`ani_training_cpu_usage_cores` 与 `ani_training_memory_working_set_bytes` 各 1 条，标签恰为 7+`__name__` |
| 6 | gRPC 端到端（训练） | PASS | 训练 kind + `pod_uid` + `container="node"` → 2 条 series、各 21 点、`SERIES_STATUS_PARTIAL`；CPU 0.035 cores、内存 815104 bytes |
| 7 | 参数拒绝 | PASS | 缺 `pod_uid`、全零 `pod_uid`、训练 kind 请求网络码，三者均返回 `InvalidArgument / MONITORING_INVALID_QUERY`（不是 502） |
| 8 | 记录随 Pod 消失 | PASS | 删除探针 Pod 后 `ani_training_*` 与 `label_ani_io_workload_kind` 全为 0 |
| 9 | 实例链路无回归 | PASS | 补丁与规则期间实例 Pod 指标正常（`ani_instance_cpu_usage_cores=1`、`binding=1`）；实例 Pod 结束归零属正常 |
| 10 | 第三标签不污染实例 | PASS | 训练 canonical 中 `container="workspace"` 恒为 0 |

说明：第 4–8 项使用一次性探针 Pod（`obs-p3-probe`，busybox，带三项标签与资源限额，
runAsNonRoot + drop ALL），不属于任何执行，验证后已删除。它证明链路与规则正确，**不替代**
真实 worker/step 的验收。

## 3. 待执行（需要真实训练 Pod 与 Gov 升级）

| # | 验收项 | 方法 / 判据 |
| --- | --- | --- |
| 1 | worker 曲线 | `count(ani_training_cpu_usage_cores{namespace="<训练 ns>", container="node"}) > 0`（需 webhook 已打三标签） |
| 2 | step 曲线 | 至少一个 `container="main"` 的 step 有数据即 PASS；无数据记 `NO_DATA` 并统计覆盖率 |
| 3 | 容器名契约 | 传 `wait` 有数据但前端不展示；传错容器名 → `NO_DATA` 且不报错 |
| 4 | 历史重放 | 保存 `pod_uid` 后查询执行期间区间可返回曲线；Pod 删除后在 retention 内仍可查 |
| 5 | Gov 端到端 | 需 Gov 升级 obs API 依赖并组装训练 scope；当前 Gov 仍锁 `api v0.1.1`，训练请求不可达 |
| 6 | 容量回归 | 500 同时存活 Pod 下的 `prometheus_tsdb_head_series`、规则求值时长、写放大（上线前单列） |

## 4. 部署前事实复验（已完成）

| # | 事实 | 实测（2026-10-27，kcn） | 结论 |
| --- | --- | --- | --- |
| 1 | init 容器不进 `kube_pod_container_info` | 训练 ns 的 `kube_pod_container_info` 仅 `wait`(28)、`main`(28)、`node`(1)；`kube_pod_init_container_info` 为 `init`(28)、`kfp-launcher`(8) | 成立 |
| 2 | worker 只有 1 个常规容器、无 init | worker Pod `md-8726f819-…-node-0-0-mppxl` 的 `kube_pod_container_info` 仅 `node` 一条；step Pod 为 `main`+`wait`，init 在 `kube_pod_init_container_info` | 成立 |

因此 CONTRACT §8.3 的系数 8 / 15、单执行 143 条、8k 稳态上限继续有效。

## 5. 已声明的语义边界（不属缺陷）

- 录制粒度是每个常规容器：`wait` 有规范序列但前端不展示（CONTRACT §2 差异 2）。
- init 容器只有瞬时中间序列，无 canonical（CONTRACT §2 差异 3）。
- 训练 kind 下 `AMBIGUOUS_SOURCE` 不可达：scope 固定 `pod_uid`，同容器多 CID 由 binding
  唯一性守卫拒绝（CONTRACT §2 差异 4）。
- 已删除训练 Pod 无法按执行 id 反查：调用方必须在 Pod 存活期间保存 `pod_uid`（CONTRACT §9.3）。
- KSM 滚动重启期间 `ani_instance_*` 与 `ani_training_*` 同时出现 45–90s `NO_DATA`（CONTRACT §7.3）。
- `NO_DATA` 不能证明训练已停止或采集正常（沿用一期 §6）。
