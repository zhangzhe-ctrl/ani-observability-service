# OBS-06 受信 gRPC 与服务部署

## 任务与结论

- 状态：进行中。真实监控业务经过受信 TLS listener 的代码验证通过；集群证书签发、静态构建、离线镜像、真实 Deployment/探针和实际 TLS 边界通过；受信 Gov 的实际业务 scope 正例及租户工作负载来源验证等待 OBS-08 环境输入。
- 已完成：mTLS 配置与部署文件、真实 TLS/metadata/方法负测、生产 composition 与独立进程测试、配置失败与单源离线行为验证。
- 测试中的 Prometheus HTTP 是明确的外部边界替身，仅返回标准空 vector/matrix；监控 service、用例、范围校验、adapter、结果规范化均调用真实业务代码。这些测试不代替真实 Prometheus/Gov 链路验收。
- S06 的 Prometheus 底层额外隔离依用户确认延期，不标为安全验收通过；Obs 受信入口仍按原合同验证。

## 改动与版本

| 仓库 | 实际起始提交 | 当前提交或分支 | 主要文件与目的 |
| --- | --- | --- | --- |
| ani-observability-service | `4827e664ee7a5dc91ec4707f407838045bec718b`（本子任务开始时） | `codex/observability-phase1-20261019`，相关改动尚未提交 | `cmd/ani-observability-service/*_test.go`、`internal/server/governance_test.go`、`Dockerfile`、`configs/config.yaml`、`deployments/observability/` |

根服务实际消费 API `github.com/zhangzhe-ctrl/ani-observability-service/api v0.1.0`，运行时测试显式 `GOWORK=off`，无本地 replace。API 发布和正式根服务版本由 OBS-01/09 报告记录。

## 验证

全部生成、格式化、构建、测试、证书和集群操作在 Fedora 执行。本地仅阅读、编辑、轻量静态检查与转运。

Fedora 任务根目录：`/home/chabking/ani-observability-runs/phase1-20261019-01a15321`。以下证据相对此根目录。

公共测试环境：`GOWORK=off GOMAXPROCS=2 GOFLAGS=-p=2 GOMEMLIMIT=2GiB GOMODCACHE=$ROOT/cache/gomod GOCACHE=$ROOT/runtime/cache/go-build`；工作目录 `$ROOT/runtime/source`。

