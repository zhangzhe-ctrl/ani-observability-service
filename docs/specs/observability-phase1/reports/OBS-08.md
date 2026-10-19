# OBS-08 真实后端链路

## 任务与结论

状态：按用户调整后的范围已完成。OBS-02/07 前置产物真实存在。主链已实际闭合：正常 tenant-a 登录 → Gov → live Res 持久化范围 → mTLS Observability → live Prometheus → Gov 的范围/最新值 DTO；不是 mock 闭环。

E01、E02、E03、E04、E06 已通过。E05 的实际跨租户、未认证、伪造身份、PromQL/scope 拒绝已通过；**有效但没有 instance:read 的真实账户负例为 SKIPPED_BY_USER**。现有两个角色都有读取权限，当前账号正常创建专用无权限角色返回403。用户确认没有该账号，并授权自行创建或跳过；因此按授权跳过，没有用 fixture、删除后的用户或 SQL 造身份替代实测。

用户明确确认：GPU 规格实例的 CPU/内存真实验证成功后，跳过独立 CPU 规格实例。本次已满足前者，后者标 SKIPPED_BY_USER，不标实际 PASS。Prometheus 底层额外隔离和到期 READONLY 产品策略仍依用户确认延期。

## 实际版本与对象

- 版本完整映射见 `version-lock.yaml`；真实服务 tag `v0.1.0` / 源码 `a5bef5d0a8ff8f97d1a6bae1d53227548ae387a4`，API `v0.1.1`。正式镜像运行 Version=v0.1.0，Go1.26.9/API1.1。
- 固定集群 `0fcdb8b1-22bf-4528-af59-82843dc88f21`，context `kubernetes-admin@ani-lab`，只访问10–12。
- 正常 Gov 创建实例 `90e2c65e-3601-44b6-9ef1-a7496c07a183`，受理 operation `745ab0a0-f430-4729-bf37-aeff45c8a807`，实际可选规格 `d51c5fb9-4a06-4a33-bbad-f528c7820ce5`（500m CPU/1Gi内存，GPU规格）。没有为本任务增加 GPU 指标、调度或规格管理逻辑。
- Res tenant `46936e6b-3a3b-4868-8d88-6833665abb70`、namespace `tenant-46936e6b-3a3b-4868-8d88-6833665abb70`、container=workspace。live Res 返回与只读 PG 持久化逐字段一致，证据 `logs/resource/live-scope-e2e.json`、`live-scope-persisted.json`。
- 原始规则首 record `2026-10-19T09:17:55.589Z`。之前没有 canonical 历史，不补回填。

## 命令、结果与证据

所有生成、测试、运行、SQL、镜像、Kubernetes 和数据对照均在 Fedora。远程根目录：
`/home/chabking/ani-observability-runs/phase1-20261019-01a15321`。
下列命令从该目录执行，真实凭据只由私有文件读取，没有归档 Authorization/cookie/token/私钥。

| 用例 | 实际操作 | 结果与证据 |
| --- | --- | --- |
| E01 | `python3 compare.py latest-first --latest`；`python3 compare.py load-history --start 2026-10-19T09:18:00Z --end 2026-10-19T09:26:00Z` | PASS：最新两点、范围34个有效点，同 scope/同求值时间/同step与最终 record 一致；相对/绝对容差均1e-9。脱敏原请求/响应、request_id、Prom查询与结果在 `evidence/` 同名前缀文件 |
| E02 | `kubectl exec -i <本实例Pod> -c workspace -- /usr/local/bin/python3 - < load.py` | PASS：实际页触碰192MiB，单线程忙循环300s，真实限额500m/1Gi。节点时间09:19:28.117–09:24:28.451；CPU最高0.4862199201727315 cores，内存最高273072128 bytes。`logs/instance-load.log`；`evidence/load-shape.json` |
| E02 恢复 | `python3 compare.py full-load-recovery --start 2026-10-19T09:18:00Z --end 2026-10-19T09:33:00Z` | PASS：62个有效点与 Prom 同网格一致，内存恢复、CPU按5m窗口回落；`evidence/full-load-recovery-*` |
| E03 同UID换CID | `python3 lifecycle.py cid`，仅向本实例 workspace PID1 发 SIGINT | PASS：UID不变、restart0→1、新CID Ready。旧raw counter仍在5m窗口，但canonical只含新CID；`logs/e2e-cid.log`、`collection/lifecycle/cid-exclusion-summary.json` |
| E03 同实例换UID | 正常 Gov `:stop`、`:start`，各使用实际 expectedVersion 与新幂等键 | PASS：同一实例ID/Pod名，UID更换；旧UID raw仍在5m窗口，规范record只绑定新UID。`evidence/lifecycle-{stop,start}.json`；`collection/lifecycle/uid-exclusion-summary.json` |
| E04 停止历史 | `python3 compare.py stopped-history --start 2026-10-19T09:18:00Z --end 2026-10-19T09:26:00Z`；`compare.py stopped-latest --latest` | PASS：Pod不存在时原34点仍一致；freshness窗口过后latest两个NO_DATA/null |
| E04 删除历史 | 正常 Gov DELETE；`compare.py deleted-full-history --start 2026-10-19T09:14:00Z --end 2026-10-19T09:43:30Z`；`compare.py deleted-latest --latest` | PASS：删除后60点/指标的完整网格、共91个有效点仍逐点一致；其余真实缺口保留null；latest两个NO_DATA/null。删除后的Res范围可继续解析 |
| 启用前缺失 | `compare.py before-rule-history --start 2026-10-19T09:14:00Z --end 2026-10-19T09:18:00Z` | PASS：每指标9点只有最后1点有效，先前8点均null/NO_DATA，无伪造历史 |
| E05 已执行 | tenant-a 请求其他租户真实已有实例；无JWT及伪造 x-ani 身份；query中夹带tenant；body夹带PromQL | PASS：分别404 INSTANCE_NOT_FOUND、401 UNAUTHORIZED、400 MONITORING_INVALID_QUERY。其他租户ID在公开归档中脱敏；`evidence/negative-*`，原始关联保留远程private |
| E05 无读取权限身份 | 现有role5实为拥有permission403=instance:read；正常创建空权限role返回403 | SKIPPED_BY_USER：没有有效受限身份。误选角色返回200符合授权，未报成越权；`logs/governance/negative-permission-db.log`，私有 `create-negative-role.json` |
| S06 实际租户来源 | 在本实例workspace内只使用公共CA，TLS1.3验证真实Obs服务身份后，无client证书发送HTTP/2 preface | PASS：明确certificate_required拒绝；`logs/e2e-tenant-source-tls.log`。没有向业务Pod拷贝客户端私钥 |
| E06 连接/超时 | `python3 fault.py`，只临时更改本任务Obs唯一datasource为任务workspace上的拒绝/无响应端口，不改共享Prometheus | PASS：503 MONITORING_SOURCE_UNAVAILABLE、504 MONITORING_QUERY_TIMEOUT；外部探针观测实际两条HTTP约4s取消。`logs/e2e-fault.log`、`evidence/fault-results.json` |
| E06 取消 | `python3 cancel.py`，等待两个真实Prom HTTP已进入外部故障边界，再关闭公共HTTP客户端 | PASS：两个上游在约1.27/1.29s内关闭，早于4s批次预算；实际request_id及时间以 `evidence/cancellation.json` 为准；原始/最终结果均保留 |
| E06 恢复 | finally中严格恢复CM原数据与原annotation；重新rollout并请求latest | PASS：原配置逐值相等，正式查询200；`evidence/fault-results.json`、`cancellation.json` |

