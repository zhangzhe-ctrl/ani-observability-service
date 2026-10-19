# OBS-01：建立并发布独立 API 子模块

## 仓库 / 前置

- 仓库：`ani-observability-service`。
- 前置：OBS-00 完成；已具备新仓库真实远端。

## 必读

- `../README.md`
- `../CONTRACT.md`
- `../contracts/api/` 中的两份固定协议。
- `../ACCEPTANCE.md`（B02–B04）

## 目标

在同仓库建立独立、可消费的 API Go 子模块。Gov 和服务实现共享契约，API 不依赖服务实现。

## 固定改动

1. 新增 `api/go.mod`、`api/go.sum`，module 固定为 `github.com/zhangzhe-ctrl/ani-observability-service/api`，Go 基线 `1.26.7`。直接 runtime 依赖固定为 `google.golang.org/grpc v1.82.1`、`google.golang.org/protobuf v1.36.11`；保留 tidy 得出的必要间接依赖，不复制根模块依赖清单。
2. 按固定协议落地 `api/observability/target/v1/target.proto`、`api/observability/monitoring/v1/monitoring.proto` 及生成代码。package、字段号、枚举和 `go_package` 以 CONTRACT/contracts 为准；MonitoringService 只注册 QueryRange、QueryLatest。
3. API 只含 proto、标准 Go protobuf/gRPC 生成代码及必要契约测试。不加入 Kratos HTTP/error/client 包装、Prometheus 查询、配置加载或根服务 import。配置 protobuf 继续属于根模块。
4. 根 `buf.yaml` 保留 `path: internal`，增加 `path: api`。保留现有 `buf.gen.yaml` 只生成内部配置，新增 `buf.gen.api.yaml`：

```yaml
version: v2
inputs:
  - directory: api
plugins:
  - local: [go, run, google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11]
    out: api
    opt: paths=source_relative
  - local: [go, run, google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1]
    out: api
    opt: paths=source_relative
```

不得设置 `clean: true`，以免清空同目录 proto/go.mod。不得关闭 Buf lint 规避目录或枚举问题。
5. `Makefile` 增加 phony API 生成入口，`make generate` 同时生成配置与 API。`scripts/verify-source` 显式执行两份生成模板，并把 API 纳入现有全树生成前后比对。根 `go generate ./...` 不代表已覆盖嵌套 module。
6. 扩展 make verify/CI，显式对 API 与根模块分别执行 tidy-diff、test、vet、mod verify；生成检查覆盖两者。修改服务 VERSION 的 tag 选择，使用 `git describe --tags --match 'v[0-9]*' --always --dirty`，避免 `api/*` tag 被当作服务版本。
7. 先提交并发布 API 可引用版本。首个 tag 使用 `api/v0.1.0`，Go require 版本为 `v0.1.0`；禁止手造版本、移动已有 tag。记录完整提交、tag/module version。
8. 从工作区外、`GOWORK=off` 且无本地 replace 的消费者验证 API 可下载和编译。本地联调可使用 go.work，但不能替代发布验收。
9. 本任务结束时根模块尚未实际 import API，不强加会被 tidy 移除的 require。把真实 API 版本交给 OBS-05、OBS-07；OBS-05 首次实现 gRPC service 并真实导入时，再固定根 go.mod/go.sum。OBS-04 只实现 domain/data，不通过无意义空白 import 保住依赖。

## 验收

- 在 API 目录使用 GOWORK=off，分别执行 `go mod tidy -diff`、`go test ./...`、`go vet ./...`、`go mod verify` 通过；根模块原有验证仍通过。
- 重新生成无差异；API 不导入根模块或服务实现。
- 干净消费者通过真实版本导入两个 API 包；包所属 module 为 `.../ani-observability-service/api`。
- 不要求 Gov 整个依赖图移除其既有 Kubernetes/OTel 依赖；只验证本 API 边界。
- 发布版本可获取后才标记完成；仅在 go.work 中编译成功不算发布。

## 交付

交付 proto、生成代码、两个 Go module 的验证结果、生成脚本改动和可消费 API 版本记录。向后续任务提供精确 `go get .../api@<版本>` 参数，不使用 `@latest`。

按 `../TASK-REPORT.md` 的格式提交任务报告。
