package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/monitoring"
	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/target"
)

func dataQuery() biz.Query {
	return biz.Query{Target: target.Scope{Kind: 1, TenantID: "11111111-1111-4111-8111-111111111111", InstanceID: "22222222-2222-4222-8222-222222222222", ClusterID: "33333333-3333-4333-8333-333333333333", Namespace: "workspace-a", ContainerName: "workspace"}, Codes: []string{biz.CPU}, Start: time.Unix(1700000000, 0).UTC(), End: time.Unix(1700000060, 0).UTC(), Step: 30 * time.Second}
}
func labels() map[string]string {
	q := dataQuery()
	return map[string]string{"__name__": "ani_instance_cpu_usage_cores", "ani_cluster_id": q.Target.ClusterID, "ani_tenant_id": q.Target.TenantID, "ani_instance_id": q.Target.InstanceID, "namespace": q.Target.Namespace, "container": "workspace", "pod_uid": "44444444-4444-4444-8444-444444444444", "runtime_container_id": strings.Repeat("a", 64)}
}
func matrix() map[string]any {
	return map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": []any{map[string]any{"metric": labels(), "values": []any{[]any{1700000000, "0"}, []any{1700000060, "2.5"}}}}}}
}
func resultItems(body map[string]any) []any { return body["data"].(map[string]any)["result"].([]any) }
func requireReason(t *testing.T, err error, want string) {
	t.Helper()
	var failure *biz.Failure
	if !errors.As(err, &failure) || failure.Reason != want {
		t.Fatalf("error=%v, want %s", err, want)
	}
}
func provider(t *testing.T, h http.HandlerFunc) *Prometheus {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	p, err := New([]Datasource{{ClusterID: dataQuery().Target.ClusterID, Address: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestFixedHTTPQueryUsesTrustedScopeAndFreshness(t *testing.T) {
	q := dataQuery()
	m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
	p := provider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/query_range" {
			t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		expected, _ := Expression(q.Target, m)
		if r.Form.Get("query") != expected || r.Form.Get("start") != "1700000000" || r.Form.Get("end") != "1700000060" || r.Form.Get("step") != "30" || r.Form.Get("timeout") != "4s" || r.Form.Get("limit") != "129" {
			t.Errorf("unexpected fixed parameters: %v", r.Form)
		}
		for _, key := range []string{"ani_cluster_id=", "ani_tenant_id=", "ani_instance_id=", "namespace=", "container=", "time() - timestamp(", "<= 45"} {
			if !strings.Contains(expected, key) {
				t.Errorf("missing trusted matcher/freshness %s", key)
			}
		}
		_ = json.NewEncoder(w).Encode(matrix())
	})
	sources, err := p.Query(context.Background(), q, m)
	if err != nil || len(sources) != 1 || len(sources[0].Samples) != 2 || sources[0].Samples[0].Value != 0 || sources[0].Samples[1].Value != 2.5 {
		t.Fatalf("decoded samples: %+v %v", sources, err)
	}
	series, err := biz.Normalize(q, m, sources)
	if err != nil || series.Points[1].Value != nil || series.Points[1].GapReason != "NO_DATA" {
		t.Fatalf("unexpected missing grid: %+v %v", series, err)
	}
}

func TestLatestAndInvalidValuesReachBusinessNormalization(t *testing.T) {
	q := dataQuery()
	q.Latest = true
	q.End = q.Start
	m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
	p := provider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Error("not instant endpoint")
		}
		_ = r.ParseForm()
		if r.Form.Get("time") != "1700000000" || r.Form.Get("step") != "" {
			t.Error("instant parameters")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "vector", "result": []any{map[string]any{"metric": labels(), "value": []any{1700000000, "NaN"}}}}})
	})
	sources, err := p.Query(context.Background(), q, m)
	if err != nil {
		t.Fatal(err)
	}
	series, err := biz.Normalize(q, m, sources)
	if err != nil || series.Points[0].GapReason != "INVALID_VALUE" {
		t.Fatalf("NaN not normalized: %+v %v", series, err)
	}
}

