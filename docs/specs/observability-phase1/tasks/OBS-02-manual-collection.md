# OBS-02 手工补齐采集与 recording rules

## 仓库与前置

仓库：ani-observability-service。前置：OBS-00、可使用的实际集群、Res 登记的集群 ID。可以与 OBS-01、OBS-03 并行，不修改 ani-installer。

必读：`../README.md`、`../CONTRACT.md` 第 3、6 节、`../MANUAL-METRICS.md`、`../ACCEPTANCE.md` 的 C 类用例。

## 目标

人工补齐现有 Prometheus、cAdvisor、KSM 的采集字段与规则，产出两个身份可靠的真实规范时序，并保留可再次应用、验证和恢复的文件。

## 固定改动

1. 在 `deployments/observability/collection/` 按 MANUAL-METRICS 的文件清单交付。记录真实 Prometheus、Operator、KSM 版本、namespace、资源名、selector、containerd/cgroup 模式和 Res cluster_id。原始样本必须来自环境，不能用虚构样本宣称适配成功。
2. 保存本次要改的原字段；生成可查看的字段 patch。只补 KSM ANI label allowlist、cAdvisor/KSM 采集身份标签与 15s 间隔及独立 PrometheusRule，保留已有 args、relabel 和其他采集项。手工应用到实际资源，不升级/重装整套栈，不创建新安装子系统。
3. 全部原始输入的 ani_cluster_id 与 Res、Obs datasource 一致。KSM 同时导出可信租户和实例标签。身份取自同 Pod UID 的 KSM 信息；从 cAdvisor id 解析 UID/CID，与 KSM 当前 CID、workspace、running 匹配，按固定规范化规则处理，不退化为名称关联。
4. 核实 CPU 维度，选择唯一整容器 counter，逐原始 counter 执行 rate(...[5m])；内存使用 working set。原始当前序列和 KSM 输入在录制时均须不超过 45s，旧 CID 的 5m 残留不能继续记录。
5. 去掉原始 id/job/instance/endpoint 前验证唯一性，重复源要可定位并使验收失败，不能 max/sum/labeldrop 掩盖。若过滤重复后无规范值，查询只能 NO_DATA；运行来源在查询点实际重叠才是 AMBIGUOUS_SOURCE。少量辅助规则及诊断要随包测试，不增加公开指标。
6. 固定两个 record：ani_instance_cpu_usage_cores、ani_instance_memory_working_set_bytes；最终七个标签按合同。PrometheusRule 与规则组均为 ani-instance-metrics-v1，30s 求值。按实际 ruleSelector 放置标签与 namespace。
7. 完整表达式先用真实样本制作 fixture，再用与目标 Prometheus 匹配的 promtool 检查和测试；从 CR 的 spec 提取纯规则文件后运行，不能把 Kubernetes 外壳当 promtool 规则输入。通过后人工应用、验证规则加载与实际数值，归档恢复本次字段的步骤。

## 验收

- C01–C10 按 ACCEPTANCE 执行：原始指标与两个 record 实际存在、七标签正确、字段 patch 保留旧配置、规则健康、有界负载可解释变化。
- fixture 覆盖两种已支持路径、非法 id、不足两样本、45s 边界、旧 CID 残留、重复源、同名 Pod 换 UID、KSM 信息过期和辅助 record 新鲜度。
- 不以 /metrics 200、YAML 可解析、Prom 查询空数组或 fixture 通过代替真实采集成功。
- 没有实际集群访问时可交付准备好的资料和离线测试，但此任务不能标记完成；列出所缺输入，其他独立任务继续。

## 交付

提交实际 patch/规则、脱敏原始样本、promtool 结果、手工应用记录、真实时序查询、冲突定位和恢复说明。环境密钥只记录引用。结果按 `../TASK-REPORT.md` 汇报，向 OBS-08 交付真实 datasource/cluster_id、规则启用时间、测试实例和证据路径。
