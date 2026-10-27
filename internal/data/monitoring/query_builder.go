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
	body := metric.Record + "{" + selector(scope) + "}"
	return body + " and (time() - timestamp(" + body + ") <= 45)", nil
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