func TestMalformedOrMismatchedResponsesFail(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"wrong result type": func(b map[string]any) { b["data"].(map[string]any)["resultType"] = "vector" },
		"missing result":    func(b map[string]any) { delete(b["data"].(map[string]any), "result") },
		"warnings":          func(b map[string]any) { b["warnings"] = []string{"partial data"} },
		"cross tenant": func(b map[string]any) {
			resultItems(b)[0].(map[string]any)["metric"].(map[string]string)["ani_tenant_id"] = "55555555-5555-4555-8555-555555555555"
		},
		"missing uid": func(b map[string]any) {
			delete(resultItems(b)[0].(map[string]any)["metric"].(map[string]string), "pod_uid")
		},
		"extra label": func(b map[string]any) {
			resultItems(b)[0].(map[string]any)["metric"].(map[string]string)["job"] = "transport-leak"
		},
		"short cid": func(b map[string]any) {
			resultItems(b)[0].(map[string]any)["metric"].(map[string]string)["runtime_container_id"] = "abc"
		},
		"duplicate series": func(b map[string]any) {
			items := resultItems(b)
			b["data"].(map[string]any)["result"] = append(items, items[0])
		},
		"duplicate point": func(b map[string]any) {
			resultItems(b)[0].(map[string]any)["values"] = []any{[]any{1700000000, "1"}, []any{1700000000, "2"}}
		},
		"off grid":             func(b map[string]any) { resultItems(b)[0].(map[string]any)["values"] = []any{[]any{1700000001, "1"}} },
		"fractional timestamp": func(b map[string]any) { resultItems(b)[0].(map[string]any)["values"] = []any{[]any{1700000000.5, "1"}} },
		"outside window":       func(b map[string]any) { resultItems(b)[0].(map[string]any)["values"] = []any{[]any{1700000090, "1"}} },
		"bad value": func(b map[string]any) {
			resultItems(b)[0].(map[string]any)["values"] = []any{[]any{1700000000, "not-a-number"}}
		},
		"numeric value": func(b map[string]any) { resultItems(b)[0].(map[string]any)["values"] = []any{[]any{1700000000, 1}} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			p := provider(t, func(w http.ResponseWriter, r *http.Request) {
				body := matrix()
				mutate(body)
				_ = json.NewEncoder(w).Encode(body)
			})
			m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
			_, err := p.Query(context.Background(), dataQuery(), m)
			requireReason(t, err, biz.SourceResponseInvalid)
		})
	}
}

