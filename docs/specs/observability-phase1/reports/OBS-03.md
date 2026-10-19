# OBS-03：Res 实例监控范围解析

## 任务与结论

- 状态：已完成代码、规定门禁及真实可获取模块发布；候选 Res 已部署到指定 10～12 集群。
- 完成行为：Gov 可通过精确受信 RPC `ResolveInstanceMetricScope(GetInstanceRequest)` 读取当前可信租户实例的持久化 tenant / instance / cluster / namespace / workspace。停止、正常删除不影响位置解析；delete-first tombstone 缺范围时携带机器可读 `google.rpc.ErrorInfo.reason=INSTANCE_METRIC_SCOPE_UNAVAILABLE`，gRPC `FailedPrecondition`。
- 查询只调用既有 `Repository.Get`，不调用 Observe / runtime / Kubernetes，不新增表、SQL 查询、写入、生命周期或配额行为。
- 本任务验证了真实 PostgreSQL + TLS 1.3 + 生成 gRPC client + 现有 GovernanceUnary 的后端切片。真实集群的 Gov → Res → Obs → Prometheus 业务查询及历史样本验收由 OBS-08 完成，本报告不将 Deployment Ready 当作该链路通过。

## 改动与版本

| 仓库 | 实际起始提交 | 当前提交 / 分支 | 主要文件与目的 |
| --- | --- | --- | --- |
| ani-resource-service | `f7459f7ae71c5573f53a50c5d20908b129c17e0c`，已 fetch 核实 origin/main | `9fa3b4aa41f90db9ca0141a2b715904073f6e2d2`；`codex/observability-scope-20261019`，已推送 | `api/instance/v1/instance.proto` 与两个生成文件；`internal/service/instance/metric_scope.go` 与行为测试；`internal/server/instance_identity.go` 精确允许项；`docs/specs/governance-calls.md` |

- 本地专属工作树：`/home/chabking/.codex/worktrees/observability-scope-20261019/ani-resource-service`；交付时 clean。原主工作树及其他任务分支未修改。
- 消费者版本：`github.com/zhangzhe-ctrl/ani-resource-service v0.0.0-20261019084504-9fa3b4aa41f9`。
- 正式下载验证：Fedora 上新独占 GOMODCACHE、`GOWORK=off`、无 replace，`GOPROXY=https://proxy.golang.org go mod download -json ...@v0.0.0-20261019084504-9fa3b4aa41f9` 退出 0。Origin.Hash 精确等于上述完整提交，Sum=`h1:9hCojnkfUm6mB4mH9cl1URkehWVkhiF7ycwyyHHAPxs=`。
- 未改 tag、未合并 main、未创建 PR。默认 GitHub SSH 22 端口关闭连接后，使用每条命令的 `core.sshCommand='ssh -p 443 -o Hostname=ssh.github.com'` 成功 fetch / push，未改全局 Git 配置。
- API 子模块版本不适用，本任务按合同保留 Res 根模块布局。

## 验证

以下证据根均为 Fedora：`/home/chabking/ani-observability-runs/phase1-20261019-01a15321/logs/resource/`。本地副本位于 `/home/chabking/tmp/runtime/observability-scope-20261019/`。

