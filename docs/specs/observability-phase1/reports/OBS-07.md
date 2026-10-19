# OBS-07：Gov 监控入口与后端调用

## 任务与结论

- 任务 ID：OBS-07。
- 状态：已完成本项实现与门禁；正式依赖、权限同步、真实部署及正向主链调用已交付。OBS-08 的真实无读取权限账户负例仍缺少可用身份，不能据本报告宣布一期全部完成。
- 主链：真实用户认证与 `instance:read` → Res `ResolveInstanceMetricScope` → Gov 使用独立 mTLS 调用 Observability → Prometheus → 同步 HTTP 200，CPU/内存范围和最新值查询均已由 OBS-08 执行者实际调用；最新值 2 点和范围 34 点与同网格 Prometheus 结果在绝对/相对 1e-9 内一致。
- 跨租户真实已有实例返回 404 `INSTANCE_NOT_FOUND`；真实上游连接故障返回 503、查询超时返回 504、恢复后返回 200。最终负权限角色验收与全部端到端证据由 OBS-08 报告承接。

## 改动与版本

| 仓库 | 实际起始提交 | 当前提交或分支 | 主要文件与目的 |
| --- | --- | --- | --- |
| ani-governance | `90b1ecc8cfa31b96ac43a8e4ff3aa18d41f03812`，已核对远端 main | `codex/observability-phase1-20261019`；`0c5cff2df30d3fa814ea4a1d591e8b9d83924437`、最终 `c905e49170b0dfeb23cf014cb4759a4e4ca2686d`，已真实推送、远端 ls-remote 匹配、工作树干净 | instance/admin proto、完整入口生成的 Go/OpenAPI；严格 HTTP handler；service 查询协调；Res 只读 RPC；独立 Obs mTLS client；手工 wiring/cleanup；精确路由超时与授权；权限 SQL；集成登记；实际 HTTP/TLS/cancel 回归 |

- API 正式锁定 `github.com/zhangzhe-ctrl/ani-observability-service/api v0.1.1`，真实 tag `api/v0.1.1`，来源提交 `44728ffbb722b30194a0cd8a25e60c5022ed0dfc`，模块 Sum `h1:SEr70/++V17GJrSzZMG6pEChKj+y9R7kThLe0bYmewQ=`。正式下载 JSON 核对 Origin.Hash/Ref。与此前 v0.1.0 的全部六个 proto/生成 Go 文件散列一致，安全补丁未更改协议。
- Res 正式锁定 `github.com/zhangzhe-ctrl/ani-resource-service v0.0.0-20261019084504-9fa3b4aa41f9`，来源提交 `9fa3b4aa41f90db9ca0141a2b715904073f6e2d2`，Sum `h1:9hCojnkfUm6mB4mH9cl1URkehWVkhiF7ycwyyHHAPxs=`。已真实下载，未使用本地 replace。
- Gov 保留 Go 1.26.9、gRPC 1.83.2；仅依赖可观测 API 子模块，没有依赖根服务模块。正式验证均 `GOWORK=off`。
- 服务间 actor 沿用 `governance:user:<canonical uint32 ID>`；每次重新构造单值 tenant/actor/request-id，request-id 为新非零规范 UUID。TLS 最低 1.3，精确服务端 DNS SAN `ani-observability-service`、唯一 Gov 客户端 DNS SAN `ani-governance`。
- 两条公共路径固定为 POST `/api/v1/instances/{instance_id}/metrics/query`、GET `/api/v1/instances/{instance_id}/metrics/latest`。请求严格校验、64 KiB 上限；nullable 实际 HTTP 编码区分缺失 null/有效零 0，正常点省略 gap_reason；只在上述精确方法/路由设置 10s 总预算，Res 2s、Obs 6s。
- 相关 vet 揭示四个既有 `QuotaChargeRef` 非命名字段字面量；仅改为 `ChargeID/QuotaCode/ChargedUnits` 命名字段，未修改断言，并执行对应 GPU 行为回归。

## 验证

以下证据相对 Fedora 独占执行根 `/home/chabking/ani-observability-runs/phase1-20261019-01a15321/`；命令从真实任务源码或真实 HEAD 的独立 Git 副本执行。本地仅源码/文档编辑、静态阅读、Git 和 SCP/SSH 转运。

