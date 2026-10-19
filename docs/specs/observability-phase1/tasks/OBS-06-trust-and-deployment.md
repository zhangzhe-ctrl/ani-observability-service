# OBS-06 受信 gRPC 与服务部署

## 仓库与前置

仓库：ani-observability-service。前置：OBS-05 监控 service。实际部署需要真实 namespace、datasource、镜像和证书引用。

必读：`../README.md`、`../CONTRACT.md` 第 8–10 节、`../ACCEPTANCE.md` 的 S 类用例。

## 目标

将监控模块接入模板的配置、启动与关闭链路，仅向受信 Gov 提供两个 gRPC 方法，并交付可运行服务。

## 固定改动

1. 扩展 `internal/conf/v1/` 的 monitoring、governance 配置，保持原有字段编号与生成链。cluster_id 重复/非法、address 缺失、证书或凭据不可读时启动失败；数据源临时离线按请求失败，不把全部 readiness 绑定单源健康。
2. 在 `internal/server/governance.go` 构造 TLS≥1.3 监听，验证客户端 CA 与精确 ani-governance 身份；校验三个单值受信 metadata 和 target tenant 一致。允许方法仅 MonitoringService 的 QueryRange/QueryLatest，reflection 关闭。
3. 服务端证书身份固定 ani-observability-service；文档明确 Gov 必须验证该身份。不要复用 Res 旧的 ani-network-service 名称，也不允许任意同 CA 客户端。
4. 在 `cmd/ani-observability-service/monitoring.go` 实现 openMonitoring，接入现有手工 wiring/cleanup；注册监控 service，传播6s父RPC期限、4s Prom预算、请求取消；退出时关闭连接/监听。保留模板 admin、健康、自身 metrics/log，业务逻辑不写入进程遥测文件。
5. 交付 Dockerfile 和 `deployments/observability/{README.md,configmap.yaml,deployment.yaml,service.yaml,secret.example.yaml}`，配置键/端口遵循合同。一期1副本、私网ClusterIP、admin探针；无业务数据库、队列、PVC或查询Kubernetes的额外RBAC。
6. secret.example 只提供键名和挂载方式；实际凭据使用已有证书与Secret流程。真实配置、镜像tag/digest、发布命令和应用结果写入报告。普通租户工作负载不能绕过受信查询入口；检查实际网络与mTLS边界。若发现直连底层查询入口的路径，以现有网络能力补最小访问策略，归档networkpolicy.yaml；只覆盖本次入口，保留采集、平台访问和探针。
7. logs/alerts 各层仅 doc.go，无空壳业务RPC、数据库表或后台任务。

## 验收

- S01–S06：有效Gov证书可调用；无证书、错CA、同CA错身份、伪造/多值/缺失元数据、租户不一致和未知方法拒绝。
- 配置错误可读且不泄密，单源查询故障不会将其他能力误判为全服务故障。
- 根服务构建、生成、audit按既有门槛执行，真实部署启动和探针通过；受信客户端调用可达配置的Prometheus，缺失业务时序仍按正常NO_DATA语义处理。
- 实际部署或证书输入缺失时明确未完成项；只构建镜像/manifest不等于部署验证完成。

## 交付

提交配置proto与生成物、监听/装配/cleanup、部署文件和安全验证。向OBS-07交付实际gRPC地址、CA与客户端证书引用、服务端身份及两个完整方法名。按 `../TASK-REPORT.md` 汇报。
