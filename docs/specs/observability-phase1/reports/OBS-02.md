# OBS-02 执行报告

## 任务与结论

- 状态：进行中；真实采集已通，等待 OBS-08 的运行对象更换及整体链路收口，**未单独宣布一期完成**。
- 主链实际进度：真实 10～12 集群已补齐 KSM allowlist、两条采集链路的权威 cluster 标签和 15s 间隔；4 个 targets 均 up。8 条 recording rules、30 项 fixture、只读验证脚本及字段逆向 patch 已交付。正常 Gov/Res 创建实例的五类原始输入和七标签 canonical record 已验证，三周期 verify 真实退出 0。
- 实例：`90e2c65e-3601-44b6-9ef1-a7496c07a183`，tenant `46936e6b-3a3b-4868-8d88-6833665abb70`，namespace `tenant-46936e6b-3a3b-4868-8d88-6833665abb70`，workspace Pod UID `7ea9b16e-c89b-4e6c-b548-d13040b606e5`，CID `2fe184cb4af92095e9087a47bc8f20ea3e2944916a34bdc6df044355084f7d60`。它是 GPU 规格实例，只核 CPU/memory 指标，不冒充 CPU 规格或 GPU 指标验收。
- root 对该真实实例施加的有界负载已出现可解释 CPU/memory 增长；C07 的真实 UID/CID 更换、停止删除与历史完整业务验证由 OBS-08 协调，尚未执行。

## 改动与版本

| 仓库 | 实际起点 | 当前分支/状态 | 主要文件 |
| --- | --- | --- | --- |
| ani-observability-service | OBS-00 初始化提交 `4827e664ee7a5dc91ec4707f407838045bec718b` | `codex/observability-phase1-20261019`；本任务文件尚未单独提交 | `deployments/observability/collection/` |

本任务未改根 go.mod、Gov、Res、Installer、装配或 API。API/Res 发布版本由对应任务报告提供，不在此处虚构。

## 环境与真实配置

- Context：`kubernetes-admin@ani-lab`；API：`https://172.16.101.10:6443`。
- Nodes：ani-01/.10、ani-02/.11、ani-03/.12；Kubernetes v1.35.8、containerd 2.3.4。
- Res cluster UUID：`0fcdb8b1-22bf-4528-af59-82843dc88f21`，OBS-03 从 live Res `Service.ClusterID` 独立确认；只引用 Secret 名，不归档其内容。
- Namespace：ani-observability；Prometheus 3.11.3、Operator 0.90.1、KSM 2.19.0。
- Prometheus ruleSelector：`release: ani-metrics`；ruleNamespaceSelector：`kubernetes.io/metadata.name: ani-observability`。
- 工具引用：Fedora `/home/chabking/ani-four-service-deploy-20261012/tools/kubectl`；kubeconfig 仅引用同根 `private/kubeconfig`。
- 全部运行、容器、promtool 和集群命令经 `ssh fedora`；本地仅阅读、源码编辑、轻量语法和转运。

2026-10-19T08:37:16Z 完成三个实际 JSON Patch：

1. Deployment/ani-metrics-kube-state-metrics，UID `0cf0d7be-3146-4c5f-804e-6e25d54cf0e9`：保留原 port/resources 两参数，append 两个 ANI Pod labels 的精确 allowlist；rollout 完成。
2. ServiceMonitor/ani-metrics-kube-state-metrics，UID `aee0dd0d-2818-4b61-97b2-4266988e3355`：endpoint 0 从继承 30s 改 15s，追加权威 cluster 标签。
3. ServiceMonitor/ani-metrics-kubelet，UID `8bd50206-f199-4caf-b3a2-e6a797bf4bac`：仅 endpoint 1 的 cAdvisor 10s→15s，在原 metricRelabelings 末尾追加 cluster 标签；保留其他 endpoints 和原认证、过滤、时间戳配置。

## 验证

远程任务证据根为 `/home/chabking/ani-observability-runs/phase1-20261019-01a15321`，下表证据路径相对该目录。

| 用例 | 真实命令/操作 | 结果 | 证据 |
| --- | --- | --- | --- |
| C01 | `kubectl config view --minify` 的 context/server 字段；`get nodes`；选择对象 JSON/版本 | PASS | `logs/collection/nodes.txt`、`collection/*.before.json`、`collection/prom-buildinfo.json` |
| C02 | 五类原始 TSDB 样本及 cAdvisor exposition；正常 Res 实例五输入逐项核对 | PASS；各恰一条 fresh≤45s，owner/UID/CID 与真实对象一致 | `collection/real-90e2-*-sample.json`、`collection/real-90e2-*-age.json`、仓库 `tests/fixtures/real-resource-workspace-profile.json` |
| C03 | `count by(ani_cluster_id) (...)`，四 targets 间隔，七标签 canonical records | PASS；原始与 record cluster 标签均与 Res 一致 | `collection/*.cluster-label-after.json`、`collection/verification-real-90e2/` |
| C04 | 真实 systemd/containerd UID/CID 对照 + offline cgroupfs fixture | 实际 systemd profile PASS；cgroupfs 仅离线 PASS | 仓库 `tests/fixtures/real-systemd-containerd-profile.json`，来源清楚标记非 Res 验收 |
| C05–C08 fixture | 目标镜像 `/bin/promtool check rules rules/ani-instance.rules.yaml` 和 `test rules tests/ani-instance.rules.test.yaml` | PASS；8 rules，30 cases，真实退出码 0 | `logs/collection/promtool-check-final.log`、`logs/collection/promtool-test-30-final.log` |
| C07 真实生命周期 | 当前实例已具备正常 scope；对象更换由 OBS-08 协调 | NOT_RUN，fixture 已 PASS | `collection/resource-instance-90e2-current.json` |
| C08 现场诊断 | 固定 PromQL 逐条按实际 record timestamp 查询 | PASS；duplicate/conflicting owner/invalid path/CID mismatch 均空，五输入 age 合格 | `collection/diagnostics-real/summary.json` |
| C09 | Rule 实际选中、8rules healthy、三周期 verify、有界负载变化 | PASS（该真实 GPU 规格实例的 CPU/memory 采集）；不代表 Gov 全链路或 CPU 规格通过 | `collection/prometheusrule.after.json`、`collection/verification-real-90e2/`、`collection/load-observations/` |
| C10 | forward patch server dry-run→apply；KSM rollout；inverse patch server dry-run | PASS；恢复输出 spec 与原 spec 完全相等；实际恢复未执行 | `collection/*.dry-run.json`、`collection/*.after.json`、`logs/collection/artifact-consistency.json` |