| 用例 ID | 实际命令或操作 | 结果 | 证据文件 |
| --- | --- | --- | --- |
| 真实模块消费 | `go get` 正式 API/Res 版本；`go mod tidy`；`go mod download -json`；`go mod verify` | PASS，无 replace/GOWORK=off | `logs/governance/api-patch-dependency.log/.rc`、`api-patch-module.json`、`res-dependency-official.log/.rc`、`tidy-api-patch.log/.rc`、`mod-verify-api-patch.log/.rc` |
| 公共生成物 | 全量正式 `make api`，11 个模板，经暂存/写回保护 | PASS，恰好 5 个正式输出变动（4 Go + OpenAPI），其他输出无差异 | `logs/governance/make-api-pinned.log/.rc` |
| 行为 / H01–H09、B04–B05 对应边界 | `go test -count=1 -run 'TestInstanceMetrics\|TestInstanceLogTimeout\|TestObservabilityClient' ./app/admin/service/internal/server ./app/admin/service/internal/data` | PASS，真实 HTTP handler、service、Operation/middleware、限流/错误、超时取消、TLS 身份；外部 Res/Obs 使用明确边界 fixture | `logs/governance/targeted-tests.log/.rc` |
| 实际 HTTP nullable | `go test -count=1 -run TestInstanceMetricsHTTPEncodingAndTrustedScope ./app/admin/service/internal/server` | PASS；直接断言普通值/0/null、data/下划线字段、有效点 gap_reason 键缺失 | `logs/governance/http-encoding-explicit-gap.log/.rc` |
| 既有 GPU 字面量回归 | `go test -count=1 -run TestGpuChargesCompleteAndOrderIndependent ./app/admin/service/internal/service` | PASS | `logs/governance/gpu-literal-regression.log/.rc` |
| 相关静态检查 | `go vet` server/service/data/cmd-server/cmd-admin 包 | PASS；首次既有非命名字面量失败保留 | `logs/governance/vet-corrected.log/.rc`；原始 `vet.log/.rc` |
| 最终构建 | `CGO_ENABLED=0 go build -ldflags '-s -w -X main.version=c905e49170b0dfeb23cf014cb4759a4e4ca2686d' -o .../server ./app/admin/service/cmd/server`；同样构建 admin | PASS | `logs/governance/build-server-final.log/.rc`、`build-admin-final.log/.rc`、`binary-modules-final.txt`、`binary-sha256-final.txt` |
| 安全门禁 | `bash scripts/ci/audit.sh`，固定 govulncheck v1.7.0/gitleaks v8.30.1，私有工具目录 | PASS，可达漏洞 0，保留 1 个导入包/8 个模块的未调用漏洞提示；gitleaks 无泄漏 | `logs/governance/audit.log/.rc`、`audit-tool-*` |
| 生成一致性 | 真实 c905e49 Git bundle 恢复独立工作树后 `make check-generated` | PASS，总退出0；两轮完整 make gen、883 managed files/220 inputs/7 roots 比较与原源树保护均通过，本仓 gow 测试和真实 redact 插件集成通过。首轮及重试 Ent 冷编译/缺 fallback 工具失败保留；安装并核对固定 ent v0.14.6 后完成，未修改生成器 | `logs/governance/check-generated-pinned-ent.log/.rc`、`generated-check-pinned-ent/`；原始 `check-generated-final.log/.rc`、`check-generated-resumed.log/.rc`、`ent-generation-diagnostic.log/.rc`、`tool-ent-identity.txt` |
| 实际目录与权限 | `admin sync-apis --dry-run` → `admin sync-apis` → 既有 `scripts/ops/sql/register-instance-permissions.sql` | PASS：新增 latest id330、query id331，均 ON/INSTANCE 且精确绑定 instance:read；角色权限 261 行及全行散列前后相同 | `logs/governance/catalog-dry-run.log/.rc`、`catalog-sync.log/.rc`、`permissions-before.log/.rc`、`permissions-register.log/.rc`、`permissions-after.log/.rc` |
| 离线镜像/部署 | `podman build --network=none --pull=never`；`podman save --format oci-archive`；既有 `image-loader/import-image.sh`；最小 strategic patch；rollout status | PASS：实际导入 10～12、Gov 85/85，1 Ready、restart0；程序比较 PodSpec 仅有授权字段差异 | `logs/governance/image-build-final.log/.rc`、`image-manifest-final.json`、`image-import-final.log/.rc`、`deployment-patch-final.log/.rc`、`deployment-rollout-final.log/.rc`、`deployment-proof-final.json` |
| 真实后端链路 | 正常 Gov 创建的真实验收实例查询，与相同网格 Prometheus 比较；跨租户与上游故障负例 | OBS-08 执行者已确认范围/最新值/跨租户/连接故障/超时/恢复 PASS；最终无权限角色负例待完成 | `private/e2e/latest-first-gov.json`、`load-history-gov.json`、对应 `*-prom.json`、`negative-cross-tenant.json`、`fault-connection.json`、`fault-timeout.json`、`fault-restored.json`；由 OBS-08 归档脱敏证据 |

