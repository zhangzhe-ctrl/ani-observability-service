# 一期验收清单

本清单的状态在任务报告中填写。本任务包交付时所有项目均未执行；示例数据、源码审阅和离线 fixture 不能替代要求真实环境的条目。

测试用于证明可信范围、历史身份、数据语义和边界，不为目录占位等低影响改动增加镜像测试。已完成的有效验证可复用，不为收口无目的重跑所有场景。

## 1. 构建与交付 B 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| B01 | OBS-00 | 从固定模板以实际命名分支初始化，模块/命令名正确；手工装配与现有健康能力正常；根模块在 API 首次发布前可独立构建。 |
| B02 | OBS-01 | API 子模块独立 tidy-diff、test、vet、mod verify；生成代码存在；API 版本已真实可获取，记录真实 tag/commit 与消费者版本。 |
| B03 | OBS-01 | 原配置与 API 两份生成链均执行，verify-source 能发现生成漂移；没有 clean:true 删除输入；API 不依赖根服务。 |
| B04 | OBS-01、05、07 | API tag 不污染服务 VERSION；根服务/Gov 从实际 API 版本构建，不依赖本地 replace/go.work；Gov 的既有其他业务依赖不作为新增 API 隔离失败依据。 |
| B05 | OBS-06、07、09 | GOWORK=off 下 API、根服务、Gov/Res 相关构建验证通过；既有 audit、生成和依赖检查结果明确，不能忽略真实失败。 |
| B06 | OBS-09 | 三个实现仓库的提交、API/Res版本、镜像digest、实际手工patch/规则、部署与验证材料对应；全部必需真实项通过才宣告一期完成。 |

## 2. Res 范围解析 R 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| R01 | OBS-03 | 请求合法时从持久化记录返回 tenant/instance/cluster/namespace/workspace，字段与存储相同。 |
| R02 | OBS-03 | 跨租户、伪造身份或不受信调用按既有机制拒绝；body tenant 与可信租户不一致不能继续读记录。 |
| R03 | OBS-03、08 | 实例停止或正常删除后，只要持久化范围存在，仍可解析；真正不存在按 NotFound。真实历史按 E04 验证。 |
| R04 | OBS-03 | delete-first tombstone 等缺少可靠位置返回 INSTANCE_METRIC_SCOPE_UNAVAILABLE，Gov 明确409；不猜namespace或cluster。 |
| R05 | OBS-03 | 查询没有 Kubernetes/runtime/Observe 请求或持久化更新，不扩展启停、配额、调度语义；精确 RPC 受信允许项与文档/生成物完整。 |

## 3. 手工采集 C 类

| ID | 验证层级 | 验收内容与通过条件 |
| --- | --- | --- |
| C01 | 真实 | 记录实际Kubernetes context、Res cluster_id、Prometheus/Operator/KSM版本、采集对象与选择器，变更前字段可复核。 |
| C02 | 真实 | 五类所需原始指标确有样本；所选CPU为唯一整容器counter，workspace、UID/CID、KSM ANI业务标签来源可验证。 |
| C03 | 真实 | 原始时序与record都含同一ani_cluster_id，与Res、Obs datasource一致；KSM allowlist保留旧项，不能仅配置external_labels。 |
| C04 | fixture + 真实 | 每种声称支持的systemd/cgroupfs格式有可追溯依据与fixture；实际环境格式被验证；UID/CID规范化一致，非法/未知路径不回退Pod名。 |
| C05 | fixture | CPU至少2有效样本、5m rate、counter reset、启动窗口、总量维度处理符合合同；内存为working set。不能用任意sum掩盖多来源。 |
| C06 | fixture | 原始当前样本、KSM info/running/labels分别检查45s；过期current/snapshot不能经lookback或下游重录而续命；边界含45s、不含超过45s。 |
| C07 | fixture + 真实 | 同UID换CID及同名换UID的旧counter不被当前身份重新带入；旧CID在5m内残留且已有新CID时仍能排除旧运行身份。真实更换见E03。 |
| C08 | fixture + 现场诊断 | 原始重复采集在删除transport标签前发现、抑制并报告冲突；禁止max/sum/topk/labeldrop掩盖。抑制后NO_DATA不伪称可反推历史AMBIG。 |
| C09 | 真实 | PrometheusRule实际被选中、无lastError、持续求值；两个record及七标签真实存在；有界负载后有可解释变化。仅promtool或/metrics200不够。 |
| C10 | 文件 + 真实变更记录 | 实际patch保留既有参数、端点、过滤和allowlist；验证脚本可执行；保存应用差异、恢复本次字段的方法；不修改installer或回滚整个release/PVC/TSDB。 |

