# OBS-00

状态：已完成。模板来源 `fd18422211c741dd5242d2992c3d307d522aa28c`，命名分支 `codex/observability-bootstrap-20261019`，通过固定 `scripts/new-service` 在 Fedora 初始化独立服务。来源工具身份及 SHA 见 `docs/scaffold/provenance.md`。

本地源码：`/home/chabking/workspace/ani-observability-service`。Fedora 任务根：`/home/chabking/ani-observability-runs/phase1-20261019-01a15321`，初始化父目录 `src/`。

实际验证（Fedora，均退出 0）：`make generate`、`make verify`、`make build`，真实生成二进制的 `/healthz`、`/readyz`、`/metrics` 以及进程终止。日志位于任务根 `logs/bootstrap/{generate,verify,build,runtime-corrected}.log`。根模块此时不导入 API。

初始失败保留：共享模块缓存 lock 权限错误改用任务私有缓存；插件 sumdb EOF 改用明确的校验镜像地址且继续校验。独立探针首轮错误地使用无 ANI 前缀的环境变量导致探针目标端口错误，日志 `runtime.log`；修正为模板实际的 `ANI_SERVER_*` 后通过，未改产品代码。

原包完整归档，原 SHA256SUMS 保留；用户变更见 `../EXECUTION-OVERRIDES.md`。监控、日志、告警目录就位，后两者仅职责说明。后续 API 与业务任务不以初始化检查代替真实指标验收。
