# OBS-02 人工补齐实例指标采集与录制

## 1. 目的、适用范围与交付状态

本专项将 v1.1 的 Installer 工作替换为现有监控环境的人工字段变更。公共约定见 [CONTRACT.md](./CONTRACT.md)，验收见 [ACCEPTANCE.md](./ACCEPTANCE.md)，执行任务为 [OBS-02-manual-collection.md](./tasks/OBS-02-manual-collection.md)。

**当前没有连接真实集群或取得真实指标样本。** 本文件规定实施要求，不提供号称已经验证、可以直接 apply 的完整 PromQL/YAML。OBS-02 先取真实样本，再交付对应配置、规则和验证证据。

仅复用现有 Prometheus、KSM、kubelet/cAdvisor，为 Res 实例的 `workspace` 容器补齐 CPU 核数和 memory working set 字节数。Gov 授权并解析 Res 可信 scope，Obs 查询 Prometheus；该链路保持原方案。静态多 cluster 映射继续支持，真实验收只要求一个 cluster，标识与 Res/Obs 一致。

**不改 `ani-installer` Go、模板、values 或验收逻辑。** 不执行 Helm install/upgrade/rollback，不改全局 selector；通过手工命令、实际 patch、一个只读验证脚本完成工作，不建设安装或回滚工具。不扩展其他指标、日志、告警或新的状态机。

## 2. 固定输出和时间语义

### 2.1 两条业务 recording 指标

| 指标 | 数值口径 |
| --- | --- |
| `ani_instance_cpu_usage_cores` | 对同一实际运行容器的原始 CPU counter 执行 `rate(...[5m])`，单位为核 |
| `ani_instance_memory_working_set_bytes` | 同一实际运行容器的 `container_memory_working_set_bytes`，单位为字节 |

两条最终业务 record 必须仅保留以下七个标签；原始诊断信息在投影前处理，不混入最终规范记录。Gov 不向外返回这些底层标签：

```text
ani_cluster_id
ani_tenant_id
ani_instance_id
namespace
pod_uid
runtime_container_id
container
```

`container` 固定为 `workspace`。不得纳入 sidecar、基础设施容器、Pod 聚合值、其他业务容器或百分比。`pod_uid` 与 `runtime_container_id` 一起区分同名 Pod 重建和同一 Pod 中容器重启；业务实例身份保持稳定。

Kubernetes 对象名和 rule group 名均为 `ani-instance-metrics-v1`。基线选择标签为 `release: ani-metrics`，但应用前必须读取真实 Prometheus 的 `ruleSelector` 和 `ruleNamespaceSelector`。若真实部署使用不同选择标签，只给本对象生成实际需要的标签并记录依据，不修改全局 selector 来迁就新规则。规则命名空间由真实 selector 决定，不猜测部署名称。

### 2.2 固定参数

| 参数 | 本方案值 | 实现要求 |
| --- | --- | --- |
| cAdvisor / KSM scrape interval | 15 s | 对选定采集端点作局部调整，保留其他端点配置 |
| recording interval | 30 s | 同一规则组按依赖顺序评估 |
| 原始 CPU / memory 样本新鲜度 | ≤45 s | 在原始输入上判断，不能用中间 record 的新时间戳代替 |
| KSM 样本新鲜度 | ≤45 s | 对参与关联的每类原始 KSM 输入分别判断 |
| 查询 record 新鲜度 | ≤45 s | 相对于每个查询评估时刻判断 |
| CPU rate window | 5 min | 至少两个有效原始样本；保留 counter 的物理与采集来源身份 |

CPU 先对同一原始 counter 执行 rate，再严格关联身份；不跨 UID、CID 或抓取源聚合 counter。五分钟窗口至少两个有效样本即可参与，启动阶段无需填满五分钟。它是包含 Prometheus 边界外推的窗口估计，不是完整五分钟的严格平均；样本不足时缺失，不补零。

45 秒 freshness 约束本点最新有效原始样本，不要求五分钟窗口里所有样本年龄都 ≤45 秒。窗口内缺采沿用既定 rate 语义，不补零或换窗口。Memory 是近期 gauge，不是 30 秒区间平均；采集时差和生命周期过渡缺口如实保留。

## 3. 实现归档目录与文件清单

实施文件固定归档在 **`ani-observability-service/deployments/observability/collection/`**。只要求以下资料和一个只读脚本：

