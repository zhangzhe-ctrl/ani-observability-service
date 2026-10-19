# OBS-09 版本与交付收口

## 任务与结论

状态：按用户调整后的范围已完成。OBS-00至OBS-08产物、正式版本、实际手工采集和真实后端证据已存在；没有以文件存在、HTTP200或mock代替真实数据。

当前能力：正常 Gov 用户通过两个固定公共接口查询容器实例 workspace 的 CPU/内存；Gov 从 live Res 获取持久化可信范围，以 TLS1.3/mTLS 调用 Observability，只查询指定集群两个固定 record。正常删除后持久化范围与历史指标仍可查询。没有前端、日志/告警假接口、任意PromQL、额外存储、配额/调度改造或 Installer 改动。

原始 v1.2 并非全数 PASS。用户确认的调整见 `../EXECUTION-OVERRIDES.md`：独立CPU规格实例和有效无instance:read账号真实负例为 SKIPPED_BY_USER；Prometheus底层额外隔离、到期READONLY策略为 DEFERRED_BY_USER。所有被跳过/延期项均未标成实际通过。

## 版本与提交

完整机器可读锁定见 `version-lock.yaml`。三个实现仓库的源码与版本已远端可获取：

| 产物 | 真实版本/提交 | 状态 |
| --- | --- | --- |
| Observability API | `api/v0.1.1`，`44728ffbb722b30194a0cd8a25e60c5022ed0dfc` | 真正下载并验证Origin/校验和，API无根模块反向依赖；最初v0.1.0标签未移动 |
| Observability服务 | `v0.1.0`，`a5bef5d0a8ff8f97d1a6bae1d53227548ae387a4` | 独占新缓存从官方Go代理下载，Origin.Hash/Ref与tag精确匹配；实际镜像Version=v0.1.0 |
| Res范围RPC | `9fa3b4aa41f90db9ca0141a2b715904073f6e2d2`；`v0.0.0-20261019084504-9fa3b4aa41f9` | 已推送、已正式获取、已部署 |
| Gov | `c905e49170b0dfeb23cf014cb4759a4e4ca2686d` | 已推送、已部署；真实pin API1.1和上述Res版本 |

源码分支分别为新服务/Gov的 `codex/observability-phase1-20261019`、Res的 `codex/observability-scope-20261019`。未擅自合并、强推或改动其他工作树。发布服务tag固定；后续文档与实际部署digest回填提交仅归档证据，不为回填自身SHA再建镜像。

三个实际 OCI manifest digest 均在version-lock记录。不要把podman源manifest、OCI index、config和实际导入manifest当成同一个值；Obs运行时CRI显示index的对应关系详见 OBS-06。registry实际只读405；使用任务自有loader导入10–12，未修改registry/installer。

## 质量验证与证据

全部重任务在 Fedora 根目录 `/home/chabking/ani-observability-runs/phase1-20261019-01a15321`；正式验证 GOWORK=off，模块图无本地Replace。

| 领域 | 真实命令/结果 | 证据 |
| --- | --- | --- |
| 模板/根API | `make generate`、`make verify`、`make build` PASS；API独立tidy-diff/test/vet/mod verify；安全补丁后联合verify再次PASS | `logs/bootstrap/service-security-final-verify.log`、`api-security-full-verify.log` |
| 根安全修复 | 首次vuln FAIL真实发现gRPC/x/net/OTel可达漏洞，修复为Go1.26.9/grpc1.83.2/xnet0.60/xtext0.42/OTel1.45后PASS，未忽略/关闭校验 | `vulnerability.log`、`vulnerability-repaired.log`、`release-audit.log` |
| 根完整audit | `make audit` PASS：vuln、全部Git历史secrets、完整SBOM、冻结notice、发布API来源及第三方license证据 | `logs/bootstrap/release-audit.log` |
| 来源/许可证 | 唯一自有API许可证UNDECLARED保持原状，34个第三方保持SPDX要求，严格核对发布来源/双checksum/BOM；故意移除第三方license被明确拒绝 | `root-first-party-reviewed.log`、`root-third-party-negative.log`；`docs/scaffold/` |
| 最终源码一致 | tag a5与业务验证071的生产Go、配置、Dockerfile、Go模块、生成入口逐文件无差异；仅正式版本metadata重建，不重跑不变逻辑全测 | `runtime/final/a5bef5d0a8ff8f97d1a6bae1d53227548ae387a4/`；tree SHA256 `17807cac44f8d150d294cf94cc6a76723105dafd1828b06e7053e2f7c2866129` |
| Res | 完整makeverify、真实独占PG+mTLS、vuln、全历史secrets、SBOM与来源/license门禁PASS；live范围与持久化一致 | OBS-03；`logs/resource/` |
| Gov | 定向行为/TLS/cancel、相关vet、server/admin构建、audit、正式check-generated PASS（两轮Ent+完整API） | OBS-07；`logs/governance/` |
| 手工采集 | 真实5族输入、15s采集/30s rules、8rules healthy、七标签、3周期；fixture30条、真实负载、CID/UID替换排除均PASS | OBS-02；`collection/`、`logs/collection/` |
| 真实链路 | 最新值/范围、负载恢复、运行身份、停止/删除历史、连接/超时/取消、实际租户源mTLS拒绝PASS | OBS-08；公开脱敏 `evidence/`，原始私有材料留Fedora |
| 原包 | 原README/CONTRACT/ACCEPTANCE/tasks及SHA256SUMS未改写，`sha256sum -c SHA256SUMS` PASS | `logs/bootstrap/task-pack-checksums.log` |