OBS-02负责C类。C05–C08的fixture必须能稳定重现失败条件；C01–C03、C09不得用模拟环境冒充。若只在一个实际cgroup模式上测试，报告清楚范围，不宣称两个模式都经过实机测试。

## 4. Prometheus 适配 D 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| D01 | OBS-04 | 固定metric code到record映射；五个可信scope matcher不可省略或被外部输入扩展；未知code与非法范围不形成PromQL。 |
| D02 | OBS-04 | 历史查询逐求值点限制record年龄45s，既不过滤所有历史，也不把矩阵求值时间当原始record时间；查询不依赖当前KSM。 |
| D03 | OBS-04 | 已知cluster精确选源，未知source503，重复配置启动拒绝；无跨源回退、用户URL或动态发现。 |
| D04 | OBS-04 | 正常matrix/vector、空结果、整秒时间、scope标签校验正确；损坏结构、重复点、缺失身份、意外resultType明确502。 |
| D05 | OBS-04 | limit129检测128条上限；有界读取8MiB；超限明确422，不能截断后返回200；边界值可通过。 |
| D06 | OBS-04 | Prom status=error、连接失败、5xx、超时、非空warnings、畸形JSON按合同映射；无错误吞成空数组。 |
| D07 | OBS-04、05 | 批次两查询共用4s与更短父deadline，无自动重试；请求取消会终止HTTP读写，关闭body和资源。 |

## 5. 业务规范化 N 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| N01 | OBS-05 | 有效0保留为present0，缺失是absent；不从GetValue默认值推断presence。 |
| N02 | OBS-05 | 全空/部分空返回完整series和网格，状态NO_DATA/PARTIAL正确；正常点无gap_reason。 |
| N03 | OBS-05 | 不重叠的UID/CID历史正常拼接；同一指标同一点多来源为null/AMBIGUOUS_SOURCE，不相加或任选。 |
| N04 | OBS-05 | NaN、±Inf、负数单来源为null/INVALID_VALUE；不能输出非法JSON数值。多来源裁决优先。 |
| N05 | OBS-05 | start=end、end不在网格、时区转换、整秒、point数正确；不擅自对齐、不插值、不前值填充。 |
| N06 | OBS-05 | 未知/重复指标、非法ID/namespace/container、未来end、倒置时间、超24h、step0/非30倍数/越界和点数上限有确定400。 |
| N07 | OBS-05 | latest批次只捕获一次evaluated_at，两个指标同一时刻且各恰1点；输出顺序按请求。 |
| N08 | OBS-05 | 任一上游失败整批失败，并取消另一项；合法单指标数据缺口仍为成功加null。 |
| N09 | OBS-05 | 总并发16、每租户4按请求而非指标计数；超限立即429；成功/失败/取消释放令牌，不同租户互不误占。 |

## 6. 信任与部署 S 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| S01 | OBS-06、07 | 真实受信Gov证书+正确scope可调用；Gov验证ani-observability-service服务身份，TLS≥1.3。 |
| S02 | OBS-06 | 无客户端证书、错CA、同CA错误客户端身份拒绝；不能照搬旧Res服务SAN。 |
| S03 | OBS-06、07 | 缺失/多值/不合法metadata、伪造actor/tenant、target tenant不一致拒绝；Gov重建可信header，不透传外部同名值。 |
| S04 | OBS-06 | 仅两个精确MonitoringService RPC允许；reflection关闭，任意方法或跨租户管理员scope不能放行。 |
| S05 | OBS-06 | 非法配置启动失败；单源临时不可用按查询错误；真实Deployment/ClusterIP/探针正常，关闭清理能结束连接和监听。 |
| S06 | OBS-06、08 | 从实际租户工作负载验证无法绕过受信入口获得其他实例指标；核实Obs与底层Prom查询面的网络/认证边界，不能只以ClusterIP推断隔离。 |

