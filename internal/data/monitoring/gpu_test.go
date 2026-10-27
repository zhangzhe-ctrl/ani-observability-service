package monitoring

import (
	"context"
	"encoding/json"
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

func TestGpuXidProcessCompanionUsesControlledExporterIdentity(t *testing.T) {
	at := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	processStart := at.Add(-time.Hour)
	source := map[string]string{"ani_cluster_id": "f39bf6e2-f3ed-4191-ada3-caeebf86c220", "ani_node_id": "node1", "ani_node_uid": "node-uid", "ani_gpu_source": "nvml", "job": "ani-gpu-nvml-v11", "instance": "10.16.5.36:9835", "namespace": "ani-observability", "pod": "ani-gpu-nvml-v11-7cvh8", "endpoint": "gpu-metrics", "service": "ani-gpu-nvml-v11"}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		expr := r.Form.Get("query")
		labels := map[string]string{}
		for k, v := range source {
			labels[k] = v
		}
		value := float64(1)
		if strings.Contains(expr, "process_start_time_seconds") {
			labels["__name__"] = "process_start_time_seconds"
			value = float64(processStart.Unix())
		} else {
			labels["__name__"] = "nvidia_smi_xid_errors_total"
			labels["uuid"] = "abc"
			labels["xid"] = "79"
			labels["device_uuid"] = "GPU-abc"
			// These come from controlled metric relabeling and are intentionally
			// present only on device metrics, never the exporter process family.
			labels["source_unit"] = "count"
		}
		if strings.HasPrefix(expr, "timestamp(") {
			delete(labels, "__name__")
			value = float64(at.Unix())
		}
		metric, _ := json.Marshal(labels)
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":%s,"values":[[%d,"%.0f"]]}]}}`, metric, at.Unix(), value)
	}))
	defer srv.Close()
	p, err := New([]Datasource{{ClusterID: source["ani_cluster_id"], Address: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	q := gpu.Query{Target: gpu.Target{Kind: "PHYSICAL", Scopes: []gpu.Scope{{ClusterID: source["ani_cluster_id"], NodeID: "node1", NodeUID: "node-uid", GPUUUID: "GPU-abc"}}}, Start: at, End: at.Add(15 * time.Second), Budget: &gpu.Budget{}}
	m, _ := gpu.Lookup("gpu.physical.xid_errors_total")
	result, err := p.QueryGPU(context.Background(), q, m)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 4 || len(result) != 1 || result[0].Dimension != "79" || len(result[0].Samples) != 1 || !result[0].Samples[0].ProcessStart.Equal(processStart) {
		t.Fatalf("process companion mismatch: calls=%d result=%+v", calls, result)
	}
}