| 用例 ID / 门禁 | 实际命令或操作 | 结果 | 证据文件 |
| --- | --- | --- | --- |
| R01、R03、R05 | `go test -count=1 -v ./internal/service/instance -run MetricScope`；真实 NewService 构造，外部 Repository / Image / Environment 边界替身，读取 pending/running/stopped/deleted 持久化范围，断言与进程配置不同且无边界调用或写入 | PASS | `generate-service-test-proxy.log` |
| R02、R04 | 同上；缺身份、不同租户、平台身份、非法请求、缺集群/namespace、非法 UUID、tombstone 与 ErrorInfo.reason | PASS | `generate-service-test-proxy.log` |
| R01～R05 真实 PostgreSQL + mTLS gRPC 切片 | `INSTANCE_TEST_RUN=^TestInstanceMetricScopePersistentGRPC$ scripts/instance-integration`；独占临时 PostgreSQL、真实迁移/Repository/Service、生成 client、TLS 和 GovernanceUnary。CPU 真实持久化生命周期到停止/正常删除；跨租户/伪造 actor/重复 tenant/缺身份/无证书/不存在/tombstone；前后记录相等 | PASS，包用时 0.439s，退出 0 | `pg-mtls-grpc-test.log` |
| 生成一致性、分层、故障构建、tidy、默认测试、vet、build、mod verify | 仓库 AGENTS 要求的 `make verify` | 首次 FAIL，既有 UNIX socket 测试因深 TMPDIR 超 Linux socket 路径上限；未改源码/断言 | `make-verify.log`，`make-verify.exit=2` |
| 上述失败定向复核及完整规定门禁 | 任务独占 0700 短目录 `TMPDIR=/tmp/obr-01a15321`；先 `go test -count=1 -v ./scripts/lb-api -run ^TestSocketRequiresPrivateDirectoryAndPreservesExistingPath$`，再 `make verify` | PASS，完整 make verify 退出 0；`go mod verify` 为 all modules verified | `make-verify-short-tmp.log`，`make-verify-short-tmp.exit=0` |
| 发布 B05 输入 | 新独占 cache 下正式 `go mod download -json`，官方代理全量下载 | PASS，真实 Origin / Sum / Zip / Dir | `module-download-official.json`、`module-info-proxy.json` |
| 源码身份 | 本地与 Fedora 八个改动文件 SHA256 清单逐项 cmp | PASS | `local-changed-files.sha256`、`fedora-changed-files.sha256` |
| 候选构建 / 镜像 | 从最终 Git commit archive 构建静态二进制，podman scratch image + OCI archive | PASS | `image-build.log`；全局根 `resource-image/SHA256SUMS`、`image-inspect.json` |
| 实际候选部署 | 精确节点镜像导入、条件 JSON Patch、`kubectl rollout status deployment/ani-resource-service --timeout=120s` | PASS，ready / available / updated 均 1 | `image-import-manifest-alias.log`、`resource-rollout.log`；全局根 `resource-deploy/deployment-after.json` |
| 现行 audit：vulnerability | 首次 `make audit` 因官方漏洞 DB EOF 中止；`curl --http1.1` 获取并校验官方 modules index 成功后，仅扫描工具临时 `GODEBUG=http2client=0 make vuln` | 最终 PASS，退出 0，No vulnerabilities found；原 make audit 退出 2 保留，不抹除 | `make-audit.log`、`make-audit.exit`、`vulndb-http1-probe.log`、`make-vuln-http1.log`、`make-vuln-http1.exit=0` |
| 现行 audit：secrets | 真实 HEAD Git bundle 恢复的独立 checkout；`make secrets`，固定 gitleaks v8.30.1 `git --redact --log-opts=--all` | PASS，136 个有差异提交、135.95MB、4m25s，no leaks found | `audit-source.log`、`make-secrets-supply-chain.log`、`make-secrets.exit=0` |
| 现行 audit：SBOM / 来源 / license | `make supply-chain-verify` 已完成固定 cyclonedx v1.12.0 的确定性 SBOM 生成；初次来源子进程失败后，独占 fresh cache 实际直连获取 ACC 固定版本完整 Origin，再以同 fresh cache 仅重验 `./scripts/verify-supply-chain` | 最终 PASS；70 components，3 exact first-party UNDECLARED，67 third-party SPDX，frozen notice hash / license review synchronized；未改 pin 或放宽校验 | `make-secrets-supply-chain.log`、`audit-acc-provenance-fresh.json`、`supply-chain-provenance-proof.log`、`supply-chain-provenance-proof.exit=0`、`bom.cdx.json` |
| 全量数据库或 race 套件 | 未执行；已完成本改动对应真实 PostgreSQL 行为测试，未扩大其他数据库套件 | NOT_RUN | 本报告 |
| 真实业务历史与 Gov 公开端到端 | 由 OBS-08 继续，未在本任务代跑 | NOT_RUN | 本报告 |

环境失败原始证据保留：首次 Go 1.26.9 工具链及生成插件因 sumdb EOF / TLS 超时失败（`generate-service-test.log`、`generate-service-test-retry.log`）；之后配置可验证 sumdb 镜像，未关闭校验。首次新模块 goproxy.cn ZIP 下载以 HTTP/2 INTERNAL_ERROR 失败（`module-download.json`），官方代理新 cache 成功，不依赖该失败下载。首次 source 传输未完成时的哈希不作为验证输入；成功运行前重新校验完整 tar SHA256 并重新解包，最终 commit archive SHA256=`fdfd0c92d4daa37f2d9a40143d86b9fe7639c856309f4ad1f23fbfd0db50d174`。

Audit 的初次供应链错误只给出 `CalledProcessError`，没有保留子进程 stderr，不能断言其具体网络原因。实际确认原 file-proxy cache 的 ACC 条目缺少 Origin；独占 fresh cache 的后续真实 `GOPROXY=direct go mod download -json` 返回完整、匹配清单的 Origin / Sum，随后失败子步通过。未手写或拼接任何来源元数据。全部规定 audit 组成目标均已通过，不为将汇总命令重新打印为绿色而重跑已通过的历史扫描 / SBOM。

SBOM 从真实提交 `9fa3b4aa41f90db9ca0141a2b715904073f6e2d2` 生成，排除 BOM 自身后的确定性 source snapshot=`a9ca86a715b36346384c6a883d51d78d2b4937e9`，工具 SHA256=`1a0f48a6f09864bcbd1b1eed98f87e3fff822301c64b1765af2b5a320c70d3f4`。作为该提交验收 artifact 保留；本地实现工作树保持原发布提交 clean，不为更新自身来源 SHA 反复提交 / 发行。

