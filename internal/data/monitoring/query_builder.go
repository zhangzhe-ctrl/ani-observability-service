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

// tenantExpression 返回租户级聚合的完整表达式。租户编码不是 record+selector
// 可拼出的形态（GPU 是 volcano 指标 join 租户标签的固定表达式），因此按编码
// 整体返回；租户身份由服务端用 strconv.Quote 注入，不接受上游任何片段。
func tenantExpression(scope target.Scope, metric biz.Metric) (string, error) {
	tenant := strconv.Quote(scope.TenantID)
	switch metric.Code {
	case biz.TenantCPU, biz.TenantMemory:
		body := metric.Record + `{ani_tenant_id=` + tenant + `}`
		// 45 秒新鲜度过滤：已删除实例的历史样本不再计入租户曲线。
		return "sum by(ani_tenant_id)(" + body + " and (time() - timestamp(" + body + ") <= 45))", nil
	case biz.TenantGPU:
		// volcano per-pod 指标的 namespace/pod 是 scheduler 自身的，业务 pod 名
		// 在 podName，必须先 label_replace 改写成 pod 再 on(pod) join；
		// 平台任务没有 label_ani_io_tenant_id，join 后自然被过滤。份额/100 = 等效卡数。
		return `sum(label_replace(volcano_vgpu_device_core_allocation_for_a_certain_pod, "pod", "$1", "podName", "(.*)") * on(pod) group_left(label_ani_io_tenant_id) kube_pod_labels{label_ani_io_tenant_id=` + tenant + `}) / 100`, nil
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