保留的环境失败：默认 Go1.26.7/GOTOOLCHAIN=local 不满足本仓 Go1.26.9；生成插件缓存初始缺失；Res 首个镜像代理等待后仅终止本任务进程（退出143）、切换官方 proxy 后成功；旧字面量 vet 失败；Ent 冷编译/工具准备问题。原始日志及退出码均存在，不将失败写成未执行或覆盖为成功。

## 环境与配置

- Kubernetes context `kubernetes-admin@ani-lab`，只操作用户允许的 10～12（节点名 ani-01/ani-02/ani-03）；Gov namespace `ani-system`，Deployment `ani-governance`。
- 最终源码提交 `c905e49170b0dfeb23cf014cb4759a4e4ca2686d`；实际 Pod `ani-governance-559c55f6fd-ds748`，node `ani-03`。
- 运行镜像与 imageID 一致：`localhost/ani-governance@sha256:084014194b6dd975c6c4ab9949402e81e0ea2282bd304e7104e2264adcf7e9b5`；保留 `imagePullPolicy=Never`。镜像仅含最终 CGO=0 二进制、仓库公开配置、许可通知，运行基础为已验证的离线 distroless digest。
- Server SHA256 `880619f867e2621694251994f9f4342eb797851e1941ebc15b58261d4eb829b8`；admin SHA256 `81c49b6cc28eb00c759d9bfea0ad8c6ad9e0048adc6c14425e2596cf319bca06`。
- mTLS Secret 仅引用 `ani-system/ani-observability-governance-client-tls`，只读挂载 `/run/secrets/observability-mtls`，使用 ca.crt/tls.crt/tls.key；不归档证书私钥或令牌。
- `ANI_OBSERVABILITY_GRPC_ADDR=ani-observability-service.ani-observability.svc:19090`，三个证书文件 env 分别引用上述挂载文件。用户已延期 Prometheus 底层入口额外隔离和到期租户策略，本任务未修改该决定，Gov 现有租户鉴权与 Gov→Obs mTLS 保留。
- 手工采集/Prometheus Operator/KSM/recording rule 版本与配置为 OBS-02 交付；本任务未操作 Installer，也未重复部署采集。
- 已有业务库只做目录和既有权限脚本同步，未迁移 schema、未首次播种、未改既有角色授权；新进程 rollout 完成策略刷新。
- 临时过程中的一次诊断错误曾向工具输出原始数据库 DSN；已告知主代理且后续凭据只在远程进程内读取，未写入源码、提交、镜像或本报告，不复述其值。任务私密配置及完整部署快照保持 0600，不作为公开证据归档。

## 阻碍与下一项输入

- 源码、版本获取、真实部署、HTTP 行为、权限同步及正式生成一致性检查均已通过，可直接供后续任务使用。
- OBS-08/09 使用上述最终源码版本、镜像、API/Res 版本与 Gov 路由；最终无权限负例请使用正常管理 API 创建的专用角色，勿误用实际含 `instance:read` 的既有 role5。
- 已只读核对真实负例用户 id19/tenant1 的 role5/no-model ACTIVE，实际拥有 permission403 `instance:read` ON/ALLOW，所以其返回200符合授权；permission295 是 `accelerator:profile:list`。证据 `logs/governance/negative-permission-db.log/.rc`。
- 正常管理 API 创建专用空业务权限角色返回403，未创建角色或修改既有角色；OBS-08 执行者已向用户请求合适受限账户/管理员凭据引用。实际缺少 `instance:read` 的身份负例尚未执行，不把最初 role5 的200算产品漏洞或负例通过。独立 CPU 规格创建由用户授权跳过，验收实例仍实际验证 CPU 与内存两种指标；此范围决定由 OBS-08 记录。
- 没有自行修改固定约定。用户已确认的两项延期沿用；未合并、未强推。
