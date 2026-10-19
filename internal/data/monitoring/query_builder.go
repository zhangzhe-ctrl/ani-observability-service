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
	registered, ok := biz.Lookup(metric.Code)
	if !ok || registered != metric {
		return "", biz.Fail(biz.InvalidQuery, fmt.Errorf("unknown metric definition"))
	}
	selector := fmt.Sprintf(`%s{ani_cluster_id=%s,ani_tenant_id=%s,ani_instance_id=%s,namespace=%s,container=%s}`,
		metric.Record, strconv.Quote(scope.ClusterID), strconv.Quote(scope.TenantID), strconv.Quote(scope.InstanceID), strconv.Quote(scope.Namespace), strconv.Quote(scope.ContainerName))
	return selector + " and (time() - timestamp(" + selector + ") <= 45)", nil
}