| 文件 | 必须交付的内容 |
| --- | --- |
| `README.md` | 真实环境参数、取证/备份/应用/验证的逐步手工命令、字段差异、预期输出、恢复说明与恢复命令 |
| `source-profile.yaml` | 实际 cluster ID、对象/selector、runtime/cgroup、来源标签、UID/CID 规范化与时间戳行为；不含凭据 |
| `patches/ksm-workload.patch.json` | 合并 allowlist 的实际字段 patch |
| `patches/ksm-servicemonitor.patch.json` | KSM 实际采集端点的字段 patch |
| `patches/cadvisor-servicemonitor.patch.json` | cAdvisor 实际采集端点的字段 patch |
| `patches/restore-*.patch.json` | 按本次改动对象保存的逆向字段 patch；执行前仍须核对当前值 |
| `rules/ani-instance.rules.yaml` | 两条规范 record 及实际必要的内部辅助规则，完整可运行表达式 |
| `rules/ani-instance-metrics-v1.yaml` | 实际 PrometheusRule，使用上述已验证 groups、真实 namespace 与 selector 标签 |
| `diagnostics.promql` | 当前重复来源、关联冲突、解析失败、新鲜度和 CID 不匹配的只读诊断表达式及注释 |
| `tests/ani-instance.rules.test.yaml`、`tests/fixtures/` | promtool 时间轴、输入、预期标签/数值/缺失；真实脱敏样本与合成负例分别标注 |
| `verify.sh` | 唯一要求实现的脚本，只读验证并输出证据，不修改集群、不操作业务实例 |

无需修改的对象不制造空 patch，README 记录“原配置已满足”。取证、patch 编写、应用和恢复均由手工命令完成，不要求专用生成器或程序。两份规则文件中的 groups 必须一致，不能人工复制后各自漂移。

备份和执行输出保存在 `collection/artifacts/manual-metrics/<run-id>/`，忽略提交。保留原始对象及 UID/resourceVersion、修改字段的原值和应用后值、实际 patch、targets/样本、promtool 输出、验证与恢复结果；无需另建固定目录框架。Git 中只提交必要的脱敏样本和配置资料，凭据、未脱敏对象及私有运行备份不提交。

## 4. 阶段 A：真实环境取证与变更前备份

先记录 Res 的稳定 `cluster_id`、kubectl context、API server 身份、Prometheus 地址和验收实例的可信 scope。cluster ID 不使用 context/release/显示名。实例必须来自正常 Res 流程，读取其真实 Pod UID、主容器 CID 和 owner 标签，不用手工打标签的任意 Pod 冒充。

按以下顺序取证，将实际 `kubectl get` 和 Prometheus 只读请求写入 README：

1. 保存 Prometheus CR/版本、规则与采集 selector、现有规则组；保存 KSM workload/args、KSM 与 cAdvisor ServiceMonitor。记录对象 kind、namespace、name、UID、resourceVersion、Helm 所有权和拟改字段原值。
2. 查看实际 scrape targets，确认 job/instance/endpoint/metrics path 和重复发现。既要保存 exposition，也要保存 TSDB 落库样本；单个 endpoint 返回正常不证明 Prometheus 已正确存储。
3. 保存 CPU、memory、Pod labels、container info、running 五类样本，核实 allowlist 转换名称、UID/CID、runtime/cgroup、CPU 整容器维度、时间戳和数个采集周期的变化。
4. 把不满足项列成字段差异，填入 source profile，编写实际 patch 和逆向恢复 patch。后续 Helm 操作可能覆盖这些人工字段，报告记录这一点，留给后续部署工作接收。

## 5. 阶段 B：保留配置的人工补采集

### 5.1 允许修改的字段

先生成、审查并保存字段差异，再逐项执行 JSON Patch。每个 patch 包含对象身份和原值的前置检查，防止在对象已被替换或同一字段已变化时继续套用旧计划。

- **KSM allowlist：** 在现有 `--metric-labels-allowlist` 配置中合并 `pods=[ani.io/tenant-id,ani.io/instance-id]`。保留其他资源类型、已有 Pod 标签、其他启动参数及其行为；不得用这两项覆盖整份 allowlist，不启用通配符。
- **KSM 和 cAdvisor 采集：** 对实际相关端点设定 15 秒间隔，合并该 profile 需要的 relabel/metric relabel。保留已有 metric 路径、认证/TLS、已有筛选规则和其他端点。不能替换整个 `endpoints` 或 relabel 数组而丢掉原项。
- **Cluster 标签：** 在两条采集链路进入本地 TSDB 前注入同一权威 `ani_cluster_id`，不信任 exporter 自报的冲突值。只设置 Prometheus `external_labels` 无法作为本地关联已经具备该标签的证明。
- **时间戳：** 核实 cAdvisor 原始样本的显式时间戳是否由 Prometheus 保留。原始 freshness 承诺依赖该行为；若必须调整相关端点的 `honorTimestamps` 或 staleness 配置，应单列字段差异与测试依据。不得把 exporter 重复返回的旧统计值重新赋予 scrape 时间，再声称统计值是新的。

