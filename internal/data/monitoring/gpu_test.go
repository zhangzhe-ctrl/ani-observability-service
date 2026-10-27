package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gpu "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
)

func TestGpuPrometheusPreservesOriginalTimeAndPairs(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if e := r.ParseForm(); e != nil {
			t.Error(e)
		}
		expr := r.Form.Get("query")
		if !strings.Contains(expr, `ani_gpu_source="nvml"`) || !strings.Contains(expr, `ani_node_id="node1"`) {
			t.Errorf("missing source controls: %s", expr)
		}
		v := "17"
		if strings.HasPrefix(expr, "timestamp(") {
			v = fmt.Sprint(now.Unix() - 30)
		}
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"nvidia_smi_memory_used_bytes","ani_cluster_id":"f39bf6e2-f3ed-4191-ada3-caeebf86c220","ani_node_id":"node1","ani_node_uid":"node-uid","ani_gpu_source":"nvml","uuid":"abc","job":"gpu"},"values":[[%d,"%s"]]}]}}`, now.Unix(), v)
	}))
	defer srv.Close()
	p, e := New([]Datasource{{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", Address: srv.URL}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	q := gpu.Query{Target: gpu.Target{Kind: "PHYSICAL", IndexComplete: true, Scopes: []gpu.Scope{{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", NodeID: "node1", NodeUID: "node-uid", GPUUUID: "GPU-ABC"}}}, Start: now, End: now.Add(15 * time.Second)}
	m, _ := gpu.Lookup("gpu.physical.memory_used_bytes")
	s, e := p.QueryGPU(context.Background(), q, m)
	if e != nil {
		t.Fatal(e)
	}
	if calls != 2 || len(s) != 1 || s[0].Samples[0].Value != 17 || !s[0].Samples[0].SampleTime.Equal(now.Add(-30*time.Second)) {
		t.Fatal(calls, s)
	}
}
func TestGpuPrometheusHalfOpenChunkBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = r.ParseForm()
		if r.Form.Get("end") == fmt.Sprint(now.Add(6*time.Hour).Unix()) {
			t.Error("chunk includes adjacent boundary")
		}
		fmt.Fprint(w, `{"status":"success","data":{"resultType":"matrix","result":[]}}`)
	}))
	defer srv.Close()
	p, e := New([]Datasource{{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", Address: srv.URL}})
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	q := gpu.Query{Target: gpu.Target{Kind: "PHYSICAL", Scopes: []gpu.Scope{{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", NodeID: "node1", NodeUID: "node-uid", GPUUUID: "abc"}}}, Start: now, End: now.Add(6*time.Hour + 15*time.Second)}
	m, _ := gpu.Lookup("gpu.physical.memory_used_bytes")
	_, e = p.QueryGPU(context.Background(), q, m)
	if e != nil || calls != 4 {
		t.Fatal(calls, e)
	}
}