## 7. Gov HTTP H 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| H01 | OBS-07 | 两个精确/api/v1路由返回200；POST同步查询不进入202、写入幂等或配额逻辑。data为实际结构，路由不重复注册。 |
| H02 | OBS-07 | body缺少data、未知/重复JSON字段、同名path/body ID、PromQL/scope/datasource、额外query参数拒绝；超过64KiB是413。 |
| H03 | OBS-07 | step省略为60、显式0拒绝，使用真实HTTP decoder验证presence；0、null、正常值在实际codec中有区别。 |
| H04 | OBS-07、08 | instance:read权限和现有选租户机制生效；API同步及权限注册实际执行，只有代码声明不算完成。 |
| H05 | OBS-07 | Res返回范围与本次tenant/instance一致才调用Obs；ID来自路径，无客户端选择集群能力；只读OwnerClient与fake同步更新。 |
| H06 | OBS-07 | handler设置Operation并执行现有Middleware；仅两个精确路由10s、Res2s、Obs6s与Prom4s；更短deadline与取消不被抹掉。 |
| H07 | OBS-07 | 各reason对应HTTP码；ResourceExhausted区分422/429；错误使用既有外壳，不按日志接口统一映射。 |
| H08 | OBS-07 | OpenAPI和实际响应同为snake_case、value可null、状态短字符串，正常点不含gap_reason；不泄露原始labels/PromQL/dev_profile。 |
| H09 | OBS-07 | latest拒绝body与query参数，固定CPU后内存且各1点，evaluated_at与points时间相同。 |

## 8. 真实链路 E 类

| ID | 负责任务 | 验收内容与通过条件 |
| --- | --- | --- |
| E01 | OBS-08 | 经真实Gov认证查询CPU与内存范围/最新值，可对应到真实Res范围、Obs调用和Prom记录；不是仅grpc fixture成功。 |
| E02 | OBS-08 | 专用实例施加有界CPU/内存负载，基线与变化可解释；同scope、同求值点、同step对比Prom结果，在记录的浮点容差内一致。 |
| E03 | OBS-08 | 保留实际CID更换和Pod UID更换证据，业务实例ID不变；历史可解释，旧counter不串到新对象，多源重叠不求和。 |
| E04 | OBS-08 | 停止/正常删除Pod后，在业务范围仍可解析、数据保留期内，查询旧时间段成功；当前规则健康不作为旧历史前置；规则启用前不伪造回填。 |
| E05 | OBS-08 | 实际跨租户、无权限、伪造内部身份失败；真实接口拒绝任意PromQL和scope控制。 |
| E06 | OBS-08 | 可控连接故障/超时/取消按约定失败，无伪正常空数组；负载和故障操作不破坏共享组件，完成后恢复验证资源。 |
| E07 | OBS-08、09 | 真实request/response、request_id、Prom对照、运行身份时间线、版本、手工配置和恢复记录归档；未实际完成项明确，不写“全链路完成”。 |

核心数据、真实认证、真实链路和运行对象更换必须有环境证据。极端NaN、精确并发重叠、上游重复点等允许由确定的fixture验证，但报告要注明其测试层级。

## 9. 证据归档

脱敏、可提交的报告放 `docs/specs/observability-phase1/reports/`；原始采集/资源对象与运行记录按手工专项放忽略提交的 artifacts 目录，报告指向其实际位置。归档请求时移除Authorization、cookie、私钥、完整kubeconfig等凭据。

所有用例记录通过/失败/未执行、真实命令、关键输出、时间和证据位置。不要伪造成功标记；环境缺失只阻挡相应任务，其他独立工作继续完成。
