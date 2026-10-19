# OBS-04 Prometheus 查询适配

## 仓库与前置

仓库：ani-observability-service。前置：OBS-01 已形成可消费 API 契约；实现可使用 HTTP fixture，不以此替代 OBS-02 的真实采集验收。

必读：`../README.md`、`../CONTRACT.md` 第 3、6、7、9 节，以及 `../ACCEPTANCE.md` 的 D 类用例。

## 目标

实现只按可信范围查询两个规范 record 的数据源层，限定地址、表达式、返回规模和超时；不暴露原始 PromQL 或访问 Kubernetes。

## 固定改动

1. 在 `internal/biz/target/` 定义所需领域范围，在 `internal/biz/monitoring/` 定义数据层需要的查询/原始时序接口；仅支撑本期两个用例，不建通用插件框架。此任务不为了添加依赖而无意义导入 API；根模块首次真实 API import 与 pin 在 OBS-05 完成。
2. 实现 `internal/data/monitoring/query_builder.go`：metric code 只能映射到固定两个 record；scope matcher 精确包含 cluster、tenant、instance、namespace、workspace；保持 UID/CID 供后续来源裁决。对每一求值点限制 record 年龄 45s，等价于 record and (time()-timestamp(record)<=45)，不得使用现在的 KSM/Pod 或只对最后一点判断年龄。
3. 实现 `internal/data/monitoring/prometheus.go`：通过静态配置中 cluster_id 唯一定位 datasource，未知源返回 SOURCE_NOT_CONFIGURED，不回退、不遍历全部源。address、证书、token 仅来自服务配置；HTTP 请求传入父 context。
4. range 使用 /api/v1/query_range、latest 使用 /api/v1/query；明确传时间、step、timeout 与 limit=129。两个查询共用一次 4s 批次预算，不重试。复用有界 HTTP client，正确关闭 response body 和生命周期资源。
5. HTTP 层有界读取单响应最多 8MiB，不能先无界读再检查。每指标最多 128 条原始返回 series，检测 129 即 RESULT_TOO_LARGE，不能静默截断。只接受所需 matrix/vector 类型、合法 timestamp 和数值表示。
6. 保留七项身份与值的 presence；验证 scope 标签、不缺 UID/CID；返回损坏数据、重复点或越界点时明确错误。NaN/Inf/负值交由业务层输出 INVALID_VALUE，不直接把它们编码为 JSON 数字。
7. 按合同分类非 2xx、Prom status=error、超时、warnings、畸形 JSON、响应过大等；不能将请求失败变成空结果。结构正确的空结果是合法输入，交由 OBS-05 补 null 网格。

## 验收

- D01–D07：固定表达式可信过滤、历史 freshness、按集群选源、matrix/vector、limit、体积上限、错误和取消。
- HTTP 级测试验证端点、参数、deadline 与 reader 上限，覆盖恶意/损坏上游；范围/最新值的无数据作为成功空输入传出。
- 构造跨租户标签、未知 datasource、重复点和超大响应，均有确定错误，不泄漏或忽略。
- 真实 Prom 测试留给 OBS-08；本任务只声明完成适配器及其可重复验证。

## 交付

提交接口、adapter、固定 query builder、必要测试和 datasource 配置字段说明；给 OBS-05 明确输入/输出、错误与 cleanup。按 `../TASK-REPORT.md` 交付实际命令和结果。
