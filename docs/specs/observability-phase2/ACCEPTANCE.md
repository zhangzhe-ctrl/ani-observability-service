# 二期验收：实例网络出入速率指标

验收分两部分：**已完成的验收**（规则层 + 服务层，无实例前提下可闭环）与**待补的验收**
（需要真实 workspace 实例）。本文不宣称未执行的验收。

## 1. 已完成（2026-10-20）

| # | 验收项 | 结果 | 证据 |
| --- | --- | --- | --- |
| 1 | promtool 3.11.3 `check rules`（12 条规则） | PASS | `SUCCESS: 12 rules found`（本地 promtool 3.11.3，与集群版本一致） |
| 2 | promtool 3.11.3 `test rules`（41 个用例，含 11 个网络用例与括号回归） | PASS | `SUCCESS`（41/41） |
| 3 | YAML 与 JSON 两份规则文件 expr 逐字等价（12 条，含末尾换行） | PASS | 生成时程序化比对 |
| 4 | Obs `go test ./internal/... -count=1`（3/4 指标通过、5 指标拒绝、网络目录登记） | PASS | 全部包 ok |
| 5 | Gov `go test ./app/admin/service/internal/server -count=1`（latest 四 code 顺序、4 通过/5 拒绝） | PASS | ok |
| 6 | 一期包完整性：`docs/specs/observability-phase1/` 内 27 文件 `sha256sum -c SHA256SUMS` | PASS | 全 PASS，路线 C 未被违反 |
| 7 | 规则 apply 后线上健康：12 条全 `health=ok`、无 `lastError` | PASS | ani-01 `/api/v1/rules` 实测 |
| 8 | helper 真实数据产出：各 112 条 = 原始 `interface="eth0"` 序列数，`pod_uid` 提取正确，`max_age < 30s` | PASS | 只读 PromQL 实测（验证判据见 MANUAL-METRICS §2） |
| 9 | canonical 无实例时为空 | PASS（符合预期） | 集群无 workspace 实例，binding 为空 |

## 2. 待补（需要真实 workspace 实例）

| # | 验收项 | 方法 |
| --- | --- | --- |
| 1 | 端到端三快照 | `verify.sh`（已扩展网络分支），`SNAPSHOTS=3`，参照一期 OBS-08 深度 |
| 2 | Gov HTTP 实测 | `GET /api/v1/instances/{id}/metrics/latest` 返回 4 个 series（顺序 CPU→内存→网络收→网络发，单位 cores/bytes/bytes_per_second/bytes_per_second）；`POST .../query` 4 个 code 200；5 个 code 400 |
| 3 | 数值合理性 | 实例内产生已知流量后，网络值与流量方向、量级一致（rate 非负） |
| 4 | 前端渲染 | `bytes_per_second` 单位渲染（前端对接文档单独出） |

## 3. 已声明的语义边界（不属缺陷）

- Pod 级语义：sidecar 流量计入（CONTRACT §3.1）。
- hostNetwork 实例恒 NO_DATA；实例 Pod 无 eth0 时网络 NO_DATA 而 CPU/内存正常
  （CONTRACT §3.2）。
- 更换 CNI 后 `eth0` 命中 0 条 → 网络 NO_DATA，需重新验证（CONTRACT §3.3）。
