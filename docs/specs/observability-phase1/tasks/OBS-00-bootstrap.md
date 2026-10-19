# OBS-00：初始化项目与归档任务包

## 仓库 / 前置

- 来源：`zhangzhe-ctrl/ani-kratos-layout`，基线 `fd18422211c741dd5242d2992c3d307d522aa28c`。
- 目标：`zhangzhe-ctrl/ani-observability-service`；前置为可用的 Go、Git 和项目父目录。
- 本任务不修改 Gov、Res、Installer 或模板业务源码。

## 必读

- `../README.md`
- `../CONTRACT.md`
- `../ACCEPTANCE.md`（B01）
- 模板 `README.md`、`AGENTS.md`、`scripts/new-service`。

## 目标

按真实模板创建可独立构建的新服务，保留模板工程能力，将本任务包归档进新仓库，供后续任务使用。

## 固定改动

1. 使用模板基线的干净 checkout，建立指向该提交的专用命名分支；不要在 detached HEAD 下执行初始化。记录模板提交、分支和实际项目父目录。目标目录须不存在；已有项目时先核对初始化来源，不重复生成覆盖。
2. 先把任务专用环境变量 `OBS_WORKSPACE_DIR` 设置为实际项目父目录的非空绝对路径，再从模板目录执行：

```bash
go install github.com/go-kratos/kratos/cmd/kratos/v3@v3.0.0-20260626125723-668db92c2c00
make tools
./scripts/new-service github.com/zhangzhe-ctrl/ani-observability-service --parent "$OBS_WORKSPACE_DIR"
```

`make tools` 安装模板固定版本的 Buf；Kratos CLI 单独安装。父目录使用执行环境提供的工作区真实路径，不照抄其他机器路径。
3. 新仓库根 Go module 固定为 `github.com/zhangzhe-ctrl/ani-observability-service`；服务名固定为 `ani-observability-service`。沿用脚本的命名、来源记录和独立 Git 初始化结果。
4. 将本包目录内容完整复制到新仓库 `docs/specs/observability-phase1/`，保留相对目录结构。必须包含公共约定、任务文件、contracts、examples 和报告模板，不能仅放外部路径或链接。后续任务以该副本为入口。
5. 保留模板配置、Kratos 生命周期、admin 健康检查、自身 metrics、脱敏日志、生成验证和供应链检查。采用手工依赖组装，不增加 Wire、ProviderSet、数据库、缓存或队列。`internal/server/observability.go` 仍表示进程自身遥测。
6. 建立 `internal/{biz,data,service}/{monitoring,logs,alerts}` 模块目录。日志、告警只放 `doc.go` 职责说明，不注册空 RPC；监控业务实现留给后续任务。先不建立假的 API 依赖或 import。
7. 核对 `cmd/ani-observability-service` 和构建产物名称。OBS-01 的 API 真实发布前，根模块保持可独立构建，不 require 一个尚不存在的 API 版本。

## 验收

- 初始化来源与目标 module、服务名正确，生成文件中无错误模板 import。
- 新服务执行 `make generate`、`make verify`、`make build` 成功；记录实际命令与退出码。
- 模板 admin 健康检查可访问；不声称本任务已经接通业务指标。
- 本包在仓库中完整归档，相对引用有效；日志、告警无伪实现。
- 后续发布 API 使用新仓库的真实独立远端，不能把提交推到模板远端。

## 交付

交付初始化提交、来源记录、模块目录、归档清单及验证结果。说明当前根模块尚未消费 API，供 OBS-01 发布后继续实施。

按 `../TASK-REPORT.md` 的格式提交任务报告。
