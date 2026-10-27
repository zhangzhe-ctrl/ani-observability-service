package gpu_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	api "github.com/zhangzhe-ctrl/ani-observability-service/api/observability/gpu/v1"
	biz "github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
	data "github.com/zhangzhe-ctrl/ani-observability-service/internal/data/monitoring"
	service "github.com/zhangzhe-ctrl/ani-observability-service/internal/service/gpu"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Deliberately opt-in: this is the bounded phase-5 dense dataset, not a routine
// unit test or a claim that Prometheus currently retains thirty actual days.
func TestDenseThirtyDaySixCardsFourMetrics(t *testing.T) {
	if os.Getenv("ANI_GPU_DENSE_SCALE") != "1" {
		t.Skip("set ANI_GPU_DENSE_SCALE=1 for the bounded dense30d scale gate")
	}
	end := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	start := end.Add(-30 * 24 * time.Hour)
	cluster := "f39bf6e2-f3ed-4191-ada3-caeebf86c220"
	metricCodes := []string{"gpu.physical.utilization_ratio", "gpu.physical.memory_used_bytes", "gpu.physical.memory_total_bytes", "gpu.physical.power_instant_watts"}
	records := []string{"nvidia_smi_utilization_gpu_ratio", "nvidia_smi_memory_used_bytes", "nvidia_smi_memory_total_bytes", "nvidia_smi_power_draw_instant_watts"}
	var calls, values atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		from, e1 := strconv.ParseInt(r.Form.Get("start"), 10, 64)
		to, e2 := strconv.ParseInt(r.Form.Get("end"), 10, 64)
		if e1 != nil || e2 != nil || r.Form.Get("step") != "15" || to-from >= 6*3600 {
			t.Error("query lost fixed base grid or chunk budget")
			w.WriteHeader(400)
			return
		}
		expr := r.Form.Get("query")
		record := ""
		for _, candidate := range records {
			if strings.Contains(expr, candidate+"{") {
				record = candidate
				break
			}
		}
		if record == "" {
			t.Error("unknown query")
			w.WriteHeader(400)
			return
		}
		paired := strings.HasPrefix(expr, "timestamp(")
		calls.Add(1)
		writer := bufio.NewWriterSize(w, 64*1024)
		defer writer.Flush()
		emitted := int64(0)
		defer func() { values.Add(emitted) }()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"status":"success","data":{"resultType":"matrix","result":[`)
		for card := 0; card < 6; card++ {
			if card > 0 {
				io.WriteString(writer, ",")
			}
			labels := map[string]string{"ani_cluster_id": cluster, "ani_node_id": fmt.Sprintf("node%d", card/2+1), "ani_node_uid": fmt.Sprintf("node-uid-%d", card/2+1), "ani_gpu_source": "nvml", "uuid": fmt.Sprintf("00000000-0000-0000-0000-%012d", card), "device_uuid": fmt.Sprintf("GPU-00000000-0000-0000-0000-%012d", card), "job": "gpu-dense-boundary", "instance": fmt.Sprintf("node%d:9835", card/2+1)}
			if !paired {
				labels["__name__"] = record
			}
			metric, _ := json.Marshal(labels)
			fmt.Fprintf(writer, `{"metric":%s,"values":[`, metric)
			first := true
			tuple := make([]byte, 0, 64)
			for at := from; at <= to; at += 15 {
				if !first {
					io.WriteString(writer, ",")
				}
				first = false
				v := float64((card + 1) * 1024)
				switch record {
				case "nvidia_smi_utilization_gpu_ratio":
					v = float64(card+1) / 10
				case "nvidia_smi_memory_total_bytes":
					v = 1 << 30
				case "nvidia_smi_power_draw_instant_watts":
					v = float64(100 + card)
				case "nvidia_smi_memory_used_bytes":
					if at == start.Add(15*24*time.Hour+7*15*time.Second).Unix() && card == 0 {
						v += 1 << 30
					}
				}
				if paired {
					v = float64(at - 1)
				}
				tuple = tuple[:0]
				tuple = append(tuple, '[')
				tuple = strconv.AppendInt(tuple, at, 10)
				tuple = append(tuple, ',', '"')
				tuple = strconv.AppendFloat(tuple, v, 'f', -1, 64)
				tuple = append(tuple, '"', ']')
				_, _ = writer.Write(tuple)
				emitted++
			}
			io.WriteString(writer, `]}`)
		}
		io.WriteString(writer, `]}}`)
	}))
	defer srv.Close()
	provider, err := data.New([]data.Datasource{{ClusterID: cluster, Address: srv.URL}})
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Close()
	svc := service.New(biz.New(provider, func() time.Time { return end }))
	target := &api.GpuTarget{Kind: "PHYSICAL", IndexComplete: true}
	for card := 0; card < 6; card++ {
		target.Scopes = append(target.Scopes, &api.GpuSourceScope{ClusterId: cluster, NodeId: fmt.Sprintf("node%d", card/2+1), NodeUid: fmt.Sprintf("node-uid-%d", card/2+1), GpuUuid: fmt.Sprintf("GPU-00000000-0000-0000-0000-%012d", card)})
	}
	began := time.Now()
	response, err := svc.QueryGpuRange(context.Background(), &api.QueryGpuRangeRequest{Target: target, MetricCodes: metricCodes, Start: timestamppb.New(start), End: timestamppb.New(end)})
	if err != nil {
		t.Fatalf("dense real usecase failed after %s: %v; prom_calls=%d sample_values=%d", time.Since(began), err, calls.Load(), values.Load())
	}
	result := response.GetResult()
	if len(result.Series) != 4 || result.EffectiveStepSeconds != 900 {
		t.Fatalf("invalid series/grid: %d %d", len(result.Series), result.EffectiveStepSeconds)
	}
	for _, s := range result.Series {
		if len(s.Points) != 2880 || s.BaseResolutionSeconds != 15 || s.Quality.Status != "OK" || s.Quality.CompletePoints != 172800 || s.Quality.ExpectedPoints != 172800 {
			t.Fatalf("incomplete dense curve: %s points=%d quality=%v", s.MetricCode, len(s.Points), s.Quality)
		}
		if s.MetricCode == "gpu.physical.memory_used_bytes" && (s.Peak == nil || *s.Peak != float64((1<<30)+21*1024)) {
			t.Fatalf("short peak lost: %v", s.Peak)
		}
	}
	want := int64(30 * 24 * 3600 / 15 * 6 * 4 * 2)
	if values.Load() != want || calls.Load() != 960 {
		t.Fatalf("companion/chunk counts wrong: calls=%d samples=%d want=%d", calls.Load(), values.Load(), want)
	}
	size := proto.Size(response)
	if size > 8*1024*1024 {
		t.Fatalf("response size exceeds 8MiB: %d", size)
	}
	previousCalls := calls.Load()
	// Nine independent sources put 30d x4 physical metrics over the 12M
	// paired-sample upper bound. Admission must fail before any Prom call.
	for card := 6; card < 9; card++ {
		target.Scopes = append(target.Scopes, &api.GpuSourceScope{ClusterId: cluster, NodeId: "extra-node", NodeUid: "extra-node-uid", GpuUuid: fmt.Sprintf("extra-%d", card)})
	}
	_, err = svc.QueryGpuRange(context.Background(), &api.QueryGpuRangeRequest{Target: target, MetricCodes: metricCodes, Start: timestamppb.New(start), End: timestamppb.New(end)})
	if status.Code(err) != codes.ResourceExhausted || !strings.Contains(err.Error(), "GPU_BUDGET_EXCEEDED") || calls.Load() != previousCalls {
		t.Fatalf("over-budget admission not explicit or queried Prom: %v", err)
	}
	t.Logf("DENSE30D PASS: cards=6 metrics=4 base=15s chunks=120/metric prom_calls=%d processed_values=%d response_bytes=%d elapsed=%s; nine-card paired budget=explicit_resource_exhausted", calls.Load(), values.Load(), size, time.Since(began))
}