初始工具、网络、fixture和业务观察脚本失败均保留在各任务报告，不清除断言、不扩known-defects、不用重试掩盖失败。最终文档/evidence提交另做泄密扫描与SBOM同步；原始认证材料不进仓。

## 验收状态索引

| 用例 | 最终状态 | 证据/边界 |
| --- | --- | --- |
| B01–B05 | PASS | OBS-00/01/03/05/06/07；正式依赖/生成/构建/audit |
| B06 | PASS（用户调整范围） | 本报告、version-lock、公开请求证据；原v1.2非全数PASS |
| R01–R05 | PASS | OBS-03和真实停止/删除后的Gov历史查询 |
| C01–C10 | PASS（各规定层级） | OBS-02；systemd/containerd实机，cgroupfs仅离线fixture |
| D01–D07 | PASS | OBS-04、E06真实错误与取消 |
| N01–N09 | PASS | OBS-05；真实启动/停止缺口也未补零 |
| S01–S05 | PASS | OBS-06/07和真实Gov链路 |
| S06 Obs受信入口 | PASS | 实际租户workspace验证certificate_required拒绝 |
| S06 Prom底层额外隔离 | DEFERRED_BY_USER | CNI无Netpol；未引入Prom mTLS/proxy/Netpol，不把ClusterIP当隔离证明 |
| H01–H09 | PASS（用户调整范围） | 真实HTTP/nullable/错误/权限注册及行为测试；有效无instance:read身份实测跳过 |
| E01–E04、E06–E07 | PASS | OBS-08；删除后60点/指标网格，91个有效点同Prom逐点一致 |
| E05 跨租户/未认证/伪造身份/恶意参数 | PASS | 实际404/401/400 |
| E05 有效无instance:read账号 | SKIPPED_BY_USER | 正常角色创建403，无越权造身份；对应代码负测仍PASS |
| 单独CPU规格实例 | SKIPPED_BY_USER | 已通过GPU规格实例的真实CPU/内存验证 |
| 到期READONLY策略 | DEFERRED_BY_USER | 沿用现有授权，不承诺到期历史可读 |

## 启动、复核与恢复

- 从真实服务tag `v0.1.0` 构建，API `v0.1.1` 可独立下载。实际Secret、证书SAN、固定datasource、部署输入见 `deployments/observability/README.md` 与 OBS-06。
- collection README给出实际字段级前向patch、原字段/恢复patch、selectors/版本、fixture与现场verify；不要回滚整个监控release、删除TSDB或改Installer。
- 使用正常Gov身份，按 `evidence/deleted-full-history-gov.json` 的请求体/路径重查同一时间段；Bearer从既有私有凭据引用取得，不能把token放URL。数据保留期内，已删除实例的持久范围仍支持复核；证据本身含完整脱敏响应及同点Prom查询。
- 单位/缺失/范围/错误以原固定CONTRACT为准；定位顺序为Gov身份权限→Res范围→ObsTLS/配置→固定record/freshness/运行身份。前端后续直接消费这两个Gov路由，日志/告警从三个业务层的既有边界扩展。

## 清理与保留

- 正常业务实例最终deleted、computeCleaned/workspaceCleaned/quotaReturned全true；临时负例用户及其唯一遗留凭据均0。公共UserDelete孤立凭据缺陷记录为NOT_FIXED，未扩展本期代码。
- 任务自有 `ani-obs-loader-01a15321` namespace及3个loader已实际删除；一次性网络probe namespace亦已删；任务使用的mTLS文件副本已移除，正式Kubernetes Certificate/Secret保留给运行服务。
- 正式Obs/Gov/Res、手工采集/rules、TSDB与历史数据保留；不删除用于版本恢复的镜像。任务port-forward在证据完成后结束，最终收尾见 `evidence/cleanup.json`。
- 发现一次诊断工具输出了原始数据库DSN，未入源码、镜像或提交；已停止这种输出并记录于OBS-07，不复述其值。测试原始配置仍仅保留私有引用与权限，不作为公开归档。

最终文档/证据门禁日志：`logs/bootstrap/final-evidence-gates.log`（全历史secrets、SBOM/来源/许可证）、`final-clean-reproduction.log`（干净提交上两条生成链和SBOM一致性）；只归档文档/证据后复核相应门禁，不重复真实负载和已通过的业务全测。

最后全历史secrets曾FAIL（5项）：四个真实幂等UUID与公开API提交SHA被generic-api-key规则误报。逐项核对规范UUID与真实API tag后，仅在 `.gitleaksignore` 登记对应提交/路径/行号的5个精确Fingerprint；没有规则或路径级allowlist。相同路径上新的惰性疑似key fixture仍被默认扫描拒绝，证据 `logs/bootstrap/final-scanner-negative.log`。初始失败及脱敏诊断 `final-gitleaks-diagnostic.log` 保留，未忽略真实凭据。
