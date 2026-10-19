# OBS-04 Prometheus 查询适配

状态：已完成代码与 HTTP 行为验证；真实数据对照归 OBS-08，不以 fixture 代替采集验收。

起始提交 `4827e664ee7a5dc91ec4707f407838045bec718b`，分支 `codex/observability-phase1-20261019`。固定查询在 `internal/data/monitoring/`，可信领域范围在 `internal/biz/target/`。唯一静态集群源、五个精确 matcher、保留 UID/CID、每点 freshness 45s；POST 到标准 query/query_range，固定 limit=129、4s 预算、有界连接、8MiB 有界读取，不重试、不接受任意表达式。启动校验地址/CA/token，退出关闭连接。

Fedora 任务根 `/home/chabking/ani-observability-runs/phase1-20261019-01a15321`，全部测试 `GOWORK=off`、私有缓存。`go test -count=1 -v ./internal/data/monitoring ./internal/service/monitoring` 通过，日志 `logs/bootstrap/monitoring-data-fixture-corrected.log`；完整联合验证见 OBS-09。覆盖 D01–D07：真实 HTTP 请求参数、相同 scope/freshness、matrix/vector、恶意标签、NaN/Inf/负值转交、重复/越界点、128/129 series、8MiB reader、HTTP/Prom 错误、warnings、损坏 JSON、源未知、取消。

初始测试中取消 fixture 在未消费 POST body 时等待请求 Context，造成测试服务器关闭等待；诊断 goroutine 堆栈及非零结果保留在 `logs/bootstrap/monitoring-behavior.log`。仅修正 fixture 先解析 body 后通过，未删除生产断言。上述外部 Prom HTTP fixture 不模拟业务用例或鉴权。

本任务没有 Kubernetes 查询、存储、动态源、GPU 指标、重试或额外 API。关闭职责由 composition 持有；监控用例及真实 API pin 见 OBS-05。
