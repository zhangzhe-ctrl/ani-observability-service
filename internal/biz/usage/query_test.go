package usage

import (
	"context"
	"testing"
	"time"

	"github.com/zhangzhe-ctrl/ani-observability-service/internal/biz/gpu"
)

const cluster = "11111111-1111-4111-8111-111111111111"
const resourceA = "22222222-2222-4222-8222-222222222222"
const resourceB = "33333333-3333-4333-8333-333333333333"

type gpuPeer func(context.Context, gpu.Query) (*gpu.Result, error)

func (f gpuPeer) Execute(ctx context.Context, q gpu.Query) (*gpu.Result, error) { return f(ctx, q) }

type podsPeer func(context.Context, PodQuery) (PodResult, error)

func (f podsPeer) QueryUsagePods(ctx context.Context, q PodQuery) (PodResult, error) {
	return f(ctx, q)
}
func sampleQuery() Query {
	start := time.Date(2026, 10, 28, 1, 0, 0, 0, time.UTC)
	return Query{Start: start, End: start.Add(time.Minute), Clusters: []string{cluster}, Metrics: []string{"gpu"}, Tenants: []Tenant{{ID: 1, ResourceID: resourceA, GPU: gpu.Target{Kind: "TENANT", TenantID: resourceA, IndexComplete: true}}, {ID: 2, ResourceID: resourceB, GPU: gpu.Target{Kind: "TENANT", TenantID: resourceB, IndexComplete: true}}}}
}

func TestPlatformPeakUsesSimultaneousGridNotIndependentPeaks(t *testing.T) {
	q := sampleQuery()
	reader := gpuPeer(func(_ context.Context, in gpu.Query) (*gpu.Result, error) {
		values := []float64{9, 1}
		if in.Target.TenantID == resourceB {
			values = []float64{1, 9}
		}
		rows := []gpu.Point{}
		for i, v := range values {
			at := in.Start.Add(time.Duration(i) * 30 * time.Second)
			rows = append(rows, gpu.Point{Timestamp: at, End: at.Add(30 * time.Second), Value: pointer(v * (1 << 30)), KnownValue: pointer(v * (1 << 30)), Quality: gpu.Quality{Status: "OK"}})
		}
		return &gpu.Result{Step: 30 * time.Second, Series: []gpu.Series{{Points: rows}}}, nil
	})
	result, err := New(reader, nil, func() time.Time { return q.End }).Execute(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	m := result.Metrics[0]
	if m.Mean == nil || *m.Mean != 10 || m.Peak == nil || *m.Peak != 10 || m.ActiveTenants == nil || *m.ActiveTenants != 2 {
		t.Fatalf("simultaneous aggregate lost: %+v", m)
	}
	if len(result.Tenants[0].Metrics[0].Points) != 0 || *result.Tenants[0].Metrics[0].Peak != 9 {
		t.Fatal("tenant summary/curve contract broken")
	}
}

func TestIncompleteTenantKeepsKnownPartAndNullTotal(t *testing.T) {
	q := sampleQuery()
	q.Tenants[1].GPUUnavailable = "GPU_SOURCE_UNAVAILABLE"
	reader := gpuPeer(func(_ context.Context, in gpu.Query) (*gpu.Result, error) {
		rows := []gpu.Point{}
		for at := in.Start; at.Before(in.End); at = at.Add(30 * time.Second) {
			rows = append(rows, gpu.Point{Timestamp: at, End: at.Add(30 * time.Second), Value: pointer(4 * (1 << 30)), KnownValue: pointer(4 * (1 << 30)), Quality: gpu.Quality{Status: "OK"}})
		}
		return &gpu.Result{Step: 30 * time.Second, Series: []gpu.Series{{Points: rows}}}, nil
	})
	r, e := New(reader, nil, func() time.Time { return q.End }).Execute(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	m := r.Metrics[0]
	if m.Mean != nil || m.Peak != nil || m.KnownMean == nil || *m.KnownMean != 4 || m.ActiveTenants != nil || m.Quality.Status != "PARTIAL" {
		t.Fatalf("partial was fabricated as complete: %+v", m)
	}
}

func TestCPURequiresInventoryAndContainerCoverageOnEveryCluster(t *testing.T) {
	q := sampleQuery()
	q.Metrics = []string{"cpu"}
	q.Tenants = q.Tenants[:1]
	reader := podsPeer(func(_ context.Context, in PodQuery) (PodResult, error) {
		return PodResult{resourceA: {"cpu": {{Timestamp: q.Start, Value: pointer(2), Expected: pointer(2), Observed: pointer(2), Inventory: pointer(1), Conflicts: pointer(0), Ownership: pointer(1)}, {Timestamp: q.Start.Add(30 * time.Second), Value: pointer(3), Expected: pointer(2), Observed: pointer(1), Inventory: pointer(1), Conflicts: pointer(0), Ownership: pointer(1)}}}}, nil
	})
	r, e := New(nil, reader, func() time.Time { return q.End }).Execute(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	m := r.Metrics[0]
	if m.Mean != nil || m.KnownMean == nil || *m.KnownMean != 2.5 || m.Quality.Status != "PARTIAL" || m.Points[0].Value == nil || m.Points[1].Value != nil {
		t.Fatalf("container coverage ignored: %+v", m)
	}
}

func TestConfirmedEmptyHistoryAndUnknownHistoryDiffer(t *testing.T) {
	q := sampleQuery()
	q.Tenants = q.Tenants[:1]
	q.Tenants[0].GPU.HistoryAvailableFrom = q.Start
	r, e := New(nil, nil, func() time.Time { return q.End }).Execute(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if r.Metrics[0].Mean == nil || *r.Metrics[0].Mean != 0 || r.Metrics[0].Quality.Status != "OK" {
		t.Fatal("complete empty inventory must be measured zero")
	}
	q.Tenants[0].GPU.IndexComplete = false
	q.Tenants[0].GPUUnavailable = "HISTORY_UNAVAILABLE"
	r, e = New(nil, nil, func() time.Time { return q.End }).Execute(context.Background(), q)
	if e != nil {
		t.Fatal(e)
	}
	if r.Metrics[0].Mean != nil || r.Metrics[0].KnownMean != nil {
		t.Fatal("unknown inventory fabricated zero")
	}
	if r.Metrics[0].Quality.Status != "UNAVAILABLE" {
		t.Fatal("unavailable dependencies must not become platform NO_DATA")
	}
}

func TestUsageRejectsOversizedOrDuplicateTrustedScope(t *testing.T) {
	q := sampleQuery()
	q.Tenants[1].ResourceID = resourceA
	_, e := New(nil, nil, func() time.Time { return q.End }).Execute(context.Background(), q)
	if e == nil {
		t.Fatal("duplicate tenant UUID accepted")
	}
}
