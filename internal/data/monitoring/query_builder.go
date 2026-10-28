package monitoring

import (
	"fmt"
	"strconv"

	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
)

func Expression(scope target.Scope, metric biz.Metric) (string, error) {
	if err := scope.Validate(); err != nil {
		return "", biz.Fail(biz.InvalidQuery, err)
	}
	registered, ok := biz.Lookup(metric.Code, scope.Kind)
	if !ok || registered != metric {
		return "", biz.Fail(biz.InvalidQuery, fmt.Errorf("unknown metric definition"))
	}
	if scope.Kind == target.TENANT {
		return tenantExpression(scope, metric)
	}
	body := metric.Record + "{" + selector(scope) + "}"
	return body + " and (time() - timestamp(" + body + ") <= 45)", nil
}

// tenantPodView 返回租户的 pod 视图（namespace/pod 二元组集合）。三路来源互有交集，
// 且各自携带不同的 label_ani_* 标签，or 不会去重，必须用外层 max by 收敛：
//
//	① pod 带 ani.io/tenant-id                  —— 容器实例
//	② pod 带 ani.kubercloud.com/tenant-id      —— ani-system 内的推理 / 模型任务
//	③ pod 所在 ns 带 ani.io/resource-tenant-id —— 训练工作流（pod 本身无租户 label）
//
// 三段标签集不同，sum(A) or sum(B) 之类的写法会静默丢弃右侧，因此这里不预聚合。
func tenantPodView(tenant string) string {
	return `max by (namespace, pod) (` +
		`kube_pod_labels{label_ani_io_tenant_id=` + tenant + `}` +
		` or label_replace(kube_pod_labels{label_ani_kubercloud_com_tenant_id=` + tenant + `}, ` +
		`"label_ani_io_tenant_id", "$1", "label_ani_kubercloud_com_tenant_id", "(.*)")` +
		` or (kube_pod_labels * on(namespace) group_left() ` +
		`max by (namespace) (kube_namespace_labels{label_ani_io_resource_tenant_id=` + tenant + `})))`
}

// tenantExpression 返回租户级聚合的完整表达式。三条曲线共用同一个租户 pod 视图，
// 保证「同范围、同新鲜度策略」；租户身份由服务端用 strconv.Quote 注入，
// 不接受上游任何片段。
func tenantExpression(scope target.Scope, metric biz.Metric) (string, error) {
	tenant := strconv.Quote(scope.TenantID)
	view := tenantPodView(tenant)
	switch metric.Code {
	case biz.TenantCPU:
		// 45 秒新鲜度：已删除 pod 的历史样本不再计入；cAdvisor 15s 抓取留 3 个周期。
		src := `rate(container_cpu_usage_seconds_total{container!="",container!="POD",namespace!="",pod!=""}[5m])`
		fresh := `time() - timestamp(container_cpu_usage_seconds_total{container!="",container!="POD",namespace!="",pod!=""}) <= 45`
		return "sum((" + src + " and (" + fresh + ")) * on(namespace, pod) group_left() " + view + ")", nil
	case biz.TenantMemory:
		src := `container_memory_working_set_bytes{container!="",container!="POD",namespace!="",pod!=""}`
		fresh := `time() - timestamp(container_memory_working_set_bytes{container!="",container!="POD",namespace!="",pod!=""}) <= 45`
		return "sum((" + src + " and (" + fresh + ")) * on(namespace, pod) group_left() " + view + ")", nil
	case biz.TenantGPU:
		// HAMi 容器级显存实际用量。label_replace 把 podnamespace/podname 投影成
		// namespace/pod（HAMi 自带的 namespace 是 device-plugin 所在的 kube-system）；
		// max by 去重新旧两个 ServiceMonitor 的重复采集；90s 新鲜度对应 HAMi 30s 采集。
		usage := `label_replace(label_replace(max by (podnamespace, podname, ctrname, vdeviceid) (` +
			`vGPU_device_memory_usage_in_bytes and (time() - timestamp(vGPU_device_memory_usage_in_bytes) <= 90)` +
			`), "namespace", "$1", "podnamespace", "(.*)"), "pod", "$1", "podname", "(.*)")`
		return "sum(" + usage + " * on(namespace, pod) group_left() " + view + ")", nil
	default:
		return "", biz.Fail(biz.InvalidQuery, fmt.Errorf("unknown tenant metric"))
	}
}

// selector 返回不带花括号的 matcher 串。训练 kind 追加 pod_uid，实例 kind 保持五个 matcher。
func selector(scope target.Scope) string {
	s := fmt.Sprintf(`ani_cluster_id=%s,ani_tenant_id=%s,ani_instance_id=%s,namespace=%s,container=%s`,
		strconv.Quote(scope.ClusterID), strconv.Quote(scope.TenantID), strconv.Quote(scope.InstanceID),
		strconv.Quote(scope.Namespace), strconv.Quote(scope.ContainerName))
	if scope.Kind == target.TrainingExecution {
		s += fmt.Sprintf(`,pod_uid=%s`, strconv.Quote(scope.PodUID))
	}
	return s
}