promtool 实际执行方式：

```bash
podman run --rm --security-opt label=disable -v "$collection:/work:ro" --workdir /work --entrypoint /bin/promtool 172.16.101.10:5000/prometheus/prometheus:v3.11.3-distroless check rules rules/ani-instance.rules.yaml
podman run --rm --security-opt label=disable -v "$collection:/work:ro" --workdir /work --entrypoint /bin/promtool 172.16.101.10:5000/prometheus/prometheus:v3.11.3-distroless test rules tests/ani-instance.rules.test.yaml
```

初次 fixture 浮点/启动外推预期及一次重启期 record 数量预期失败，按目标 Prometheus 的原生 rate/staleness 语义修正测试预期后通过；没有修改生产算法或删断言。保存的失败日志 `logs/collection/promtool-test-attempt02.log` 与后续通过日志均保留。额外两规则文件 groups 的末尾换行差异已修正并核实完全一致。

采集 patch 生效后的五族真实带 cluster 标签条数：CPU 497、memory 497、Pod labels 284、container info 444、running 444。这些是全环境 source 验证；后续正常 Res 实例证据另存，不将前述计数当业务完成。

真实 rule 当前 UID `a4923d51-418e-4dc7-b244-dffb3435c5cf`，Operator ConfigMap 确实选中同 UID 文件；8 rules 全部 `health=ok`、无 lastError。两个 canonical record 首个原始样本时间均为 `1792401475.589`，即 `2026-10-19T09:17:55.589Z`，不能将 apply 时间当更早的已录制历史。

三周期 `verify.sh` 命令使用实际 tenant/instance/namespace、固定 cluster UUID、现有 kubectl/kubeconfig 引用及 `PROMETHEUS_URL=http://127.0.0.1:19098`；输出 `status=PASS,snapshots=3`、真实退出码 0，见 `logs/collection/verify-real-90e2.log`。

负载变化：首点 CPU `0.02575190093058797 cores` / memory `65642496 bytes`；`09:20:25.589Z` 变为 CPU `0.09390427708194803` / memory `273072128`；`09:22:25.589Z` CPU 达 `0.29602475114908494`。CPU 是 5m rate，加载后渐增符合窗口语义；memory 增量与 root 的实际触碰 192MiB 页及 Python 开销一致。负载由 root 通过真实实例执行，本任务只观测。

## 保留的实际失败与已恢复问题

1. **规则恢复命令验证误删了本任务新建 rule。** 初次 rule UID 为 `7c671154-8c45-45c1-adbb-180587d54e68`。使用 raw DELETE 且仅在 URL 设置 `dryRun=All` 未保留该对象；该恢复演练失败，不能写成 dry-run 通过。没有删除其他规则、业务 Pod 或 TSDB；当时尚无 canonical 历史。已重新 apply，新 UID 见上，正常选中和持续求值已验证。README 改为 DeleteOptions body 中包含 dryRun，仍标为未实际验证，不再对 live rule 做删除演练。保留 `collection/rule-uid-precondition-delete.dry-run.json`（名称为原始失败现场文件名）、`collection/rule-delete-dryrun-correction.json`、`logs/collection/rule-reapplied.log`。
2. **额外的事后 raw 重算精确相等断言失败。** `09:20:55.589Z` 冻结 CPU record 为 `0.15174507282852964`，事后 raw `rate` 为 `0.16711936559192164`，超出 rel1e-9/abs1e-12；邻接两点及 memory 相等。冻结的 `ani:instance:cpu_source` 同时为 `0.15174507282852964`，与 canonical 一致。cAdvisor 显式统计时间戳及异步抓取导致较晚写入样本改变事后 raw rate 是尚未独立证实的解释；未修改规则/断言或重试洗绿。正式 Gov 对照对象仍是最终 canonical record 的同一网格。证据 `collection/load-observations/raw-comparison-failed.json`、`historical-raw-comparison-forensic.json`、`historical-cpu-source-helper-at-difference.json`。
3. Fedora 与 Prometheus 默认 query 的时钟约相差 17 秒；同点对照使用显式 record timestamp，不把不同机器的 now 视为同一点。未修改时间配置。

## 下一项输入与限制

- 已向 root 交付真实 scope、规范起始样本时间、当前 rule UID、三周期成功证据及负载观测，供 OBS-08 同网格 Gov/Prom 对照。
- UID/CID 更换及已停止/删除历史仍待 OBS-08 正常生命周期验证；不自行操作业务对象。
- 只读脚本不操作业务实例；有界负载、CID/UID 更换、停止/删除与历史归 OBS-08 协调。
- 用户明确延期 Prometheus 底层网络/写入限制和到期租户策略；未部署 netpol/认证代理/额外 Prometheus mTLS，不能将该边界标安全通过。
- 恢复 patch 已准备并 server dry-run；未进行实际恢复演练。后续 Helm 操作可能覆盖手工字段。