func TestTrainingExpressionAndLabelsPinPodUID(t *testing.T) {
	s := target.Scope{Kind: target.TrainingExecution, TenantID: "11111111-1111-4111-8111-111111111111",
		InstanceID: "22222222-2222-4222-8222-222222222222", ClusterID: "33333333-3333-4333-8333-333333333333",
		Namespace: "ani-modeldev-tenant-a-v21", ContainerName: "node", PodUID: "44444444-4444-4444-8444-444444444444"}
	m, ok := biz.Lookup(biz.CPU, target.TrainingExecution)
	if !ok {
		t.Fatal("training cpu not registered")
	}
	expr, err := Expression(s, m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(expr, "pod_uid=") != 2 {
		t.Fatalf("training expression must pin pod_uid in body and guard: %s", expr)
	}
	if !strings.Contains(expr, "ani_training_cpu_usage_cores{") || !strings.Contains(expr, "<= 45") {
		t.Fatalf("training expression shape wrong: %s", expr)
	}
	instanceMetric, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
	instanceExpr, err := Expression(dataQuery().Target, instanceMetric)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(instanceExpr, "pod_uid") {
		t.Fatalf("instance expression must not match pod_uid: %s", instanceExpr)
	}

	labels := map[string]string{"__name__": m.Record, "ani_cluster_id": s.ClusterID,
		"ani_tenant_id": s.TenantID, "ani_instance_id": s.InstanceID, "namespace": s.Namespace,
		"container": s.ContainerName, "pod_uid": s.PodUID, "runtime_container_id": strings.Repeat("b", 64)}
	if !validLabels(labels, s, m) {
		t.Fatal("exact training labels rejected")
	}
	labels["pod_uid"] = "55555555-5555-4555-8555-555555555555"
	if validLabels(labels, s, m) {
		t.Fatal("foreign pod uid accepted")
	}
}

func TestSeriesAndResponseByteBoundaries(t *testing.T) {
	for _, count := range []int{128, 129} {
		t.Run(fmt.Sprintf("series-%d", count), func(t *testing.T) {
			p := provider(t, func(w http.ResponseWriter, r *http.Request) {
				b := matrix()
				items := make([]any, count)
				for i := range items {
					l := labels()
					l["runtime_container_id"] = fmt.Sprintf("%064x", i+1)
					items[i] = map[string]any{"metric": l, "values": []any{[]any{1700000000, "1"}}}
				}
				b["data"].(map[string]any)["result"] = items
				_ = json.NewEncoder(w).Encode(b)
			})
			m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
			series, err := p.Query(context.Background(), dataQuery(), m)
			if count == 128 {
				if err != nil || len(series) != 128 {
					t.Fatalf("boundary rejected: %v", err)
				}
			} else {
				requireReason(t, err, biz.ResultTooLarge)
			}
		})
	}
	for _, size := range []int{maxBody, maxBody + 1} {
		t.Run(fmt.Sprintf("bytes-%d", size), func(t *testing.T) {
			p := provider(t, func(w http.ResponseWriter, r *http.Request) {
				body, _ := json.Marshal(matrix())
				_, _ = w.Write(body)
				_, _ = w.Write([]byte(strings.Repeat(" ", size-len(body))))
			})
			m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
			_, err := p.Query(context.Background(), dataQuery(), m)
			if size == maxBody {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				requireReason(t, err, biz.ResultTooLarge)
			}
		})
	}
}

func TestUpstreamFailuresDoNotBecomeNoData(t *testing.T) {
	for _, item := range []struct {
		name         string
		status       int
		body, reason string
	}{{"server", 503, `{"status":"error","errorType":"internal"}`, biz.SourceUnavailable}, {"timeout", 503, `{"status":"error","errorType":"timeout"}`, biz.QueryTimeout}, {"execution", 422, `{"status":"error","errorType":"execution"}`, biz.QueryFailed}, {"malformed", 200, `{"status":`, biz.SourceResponseInvalid}, {"trailing", 200, `{"status":"success","data":{"resultType":"matrix","result":[]}} {}`, biz.SourceResponseInvalid}} {
		t.Run(item.name, func(t *testing.T) {
			p := provider(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(item.status)
				_, _ = w.Write([]byte(item.body))
			})
			m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
			_, err := p.Query(context.Background(), dataQuery(), m)
			requireReason(t, err, item.reason)
		})
	}
	p := provider(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"resultType": "matrix", "result": []any{}}})
	})
	m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
	series, err := p.Query(context.Background(), dataQuery(), m)
	if err != nil || len(series) != 0 {
		t.Fatalf("legitimate empty result: %v", err)
	}
}

func TestDatasourceConfigurationAndCancellation(t *testing.T) {
	c := dataQuery().Target.ClusterID
	for _, config := range [][]Datasource{{{ClusterID: c, Address: "http://one"}, {ClusterID: c, Address: "http://two"}}, {{ClusterID: "bad", Address: "http://one"}}, {{ClusterID: c, Address: "file:///tmp/prom"}}, {{ClusterID: c, Address: "http://user:password@one"}}, {{ClusterID: c, Address: "http://one", BearerTokenFile: "/no-such-credential"}}} {
		if p, err := New(config); err == nil {
			p.Close()
			t.Fatal("invalid datasource configuration accepted")
		}
	}
	p, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := biz.Lookup(biz.CPU, target.ContainerInstance)
	_, err = p.Query(context.Background(), dataQuery(), m)
	requireReason(t, err, biz.SourceNotConfigured)
	canceled := make(chan struct{})
	p = provider(t, func(w http.ResponseWriter, r *http.Request) { _ = r.ParseForm(); <-r.Context().Done(); close(canceled) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = p.Query(ctx, dataQuery(), m)
	requireReason(t, err, biz.QueryTimeout)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("HTTP work survived cancellation")
	}
}