数据对照 helper 调用真实登录取得的 JWT，不构造身份。一次重复登录返回500的原始证据保留在私有 `login-failure.json`，之后复用此前已成功认证的有效session；没有绕过认证。Fedora与节点存在约17s时钟差，所有对照用响应 evaluated_at 或显式相同UTC网格，未拿不同机器的“现在”比较。

## 三个实际运行身份

| 顺序 | Pod UID | 完整 CID |
| --- | --- | --- |
| 初始 | 7ea9b16e-c89b-4e6c-b548-d13040b606e5 | 2fe184cb4af92095e9087a47bc8f20ea3e2944916a34bdc6df044355084f7d60 |
| 同Pod重启 | 7ea9b16e-c89b-4e6c-b548-d13040b606e5 | 0c277062ca5cb1b91ff79892b00eaa18afb5ccc5ae8b1a699b6c2cf1cdcb3f4b |
| 正常stop/start | 2ac55f6c-808a-44b7-89bd-30a164ee6ff9 | cfb0af3bda0bbd04004d8b4a52e28480c786f071aa9c44a86dd373e478eedcf3 |

完整原始record时间线、旧counter排除和删除后逐身份重放见 `collection/lifecycle/`。新UID内存先于CPU产生记录是CPU至少两个样本的正常warmup；没有补零、跨身份求和或拿当前KSM过滤历史。

## 失败与清理

- 取消探针首版未正确消费chunked POST且按行解析并发JSON输出；后续分别修正外部HTTP fixture、改为有界事件等待与流式JSON解析。保留 `e2e-cancel.log`、`e2e-cancel-second.log`、最终 `e2e-cancel-third.log`；没有改生产取消逻辑或删除断言。第一次等待采样过早的失败仍保留。
- start观察helper在Pending阶段假设已存在containerStatuses而报StopIteration；正常start已202受理，之后仅续看该operation到Ready，没有重复命令制造新对象。
- 正常业务DELETE于Fedora09:43:30.767确认：version4/deleted，computeCleaned/workspaceCleaned/quotaReturned全部true。本任务Pod/PVC已由正常流程清理。
- 临时负例用户19已通过正常API删除；发现既有公共UserDelete遗留一条自有孤立凭据，未扩为本期代码修复。经事务核对唯一用户名/用户不存在后只清除此一行，用户、凭据、有效凭据均0；见 `logs/governance/negative-user-credential-cleanup.json`。不计作E05受限身份测试。
- 临时镜像loader、port-forward和私钥副本收尾结果见 OBS-09；正式服务、证书、TSDB与采集配置继续保留。

## 跳过边界

用户已授权跳过有效但无 `instance:read` 身份的真实拒绝子用例，不将其写成实际通过。日后补验只需取得该类有效身份，对已删除但持久范围仍在的本实例请求metrics并验证403；无需重做负载、重建GPU实例或重跑已通过门禁。原始 v1.2 全部用例并非全数 PASS，当前完成结论只针对 `EXECUTION-OVERRIDES.md` 记录的调整后范围。