KSM 样本新鲜度表示该 KSM 指标样本近期可用，不代表 Kubernetes API watch 缓存与 cAdvisor 在现实世界中原子同步。该边界由严格 UID/CID 匹配和缺失处理兜住，不增加新的同步状态机。

修改 KSM 参数通常会引起其 workload rollout。操作前在字段差异报告中写清影响对象；完成后等 KSM 恢复可抓取并核验标签，再进行规则加载。不得为验证而重启或删除业务 Pod。

### 5.2 来源 profile 与规范化

本期只接收经实际样本验证的 Linux/containerd profile；systemd 与 cgroupfs 分别按样本实现，不用一个宽泛正则猜所有格式。

| 来源 | 必须得到的规范值 |
| --- | --- |
| cAdvisor `id` | 提取真实 Pod UID 和运行容器 CID；原始 `id` 留到 rate 与唯一性检查结束 |
| KSM `uid` | 标准 UUID，五组格式 `8-4-4-4-12`，与原始来源规范值完全相同 |
| KSM `container_id` | 验证 `containerd://` 前缀后提取 64 位小写十六进制 CID |
| systemd cgroup 路径 | 仅对已识别的 Pod UID 段把下划线恢复为连字符；识别并去除 CID 段的 `cri-containerd-` 与 `.scope` |
| cgroupfs cgroup 路径 | 从已验证的 `pod<UID>/<CID>` 结构提取 UID 和完整 CID |

两个输入都必须执行同一规范化验证，不能只有 cAdvisor 一边转换。CID 不截短，不接受带前后缀的最终值；Pod UID 不允许用 Pod 名称、Notebook 名称或 intent version 代替。未知格式、缺字段、解析不唯一或实际 CPU profile 不符时，明确拒绝该 profile 的验收，不自由发挥为“按 namespace + pod 关联”或“把所有 CPU 分量相加”。

KSM 必须提供 `kube_pod_labels` 中的 `uid` 和 owner 标签，以及 `kube_pod_container_info{container="workspace"}` 中的同一 `uid`、当前 `container_id`，再以 `kube_pod_container_status_running == 1` 约束运行状态。标签 allowlist 转换结果预计为 `label_ani_io_tenant_id`、`label_ani_io_instance_id`；实际名称、冲突后缀必须在真实样本中核实。owner 标签应来自 Res 写入的 Pod metadata，不能假定 cAdvisor 自动带出这些标签。

## 6. 阶段 C：完整 recording rules 的生成契约

OBS-02 按以下顺序生成完整表达式。这里规定算法和可验收不变量，不提供未经真实样本验证的“通用可 apply 规则”。任何内部 current/snapshot 或 diagnostic 辅助 record 都必须与最终规则同包交付、同样测试，不得藏在手工终端中。

1. **筛选与规范化原始输入。** 仅选中 `workspace` 的目标 CPU total counter、memory gauge 和三类 KSM 输入。保留原始 cgroup ID、来源 job/instance/endpoint 等标签；注入权威 cluster ID，规范化 UID/CID。
2. **检查原始 freshness。** 在原始样本上，以本次规则评估时刻减去输入样本时间戳，检查 ≤45 秒。KSM 的 Pod labels、container info 和 running 状态分别检查，不能通过一个新写入的 snapshot 时间戳替代所有输入检查。
3. **检查来源唯一性。** 对每类输入按对应物理身份检查有效来源数量；同一 UID/CID 被多个抓取源重复提供，或同一 UID 的业务身份/CID 关联存在冲突，先诊断并抑制该身份的输出。必须在删除 job/instance/endpoint 等标签之前完成，避免 many-to-many join 或落库标签冲突影响整个规则组。
4. **建立当前容器身份关联。** 通过 cluster、namespace、Pod、Pod UID、container 名称以及规范化的当前 CID 建立严格关联条件。Pod 名只作为一致性检查字段，不能替代 UID。已有同名 Pod 或历史 CID 不因名字相同而获得关联。本步构建匹配条件，不把关联后的 counter 重新录制、聚合后再执行 rate。
5. **计算 CPU 与 memory。** CPU 对原始、未混合的 counter 执行 5 分钟 rate，至少两个有效样本，并与当前仍然存在且 fresh 的同一条原始来源相交；相交键必须保留完整采集来源身份。只按 UID/CID 相交可能把已经消失的旧抓取源 rate 重新带入。Memory 使用同一合格身份的近期 working set gauge。
6. **写入最终 record。** 只有来源唯一、身份匹配且原始输入合格时，投影到固定七标签并写入两条最终 record。内部辅助规则按依赖顺序位于同一 `ani-instance-metrics-v1` 规则组，统一 30 秒评估。

