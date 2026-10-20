# 二期手工采集增量：实例网络出入速率指标

本文是 `docs/specs/observability-phase1/MANUAL-METRICS.md` 的**增量补充**，不替代一期文件。
一期的人工操作、patch、恢复说明继续有效。本期不改任何 collector patch、不加 Installer 开关，
只扩展规则组与配套验证资料。

## 1. 规则增量

规则组仍为 `ani-instance-metrics-v1`（30s），由 8 条扩为 12 条：

```text
正确顺序（组内按文件顺序求值）：
  ani:instance:cpu_source          （一期）
  ani:instance:memory_source       （一期）
  ani:instance:info_source         （一期）
  ani:instance:running_source      （一期）
  ani:instance:labels_source       （一期）
  ani:instance:network_receive_source    ← 新增 helper
  ani:instance:network_transmit_source   ← 新增 helper
  ani:instance:binding             （一期）
  ani_instance_cpu_usage_cores           （一期）
  ani_instance_memory_working_set_bytes  （一期）
  ani_instance_network_receive_bytes_per_second   ← 新增 canonical
  ani_instance_network_transmit_bytes_per_second  ← 新增 canonical
```

**易错点 1（插入位置）**：两个 helper 若追加到文件末尾，canonical 在同一周期内只能读到上一
周期的 helper；因 canonical 带 `timestamp(helper) == time()` 快照守卫，网络 canonical 将恒为
空，且规则 health 仍为 `ok`、无 `lastError`，极难排查。helper 必须插在 `ani:instance:binding`
之前。

**易错点 2（两份文件等价）**：`rules/ani-instance.rules.yaml`（纯 groups 文件，供 promtool）
与 `rules/ani-instance-metrics-v1.yaml`（PrometheusRule 对象，实际 apply）必须逐字等价：
PromQL 双引号字符串内的 `\` 写作 `\\.`（YAML 块标量原样保留）；JSON 在 PromQL 转义之上再
做 JSON 转义，写作 `\\\\.`；JSON 版 expr 末尾带 `\n`。

**易错点 3（括号）**：canonical 中 helper 引用与乘积的外层括号是承重的
（`(helper) * on(...) group_left(...) (binding)` 与去掉括号的解析结果不同）；规则文本与
`tests/ani-instance.rules.test.yaml` 用例 1 的括号回归断言共同固化该形态。

helper 结构要点（与一期 `cpu_source` 同构）：

- 选择器 `container_network_{receive,transmit}_bytes_total{interface="eth0",ani_cluster_id=~"<UUID>",namespace!="",pod!=""}`，**不追加** `container="workspace"`；
- 只从 `id` 提取 `pod_uid`（systemd 模板 `$1-$2-$3-$4-$5`，cgroupfs `$1`），不提取
  `runtime_container_id`——网络序列的 `id` 是 pause 沙箱 CID；
- `rate(...[5m])` + 45s 双向新鲜度守卫 + `and on(id) {__name__=...,id=~"<同一正则>"}` 形状过滤
  + systemd/cgroupfs 双分支 `or`。

canonical 与一期 `memory` canonical 逐字对齐：join 键 6→4（`ani_cluster_id,namespace,pod,pod_uid`）、
`group_left` 投影 2→4（`ani_tenant_id,ani_instance_id,runtime_container_id,container`），
其余（`sum by` 七标签、`count by` 唯一性守卫、binding 快照守卫）一致。

## 2. 验证

### promtool（目标版本 3.11.3，与集群一致）

```bash
promtool check rules rules/ani-instance.rules.yaml        # 期望 SUCCESS: 12 rules found
promtool test rules tests/ani-instance.rules.test.yaml    # 期望 41 个用例全 PASS
```

fixtures 新增 11 个网络用例：正常双向（含括号回归断言）、hostNetwork（无 eth0）、原始样本
陈旧、45s 含端点边界、Pod 级重复（守卫拦住不相加）、binding 缺失、binding 容器非 workspace、
binding 陈旧、cgroupfs 分支、未知 cgroup 路径、rate 数值精确性。业务身份全部为合成值。

### 线上 apply 与只读验证（2026-10-20 已执行，ani-01）

```bash
kubectl -n ani-observability apply --dry-run=server -f rules/ani-instance-metrics-v1.yaml
kubectl -n ani-observability apply -f rules/ani-instance-metrics-v1.yaml
```

原地更新（UID 不变）。验证判据与实测结果：

| 检查 | 判据 | 实测 |
| --- | --- | --- |
| 规则加载 | `/api/v1/rules` 组内 12 条全 `health=ok`、无 `lastError` | PASS |
| 原始序列 | `count(container_network_{receive,transmit}_bytes_total{interface="eth0"})` | 各 112 |
| helper 当前周期产出 | `count(helper)` = 原始序列数 且 `max(time()-timestamp(helper)) < 30s` | 112、26.7s |
| `pod_uid` 提取 | helper 样本带规范 UUID `pod_uid`，无 `runtime_container_id`/`container` | PASS |
| canonical | 无 workspace 实例时为 0 条（binding 为空） | 0（符合预期） |

**HTTP 侧验证注意**：`timestamp(helper) == time()` 在任意时刻的 HTTP 查询中几乎恒假——求值
时间戳为整数毫秒除以 1000 的浮点，与查询参数字符串解析的浮点相差 1 ULP（一期 fixture 的
one-ULP 比较选项即此因）。对线上 helper 的「当前周期产出」验证必须使用上表 age 判据，不要
用等号对齐。

## 3. verify.sh 增量

- 规则数断言 8→12；
- `raw_names` 追加两个网络原始指标；
- 网络指标走**独立校验分支**：选择器追加 `interface="eth0"` 且**不追加** `container`；
  身份校验只比对「从 `id` 解析出的 `pod_uid` == KSM 的 uid」，不比对沙箱 CID；
  `len(rows)==1` 同时充当「同一 4 键不多于 1 条」的断言；
- canonical 循环追加两个 record（七标签与 `runtime_container_id == cid` 断言直接复用）。

端到端 `verify.sh` 依赖真实 workspace 实例 Pod，当前集群无实例，待实例可用后按一期流程
执行三快照验证。

## 4. 诊断与契约记录

- `diagnostics.promql` 追加网络段：helper Pod 级重复计数（4 键）、原始序列新鲜度、未知
  cgroup 路径、helper↔binding 身份 `unless`（**必须 4 键**——helper 无
  `runtime_container_id`/`container`，6 键 `on()` 恒空会误报全部序列）。
- `source-profile.yaml` 的 `network:` 段记录：来源指标、`eth0` 收敛依据、Pod 级语义、CNI
  依赖契约、条件式覆盖、身份投影、唯一性分组与线上实测记录。