## 环境与配置

- 所有生成、gofmt、Go 编译测试、PostgreSQL、镜像和集群操作均从 `ssh fedora` 执行；本机仅源码编辑、静态检查、Git 和传输。
- Res 当前基线 `go.mod` 要求 Go **1.26.9**，保持原要求，未降级。实际 Fedora 工具链 `/home/chabking/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.9.linux-amd64/bin/go`；Buf 1.60.0、sqlc v1.31.1 经版本及 `go version -m` 校验；生成插件沿用本仓固定 v1.36.11 / v1.5.1。
- 使用任务独占 GOMODCACHE / GOCACHE，`GOMAXPROCS=2 GOFLAGS=-p=2 GOMEMLIMIT=2GiB GOWORK=off`；未修改共享 root-owned cache 权限。
- Kubernetes context：`kubernetes-admin@ani-lab`；节点 `ani-01/02/03` 为 `172.16.101.10/11/12`；服务 namespace=`ani-system`。
- 实例集群 ID 经真实 Secret `resource-instance-private` 的 `instance-config.json` → `Service.ClusterID` 只读核定：`0fcdb8b1-22bf-4528-af59-82843dc88f21`；脱敏证据 `cluster-id.json`。未记录 Secret 内容或凭据。
- 候选镜像实际 OCI manifest reference：`localhost/ani-resource@sha256:54a1d322a366a364b8e5216ef177f8f1f87c41172969bf96bd863f90c42bd8bc`；source label=`9fa3b4aa41f90db9ca0141a2b715904073f6e2d2`。OCI archive=`resource-image/resource.oci.tar`。
- 部署前保存 `resource-deploy/deployment-before.json`，原 `imagePullPolicy=Never`、`strategy=Recreate`。JSON Patch 带 UID / container name / 原 image / 原 pullPolicy 测试，仅改 server 的 image；保留 Never，存在短暂 Recreate 中断。回退文件 `resource-deploy/image-rollback.json` 仅恢复该 image，未修改配置、证书、RBAC 或运行时。

## 镜像传输与交接

- Registry `172.16.101.10:5000` 实际只读，写入返回 405；SSH agent forwarding 到节点被拒绝。均保留 `image-push.log`、`node-transport-probe.log`，未修改 registry 或节点 SSH。
- 已验证唯一共享导入方法：全局根 `image-loader/import-image.sh`；固定 busybox image `172.16.101.10:5000/library/busybox@sha256:7a3ebe5bfd1a4a19797d20b0c0bb39d44393e9a03fd852c0865b0f540d868df0`，独占 namespace `ani-obs-loader-01a15321`、DaemonSet `ani-obs-image-loader-01a15321`，只匹配三指定节点。
- Pod 不设 privileged，drop ALL 仅 SYS_CHROOT、readOnlyRootFilesystem、hostRoot 只读、RuntimeDefault seccomp、无 ServiceAccount token。通过 `/bin/chroot /host /usr/local/bin/ctr --address /run/containerd/containerd.sock --namespace k8s.io` 流式 stdin 导入；不写 host 归档文件、不修改全局 RBAC / 网络 / containerd 配置。实际 containerd / ctr v2.3.4。
- 原 ani-system restricted PodSecurity 阻止第一组 loader Pod；保留 Forbidden DS / events 后删除该自有 DS，改用上述任务专属 namespace 的 admission label，未更改 ani-system 策略。完整创建对象与身份见 `image-loader/daemonset-isolated.yaml`、`namespace-created.json`、`daemonset-created.json`、`pods-created.json`。
- ctr v2 的 `--digests` 首次仅产生 archive index digest alias，脚本显式创建并验证实际单 manifest digest alias；初次不完整校验记录 `image-import.log`，最终三节点通过记录 `image-import-manifest-alias.log`。
- 同一方法已为 Obs archive 完成三节点导入（`obs-image-import.log`）；其 archive manifest 为 `sha256:98f89b379e9375dcb665b37d409a4e0e332876e77093bbb6ebab72fc87153a6c`，已通知运行时 owner，不使用 save 转换前的不同 digest。
- loader 交给 root 及 Gov / Obs owner 复用，待两者镜像均导入后由 root 按创建 UID / task label 删除本任务 namespace；本报告不提前宣称该共享资源已清理。

## 阻碍与下一项输入

- OBS-03 规定产物无阻碍，真实可消费版本与 Res 候选部署均已交给 Gov owner。
- 下一项消费版本：`v0.0.0-20261019084504-9fa3b4aa41f9`；ErrorInfo domain=`instance.ani.io`，reason=`INSTANCE_METRIC_SCOPE_UNAVAILABLE`。
- 真实公开链路、真实运行身份变化、历史样本对照和共享 loader 最终清理仍由 OBS-08/09 完成。
- 未修改固定业务合同；Go 1.26.9 来自 Res 实际基线，不是对新服务 API Go 版本的改动。
