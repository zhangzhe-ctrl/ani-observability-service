# 可观测服务迁移到 ani-system

用户授权：仅迁移 ANI 可观测查询服务、更新 Gov 地址并验证真实查询；Prometheus、采集规则和历史存储保留原位。本次不修改 Installer 或业务源码。

执行环境：全部部署、生成和测试通过 Fedora，目标为 10–12 集群 `kubernetes-admin@ani-lab`。远程证据目录为 `/home/chabking/ani-observability-runs/phase1-20261019-01a15321/namespace-migration/`。

## 变更

- 服务端 Deployment、Service、ConfigMap、Certificate 和 Secret 示例的 namespace 改为 `ani-system`。
- Gov 使用独立 strategic merge patch，仅更新 `server` 容器的 `ANI_OBSERVABILITY_GRPC_ADDR` 为 `ani-observability-service.ani-system.svc:19090`。
- 保留镜像、服务端身份 `ani-observability-service`、Gov 客户端身份 `ani-governance`、既有客户端证书与挂载。
- 数据源地址仍为 `http://ani-metrics-prometheus.ani-observability.svc:9090`，collection 目录保持不变。

## 顺序与当前结果

1. 已确认原服务 Ready、目标对象名称无冲突，ani-system 配额可容纳新增请求和限制。
2. 迁移前真实租户登录成功。通过 Gov 查询一期已删除实例的历史 CPU、内存，各 17 个有效点，与同网格 Prometheus 逐点一致；latest 均为预期的 NO_DATA/null。当前租户没有运行实例，本次无需新建计算资源。
3. 创建 ani-system 服务端 Certificate，等待签发，再创建 ConfigMap、Service、Deployment 并等待 Ready。
4. 仅更新 Gov 地址并等待 rollout；再次真实查询，通过后才删除 ani-observability 中的旧服务专属对象。
5. 删除旧服务后再次查询，核对 Prometheus 对象、配置、Pod/容器与存储标识、历史值及 Gov 客户端 Secret。

迁移及最终检查结果：PASS。`ani-system/ani-observability-service` 与 Gov 均 Ready 1/1；Gov 的 Pod 模板仅地址发生变化，两个服务均沿用已验证镜像。旧 namespace 中本服务的 Deployment、Service、ConfigMap、Certificate 和 Secret 已按原 UID 核对后删除，旧副本也已消失。

## 实际命令与验证

部署和查询使用本次远程目录下的 `migrate.py`。它复用一期的真实认证客户端与 Prometheus 对照 helper，凭据仅从原私有文件读取；查询均经过 Gov 认证、Res 可信范围、mTLS Obs 和真实 Prometheus。

| 命令 / 检查 | 实际结果 | 远程证据 |
| --- | --- | --- |
| `python3 migrate.py before` | 真实租户登录；迁移前历史各17点有效，latest 两项均 NO_DATA | `before-*-summary.json`、`historical-series-before.json` |
| `python3 migrate.py deploy` | Certificate 签发、创建新服务、rollout 成功；镜像和完整容器运行规格、数据源配置与原服务一致，exit 0 | `deploy.log/.rc`、`new-service-proof.json` |
| `python3 migrate.py switch` | 实际应用仓库中的 Gov endpoint patch；rollout 成功，模板除地址外逐字段相等，exit 0 | `switch.log/.rc`、`gov-switch-proof.json` |
| `python3 migrate.py after` | 切换后历史与迁移前逐点相同，且与 Prometheus 同网格、1e-9 容差匹配；保护对象未变，exit 0 | `after.log/.rc`、`after-*-summary.json` |
| `python3 migrate.py cleanup` | 仅删除旧服务5类专属对象；Prometheus/规则/数据未操作，exit 0 | `cleanup.log/.rc`、`old-service-cleanup.json` |
| `python3 migrate.py final` | 旧服务清理后历史各17点仍有效、latest 缺失语义一致；Prometheus 等25个保护对象的 spec/data/UID，以及 Prometheus Pod 容器标识和重启数均相同；Gov 客户端 Secret UID/版本未变，exit 0 | `final.log/.rc`、`final-protection-proof.json`、`final-*-gov.json`、对应 `*-prom.json` |
| `GOWORK=off make verify BUF=/home/chabking/ani-observability-runs/phase1-20261019-01a15321/service-build/.tools/bin/buf` | Fedora Go1.26.9，buf1.60.0，实际根服务/API tidy-diff、test、vet、build、mod verify、生成源检查与diff检查通过，exit 0 | `verify.log/.rc` |

保护对象包括现有 Prometheus CR、recording rule、rulefiles ConfigMap、StatefulSet、Prometheus/Alertmanager/Loki PVC 和现有 Service 等。不是通过重新部署、复制或恢复 TSDB 得到相同结果。

最终脱敏证据见 [namespace-migration-evidence.json](namespace-migration-evidence.json)。原始非秘密查询与命令证据保留在远程目录；完整部署前快照为私有文件。临时 port-forward 均已退出。

本次没有运行实例可用于获取新的非空 latest；实际验证了已删除实例的历史有效值和 latest 的 NO_DATA/null，不将它描述成运行实例的实时数值验证。没有新建 GPU/CPU 实例，没有重做负载或生命周期实验。既有 Gov main 的 ModelDev 编译问题不在此次部署配置迁移范围；Gov 使用原有已验证镜像，仅滚动更新环境变量。

## 回退

切换后的真实查询未通过时，先将 Gov 地址恢复为 `ani-observability-service.ani-observability.svc:19090` 并等待 rollout；成功验收前保留旧服务完整运行。若旧服务已清理，需先用原清单在 ani-observability 重建服务专属对象、等待证书与副本 Ready，再恢复 Gov 地址。回退不操作 Prometheus、rules、PVC 或 TSDB。