| 用例 | 实际命令或操作 | 结果 | 证据 |
| --- | --- | --- | --- |
| S01–S04，生产启动/退出 | `go test -count=1 ./cmd/ani-observability-service ./internal/server` 首轮 | FAIL，退出 1。新测试 fixture 的静态地址没有沿用配置插值，且 `metadata.MD.Set(key,nil)` 没有删除既有键。保留首轮失败后修正 fixture，不修改生产授权行为 | `logs/runtime/runtime-tests-first.log` |
| S01–S04，生产启动/退出 | 修正 fixture 后同一命令 | PASS，退出 0；cmd 9.777s，server 0.071s | `logs/runtime/runtime-tests-second.log` |
| S05 配置与单源离线 | 增补定向断言后 `go test -count=1 ./cmd/ani-observability-service` | PASS，退出 0，7.077s | `logs/runtime/runtime-tests-offline.log` |
| S01–S02 真实证书输入 | `kubectl --context=kubernetes-admin@ani-lab apply -f deployments/observability/certificates.yaml`，分别 `wait --for=condition=Ready ... --timeout=60s` | PASS；两个 Certificate 创建且 Ready | `logs/runtime/certificates-apply.log` |
| B05 静态构建 | `GOWORK=off CGO_ENABLED=0 make build VERSION=4827e66-dirty`，其余公共环境同上 | PASS，真实静态二进制形成 | `logs/runtime/image-build.log`、`runtime/source-input.sha256` |
| S05 离线镜像首轮 | `podman build --pull=never --network=none ...` | FAIL，退出 125；Dockerfile 错用了 Gov 的 notice 文件名。改为本模板原始 `LICENSE` 与 `THIRD_PARTY_NOTICES.go-kratos-layout.txt` 后重建，未改写来源文件 | `logs/runtime/image-build.log` |
| S05 离线镜像修复后 | `podman build --pull=never --network=none -t 172.16.101.10:5000/ani-observability-service:phase1-20261019-4827e66-dirty .`，随后 `podman save --format oci-archive ...` | PASS，退出 0 | `logs/runtime/image-build-second.log`、`runtime/image-identity.txt`、`runtime/image-archive.sha256` |
| S05 配置与Service | `kubectl apply -f configmap.yaml` 与 `service.yaml`，`kubectl apply --dry-run=server -f deployment.yaml` | PASS；前两项真实创建，Deployment 仅 server dry-run | `runtime/objects.before.json`、`logs/runtime/deployment-inputs-apply.log` |
| B05 根服务完整质量门槛 | 根服务 verify/build/audit | 本子任务未执行；由协调任务运行并记录 |
| S05 真实部署 | 共用 loader 导入 10–12 后，`kubectl apply -f deployment.yaml`，`kubectl -n ani-observability rollout status deployment/ani-observability-service --timeout=60s` | PASS；真实 Pod、ClusterIP 与端点就绪；实际业务数据查询等待 OBS-08 | `logs/runtime/deployment-apply.log`、`runtime/deployment.after.json`、`runtime/pods.after.json` |
| S05 admin/遥测 | 真实 Deployment port-forward 后 `curl --fail --max-time 2` 请求 `/healthz`、`/readyz`、`/metrics` | PASS；ok、ready、`ani_runtime_ready 1` 与请求计数均存在。首次转发刚启动时 curl 退出 7，端口转发就绪后再执行上述观测；未将初始化连接失败当作产品验收通过 | `logs/runtime/port-forward.log`、`runtime/healthz.json`、`runtime/readyz.json`、`runtime/process-metrics.prom`、`logs/runtime/deployment-startup.log` |
| S01 实际 TLS 身份 | `openssl s_client -connect 127.0.0.1:49190 -servername ani-observability-service -verify_hostname ani-observability-service -tls1_3 -CAfile ... -cert ... -key ... -brief`，证书文件位于任务 private 目录 | PASS；TLS1.3，Verification OK，Verified peername ani-observability-service。此项为握手身份验证，不代替业务 RPC | `logs/runtime/live-tls-valid.log` |
| S02 实际 TLS 负例 | TLS1.2 连接；无 client cert 时发送 HTTP/2 preface 后读取响应 | PASS；分别 `protocol version` 与 `certificate required` 告警，退出均 1。最初无数据即 EOF 的 openssl exit 0 不构成授权结果，保留该记录并改为主动请求探针 | `logs/runtime/live-tls12.log`、`logs/runtime/live-tls-no-client.log`、`logs/runtime/live-tls-no-client-with-request.log`、`runtime/live-tls-exit-codes.txt`、`runtime/live-tls-no-client-request-exit.txt` |
| S06 真实租户来源访问 | 租户工作负载无证书/伪造身份访问实际 Obs | 尚未执行；等待本任务真实实例与服务部署 |

代码测试实际覆盖：TLS 1.3 的真实证书 QueryLatest 与 QueryRange；无证书、错误 CA、同 CA 错身份、多 DNS 身份、通配身份、错误服务端身份与 TLS 1.2 拒绝；三个 metadata 的缺失/重复/非法值、actor 规范十进制、跨租户 scope 拒绝；仅两个精确 RPC 可用，标准 health、未知方法、reflection/stream 被拒；真实进程 SIGINT 后 admin 与 gRPC 监听关闭。日志身份、trace/span 与令牌脱敏的原有效断言保留。