### 6.1 Current / snapshot 必须避免的错误

`rate(raw[5m])` 在原始容器已消失后仍可能存在，不能把“窗口里还有 counter”当作当前运行身份。只要求其原始样本年龄 ≤45 秒，也不能替代当前 CID 匹配。

如果先录制 current/snapshot，再让最终规则读取它，必须证明：snapshot 只由本轮合格的原始输入生成，保留真实 UID/CID，并且下游不会通过默认 lookback 读回旧 snapshot 来继续录值。实现需对本轮 snapshot 的评估时间或等价的原始来源时间实施明确约束，并用同 UID 换 CID、消失与重复源 fixture 证明。**`timestamp(snapshot)` 刚刚刷新，不代表 snapshot 内部采用的原始 CPU、memory 或 KSM 身份也刚刚刷新。**

同组顺序评估只能统一规则的评估时刻，不能让 KSM 与 cAdvisor 的异步采集变成现实世界中的原子快照。过渡时刻允许出现缺失；不通过补零、延长窗口、复用上一 CID 或选最新 Pod 把缺口藏起来。

### 6.2 重复采集与多运行身份是两种裁决

| 情况 | recording / 验收处理 | Obs 仅查询最终 record 时能表达的结果 |
| --- | --- | --- |
| 同一物理 UID/CID 由多个原始采集源提供 | 在投影前识别并抑制受影响输出；验收失败，记录当前冲突并修正重复抓取 | 该点没有合格 record 时为 `NO_DATA`，不能反推出 `AMBIGUOUS_SOURCE` |
| 规范 record 在同一查询时刻存在多个不同 UID/CID | 保留实际已经写入的各身份，不能求和或选择一份 | `null` 与 `AMBIGUOUS_SOURCE` |
| 输入 stale、标签缺失、CID 不匹配、CPU 样本不足 | 不写入该点业务值，诊断记录实际证据 | `NO_DATA`，不擅自判为停止或采集故障 |

原始冲突经抑制后，两条业务数值 record 无法事后还原原因。本期仅要求 `diagnostics.promql` 能诊断当前冲突，并保存本次验证证据，**不要求建设历史冲突存储**。只有实际实现并验证了内部 diagnostic record 等保存方式，才可承诺相应历史诊断能力；该项可选。内部记录不构成第三个公开业务指标，不新增 HTTP 指标接口或前端状态码。

不得用全局 `labeldrop`、`max`、`sum`、只取最新源或任意丢弃副本标签消除重复。规范标签仅在唯一性已经证明的结果上投影，不改变其他业务 metrics 的标签和采集行为。

## 7. 阶段 D：离线验证与应用顺序

### 7.1 离线验证先于规则应用

使用与目标 Prometheus 版本相容的 promtool，保存以下命令结果：

```bash
promtool check rules deployments/observability/collection/rules/ani-instance.rules.yaml
promtool test rules deployments/observability/collection/tests/ani-instance.rules.test.yaml
```

语法通过不代表关联正确。固定输入时间、评估点和预期标签，至少覆盖：路径有效/拒绝样本；同名不同 UID 与同 UID 新 CID；旧 counter 残留、reset、一个/两个样本及 memory 独立可用；45 秒边界与 snapshot 重刷；同身份多个 job、KSM 重复或身份冲突；跨 cluster/tenant 防误关联；删除后历史；旧新 CID 的查询重叠。原始重复应抑制输出，当前 diagnostics 可核验；最终多身份不汇总。合成故障样本与真实脱敏样本分开标注，不冒充真实测试。

### 7.2 人工应用顺序

1. 检查 context、cluster ID、对象 UID 和涉及字段仍与基线一致；变化时重新读取并修订 patch，保留他人变更。
2. 审查 README 字段差异与恢复值，对 `patches/` 中的实际 patch 和规则对象执行服务端 dry-run。
3. 手工应用 KSM allowlist patch，等采集组件恢复，再应用两个 ServiceMonitor patch；保存各步应用后值。
4. 用 `verify.sh` 检查 targets、TSDB 样本、15 秒间隔、cluster/owner 标签、UID/CID、时间戳和唯一性。失败则修复对应配置，暂停规则加载。
5. 应用 `ani-instance-metrics-v1`，确认被实际 selector 选中、完整规则加载且连续数次评估成功，再做真实联调。不得修改全局 selector。

