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
		if strings.Contains(expr, "timestamp(") {
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
	q := gpu.Query{Target: gpu.Target{Kind: "PHYSICAL", Scopes: []gpu.Scope{{ClusterID: "f39bf6e2-f3ed-4191-ada3-caeebf86c220", NodeID: "node1", NodeUID: "node-uid", GPUUUID: "GPU-ABC"}}}, Start: now, End: now.Add(15 * time.Second)}
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
		if strings.Contains(expr, "timestamp(") {
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

func TestBusinessGPUUsesRecordedIdentityAndActualRecordingTime(t *testing.T) {
	at := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	labels := businessGPULabels()
	ref := gpu.UsageRef{TenantID: labels["ani_tenant_id"], OwnerService: labels["owner_service"], ResourceID: labels["resource_id"], CreateOperationID: labels["create_operation_id"]}
	target := gpu.Target{Kind: "RESOURCE", TenantID: ref.TenantID, ResourceKind: "inference", ResourceID: ref.ResourceID, UsageRef: &ref}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		expr := r.Form.Get("query")
		for _, fragment := range []string{"ani_gpu_resource_memory_used_bytes{", `ani_gpu_source="hami"`, `ani_tenant_id="` + ref.TenantID + `"`, `owner_service="` + ref.OwnerService + `"`, `resource_id="` + ref.ResourceID + `"`, `create_operation_id="` + ref.CreateOperationID + `"`} {
			if !strings.Contains(expr, fragment) {
				t.Errorf("missing direct identity selector %q in %s", fragment, expr)
			}
		}
		if strings.Contains(expr, "vGPU_device_") {
			t.Error("business query must not fall back to unlabeled raw metrics")
		}
		value := "0"
		metric := make(map[string]string, len(labels)+1)
		for k, v := range labels {
			metric[k] = v
		}
		if strings.HasPrefix(expr, "timestamp(") {
			value = fmt.Sprint(at.Unix() - 20)
		} else {
			metric["__name__"] = "ani_gpu_resource_memory_used_bytes"
		}
		encoded, _ := json.Marshal(metric)
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":%s,"values":[[%d,"%s"]]}]}}`, encoded, at.Unix(), value)
	}))
	defer srv.Close()
	p, err := New([]Datasource{{ClusterID: labels["ani_cluster_id"], Address: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	m, _ := gpu.Lookup("gpu.resource.memory_used_bytes")
	q := gpu.Query{Target: target, Start: at, End: at.Add(30 * time.Second), Budget: &gpu.Budget{}}
	result, err := p.QueryGPU(context.Background(), q, m)
	if err != nil || calls != 2 || len(result) != 1 || len(result[0].Samples) != 1 {
		t.Fatal(calls, result, err)
	}
	if result[0].Samples[0].Value != 0 || !result[0].Samples[0].SampleTime.Equal(at.Add(-20*time.Second)) || result[0].Scope.PodUID != labels["pod_uid"] || result[0].Scope.Usage != ref {
		t.Fatal("record identity, observed zero or actual sample time lost", result)
	}
}

func TestBusinessGPURejectsUntrustedReturnedLabels(t *testing.T) {
	labels := businessGPULabels()
	ref := gpu.UsageRef{TenantID: labels["ani_tenant_id"], OwnerService: labels["owner_service"], ResourceID: labels["resource_id"], CreateOperationID: labels["create_operation_id"]}
	target := gpu.Target{Kind: "RESOURCE", TenantID: ref.TenantID, ResourceKind: "inference", ResourceID: ref.ResourceID, UsageRef: &ref}
	for _, field := range []string{"ani_cluster_id", "ani_tenant_id", "owner_service", "resource_id", "create_operation_id", "resource_kind", "pod_uid", "ani_node_uid"} {
		t.Run(field, func(t *testing.T) {
			changed := businessGPULabels()
			changed[field] = ""
			if _, _, err := gpuBusinessIdentity(changed, labels["ani_cluster_id"], target); err == nil {
				t.Fatal("missing or mismatched source identity accepted")
			}
		})
	}
	labels["deviceuuid"] = "GPU-" + strings.ToUpper(labels["deviceuuid"])
	if _, _, err := gpuBusinessIdentity(labels, labels["ani_cluster_id"], target); err == nil {
		t.Fatal("noncanonical UUID must be fixed in the controlled scrape configuration")
	}
}

func businessGPULabels() map[string]string {
	return map[string]string{
		"ani_cluster_id": "f39bf6e2-f3ed-4191-ada3-caeebf86c220",
		"ani_node_id":    "node1", "ani_node_uid": "node-uid", "ani_gpu_source": "hami",
		"deviceuuid": "01234567-89ab-cdef-0123-456789abcdef", "vdeviceid": "0",
		"podnamespace": "ns", "podname": "pod-a", "ctrname": "main",
		"pod_uid":       "30000000-0000-0000-0000-000000000001",
		"ani_tenant_id": "7ef3ded1-a63f-4ad8-8fe1-cf987a562534", "owner_service": "ani-inference",
		"resource_id": "10000000-0000-0000-0000-000000000001", "create_operation_id": "20000000-0000-0000-0000-000000000001",
		"resource_kind": "inference",
	}
}
