# OBS-01

状态：已完成。

API module 为 `github.com/zhangzhe-ctrl/ani-observability-service/api`，Go 1.26.7、gRPC 1.82.1、protobuf 1.36.11，固定 proto 原文生成。两套 Buf 生成模板及 Makefile/verify-source 显式覆盖 API；服务 VERSION 排除 API tags。API 不导入根服务；根模块在首次真实业务 import 后才固定 API 依赖。

Fedora `make api`、API `go mod tidy`、`make verify-api` 已通过，包含 optional value 的 protobuf presence 业务测试；日志任务根 `logs/bootstrap/api-{generate,tidy,verify}.log`。根/API 联合 `make verify` 已通过（`api-root-verify-with-git.log`）。首次联合检查在最后 `git diff --check` 因转运副本缺少 Git 元数据返回129；保留 `api-root-verify.log`，补齐任务独占 Git 边界后按原门禁通过。

已发布 `api/v0.1.0`，commit `4827e664ee7a5dc91ec4707f407838045bec718b`，远端为 `ssh://git@ssh.github.com:443/zhangzhe-ctrl/ani-observability-service.git`，消费者 module version `v0.1.0`。

独立 Fedora `api-consumer/` 位于服务工作区外，独占空模块缓存，GOWORK=off、无 replace；`go mod tidy`、`go test -v ./...`、`go list -m` 实际通过，证明两个 API 包可从发布版本取得并编译，日志 `logs/bootstrap/api-consumer.log`。Gov 的独立真实下载也通过。上述发布和构建不代表监控链路已经验收。

安全修复补丁：后续根服务 `make vuln` 发现模板依赖中实际可达的 gRPC / x/net / OTel 漏洞，原始失败保留。API `api/v0.1.1` 对应 `44728ffbb722b30194a0cd8a25e60c5022ed0dfc`，升级 Go 1.26.9、gRPC 1.83.2、x/net 0.60.0、x/text 0.42.0；proto 字段与编号不变，未移动原 v0.1.0 标签。补丁的 `make verify`（含 API GOWORK=off tidy-diff/test/vet/mod verify）通过，日志 `logs/bootstrap/api-security-{tidy,full-verify}.log`。正式根服务和 Gov 改为消费 v0.1.1。