配置测试实际覆盖：TLS 缺失/私钥不可读、monitoring 缺失、错误固定批次预算/并发、非法/重复集群 ID、地址缺失、凭据不可读时启动失败；单源离线时服务 admin 仍 Ready，真实监控 RPC 明确 Unavailable。

## 环境与配置

- Kubernetes context：`kubernetes-admin@ani-lab`，限定节点 10–12。
- kubeconfig 仅引用远程 `/home/chabking/ani-four-service-deploy-20261012/private/kubeconfig`，未归档其内容。
- 现有 Ready `ClusterIssuer/ani-ca`，复用 cert-manager 签发；没有读取 CA 私钥。
- 服务端 Certificate/Secret：`ani-observability/ani-observability-server-tls`，DNS SAN 精确 `ani-observability-service`，server auth。
- Gov 客户端 Certificate/Secret：`ani-system/ani-observability-governance-client-tls`，唯一 DNS SAN `ani-governance`，client auth。
- Secret keys：`ca.crt`、`tls.crt`、`tls.key`。私钥没有写入报告或 Git。
- Res 实际集群 ID：`0fcdb8b1-22bf-4528-af59-82843dc88f21`。
- Prometheus：`http://ani-metrics-prometheus.ani-observability.svc:9090`。
- Obs 内部目标：`ani-observability-service.ani-observability.svc:19090`，Gov 验证字面 ServerName `ani-observability-service`。
- 单副本、私网 ClusterIP，仅公开受信 gRPC 端口；admin 19091 用于 Pod 探针。无额外 RBAC、数据库、队列、PVC。
- 当前准确 dirty 候选：`172.16.101.10:5000/ani-observability-service:phase1-20261019-4827e66-dirty`，VERSION `4827e66-dirty`。这是当前源码快照构建，不声称最终发布版本。
- Podman 本地原 manifest digest：`sha256:9ab462b0e4f90d362aa7ab66d845317e81751d5846f0d81f3ef9a9857aab029f`；config ID `1aa120d03a474ec293dd00b9ef93466202474d7842810f765348b98b83d192e6`。`podman save --format oci-archive` 转换后的实际 OCI manifest digest 为 `sha256:98f89b379e9375dcb665b37d409a4e0e332876e77093bbb6ebab72fc87153a6c`，10–12 导入与 Deployment 均使用后者，不混用两种 manifest。
- OCI 归档：`runtime/ani-observability-service.oci.tar`；SHA256 `204207dd04dc0c67c9bb11db0ed8dc36cd129c40eb3fcb955c6fc441c4d3c2e4`。生产输入逐文件校验：`runtime/source-input.sha256`。
- 实际 Pod：`ani-observability-service-7cd5bd48c8-g28ws`，node `ani-03`，Pod IP `10.16.1.7`。ClusterIP `10.96.148.142:19090`，真实 endpoint `10.16.1.7:19090`。运行 imageID 为上述实际 OCI digest `98f89b...`；日志 service.version 为准确 dirty 候选 `4827e66-dirty`。

## 阻碍与下一项输入

- 现有 registry `172.16.101.10:5000` 的 push 已由协作任务实测返回 405，只读。复用经授权的任务自有 10–12 节点镜像导入流程，不修改 registry 或 installer。
- 协调任务已确认当前生产源码收敛。准确 dirty 候选已经共用 image-loader 导入 10–12 并真实部署，Gov 可继续查询链路。最终版本元数据按 OBS-09 收口；只回传本子任务拥有文件，未覆盖其他代理生产源码。
- 远程专用 Obs port-forward：127.0.0.1:49190（gRPC）、49191（admin），PID 文件 `runtime/port-forward.pid`；业务验收后结束本任务转发。证书临时引用仅在 `runtime/private/`，目录 0700、文件 0600，不回传 Git。
- Gov 已获得真实地址与上述 Ready Secret 引用；Gov deployment 的 env/mount 由 OBS-07 独占修改。
- 固定合同未自行修改；已确认执行差异仅见 `EXECUTION-OVERRIDES.md`。
