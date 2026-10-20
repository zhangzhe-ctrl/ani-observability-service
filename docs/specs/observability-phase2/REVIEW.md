# 二期阅读视图：实例网络出入速率指标

本期在一期 CPU/内存实例指标链路上**加法式**新增两个 Pod 级网络速率指标（收/发），保持既有
七标签契约、45s 新鲜度守卫、唯一性守卫与 recording 规则架构不变。

| 文件 | 读什么 |
| --- | --- |
| [CONTRACT.md](CONTRACT.md) | 合同主体：对一期条款的逐条覆盖（§1）、指标口径（§2）、语义与契约声明（§3）、实现落点与同批部署约束（§4–§5）、边界（§6） |
| [MANUAL-METRICS.md](MANUAL-METRICS.md) | 规则增量与三个易错点（插入位置 / 两份文件等价 / 括号）、promtool 与线上验证判据、verify.sh 增量、诊断与契约记录 |
| [ACCEPTANCE.md](ACCEPTANCE.md) | 已完成验收 9 项（2026-10-20）与待补验收 4 项（需真实实例） |
| [SHA256SUMS](SHA256SUMS) | 二期目录自身文件的校验值（不含本文件） |

## 快速事实

- 指标：`network.receive_bytes_per_second` / `network.transmit_bytes_per_second`，
  record `ani_instance_network_{receive,transmit}_bytes_per_second`，单位 `bytes_per_second`，
  口径 `rate(...[5m])`。
- 数据源：现有 cAdvisor 的 `container_network_{receive,transmit}_bytes_total{interface="eth0"}`，
  不新增数据源、不新增 Installer 开关。
- 请求指标数上限 2→4：Obs `query.go` 与 Gov `instance_metrics.go` 两处独立硬编码，
  **必须同批部署**。
- 网络序列 `id` 是 pause 沙箱 CID；`runtime_container_id`/`container` 由 binding 4 键投影，
  不从 `id` 提取；唯一性守卫按 4 键。
- 一期目录零改动（含 `SHA256SUMS`，仍可 `sha256sum -c` 全 PASS）。

## 分支与提交

- 分支：`feat/instance-network-metrics`（obs 仓与 Gov 仓同名）。
- obs 仓：阶段 1 规则层 → 阶段 2 Obs Go → 阶段 4 文档，各一 commit，未 push。
- Gov 仓：阶段 3，一 commit，未 push。
- 部署：待环境空闲后 Obs + Gov 两镜像同窗口替换，端到端验收见 ACCEPTANCE §2。
