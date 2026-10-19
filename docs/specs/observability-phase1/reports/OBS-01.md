# OBS-01

状态：进行中，尚未完成发布消费者验收。

API module 为 `github.com/zhangzhe-ctrl/ani-observability-service/api`，Go 1.26.7、gRPC 1.82.1、protobuf 1.36.11，固定 proto 原文生成。两套 Buf 生成模板及 Makefile/verify-source 显式覆盖 API；服务 VERSION 排除 API tags。API 不导入根服务，根模块尚不 require API。

Fedora `make api`、API `go mod tidy`、`make verify-api` 已通过，包含 optional value 的 protobuf presence 业务测试；日志任务根 `logs/bootstrap/api-{generate,tidy,verify}.log`。根/API 两模块联合 `make verify` 正在执行。

下一步：发布真实 `api/v0.1.0`，在工作区外 GOWORK=off、无 replace 的消费者下载并编译，然后记录确切提交与消费者版本，才标记完成。
