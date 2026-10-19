# OBS-05 监控用例与结果规范化

## 仓库与前置

仓库：ani-observability-service。前置：OBS-04 数据层、OBS-01 API 真实版本。

必读：`../README.md`、`../CONTRACT.md` 第 4–7、9 节、`../contracts/api/`、`../ACCEPTANCE.md` 的 N 类用例。

## 目标

实现 QueryRange/QueryLatest、固定指标定义、完整时间网格和逐点缺失语义，提供可注册的 gRPC service。本任务首次实际使用已发布 API，并固定根模块依赖；网络信任与部署在 OBS-06 完成。

## 固定改动

1. catalog.go 仅登记 cpu.usage_cores/cores 和 memory.working_set_bytes/bytes、对应 record、口径 v1 与 workspace 范围，不新增目录查询 API 或动态配置。
2. query.go 校验 target、指标数量/重复/未知、时间顺序/整秒/未来、24h、step 30–3600 且为 30 倍数、3000 点上限。内部 step 不允许 0；公共省略默认由 Gov 处理。重复 code 拒绝，不能自行去重。
3. QueryRange 保持 start+n*step≤end 的网格，不对齐、不插值；同请求不同指标的点时间完全一致。QueryLatest 一次捕获整秒 evaluated_at，两项调用复用同一时刻，每项返回一个点。
4. normalize.go 对每指标每点执行来源裁决：0 个为 null/NO_DATA；1 个有限非负值直接返回，0 保留；多 UID/CID 来源为 null/AMBIGUOUS_SOURCE；1 个 NaN、Inf 或负数为 null/INVALID_VALUE。先后来源不在同一时刻重叠时正常拼接。
5. series 顺序按请求；正常点不带缺失原因，所有点有效 OK、部分有效 PARTIAL、无有效值 NO_DATA。空 Prom 输入仍产出全部所需 series 与 null 网格，不能省略指标或伪装成 0。
6. 一次请求最多并发两条 Prom 查询，共用 4s 预算；一项上游失败取消另一项并整批失败。每副本总并发16、每租户4，按请求计数，超限立即429对应错误。所有成功/失败/取消路径释放令牌，不增加重试或排队系统。
7. 实现 `internal/service/monitoring/service.go` 的两个 RPC 转换，检查 optional double presence，按合同处理枚举与错误 reason。根 go.mod/go.sum 此时引用 OBS-01 真实 API 版本，不用本地 replace。不得把领域逻辑塞进生成文件。

## 验收

- N01–N09：零值/缺失、全部空、部分空、来源前后更换/重叠、NaN/Inf/负值、未对齐 end、同一 latest 时刻、非法查询、批次失败、租户并发和取消资源释放。
- 用固定 clock/受控上游验证时间和并发行为；不依赖长时间 sleep 或仅对实现细节作镜像断言。
- gRPC 返回中 present 0 与 absent value 可区分，未知枚举/无效 scope 被拒绝；全空网格点数准确。
- GOWORK=off 的根模块验证通过，证明第一次 API pin 可独立消费。

## 交付

提交 usecase、catalog、normalize、gRPC service 和必要测试；向 OBS-06 交付构造依赖、已固定 API 版本和关闭职责。按 `../TASK-REPORT.md` 汇报。