README 使用实际对象参数给出逐步命令。例如字段 patch 先执行下面的 dry-run，再在该步骤明确移除 `--dry-run=server` 应用；变量从本次 profile 人工填写，不要求配置渲染程序：

```bash
kubectl --context "$obs_context" -n "$obs_namespace" patch "$obs_kind" "$obs_name" --type=json --patch-file "$obs_patch_file" --dry-run=server -o yaml
```

每步失败即停。

## 8. 阶段 E：真实验收与历史查询约束

在一个真实 cluster 上，用正常 Res 实例证明可信 scope 与七标签一致，UID/CID 对应实际 Pod 和主容器，owner 标签来源正确，CPU 为 cores、内存为 bytes；在同一时间点对照原始表达式和规范 record。返回 `mode: real` 不能作为证据。

通过正常 Res 生命周期验证停止/重启/重建及其缺失点，`verify.sh` 自身不操作业务实例。某个场景未在真实环境执行时，明确标为仅离线通过或待实测。授权和持久化范围仍有效、数据在保留期内时，已删除 Pod 的历史仍可查询；历史只读已录制数据，不 join 当前 KSM，不回填规则上线前的原始历史。

**查询 freshness 按每个历史评估时刻 `t` 检查。** 用 `time() - timestamp(record) <= 45` 或严格等价的过滤保留合格 record，不能与当前墙钟比较；`query_range` 矩阵点时间是查询评估时间，不能代替实际 record 样本时间。

每个指标、每个点独立裁决：0 个来源为 `null/NO_DATA`；1 个且有限非负则返回数值；多个不同 UID/CID 为 `null/AMBIGUOUS_SOURCE`。仅一个来源但 NaN、无穷或负数时为 `null/INVALID_VALUE`，多来源裁决优先。CPU warmup 不强迫有效 memory 同时缺失。

旧 CID 最后一个 record 与新 CID 首个 record 可能同时满足 45 秒 freshness，按多来源处理，不相加两个 trailing rate、不选最新，也不借当前 KSM 改写历史。当前规则健康属于部署验收；不为历史查询新增每请求 `/api/v1/rules` 检查，不让 Obs `/readyz` 依赖各数据源即时健康。

## 9. 恢复步骤与并发修改保护

README 逐对象列出恢复命令和 `patches/restore-*.patch.json` 的适用前提，人工恢复仅撤销本次字段：

1. 核对当前对象 UID，以及字段原值、本任务应用值、当前值。对象重建或相关字段被他人修改时暂停该项，先核对差异；保留其他字段的并发改动。
2. 优先恢复规则：本次新建的规则只在对象名与记录 UID 一致时删除；原先已有则恢复本次改动字段。
3. 恢复两类 ServiceMonitor 的 interval/relabel/时间戳字段，再恢复 KSM allowlist/args。数组按精确差异恢复，不能整个旧数组覆盖回去；等待采集组件恢复。
4. 验证原有 targets、采集与规则，记录已恢复、无需恢复和因并发修改待处理的项。

不将整个旧 YAML 重新 apply，不执行 Helm rollback，不删除 namespace、PVC、Prometheus 数据目录或他人的规则，不重启业务实例。移除新规则不删除已有 TSDB 历史，本任务不清理历史数据。恢复说明和 patch 是交付物，不要求实现回滚程序。

## 10. OBS-02 完成报告

报告链接手工命令、实际 patch、规则、fixture、promtool 和真实验证结果，逐项标注已实测、仅离线或待真实验证。写清 cluster 映射、对象/selector、来源 profile、两侧 UID/CID 规范化、owner 标签、字段差异，以及 freshness/current CID/重复来源的验收结果。恢复部分区分“说明和 patch 已准备”与“已执行演练”，有演练才写实测结论。

来源若不能提供严格匹配的 UID/CID 或原始 freshness 依据，应明确该 profile 未通过，提出使其满足契约的最小配置追加；不能退化为弱关联后标记完成。

## 11. 核对依据

以下定义不构成真实部署证据：[recording rules](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/)、[rate 与 timestamp](https://prometheus.io/docs/prometheus/latest/querying/functions/)、[lookback/staleness](https://prometheus.io/docs/prometheus/latest/querying/basics/)、[relabel 与时间戳配置](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)、[KSM Pod 指标](https://github.com/kubernetes/kube-state-metrics/blob/v2.19.0/docs/metrics/workload/pod-metrics.md)、[kubelet cAdvisor 标签](https://github.com/kubernetes/kubernetes/blob/v1.35.8/pkg/kubelet/server/server.go#L1218-L1252)。运行环境版本和完整样本仍须由 OBS-02 核实。

