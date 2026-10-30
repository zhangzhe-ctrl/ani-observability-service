package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	usage "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/usage"
)

func TestUsageGroupedQueryDecodesCoverageAndPreservesMissing(t *testing.T) {
	tenant := "11111111-1111-4111-8111-111111111111"
	start := time.Unix(1700000000, 0).UTC()
	q := usage.PodQuery{Start: start, End: start.Add(time.Minute), Step: 30 * time.Second, Cluster: dataQuery().Target.ClusterID, Tenants: []string{tenant}, Metrics: []string{"cpu"}}
	p := provider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		expr := r.Form.Get("query")
		for _, part := range []string{"usage_tenant", "kube_pod_container_status_running", "observed_cpu", "inventory", "conflicts", "<=45"} {
			if !strings.Contains(expr, part) {
				t.Errorf("missing coverage evidence %s", part)
			}
		}
		rows := []any{}
		for stat, value := range map[string]string{"expected": "2", "observed_cpu": "1", "cpu": "0", "inventory": "1", "conflicts": "0"} {
			rows = append(rows, map[string]any{"metric": map[string]string{"usage_tenant": tenant, "usage_metric": stat}, "values": []any{[]any{start.Unix(), value}}})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": rows}})
	})
	result, err := p.QueryUsagePods(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	points := result[tenant]["cpu"]
	if len(points) != 2 || points[0].Value == nil || *points[0].Value != 0 || *points[0].Expected != 2 || *points[0].Observed != 1 || points[1].Value != nil {
		t.Fatalf("coverage or missing sample lost: %+v", points)
	}
}

func TestUsageRejectsUntrustedResultTenant(t *testing.T) {
	q := usage.PodQuery{Start: time.Unix(1700000000, 0), End: time.Unix(1700000060, 0), Step: 30 * time.Second, Cluster: dataQuery().Target.ClusterID, Tenants: []string{"11111111-1111-4111-8111-111111111111"}, Metrics: []string{"cpu"}}
	p := provider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": []any{map[string]any{"metric": map[string]string{"usage_tenant": "22222222-2222-4222-8222-222222222222", "usage_metric": "cpu"}, "values": []any{[]any{1700000000, "5"}}}}}})
	})
	if _, err := p.QueryUsagePods(context.Background(), q); err == nil {
		t.Fatal("unexpected cross-tenant series accepted")
	}
}

func TestUsageEmptyCountsNeedFreshTenantOwnershipProof(t *testing.T) {
	tenant := "11111111-1111-4111-8111-111111111111"
	start := time.Unix(1700000000, 0).UTC()
	for _, ownership := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing tenant labels", true: "confirmed mapped idle pod"}[ownership], func(t *testing.T) {
			q := usage.PodQuery{Start: start, End: start.Add(30 * time.Second), Step: 30 * time.Second, Cluster: dataQuery().Target.ClusterID, Tenants: []string{tenant}, Metrics: []string{"cpu"}}
			p := provider(t, func(w http.ResponseWriter, r *http.Request) {
				values := map[string]string{"expected": "0", "inventory": "1", "conflicts": "0"}
				if ownership {
					values["ownership"] = "1"
				}
				rows := []any{}
				for stat, value := range values {
					rows = append(rows, map[string]any{"metric": map[string]string{"usage_tenant": tenant, "usage_metric": stat}, "values": []any{[]any{start.Unix(), value}}})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": rows}})
			})
			result, err := p.QueryUsagePods(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			point := result[tenant]["cpu"][0]
			if ownership && (point.Value == nil || *point.Value != 0 || point.Observed == nil) {
				t.Fatalf("confirmed empty scope not zero: %+v", point)
			}
			if !ownership && point.Value != nil {
				t.Fatalf("missing tenant labels fabricated zero: %+v", point)
			}
		})
	}
}

func TestUsageRejectsSharedDatasourceWithoutClusterIdentity(t *testing.T) {
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("ambiguous shared datasource must not be queried")
	}))
	defer endpoint.Close()
	p, err := New([]Datasource{{ClusterID: "11111111-1111-4111-8111-111111111111", Address: endpoint.URL}, {ClusterID: "22222222-2222-4222-8222-222222222222", Address: endpoint.URL + "/"}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	_, err = p.QueryUsagePods(context.Background(), usage.PodQuery{Cluster: "11111111-1111-4111-8111-111111111111"})
	if err == nil {
		t.Fatal("shared cluster datasource was silently double counted")
	}
}
