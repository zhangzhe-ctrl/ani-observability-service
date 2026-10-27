package monitoring

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	gpu "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
)

// This checks the production expressions in the real PromQL engine. Returning
// ideal timestamp companions from an HTTP double cannot detect set operators
// replacing the input sample timestamp with the expression evaluation time.
func TestGpuPromQLPreservesSourceTimesBeforeHealthAndUnion(t *testing.T) {
	promtool := os.Getenv("ANI_GPU_PROMTOOL")
	if promtool == "" {
		t.Skip("set ANI_GPU_PROMTOOL to the pinned Prometheus 3.11.3 promtool")
	}
	version, err := exec.Command(promtool, "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(version), "version 3.11.3") || !strings.Contains(string(version), "eb173f5256d4022afba1e9bc3d19740a76859fae") {
		t.Fatalf("unexpected PromQL engine: %s (%v)", version, err)
	}
	scopes := []gpu.Scope{}
	for _, node := range []string{"one", "two", "unhealthy", "missing"} {
		scopes = append(scopes, gpu.Scope{ClusterID: "cluster", NodeID: node, NodeUID: node + "-uid", GPUUUID: node + "-gpu"})
	}
	labels := func(s gpu.Scope, device, event bool) string {
		fields := map[string]string{"ani_cluster_id": s.ClusterID, "ani_node_id": s.NodeID, "ani_node_uid": s.NodeUID, "ani_gpu_source": "nvml", "job": "gpu", "instance": s.NodeID}
		if device {
			fields["uuid"] = s.GPUUUID
		}
		if event {
			fields["xid"] = "79"
		}
		keys := []string{}
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := []string{}
		for _, k := range keys {
			parts = append(parts, k+"="+strconv.Quote(fields[k]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	inputs := []map[string]any{}
	for i, s := range scopes[:3] {
		values := "17 17"
		if i == 0 {
			values = "17 _" // Original time 0s, despite evaluation at 17s.
		}
		inputs = append(inputs, map[string]any{"series": "nvidia_smi_memory_used_bytes" + labels(s, true, false), "values": values})
		health := "1 1"
		if i == 2 {
			health = "0 0"
		}
		inputs = append(inputs,
			map[string]any{"series": "nvidia_smi_last_collect_success" + labels(s, false, false), "values": health},
			map[string]any{"series": "nvidia_smi_last_collect_success_timestamp_seconds" + labels(s, false, false), "values": "0 10"})
		if i < 2 {
			inputs = append(inputs,
				map[string]any{"series": "process_start_time_seconds" + labels(s, false, false), "values": values},
				map[string]any{"series": "nvidia_smi_xid_errors_total" + labels(s, true, true), "values": values})
		}
	}
	want := func(device, event bool) []map[string]any {
		return []map[string]any{{"labels": labels(scopes[0], device, event), "value": 0}, {"labels": labels(scopes[1], device, event), "value": 10}}
	}
	memory, _ := gpu.Lookup("gpu.physical.memory_used_bytes")
	xid, _ := gpu.Lookup("gpu.physical.xid_errors_total")
	fixture := map[string]any{"rule_files": []string{}, "evaluation_interval": "1s", "tests": []any{map[string]any{
		"interval": "10s", "input_series": inputs,
		"promql_expr_test": []any{
			map[string]any{"expr": gpuTimestampExpression(scopes, memory), "eval_time": "17s", "exp_samples": want(true, false)},
			map[string]any{"expr": gpuProcessTimestampExpression(scopes), "eval_time": "17s", "exp_samples": want(false, false)},
			map[string]any{"expr": gpuTimestampExpression(scopes, xid), "eval_time": "17s", "exp_samples": want(true, true)},
		},
	}}}
	data, err := json.Marshal(fixture) // JSON is also valid YAML for promtool.
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source-times.yaml")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(promtool, "test", "rules", path).CombinedOutput(); err != nil {
		t.Fatalf("production PromQL did not preserve original times: %v\n%s", err, output)
	}
}
